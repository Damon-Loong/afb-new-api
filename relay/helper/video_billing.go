package helper

import (
	"encoding/json"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

// PrepareVideoBilling only runs for explicitly configured video pricing. It
// never fetches user-provided media URLs or trusts client-supplied token usage.
func PrepareVideoBilling(c *gin.Context, info *relaycommon.RelayInfo, body []byte) error {
	if billing_setting.GetBillingMode(info.OriginModelName) != billing_setting.BillingModeVideoTokens {
		return nil
	}
	p, ok := billing_setting.GetVideoPricing(info.OriginModelName)
	if !ok {
		return fmt.Errorf("模型缺少视频计费配置")
	}
	if err := p.Validate(); err != nil {
		return err
	}
	var req struct {
		Resolution  string            `json:"resolution"`
		Draft       bool              `json:"draft"`
		ServiceTier string            `json:"service_tier"`
		Tools       []json.RawMessage `json:"tools"`
		Content     []struct {
			Type      string `json:"type"`
			DraftTask *struct {
				ID string `json:"id"`
			} `json:"draft_task"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return err
	}
	if req.ServiceTier != "" && req.ServiceTier != "default" {
		return fmt.Errorf("当前视频计费仅配置标准价，service_tier 必须为 default")
	}
	if len(req.Tools) > 0 {
		return fmt.Errorf("当前视频模板未配置工具附加费用，请勿传入 tools")
	}
	if req.Resolution == "" {
		req.Resolution = "720p"
	}
	if req.Draft {
		req.Resolution = "480p"
	}
	hasVideo := false
	for _, item := range req.Content {
		if item.Type == "video_url" {
			hasVideo = true
		}
		if item.Type == "draft_task" {
			if len(req.Content) != 1 || req.Draft || req.Resolution != "1080p" {
				return fmt.Errorf("Draft 转正式视频须仅传 draft_task，resolution 为 1080p，draft 为 false")
			}
			if item.DraftTask == nil || item.DraftTask.ID == "" {
				return fmt.Errorf("缺少 Draft 任务 ID")
			}
			var draft model.Task
			err := model.DB.Where("user_id = ? AND task_id = ?", info.UserId, item.DraftTask.ID).Find(&draft).Error
			if err != nil {
				return fmt.Errorf("读取 Draft 计费记录失败")
			}
			bc := draft.PrivateData.BillingContext
			if draft.ID == 0 || draft.GetUpstreamTaskID() != item.DraftTask.ID || bc == nil || bc.VideoBilling == nil || !bc.VideoBilling.Draft || draft.Status != model.TaskStatusSuccess || draft.ChannelId != info.ChannelId || bc.OriginModelName != info.OriginModelName {
				return fmt.Errorf("Draft 必须是当前用户、模型及渠道下已成功且具有视频计费快照的任务")
			}
			hasVideo = bc.VideoBilling.HasVideoInput
		}
	}
	if _, err := p.Price(req.Resolution, hasVideo); err != nil {
		return err
	}
	if info.TaskRelayInfo == nil {
		info.TaskRelayInfo = &relaycommon.TaskRelayInfo{}
	}
	info.VideoBilling = &types.VideoBillingSnapshot{
		Pricing: p, Resolution: req.Resolution, HasVideoInput: hasVideo, Draft: req.Draft,
		GroupRatio: HandleGroupRatio(c, info).GroupRatio, QuotaPerUnit: common.QuotaPerUnit,
	}
	return nil
}
