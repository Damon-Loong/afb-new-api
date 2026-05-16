package system_setting

import "github.com/QuantumNous/new-api/setting/config"

// StorageSetting 文件存储（本地 / 阿里云 OSS）
type StorageSetting struct {
	// OSS 总开关，关闭时仅使用本地存储
	Enabled bool `json:"enabled"`
	// OSS Endpoint，如 oss-cn-hangzhou.aliyuncs.com
	Endpoint string `json:"endpoint"`
	Bucket   string `json:"bucket"`
	// AccessKey ID / Secret
	AccessKeyId     string `json:"access_key_id"`
	AccessKeySecret string `json:"access_key_secret"`
	// 对象键前缀，如 files/
	Prefix string `json:"prefix"`
	// 对外访问基础 URL（可选，CDN/自定义域名），如 https://cdn.example.com
	PublicBaseURL string `json:"public_base_url"`
	// 上传后设为公共读（便于视频/图像生成上游拉取）
	PublicRead bool `json:"public_read"`
}

var defaultStorageSetting = StorageSetting{
	Enabled: false,
	Prefix:  "files/",
}

func init() {
	config.GlobalConfig.Register("storage_setting", &defaultStorageSetting)
}

func GetStorageSetting() *StorageSetting {
	return &defaultStorageSetting
}

func OSSConfigured() bool {
	s := GetStorageSetting()
	if !s.Enabled {
		return false
	}
	return s.Endpoint != "" && s.Bucket != "" && s.AccessKeyId != "" && s.AccessKeySecret != ""
}
