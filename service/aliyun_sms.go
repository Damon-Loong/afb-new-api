package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	dysmsapi "github.com/alibabacloud-go/dysmsapi-20170525/v3/client"
	"github.com/alibabacloud-go/tea/tea"
)

type AliyunSMSConfig struct {
	AccessKeyId     string
	AccessKeySecret string
	RegionId        string
	SignName        string
	TemplateCode    string
}

func LoadAliyunSMSConfig() (AliyunSMSConfig, bool) {
	setting.InitAliyunSMSFromEnvIfEmpty()
	cfg := AliyunSMSConfig{
		AccessKeyId:     strings.TrimSpace(setting.AliyunSMSAccessKeyId),
		AccessKeySecret: strings.TrimSpace(setting.AliyunSMSAccessKeySecret),
		RegionId:        strings.TrimSpace(setting.AliyunSMSRegionId),
		SignName:        strings.TrimSpace(setting.AliyunSMSSignName),
		TemplateCode:    strings.TrimSpace(setting.AliyunSMSTemplateCode),
	}
	if cfg.RegionId == "" {
		cfg.RegionId = "cn-hangzhou"
	}
	ok := cfg.AccessKeyId != "" && cfg.AccessKeySecret != "" && cfg.SignName != "" && cfg.TemplateCode != ""
	return cfg, ok
}

func newAliyunSMSClient(cfg AliyunSMSConfig) (*dysmsapi.Client, error) {
	o := &openapi.Config{
		AccessKeyId:     tea.String(cfg.AccessKeyId),
		AccessKeySecret: tea.String(cfg.AccessKeySecret),
		RegionId:        tea.String(cfg.RegionId),
		Endpoint:        tea.String("dysmsapi.aliyuncs.com"),
	}
	return dysmsapi.NewClient(o)
}

func SendAliyunLoginSMS(ctx context.Context, phone11, code string) error {
	cfg, ok := LoadAliyunSMSConfig()
	if !ok {
		missing := make([]string, 0, 4)
		if cfg.AccessKeyId == "" {
			missing = append(missing, "AccessKeyId")
		}
		if cfg.AccessKeySecret == "" {
			missing = append(missing, "AccessKeySecret")
		}
		if cfg.SignName == "" {
			missing = append(missing, "SignName")
		}
		if cfg.TemplateCode == "" {
			missing = append(missing, "TemplateCode")
		}
		return fmt.Errorf("aliyun sms not configured: missing %s", strings.Join(missing, ","))
	}
	client, err := newAliyunSMSClient(cfg)
	if err != nil {
		return err
	}

	// Most common "通用验证码" templates accept variable "code".
	param := fmt.Sprintf(`{"code":"%s"}`, code)
	req := &dysmsapi.SendSmsRequest{
		PhoneNumbers:  tea.String(phone11),
		SignName:      tea.String(cfg.SignName),
		TemplateCode:  tea.String(cfg.TemplateCode),
		TemplateParam: tea.String(param),
	}
	resp, err := client.SendSms(req)
	if err != nil {
		return err
	}
	if resp == nil || resp.Body == nil || resp.Body.Code == nil {
		return fmt.Errorf("aliyun sms empty response")
	}
	if tea.StringValue(resp.Body.Code) != "OK" {
		// Do not leak provider details to client; keep in server logs.
		msg := ""
		if resp.Body.Message != nil {
			msg = tea.StringValue(resp.Body.Message)
		}
		logger.LogWarn(ctx, fmt.Sprintf("aliyun sms send failed code=%s msg=%q", tea.StringValue(resp.Body.Code), msg))
		return fmt.Errorf("aliyun sms send failed: %s", tea.StringValue(resp.Body.Code))
	}
	return nil
}

