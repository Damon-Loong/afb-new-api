package controller

import (
	"errors"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func checkFileUploadPermission(c *gin.Context) bool {
	role := c.GetInt("role")
	return role >= common.FileUploadPermission
}

func checkFileDownloadPermission(c *gin.Context) bool {
	role := c.GetInt("role")
	return role >= common.FileDownloadPermission
}

func ListFiles(c *gin.Context) {
	if !checkFileDownloadPermission(c) {
		abortOpenAIFileError(c, http.StatusForbidden, "无权访问文件列表")
		return
	}
	userID := c.GetInt("id")
	limit := common.String2Int(c.Query("limit"))
	if limit <= 0 {
		limit = 20
	}
	files, err := model.ListApiFilesByUser(userID, c.Query("after"), limit)
	if err != nil {
		abortOpenAIFileError(c, http.StatusInternalServerError, err.Error())
		return
	}
	data := make([]map[string]interface{}, 0, len(files))
	for i := range files {
		data = append(data, service.ToOpenAIFileObject(&files[i], false))
	}
	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   data,
	})
}

func CreateFile(c *gin.Context) {
	if !checkFileUploadPermission(c) {
		abortOpenAIFileError(c, http.StatusForbidden, "无权上传文件")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.ApiFileUploadMaxSize()+1024)
	fileHeader, err := c.FormFile("file")
	if err != nil {
		abortOpenAIFileError(c, http.StatusBadRequest, "请选择要上传的文件")
		return
	}
	if fileHeader.Size > service.ApiFileUploadMaxSize() {
		abortOpenAIFileError(c, http.StatusBadRequest, "单个文件不能超过 100MB")
		return
	}
	src, err := fileHeader.Open()
	if err != nil {
		abortOpenAIFileError(c, http.StatusBadRequest, err.Error())
		return
	}
	defer src.Close()

	purpose := strings.TrimSpace(c.PostForm("purpose"))
	if purpose == "" {
		purpose = "assistants"
	}

	result, err := service.UploadAPIFile(
		c.GetInt("id"),
		fileHeader.Filename,
		purpose,
		fileHeader.Header.Get("Content-Type"),
		fileHeader.Size,
		src,
	)
	if err != nil {
		abortOpenAIFileError(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, service.ToOpenAIFileObject(result.Record, true))
}

func RetrieveFile(c *gin.Context) {
	if !checkFileDownloadPermission(c) {
		abortOpenAIFileError(c, http.StatusForbidden, "无权访问文件")
		return
	}
	file, err := loadOwnedAPIFile(c)
	if err != nil {
		return
	}
	c.JSON(http.StatusOK, service.ToOpenAIFileObject(file, true))
}

func DeleteFile(c *gin.Context) {
	if !checkFileUploadPermission(c) {
		abortOpenAIFileError(c, http.StatusForbidden, "无权删除文件")
		return
	}
	file, err := loadOwnedAPIFile(c)
	if err != nil {
		return
	}
	if err := service.DeleteAPIFileStorage(file); err != nil {
		abortOpenAIFileError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := model.DeleteApiFile(file.FileID, file.UserID); err != nil {
		abortOpenAIFileError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":      file.FileID,
		"object":  "file",
		"deleted": true,
	})
}

func GetFileContent(c *gin.Context) {
	if !checkFileDownloadPermission(c) {
		abortOpenAIFileError(c, http.StatusForbidden, "无权下载文件")
		return
	}
	file, err := loadOwnedAPIFile(c)
	if err != nil {
		return
	}
	serveAPIFileContent(c, file)
}

// GetFileContentPublic 供 vision / 上游模型直连：凭不可猜测的 file ID 读取，无需 API Token（与 /api/market/uploads 一致）。
func GetFileContentPublic(c *gin.Context) {
	file, err := loadAPIFileByID(c)
	if err != nil {
		return
	}
	serveAPIFileContent(c, file)
}

func serveAPIFileContent(c *gin.Context, file *model.ApiFile) {
	if file.Storage == model.ApiFileStorageOSS && strings.TrimSpace(file.FileURL) != "" {
		c.Redirect(http.StatusFound, file.FileURL)
		return
	}
	reader, contentType, err := service.OpenAPIFileContent(file)
	if err != nil {
		abortOpenAIFileError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer reader.Close()
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Disposition", "inline; filename=\""+file.FileName+"\"")
	c.DataFromReader(http.StatusOK, file.FileSize, contentType, reader, nil)
}

func loadAPIFileByID(c *gin.Context) (*model.ApiFile, error) {
	fileID := strings.TrimSpace(c.Param("id"))
	if fileID == "" {
		abortOpenAIFileError(c, http.StatusBadRequest, "文件 ID 无效")
		return nil, errors.New("invalid id")
	}
	file, err := model.GetApiFileByFileID(fileID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			abortOpenAIFileError(c, http.StatusNotFound, "文件不存在")
			return nil, err
		}
		abortOpenAIFileError(c, http.StatusInternalServerError, err.Error())
		return nil, err
	}
	return file, nil
}

func loadOwnedAPIFile(c *gin.Context) (*model.ApiFile, error) {
	file, err := loadAPIFileByID(c)
	if err != nil {
		return nil, err
	}
	if file.UserID != c.GetInt("id") {
		abortOpenAIFileError(c, http.StatusNotFound, "文件不存在")
		return nil, errors.New("forbidden")
	}
	return file, nil
}

func abortOpenAIFileError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"message": message,
			"type":    "invalid_request_error",
		},
	})
}
