package controller

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func isVolcSeedanceNativeChannel(channelType int) bool {
	return channelType == constant.ChannelTypeDoubaoVideo || channelType == constant.ChannelTypeVolcEngine
}

func volcContentsUpstreamBaseURL(ch *model.Channel) string {
	base := strings.TrimSpace(ch.GetBaseURL())
	if base == "" && ch.Type >= 0 && ch.Type < len(constant.ChannelBaseURLs) {
		base = strings.TrimSpace(constant.ChannelBaseURLs[ch.Type])
	}
	return strings.TrimRight(base, "/")
}

// volcBuildUpstreamURL 拼接上游 path，并把客户端请求的 RawQuery 原样附加（与官方 URL 形态一致）。
func volcBuildUpstreamURL(baseURL, path string, c *gin.Context) string {
	u := strings.TrimRight(baseURL, "/") + path
	if rq := c.Request.URL.RawQuery; rq != "" {
		if strings.Contains(u, "?") {
			u += "&" + rq
		} else {
			u += "?" + rq
		}
	}
	return u
}

var volcPassthroughSkipRequestHeader = map[string]struct{}{
	"authorization": {}, "host": {}, "content-length": {},
	"connection": {}, "proxy-authenticate": {}, "proxy-authorization": {},
	"te": {}, "trailer": {}, "transfer-encoding": {}, "upgrade": {},
	"accept-encoding": {}, "cookie": {},
}

func volcCopyForwardableRequestHeaders(dst *http.Request, src *http.Request) {
	for k, vv := range src.Header {
		if _, skip := volcPassthroughSkipRequestHeader[strings.ToLower(k)]; skip {
			continue
		}
		for _, v := range vv {
			dst.Header.Add(k, v)
		}
	}
}

var volcPassthroughSkipResponseHeader = map[string]struct{}{
	"connection": {}, "keep-alive": {}, "proxy-authenticate": {},
	"proxy-authorization": {}, "te": {}, "trailer": {}, "transfer-encoding": {},
	"upgrade": {},
}

func volcWriteUpstreamResponse(c *gin.Context, resp *http.Response, body []byte) {
	for k, vv := range resp.Header {
		if _, skip := volcPassthroughSkipResponseHeader[strings.ToLower(k)]; skip {
			continue
		}
		for _, v := range vv {
			c.Writer.Header().Add(k, v)
		}
	}
	c.Writer.WriteHeader(resp.StatusCode)
	if len(body) > 0 {
		_, _ = c.Writer.Write(body)
	}
}

// RelayVolcContentsGenerationsTaskCreate 透传火山方舟「创建视频生成任务」官方接口：
// POST /api/v3/contents/generations/tasks
// JSON body、URL query、可转发请求头均原样转发至上游；仅替换 Authorization 为渠道 Key，并做网关侧选路与计费。
func RelayVolcContentsGenerationsTaskCreate(c *gin.Context) {
	channelType := common.GetContextKeyInt(c, constant.ContextKeyChannelType)
	if !isVolcSeedanceNativeChannel(channelType) {
		respondTaskError(c, service.TaskErrorWrapperLocal(
			fmt.Errorf("channel type %d does not support official /api/v3/contents/generations/tasks passthrough; use Doubao video or VolcEngine channel", channelType),
			"unsupported_channel_type",
			http.StatusBadRequest,
		))
		return
	}

	ch, err := model.GetChannelById(common.GetContextKeyInt(c, constant.ContextKeyChannelId), true)
	if err != nil || ch == nil {
		respondTaskError(c, service.TaskErrorWrapper(err, "channel_not_found", http.StatusInternalServerError))
		return
	}

	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		respondTaskError(c, service.TaskErrorWrapper(err, "read_request_body_failed", http.StatusBadRequest))
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(rawBody))

	relayInfo, genErr := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if genErr != nil {
		respondTaskError(c, service.TaskErrorWrapper(genErr, "gen_relay_info_failed", http.StatusInternalServerError))
		return
	}
	relayInfo.RelayFormat = types.RelayFormatTask
	relayInfo.Action = constant.TaskActionGenerate
	if relayInfo.TaskRelayInfo == nil {
		relayInfo.TaskRelayInfo = &relaycommon.TaskRelayInfo{}
	}
	relayInfo.InitChannelMeta(c)
	if relayInfo.ChannelMeta != nil {
		relayInfo.ChannelId = relayInfo.ChannelMeta.ChannelId
	}

	priceData, perr := helper.ModelPriceHelperPerCall(c, relayInfo)
	if perr != nil {
		respondTaskError(c, service.TaskErrorWrapper(perr, "model_price_error", http.StatusBadRequest))
		return
	}
	relayInfo.PriceData = priceData

	defer func() {
		if relayInfo.Billing != nil && relayInfo.Billing.NeedsRefund() {
			relayInfo.Billing.Refund(c)
		}
	}()

	if !relayInfo.PriceData.FreeModel {
		relayInfo.ForcePreConsume = true
		if apiErr := service.PreConsumeBilling(c, relayInfo.PriceData.Quota, relayInfo); apiErr != nil {
			respondTaskError(c, service.TaskErrorFromAPIError(apiErr))
			return
		}
	}

	addUsedChannel(c, relayInfo.ChannelId)

	upURL := volcBuildUpstreamURL(volcContentsUpstreamBaseURL(ch), "/api/v3/contents/generations/tasks", c)
	if !strings.HasPrefix(upURL, "http") {
		respondTaskError(c, service.TaskErrorWrapperLocal(fmt.Errorf("channel base url is empty"), "invalid_channel_base_url", http.StatusBadRequest))
		return
	}
	apiKey := common.GetContextKeyString(c, constant.ContextKeyChannelKey)
	if strings.TrimSpace(apiKey) == "" {
		respondTaskError(c, service.TaskErrorWrapperLocal(fmt.Errorf("empty channel api key"), "channel_key_missing", http.StatusInternalServerError))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 300*time.Second)
	defer cancel()

	upReq, err := http.NewRequestWithContext(ctx, http.MethodPost, upURL, bytes.NewReader(rawBody))
	if err != nil {
		respondTaskError(c, service.TaskErrorWrapper(err, "build_upstream_request_failed", http.StatusInternalServerError))
		return
	}
	volcCopyForwardableRequestHeaders(upReq, c.Request)
	upReq.Header.Set("Authorization", "Bearer "+apiKey)
	if strings.TrimSpace(upReq.Header.Get("Content-Type")) == "" {
		upReq.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(upReq.Header.Get("Accept")) == "" {
		upReq.Header.Set("Accept", "application/json")
	}

	client, err := service.GetHttpClientWithProxy(ch.GetSetting().Proxy)
	if err != nil {
		respondTaskError(c, service.TaskErrorWrapper(err, "http_client_failed", http.StatusInternalServerError))
		return
	}

	resp, err := client.Do(upReq)
	if err != nil {
		respondTaskError(c, service.TaskErrorWrapper(err, "upstream_request_failed", http.StatusBadGateway))
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		respondTaskError(c, service.TaskErrorWrapper(err, "read_upstream_body_failed", http.StatusBadGateway))
		return
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		volcWriteUpstreamResponse(c, resp, respBody)
		return
	}

	var upID struct {
		ID string `json:"id"`
	}
	if err := common.Unmarshal(respBody, &upID); err != nil || strings.TrimSpace(upID.ID) == "" {
		logger.LogError(c, fmt.Sprintf("volc native submit: bad upstream json or missing id: %v", err))
		respondTaskError(c, service.TaskErrorWrapperLocal(fmt.Errorf("upstream returned success but no task id"), "invalid_upstream_response", http.StatusBadGateway))
		return
	}

	relayInfo.TaskRelayInfo.PublicTaskID = upID.ID

	if settleErr := service.SettleBilling(c, relayInfo, relayInfo.PriceData.Quota); settleErr != nil {
		common.SysError("volc native submit settle billing: " + settleErr.Error())
	}
	service.LogTaskConsumption(c, relayInfo)

	platform := constant.TaskPlatform(strconv.Itoa(channelType))
	task := model.InitTask(platform, relayInfo)
	task.PrivateData.UpstreamTaskID = upID.ID
	task.PrivateData.BillingSource = relayInfo.BillingSource
	task.PrivateData.SubscriptionId = relayInfo.SubscriptionId
	task.PrivateData.TokenId = relayInfo.TokenId
	task.PrivateData.BillingContext = &model.TaskBillingContext{
		ModelPrice:      relayInfo.PriceData.ModelPrice,
		GroupRatio:      relayInfo.PriceData.GroupRatioInfo.GroupRatio,
		ModelRatio:      relayInfo.PriceData.ModelRatio,
		OtherRatios:     relayInfo.PriceData.OtherRatios,
		OriginModelName: relayInfo.OriginModelName,
		PerCallBilling:  common.StringsContains(constant.TaskPricePatches, relayInfo.OriginModelName) || relayInfo.PriceData.UsePrice,
	}
	task.Quota = relayInfo.PriceData.Quota
	task.Data = respBody
	task.Action = relayInfo.Action
	task.Status = model.TaskStatusQueued
	if insertErr := task.Insert(); insertErr != nil {
		common.SysError("volc native submit insert task: " + insertErr.Error())
	}

	volcWriteUpstreamResponse(c, resp, respBody)
}

// RelayVolcContentsGenerationsTaskGet 透传火山方舟「查询视频生成任务」官方接口：
// GET /api/v3/contents/generations/tasks/{task_id}
// 与官方一致；query 与可转发请求头原样附加到上游请求。网关根据 path id 匹配本用户任务以还原渠道与密钥。
func RelayVolcContentsGenerationsTaskGet(c *gin.Context) {
	channelType := common.GetContextKeyInt(c, constant.ContextKeyChannelType)
	if !isVolcSeedanceNativeChannel(channelType) {
		respondTaskError(c, service.TaskErrorWrapperLocal(
			fmt.Errorf("channel type %d does not support official /api/v3/contents/generations/tasks passthrough", channelType),
			"unsupported_channel_type",
			http.StatusBadRequest,
		))
		return
	}

	ch, err := model.GetChannelById(common.GetContextKeyInt(c, constant.ContextKeyChannelId), true)
	if err != nil || ch == nil {
		respondTaskError(c, service.TaskErrorWrapper(err, "channel_not_found", http.StatusInternalServerError))
		return
	}

	taskID := strings.TrimSpace(c.Param("task_id"))
	if taskID == "" {
		respondTaskError(c, service.TaskErrorWrapperLocal(fmt.Errorf("task_id is required"), "invalid_request", http.StatusBadRequest))
		return
	}

	path := "/api/v3/contents/generations/tasks/" + url.PathEscape(taskID)
	upURL := volcBuildUpstreamURL(volcContentsUpstreamBaseURL(ch), path, c)
	if !strings.HasPrefix(upURL, "http") {
		respondTaskError(c, service.TaskErrorWrapperLocal(fmt.Errorf("channel base url is empty"), "invalid_channel_base_url", http.StatusBadRequest))
		return
	}
	apiKey := common.GetContextKeyString(c, constant.ContextKeyChannelKey)
	if strings.TrimSpace(apiKey) == "" {
		respondTaskError(c, service.TaskErrorWrapperLocal(fmt.Errorf("empty channel api key"), "channel_key_missing", http.StatusInternalServerError))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	upReq, err := http.NewRequestWithContext(ctx, http.MethodGet, upURL, nil)
	if err != nil {
		respondTaskError(c, service.TaskErrorWrapper(err, "build_upstream_request_failed", http.StatusInternalServerError))
		return
	}
	volcCopyForwardableRequestHeaders(upReq, c.Request)
	upReq.Header.Set("Authorization", "Bearer "+apiKey)
	if strings.TrimSpace(upReq.Header.Get("Accept")) == "" {
		upReq.Header.Set("Accept", "application/json")
	}

	client, err := service.GetHttpClientWithProxy(ch.GetSetting().Proxy)
	if err != nil {
		respondTaskError(c, service.TaskErrorWrapper(err, "http_client_failed", http.StatusInternalServerError))
		return
	}

	resp, err := client.Do(upReq)
	if err != nil {
		respondTaskError(c, service.TaskErrorWrapper(err, "upstream_request_failed", http.StatusBadGateway))
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		respondTaskError(c, service.TaskErrorWrapper(err, "read_upstream_body_failed", http.StatusBadGateway))
		return
	}

	volcWriteUpstreamResponse(c, resp, respBody)
}
