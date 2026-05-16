package service

import (
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/google/uuid"
)

const (
	apiFileUploadDir = "uploads/files"
)

const APIFileUploadMaxSize = 100 * 1024 * 1024

func ApiFileUploadMaxSize() int64 {
	return APIFileUploadMaxSize
}

type UploadedFileResult struct {
	Record   *model.ApiFile
	PublicURL string
}

func SanitizeUploadFileName(name string) string {
	name = strings.TrimSpace(filepath.Base(name))
	if name == "" || name == "." {
		return "upload"
	}
	if len([]rune(name)) > 180 {
		ext := filepath.Ext(name)
		stem := strings.TrimSuffix(name, ext)
		stemRunes := []rune(stem)
		if len(stemRunes) > 160 {
			stem = string(stemRunes[:160])
		}
		name = stem + ext
	}
	return name
}

func BuildLocalFileContentURL(fileID string) string {
	base := strings.TrimRight(strings.TrimSpace(system_setting.ServerAddress), "/")
	if base == "" {
		return "/v1/files/" + fileID + "/content"
	}
	return base + "/v1/files/" + fileID + "/content"
}

func normalizeOSSEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")
	return strings.Trim(endpoint, "/")
}

func buildOSSObjectKey(prefix, storageKey string) string {
	prefix = strings.Trim(prefix, "/")
	if prefix == "" {
		return storageKey
	}
	return prefix + "/" + storageKey
}

func buildOSSPublicURL(objectKey string) string {
	cfg := system_setting.GetStorageSetting()
	if base := strings.TrimRight(strings.TrimSpace(cfg.PublicBaseURL), "/"); base != "" {
		return base + "/" + strings.TrimPrefix(objectKey, "/")
	}
	endpoint := normalizeOSSEndpoint(cfg.Endpoint)
	if endpoint == "" || cfg.Bucket == "" {
		return ""
	}
	return fmt.Sprintf("https://%s.%s/%s", cfg.Bucket, endpoint, strings.TrimPrefix(objectKey, "/"))
}

func newOSSBucket() (*oss.Bucket, error) {
	cfg := system_setting.GetStorageSetting()
	endpoint := normalizeOSSEndpoint(cfg.Endpoint)
	if endpoint == "" || cfg.Bucket == "" || cfg.AccessKeyId == "" || cfg.AccessKeySecret == "" {
		return nil, fmt.Errorf("OSS 配置不完整")
	}
	client, err := oss.New(endpoint, cfg.AccessKeyId, cfg.AccessKeySecret)
	if err != nil {
		return nil, err
	}
	return client.Bucket(cfg.Bucket)
}

func uploadToOSS(storageKey string, reader io.Reader, size int64, contentType string) (string, error) {
	cfg := system_setting.GetStorageSetting()
	bucket, err := newOSSBucket()
	if err != nil {
		return "", err
	}
	objectKey := buildOSSObjectKey(cfg.Prefix, storageKey)
	options := []oss.Option{}
	if contentType != "" {
		options = append(options, oss.ContentType(contentType))
	}
	if cfg.PublicRead {
		options = append(options, oss.ObjectACL(oss.ACLPublicRead))
	}
	if err := bucket.PutObject(objectKey, reader, options...); err != nil {
		return "", err
	}
	return buildOSSPublicURL(objectKey), nil
}

func saveToLocal(storageKey string, reader io.Reader) (string, error) {
	if err := os.MkdirAll(apiFileUploadDir, 0755); err != nil {
		return "", err
	}
	dst := filepath.Join(apiFileUploadDir, storageKey)
	out, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, reader); err != nil {
		return "", err
	}
	return dst, nil
}

func detectContentType(fileName, headerType string) string {
	if headerType != "" && headerType != "application/octet-stream" {
		return headerType
	}
	if ext := filepath.Ext(fileName); ext != "" {
		if t := mime.TypeByExtension(ext); t != "" {
			return t
		}
	}
	return "application/octet-stream"
}

// UploadAPIFile 保存文件：OSS 开启且配置完整时优先 OSS，否则落本地。
func UploadAPIFile(userID int, fileName string, purpose string, contentType string, size int64, reader io.Reader) (*UploadedFileResult, error) {
	if size <= 0 {
		return nil, fmt.Errorf("空文件")
	}
	if size > APIFileUploadMaxSize {
		return nil, fmt.Errorf("单个文件不能超过 100MB")
	}

	originalName := SanitizeUploadFileName(fileName)
	storageKey := uuid.NewString() + filepath.Ext(originalName)
	fileID := "file-" + strings.ReplaceAll(uuid.NewString(), "-", "")
	contentType = detectContentType(originalName, contentType)

	record := &model.ApiFile{
		FileID:     fileID,
		UserID:     userID,
		FileName:   originalName,
		Purpose:    strings.TrimSpace(purpose),
		FileSize:   size,
		MimeType:   contentType,
		Status:     model.ApiFileStatusProcessed,
	}

	var publicURL string
	if system_setting.OSSConfigured() {
		url, err := uploadToOSS(storageKey, reader, size, contentType)
		if err != nil {
			return nil, err
		}
		record.Storage = model.ApiFileStorageOSS
		record.StorageKey = storageKey
		record.FileURL = url
		publicURL = url
	} else {
		localPath, err := saveToLocal(storageKey, reader)
		if err != nil {
			return nil, err
		}
		_ = localPath
		record.Storage = model.ApiFileStorageLocal
		record.StorageKey = storageKey
		publicURL = BuildLocalFileContentURL(fileID)
		record.FileURL = publicURL
	}

	if err := model.CreateApiFile(record); err != nil {
		return nil, err
	}
	return &UploadedFileResult{Record: record, PublicURL: publicURL}, nil
}

func OpenAPIFileContent(file *model.ApiFile) (io.ReadCloser, string, error) {
	switch file.Storage {
	case model.ApiFileStorageOSS:
		bucket, err := newOSSBucket()
		if err != nil {
			return nil, "", err
		}
		cfg := system_setting.GetStorageSetting()
		objectKey := buildOSSObjectKey(cfg.Prefix, file.StorageKey)
		body, err := bucket.GetObject(objectKey)
		if err != nil {
			return nil, "", err
		}
		return body, file.MimeType, nil
	default:
		path := filepath.Join(apiFileUploadDir, file.StorageKey)
		f, err := os.Open(path)
		if err != nil {
			return nil, "", err
		}
		return f, file.MimeType, nil
	}
}

func DeleteAPIFileStorage(file *model.ApiFile) error {
	switch file.Storage {
	case model.ApiFileStorageOSS:
		if !system_setting.OSSConfigured() {
			return nil
		}
		bucket, err := newOSSBucket()
		if err != nil {
			return err
		}
		cfg := system_setting.GetStorageSetting()
		objectKey := buildOSSObjectKey(cfg.Prefix, file.StorageKey)
		return bucket.DeleteObject(objectKey)
	default:
		path := filepath.Join(apiFileUploadDir, file.StorageKey)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
}

func ToOpenAIFileObject(file *model.ApiFile, includeURL bool) map[string]interface{} {
	obj := map[string]interface{}{
		"id":         file.FileID,
		"object":     "file",
		"bytes":      file.FileSize,
		"created_at": file.CreatedAt,
		"filename":   file.FileName,
		"purpose":    file.Purpose,
		"status":     file.Status,
	}
	if includeURL && strings.TrimSpace(file.FileURL) != "" {
		obj["url"] = file.FileURL
	}
	return obj
}

func LogStorageInfo() {
	if system_setting.OSSConfigured() {
		common.SysLog("file storage: aliyun OSS enabled")
	} else {
		common.SysLog("file storage: local (" + apiFileUploadDir + ")")
	}
}
