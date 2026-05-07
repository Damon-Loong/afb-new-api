package setting

import "github.com/QuantumNous/new-api/common"

// Aliyun SMS settings (for phone verification login).
// These are managed via /api/option (admin panel) and can fallback to env vars.
var (
	AliyunSMSAccessKeyId     string
	AliyunSMSAccessKeySecret string
	AliyunSMSRegionId        string = "cn-hangzhou"
	AliyunSMSSignName        string
	AliyunSMSTemplateCode    string
)

func InitAliyunSMSFromEnvIfEmpty() {
	if AliyunSMSAccessKeyId == "" {
		AliyunSMSAccessKeyId = common.GetEnvOrDefaultString("ALIYUN_SMS_ACCESS_KEY_ID", "")
	}
	if AliyunSMSAccessKeySecret == "" {
		AliyunSMSAccessKeySecret = common.GetEnvOrDefaultString("ALIYUN_SMS_ACCESS_KEY_SECRET", "")
	}
	if AliyunSMSRegionId == "" {
		AliyunSMSRegionId = common.GetEnvOrDefaultString("ALIYUN_SMS_REGION_ID", "cn-hangzhou")
	}
	if AliyunSMSSignName == "" {
		AliyunSMSSignName = common.GetEnvOrDefaultString("ALIYUN_SMS_SIGN_NAME", "")
	}
	if AliyunSMSTemplateCode == "" {
		AliyunSMSTemplateCode = common.GetEnvOrDefaultString("ALIYUN_SMS_TEMPLATE_CODE_LOGIN", "")
	}
}

