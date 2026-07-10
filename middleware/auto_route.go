package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type autoRouteEmbeddingTierCacheEntry struct {
	embeddings [][]float64
}

type autoRouteCandidateCacheEntry struct {
	candidates []autoRouteCandidate
	expiresAt  time.Time
}

var autoRouteCandidateCache sync.Map
var autoRouteEmbeddingTierCache sync.Map
var autoRouteEmbeddingTierWarmupInFlight sync.Map

const (
	autoRouteCandidateCacheTTL = 2 * time.Minute
	autoRouteScorerTimeout     = 1500 * time.Millisecond
	autoRouteMaxScoringRunes   = 6000
	autoRouteScoringMaxTokens  = uint(120)
)

type autoRouteCandidate struct {
	ModelName     string
	RouterScore   int
	Group         string
	EstimatedCost float64
}

type autoRouteFeatures struct {
	Text          string
	LatestText    string
	ScoringText   string
	PromptTokens  int
	MaxTokens     int
	MessagesCount int
	HistoryTurns  int
	ToolsCount    int
	HasImage      bool
	SchemaBytes   int
	ForcedTool    bool
}

type autoRouteDecision struct {
	RoutedModel  string
	Difficulty   int
	Source       string
	ScorerModel  string
	ScorerFailed bool
	Underpowered bool
	Reason       string
}

type autoRouteScoringResult struct {
	Difficulty int     `json:"difficulty"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

type autoRouteEmbeddingTier struct {
	Score   int
	Label   string
	Text    string
	Samples []string
}

func maybeApplyAutoRoute(c *gin.Context, modelRequest *ModelRequest, usingGroup string, tokenModelLimit map[string]bool, modelLimitEnable bool) (bool, string) {
	if modelRequest == nil || modelRequest.Model != constant.AutoRouteModelName {
		return false, ""
	}
	relayMode := relayconstant.Path2RelayMode(c.Request.URL.Path)
	if relayMode != relayconstant.RelayModeChatCompletions && relayMode != relayconstant.RelayModeResponses {
		return false, fmt.Sprintf("%s 仅支持 /v1/chat/completions 和 /v1/responses", constant.AutoRouteModelName)
	}

	features, err := buildAutoRouteFeatures(c, relayMode)
	if err != nil {
		return false, err.Error()
	}
	candidates := buildAutoRouteCandidates(c, usingGroup, tokenModelLimit, modelLimitEnable)
	if len(candidates) == 0 {
		return false, fmt.Sprintf("分组 %s 下没有可用于 %s 的候选模型", usingGroup, constant.AutoRouteModelName)
	}

	decision := decideAutoRoute(c, features, candidates, usingGroup, tokenModelLimit, modelLimitEnable)
	if decision.RoutedModel == "" {
		return false, fmt.Sprintf("无法为 %s 选择真实模型", constant.AutoRouteModelName)
	}

	common.SetContextKey(c, constant.ContextKeyAutoRouteRequestedModel, constant.AutoRouteModelName)
	common.SetContextKey(c, constant.ContextKeyAutoRouteRoutedModel, decision.RoutedModel)
	common.SetContextKey(c, constant.ContextKeyAutoRouteDifficulty, decision.Difficulty)
	common.SetContextKey(c, constant.ContextKeyAutoRouteSource, decision.Source)
	common.SetContextKey(c, constant.ContextKeyAutoRouteUnderpowered, decision.Underpowered)
	if decision.ScorerModel != "" {
		common.SetContextKey(c, constant.ContextKeyAutoRouteScorerModel, decision.ScorerModel)
	}
	if decision.ScorerFailed {
		common.SetContextKey(c, constant.ContextKeyAutoRouteScorerFailed, true)
	}
	if decision.Reason != "" {
		common.SetContextKey(c, constant.ContextKeyAutoRouteReason, decision.Reason)
	}

	modelRequest.Model = decision.RoutedModel
	return true, ""
}

func buildAutoRouteCandidates(c *gin.Context, usingGroup string, tokenModelLimit map[string]bool, modelLimitEnable bool) []autoRouteCandidate {
	userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
	groups := autoRouteCandidateGroups(userGroup, usingGroup)
	baseCandidates := loadAutoRouteCandidateCache(userGroup, usingGroup, groups)
	return filterAutoRouteCandidatesForToken(baseCandidates, tokenModelLimit, modelLimitEnable)
}

func autoRouteCandidateGroups(userGroup string, usingGroup string) []string {
	groups := []string{usingGroup}
	if usingGroup == "auto" {
		groups = service.GetUserAutoGroup(userGroup)
	}
	return groups
}

func loadAutoRouteCandidateCache(userGroup string, usingGroup string, groups []string) []autoRouteCandidate {
	cacheKey := autoRouteCandidateCacheKey(userGroup, usingGroup, groups)
	now := time.Now()
	if cached, ok := autoRouteCandidateCache.Load(cacheKey); ok {
		if entry, ok := cached.(autoRouteCandidateCacheEntry); ok && now.Before(entry.expiresAt) {
			return cloneAutoRouteCandidates(entry.candidates)
		}
		autoRouteCandidateCache.Delete(cacheKey)
	}

	candidates := buildBaseAutoRouteCandidates(userGroup, groups)
	autoRouteCandidateCache.Store(cacheKey, autoRouteCandidateCacheEntry{
		candidates: cloneAutoRouteCandidates(candidates),
		expiresAt:  now.Add(autoRouteCandidateCacheTTL),
	})
	return candidates
}

func autoRouteCandidateCacheKey(userGroup string, usingGroup string, groups []string) string {
	return strings.Join([]string{
		strings.TrimSpace(userGroup),
		strings.TrimSpace(usingGroup),
		strings.Join(groups, ","),
	}, "\x00")
}

func buildBaseAutoRouteCandidates(userGroup string, groups []string) []autoRouteCandidate {
	modelNames := make([]string, 0)
	seen := make(map[string]struct{})
	for _, group := range groups {
		for _, modelName := range model.GetGroupEnabledModels(group) {
			if modelName == "" || modelName == constant.AutoRouteModelName {
				continue
			}
			if _, ok := seen[modelName]; ok {
				continue
			}
			seen[modelName] = struct{}{}
			modelNames = append(modelNames, modelName)
		}
	}
	scores := model.GetAutoRouteScoresForModels(modelNames)
	candidates := make([]autoRouteCandidate, 0, len(scores))
	for _, modelName := range modelNames {
		score := scores[modelName]
		if score <= 0 || !model.IsModelPriced(modelName) {
			continue
		}
		group, ok := firstAvailableGroupForModel(groups, modelName)
		if !ok {
			continue
		}
		candidates = append(candidates, autoRouteCandidate{
			ModelName:     modelName,
			RouterScore:   score,
			Group:         group,
			EstimatedCost: estimateAutoRouteCost(modelName, group, userGroup),
		})
	}
	return candidates
}

func filterAutoRouteCandidatesForToken(candidates []autoRouteCandidate, tokenModelLimit map[string]bool, modelLimitEnable bool) []autoRouteCandidate {
	if !modelLimitEnable {
		return cloneAutoRouteCandidates(candidates)
	}
	filtered := make([]autoRouteCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if tokenAllowsModel(tokenModelLimit, candidate.ModelName) {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

func cloneAutoRouteCandidates(candidates []autoRouteCandidate) []autoRouteCandidate {
	if len(candidates) == 0 {
		return nil
	}
	cloned := make([]autoRouteCandidate, len(candidates))
	copy(cloned, candidates)
	return cloned
}

func firstAvailableGroupForModel(groups []string, modelName string) (string, bool) {
	group, _, ok := firstAvailableGroupChannelForModel(groups, modelName)
	return group, ok
}

func firstAvailableGroupChannelForModel(groups []string, modelName string) (string, *model.Channel, bool) {
	for _, group := range groups {
		channel, err := model.GetRandomSatisfiedChannel(group, modelName, 0)
		if err == nil && channel != nil {
			return group, channel, true
		}
	}
	return "", nil, false
}

func autoRouteWarmupCandidateGroups() []string {
	seen := map[string]bool{}
	groups := make([]string, 0, 8)
	addGroup := func(group string) {
		group = strings.TrimSpace(group)
		if group == "" || seen[group] {
			return
		}
		seen[group] = true
		groups = append(groups, group)
	}
	addGroup("default")
	for _, group := range setting.GetAutoGroups() {
		addGroup(group)
	}
	for group := range setting.GetUserUsableGroupsCopy() {
		addGroup(group)
	}
	for group := range ratio_setting.GetGroupRatioCopy() {
		addGroup(group)
	}
	return groups
}

func tokenAllowsModel(tokenModelLimit map[string]bool, modelName string) bool {
	if len(tokenModelLimit) == 0 {
		return false
	}
	matchName := ratio_setting.FormatMatchingModelName(modelName)
	return tokenModelLimit[matchName]
}

func estimateAutoRouteCost(modelName string, group string, userGroup string) float64 {
	groupRatio := service.GetUserGroupRatio(userGroup, group)
	if modelPrice, ok := ratio_setting.GetModelPrice(modelName, false); ok {
		return modelPrice * common.QuotaPerUnit * groupRatio
	}
	if modelRatio, ok, _ := ratio_setting.GetModelRatio(modelName); ok {
		return modelRatio * groupRatio
	}
	return math.Inf(1)
}

func decideAutoRoute(c *gin.Context, features autoRouteFeatures, candidates []autoRouteCandidate, usingGroup string, tokenModelLimit map[string]bool, modelLimitEnable bool) autoRouteDecision {
	target := 50
	source := "heuristic_medium"
	scorerModel := strings.TrimSpace(setting.AutoRouteScoringModel)
	embeddingModel := strings.TrimSpace(setting.AutoRouteEmbeddingModel)
	reason := ""
	scorerFailed := false

	if isTrivialLatestTurn(features) {
		source = "heuristic_trivial_latest"
		target = 15
	} else {
		score, usedModel, usedSource, err := scoreAutoRoute(c, embeddingModel, scorerModel, usingGroup, features, tokenModelLimit, modelLimitEnable)
		if usedModel != "" {
			scorerModel = usedModel
		}
		if usedSource != "" {
			source = usedSource
		}
		if err != nil {
			scorerFailed = true
			source = "scorer_failed"
			reason = err.Error()
			target = 50
		} else if usedSource == "fixed_middle" || usedSource == "embedding_scorer" {
			target = score.Difficulty
		} else {
			target = score.Difficulty + 5
			if score.Confidence < 0.65 {
				target += 10
			}
			reason = score.Reason
		}
	}

	target = clampDifficulty(target)
	selected, underpowered := selectAutoRouteCandidate(candidates, target)
	logScorerModel := ""
	if source == "scorer" || source == "scorer_failed" {
		logScorerModel = scorerModel
	}
	return autoRouteDecision{
		RoutedModel:  selected.ModelName,
		Difficulty:   target,
		Source:       source,
		ScorerModel:  logScorerModel,
		ScorerFailed: scorerFailed,
		Underpowered: underpowered,
		Reason:       reason,
	}
}

func scoreAutoRoute(c *gin.Context, embeddingModel string, scoringModel string, usingGroup string, features autoRouteFeatures, tokenModelLimit map[string]bool, modelLimitEnable bool) (autoRouteScoringResult, string, string, error) {
	embeddingModel = strings.TrimSpace(embeddingModel)
	scoringModel = strings.TrimSpace(scoringModel)
	var embeddingErr error
	if embeddingModel != "" {
		if strings.EqualFold(embeddingModel, constant.AutoRouteModelName) {
			return autoRouteScoringResult{}, embeddingModel, "scorer_failed", fmt.Errorf("embedding scoring model cannot be afb-auto")
		}
		if modelLimitEnable && !tokenAllowsModel(tokenModelLimit, embeddingModel) {
			if scoringModel == "" {
				return autoRouteScoringResult{}, embeddingModel, "scorer_failed", fmt.Errorf("embedding scoring model is not allowed by token model limit")
			}
		} else {
			result, err := scoreAutoRouteDifficultyWithEmbedding(c, embeddingModel, usingGroup, features)
			if err == nil {
				return result, embeddingModel, "embedding_scorer", nil
			}
			embeddingErr = err
			if scoringModel == "" {
				return autoRouteScoringResult{}, embeddingModel, "scorer_failed", err
			}
		}
	}

	if scoringModel == "" {
		return autoRouteScoringResult{Difficulty: 50, Confidence: 1}, "", "fixed_middle", nil
	}
	if strings.EqualFold(scoringModel, constant.AutoRouteModelName) {
		return autoRouteScoringResult{}, scoringModel, "scorer_failed", fmt.Errorf("scoring model cannot be afb-auto")
	}
	if modelLimitEnable && !tokenAllowsModel(tokenModelLimit, scoringModel) {
		return autoRouteScoringResult{}, scoringModel, "scorer_failed", fmt.Errorf("scoring model is not allowed by token model limit")
	}
	result, err := scoreAutoRouteDifficulty(c, scoringModel, usingGroup, features)
	if err != nil {
		if embeddingErr != nil {
			return autoRouteScoringResult{}, scoringModel, "scorer_failed", fmt.Errorf("embedding scorer failed: %v; scoring model failed: %w", embeddingErr, err)
		}
		return autoRouteScoringResult{}, scoringModel, "scorer_failed", err
	}
	return result, scoringModel, "scorer", nil
}

func selectAutoRouteCandidate(candidates []autoRouteCandidate, target int) (autoRouteCandidate, bool) {
	eligible := make([]autoRouteCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.RouterScore >= target {
			eligible = append(eligible, candidate)
		}
	}
	if len(eligible) > 0 {
		sort.SliceStable(eligible, func(i, j int) bool {
			if eligible[i].RouterScore == eligible[j].RouterScore {
				return eligible[i].EstimatedCost < eligible[j].EstimatedCost
			}
			return eligible[i].RouterScore < eligible[j].RouterScore
		})
		return eligible[0], false
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].RouterScore == candidates[j].RouterScore {
			return candidates[i].EstimatedCost < candidates[j].EstimatedCost
		}
		return candidates[i].RouterScore > candidates[j].RouterScore
	})
	return candidates[0], true
}

func isTrivialLatestTurn(features autoRouteFeatures) bool {
	if features.ToolsCount > 0 || features.ForcedTool || features.HasImage || features.SchemaBytes > 0 {
		return false
	}
	latest := strings.TrimSpace(features.LatestText)
	if latest == "" || estimateTextTokens(latest) > 30 || utf8.RuneCountInString(latest) > 80 {
		return false
	}
	lower := strings.ToLower(strings.Trim(latest, " \t\r\n。！？!?,，."))
	if lower == "" {
		return false
	}
	if containsAny(lower, []string{"```", "traceback", "exception", "stack trace", "sql", "regex", "正则", "代码", "报错", "bug", "debug", "函数", "接口", "证明", "推导", "数学", "概率", "复杂度", "算法", "公式", "架构", "方案", "分析", "严谨", "全面", "性能优化", "设计", "重构", "review", "architecture"}) {
		return false
	}
	trivialPhrases := []string{
		"你好", "你好呀", "您好", "在吗", "早上好", "上午好", "中午好", "下午好", "晚上好",
		"谢谢", "谢谢你", "感谢", "好的", "好", "嗯", "嗯嗯", "哦", "ok", "okay",
		"hi", "hello", "hey", "thanks", "thank you",
	}
	for _, phrase := range trivialPhrases {
		if lower == phrase {
			return true
		}
	}
	return false
}

func buildAutoRouteFeatures(c *gin.Context, relayMode int) (autoRouteFeatures, error) {
	switch relayMode {
	case relayconstant.RelayModeChatCompletions:
		req := &dto.GeneralOpenAIRequest{}
		if err := common.UnmarshalBodyReusable(c, req); err != nil {
			return autoRouteFeatures{}, err
		}
		meta := req.GetTokenCountMeta()
		features := featuresFromTokenMeta(meta)
		latestText, previousUserText := latestAndPreviousChatUserText(req.Messages)
		features.LatestText = latestText
		features.HistoryTurns = chatHistoryTurns(req.Messages)
		features.ScoringText = autoRouteScoringText(features.LatestText, previousUserText, features)
		features.ToolsCount = maxInt(features.ToolsCount, len(req.Tools))
		features.ForcedTool = req.ToolChoice != nil && fmt.Sprintf("%v", req.ToolChoice) != "" && fmt.Sprintf("%v", req.ToolChoice) != "auto"
		if req.ResponseFormat != nil && len(req.ResponseFormat.JsonSchema) > 0 {
			features.SchemaBytes = len(req.ResponseFormat.JsonSchema)
		}
		return features, nil
	case relayconstant.RelayModeResponses:
		req := &dto.OpenAIResponsesRequest{}
		if err := common.UnmarshalBodyReusable(c, req); err != nil {
			return autoRouteFeatures{}, err
		}
		meta := req.GetTokenCountMeta()
		features := featuresFromTokenMeta(meta)
		features.LatestText = latestResponsesInputText(req)
		features.ScoringText = autoRouteScoringText(features.LatestText, "", features)
		features.ToolsCount = maxInt(features.ToolsCount, len(req.GetToolsMap()))
		features.ForcedTool = len(req.ToolChoice) > 0 && string(req.ToolChoice) != `"auto"`
		features.SchemaBytes = len(req.Text)
		return features, nil
	default:
		return autoRouteFeatures{}, fmt.Errorf("unsupported auto route relay mode")
	}
}

func featuresFromTokenMeta(meta *types.TokenCountMeta) autoRouteFeatures {
	if meta == nil {
		meta = &types.TokenCountMeta{}
	}
	features := autoRouteFeatures{
		Text:          meta.CombineText,
		LatestText:    meta.CombineText,
		ScoringText:   meta.CombineText,
		PromptTokens:  estimateTextTokens(meta.CombineText),
		MaxTokens:     meta.MaxTokens,
		MessagesCount: meta.MessagesCount,
		ToolsCount:    meta.ToolsCount,
	}
	for _, file := range meta.Files {
		if file != nil && file.FileType == types.FileTypeImage {
			features.HasImage = true
			break
		}
	}
	return features
}

func latestChatUserText(messages []dto.Message) string {
	latest, _ := latestAndPreviousChatUserText(messages)
	return latest
}

func latestAndPreviousChatUserText(messages []dto.Message) (string, string) {
	latest := ""
	priorContinuations := make([]string, 0, 3)
	for i := len(messages) - 1; i >= 0; i-- {
		if !strings.EqualFold(messages[i].Role, "user") {
			continue
		}
		content := strings.TrimSpace(messages[i].StringContent())
		if content == "" {
			continue
		}
		if latest == "" {
			latest = content
			continue
		}
		if isContinuationLatestTurn(latest) && isContinuationLatestTurn(content) {
			priorContinuations = append(priorContinuations, content)
			continue
		}
		return latest, autoRoutePreviousTaskContext(content, priorContinuations)
	}
	return latest, autoRoutePreviousTaskContext("", priorContinuations)
}

func autoRouteScoringText(latestText string, previousUserText string, features autoRouteFeatures) string {
	latestText = strings.TrimSpace(latestText)
	previousUserText = strings.TrimSpace(previousUserText)
	if isContinuationLatestTurn(latestText) && previousUserText != "" {
		return strings.Join([]string{
			"当前用户请求: " + latestText,
			"上文任务线索: " + truncateRunes(previousUserText, autoRouteMaxScoringRunes),
		}, "\n")
	}
	if latestText != "" {
		return latestText
	}
	return strings.TrimSpace(features.Text)
}

func autoRoutePreviousTaskContext(previousTask string, priorContinuations []string) string {
	parts := make([]string, 0, 2)
	previousTask = strings.TrimSpace(previousTask)
	if previousTask != "" {
		parts = append(parts, "最近真实任务: "+truncateRunes(previousTask, autoRouteMaxScoringRunes))
	}
	if len(priorContinuations) > 0 {
		reversed := make([]string, 0, len(priorContinuations))
		for i := len(priorContinuations) - 1; i >= 0; i-- {
			if item := strings.TrimSpace(priorContinuations[i]); item != "" {
				reversed = append(reversed, item)
			}
		}
		if len(reversed) > 0 {
			parts = append(parts, "中间续写指令: "+strings.Join(reversed, " / "))
		}
	}
	return strings.Join(parts, "\n")
}

func isContinuationLatestTurn(text string) bool {
	normalized := normalizeAutoRouteShortText(text)
	if normalized == "" {
		return false
	}
	exact := map[string]bool{
		"继续":         true,
		"接着":         true,
		"继续写":        true,
		"接着写":        true,
		"往下写":        true,
		"继续上面":       true,
		"接着上面":       true,
		"继续刚才":       true,
		"接着刚才":       true,
		"继续分析":       true,
		"继续优化":       true,
		"继续改":        true,
		"继续说":        true,
		"继续讲":        true,
		"展开":         true,
		"再展开":        true,
		"go on":      true,
		"continue":   true,
		"keep going": true,
		"more":       true,
		"next":       true,
	}
	if exact[normalized] {
		return true
	}
	prefixes := []string{
		"继续上", "接着上", "继续刚", "接着刚", "继续这个", "接着这个",
		"继续前", "接着前", "按上面", "按刚才", "在上面基础", "在刚才基础",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(normalized, prefix) {
			return true
		}
	}
	return false
}

func normalizeAutoRouteShortText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	text = strings.Trim(text, " \t\r\n。！？!?,，.;；:：")
	text = strings.Join(strings.Fields(text), " ")
	return text
}

func autoRouteContinuationMinScore(features autoRouteFeatures) int {
	if features.HistoryTurns <= 0 || !isContinuationLatestTurn(features.LatestText) {
		return 0
	}
	if features.ToolsCount > 0 || features.ForcedTool || features.HasImage || features.SchemaBytes > 0 {
		return 45
	}
	return 30
}

func chatHistoryTurns(messages []dto.Message) int {
	userMessages := 0
	for _, message := range messages {
		if message.Role == "user" {
			userMessages++
		}
	}
	if userMessages <= 1 {
		return 0
	}
	return userMessages - 1
}

func latestResponsesInputText(req *dto.OpenAIResponsesRequest) string {
	if req == nil {
		return ""
	}
	inputs := req.ParseInput()
	for i := len(inputs) - 1; i >= 0; i-- {
		if inputs[i].Text != "" {
			return strings.TrimSpace(inputs[i].Text)
		}
	}
	return ""
}

func scoreAutoRouteDifficultyWithEmbedding(c *gin.Context, embeddingModel string, usingGroup string, features autoRouteFeatures) (autoRouteScoringResult, error) {
	userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
	groups := []string{usingGroup}
	if usingGroup == "auto" {
		groups = service.GetUserAutoGroup(userGroup)
	}
	group, channel, ok := firstAvailableGroupChannelForModel(groups, embeddingModel)
	if !ok {
		return autoRouteScoringResult{}, fmt.Errorf("embedding scoring model has no available channel")
	}

	scoreCtx := c.Copy()
	scoreCtx.Request = c.Request.Clone(c.Request.Context())
	scoreCtx.Set("use_channel", []string{fmt.Sprintf("%d", channel.Id)})
	common.SetContextKey(scoreCtx, constant.ContextKeyAutoRouteScoring, true)
	common.SetContextKey(scoreCtx, constant.ContextKeyAutoRouteRequestedModel, constant.AutoRouteModelName)
	common.SetContextKey(scoreCtx, constant.ContextKeyAutoRouteScorerModel, embeddingModel)
	if usingGroup == "auto" {
		common.SetContextKey(scoreCtx, constant.ContextKeyAutoGroup, group)
	}
	if newAPIError := SetupContextForSelectedChannel(scoreCtx, channel, embeddingModel); newAPIError != nil {
		return autoRouteScoringResult{}, errors.New(newAPIError.Error())
	}

	embeddingReq := &dto.EmbeddingRequest{
		Model: embeddingModel,
		Input: autoRouteEmbeddingInputs(features),
	}
	relayInfo := relaycommon.GenRelayInfoEmbedding(scoreCtx, embeddingReq)
	relayInfo.RelayMode = relayconstant.RelayModeEmbeddings
	relayInfo.RequestURLPath = "/v1/embeddings"
	relayInfo.InitRequestConversionChain()
	relayInfo.InitChannelMeta(scoreCtx)
	if err := helper.ModelMappedHelper(scoreCtx, relayInfo, embeddingReq); err != nil {
		return autoRouteScoringResult{}, err
	}

	meta := embeddingReq.GetTokenCountMeta()
	tokens, err := service.EstimateRequestToken(scoreCtx, meta, relayInfo)
	if err != nil {
		return autoRouteScoringResult{}, err
	}
	relayInfo.SetEstimatePromptTokens(tokens)
	priceData, err := helper.ModelPriceHelper(scoreCtx, relayInfo, tokens, meta)
	if err != nil {
		return autoRouteScoringResult{}, err
	}
	if !priceData.FreeModel {
		if apiErr := service.PreConsumeBilling(scoreCtx, priceData.QuotaToPreConsume, relayInfo); apiErr != nil {
			return autoRouteScoringResult{}, errors.New(apiErr.Error())
		}
	}

	embeddings, usage, err := doAutoRouteEmbeddingRequest(scoreCtx, relayInfo, embeddingReq)
	if err != nil {
		if relayInfo.Billing != nil {
			relayInfo.Billing.Refund(scoreCtx)
		}
		return autoRouteScoringResult{}, err
	}
	service.PostTextConsumeQuota(scoreCtx, relayInfo, usage, []string{"AFB Auto 路由向量打分"})

	result, err := scoreAutoRouteFromEmbeddings(scoreCtx, embeddings, features)
	if err != nil {
		return autoRouteScoringResult{}, err
	}
	return result, nil
}

func StartAutoRouteCentroidWarmup(reason string) {
	embeddingModel := strings.TrimSpace(setting.AutoRouteEmbeddingModel)
	if embeddingModel == "" || strings.EqualFold(embeddingModel, constant.AutoRouteModelName) {
		return
	}
	go warmAutoRouteCentroidsForModel(embeddingModel, reason)
}

func warmAutoRouteCentroidsForModel(embeddingModel string, reason string) {
	_, channel, ok := firstAvailableGroupChannelForModel(autoRouteWarmupCandidateGroups(), embeddingModel)
	if !ok {
		return
	}

	c := newAutoRouteBackgroundContext()
	common.SetContextKey(c, constant.ContextKeyAutoRouteScoring, true)
	common.SetContextKey(c, constant.ContextKeyAutoRouteRequestedModel, constant.AutoRouteModelName)
	common.SetContextKey(c, constant.ContextKeyAutoRouteScorerModel, embeddingModel)
	if newAPIError := SetupContextForSelectedChannel(c, channel, embeddingModel); newAPIError != nil {
		return
	}

	embeddingReq := &dto.EmbeddingRequest{Model: embeddingModel}
	relayInfo := relaycommon.GenRelayInfoEmbedding(c, embeddingReq)
	relayInfo.RelayMode = relayconstant.RelayModeEmbeddings
	relayInfo.RequestURLPath = "/v1/embeddings"
	relayInfo.InitRequestConversionChain()
	relayInfo.InitChannelMeta(c)
	if err := helper.ModelMappedHelper(c, relayInfo, embeddingReq); err != nil {
		return
	}
	scheduleAutoRouteCentroidWarmupForRelayInfo(c, relayInfo, embeddingReq, reason)
}

func newAutoRouteBackgroundContext() *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", nil)
	c.Set("use_channel", []string{})
	return c
}

func scheduleAutoRouteCentroidWarmupForRelayInfo(c *gin.Context, info *relaycommon.RelayInfo, request *dto.EmbeddingRequest, _ string) {
	cacheKey := autoRouteEmbeddingTierCacheKey(info, request)
	if _, ok := loadAutoRouteEmbeddingTierCache(cacheKey, len(autoRouteEmbeddingTiers)); ok {
		return
	}
	if _, loaded := autoRouteEmbeddingTierWarmupInFlight.LoadOrStore(cacheKey, true); loaded {
		return
	}

	infoCopy := *info
	requestCopy := *request
	go func() {
		defer autoRouteEmbeddingTierWarmupInFlight.Delete(cacheKey)
		warmupCtx := newAutoRouteBackgroundContext()
		common.SetContextKey(warmupCtx, constant.ContextKeyAutoRouteScoring, true)
		common.SetContextKey(warmupCtx, constant.ContextKeyAutoRouteRequestedModel, constant.AutoRouteModelName)
		common.SetContextKey(warmupCtx, constant.ContextKeyAutoRouteScorerModel, requestCopy.Model)

		centroids := make([][]float64, 0, len(autoRouteEmbeddingTiers))
		for _, tier := range autoRouteEmbeddingTiers {
			prototypes := autoRouteEmbeddingTierPrototypes(tier)
			embedding, _, err := buildAutoRouteTierCentroid(warmupCtx, &infoCopy, &requestCopy, prototypes)
			if err != nil {
				return
			}
			centroids = append(centroids, embedding)
		}

		storeAutoRouteEmbeddingTierCache(cacheKey, autoRouteEmbeddingTierCacheEntry{embeddings: centroids})
	}()
}

func autoRouteEmbeddingInputs(features autoRouteFeatures) []any {
	latestText := strings.TrimSpace(features.ScoringText)
	if latestText == "" {
		latestText = strings.TrimSpace(features.LatestText)
	}
	if latestText == "" {
		latestText = strings.TrimSpace(features.Text)
	}
	if latestText == "" {
		latestText = "空请求"
	}
	inputs := []any{truncateRunes(latestText, autoRouteMaxScoringRunes)}
	for _, tier := range autoRouteEmbeddingTiers {
		inputs = append(inputs, autoRouteEmbeddingTierText(tier))
	}
	return inputs
}

func doAutoRouteEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.EmbeddingRequest) ([][]float64, *dto.Usage, error) {
	inputs := request.ParseInput()
	if len(inputs) > 1 {
		return doAutoRouteCachedTierEmbeddingRequest(c, info, request, inputs)
	}
	return doAutoRouteEmbeddingSingleRequest(c, info, request, 0, 0)
}

func doAutoRouteCachedTierEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.EmbeddingRequest, inputs []string) ([][]float64, *dto.Usage, error) {
	queryInput := inputs[0]
	tierInputs := inputs[1:]
	cacheKey := autoRouteEmbeddingTierCacheKey(info, request)
	if entry, ok := loadAutoRouteEmbeddingTierCache(cacheKey, len(tierInputs)); ok {
		queryEmbedding, usage, err := doAutoRouteEmbeddingInput(c, info, request, queryInput, 1, 1)
		if err != nil {
			return nil, nil, err
		}
		embeddings := make([][]float64, 0, len(entry.embeddings)+1)
		embeddings = append(embeddings, queryEmbedding)
		embeddings = append(embeddings, entry.embeddings...)
		return embeddings, usage, nil
	}
	scheduleAutoRouteCentroidWarmupForRelayInfo(c, info, request, "cache_miss")
	return nil, nil, fmt.Errorf("embedding tier cache not warmed for model %s", request.Model)
}

type autoRouteEmbeddingTierRedisEntry struct {
	Embeddings [][]float64 `json:"embeddings"`
}

func loadAutoRouteEmbeddingTierCache(cacheKey string, expectedTiers int) (autoRouteEmbeddingTierCacheEntry, bool) {
	if cached, ok := autoRouteEmbeddingTierCache.Load(cacheKey); ok {
		if entry, ok := cached.(autoRouteEmbeddingTierCacheEntry); ok && len(entry.embeddings) == expectedTiers {
			return entry, true
		}
	}

	if !common.RedisEnabled || common.RDB == nil {
		return autoRouteEmbeddingTierCacheEntry{}, false
	}
	payload, err := common.RedisGet(autoRouteEmbeddingTierRedisKey(cacheKey))
	if err != nil || payload == "" {
		return autoRouteEmbeddingTierCacheEntry{}, false
	}
	var redisEntry autoRouteEmbeddingTierRedisEntry
	if err := json.Unmarshal([]byte(payload), &redisEntry); err != nil || len(redisEntry.Embeddings) != expectedTiers {
		return autoRouteEmbeddingTierCacheEntry{}, false
	}
	entry := autoRouteEmbeddingTierCacheEntry{embeddings: redisEntry.Embeddings}
	autoRouteEmbeddingTierCache.Store(cacheKey, entry)
	return entry, true
}

func storeAutoRouteEmbeddingTierCache(cacheKey string, entry autoRouteEmbeddingTierCacheEntry) {
	autoRouteEmbeddingTierCache.Store(cacheKey, entry)
	if !common.RedisEnabled || common.RDB == nil {
		return
	}
	payload, err := json.Marshal(autoRouteEmbeddingTierRedisEntry{Embeddings: entry.embeddings})
	if err != nil {
		return
	}
	if err := common.RedisSet(autoRouteEmbeddingTierRedisKey(cacheKey), string(payload), 0); err != nil {
		return
	}
}

func autoRouteEmbeddingTierRedisKey(cacheKey string) string {
	return "auto_route:embedding_tier:" + cacheKey
}

func buildAutoRouteTierCentroid(c *gin.Context, info *relaycommon.RelayInfo, request *dto.EmbeddingRequest, prototypes []string) ([]float64, *dto.Usage, error) {
	if len(prototypes) == 0 {
		return nil, nil, fmt.Errorf("embedding tier has no prototypes")
	}
	vectors, usage, err := doAutoRouteEmbeddingInputs(c, info, request, prototypes)
	if err != nil {
		return nil, nil, err
	}
	centroid := meanNormalizedAutoRouteEmbeddings(vectors)
	if len(centroid) == 0 {
		return nil, nil, fmt.Errorf("embedding tier produced empty centroid")
	}
	return centroid, usage, nil
}

func doAutoRouteEmbeddingInputs(c *gin.Context, info *relaycommon.RelayInfo, request *dto.EmbeddingRequest, inputs []string) ([][]float64, *dto.Usage, error) {
	if len(inputs) == 0 {
		return nil, nil, fmt.Errorf("embedding tier has no inputs")
	}
	batchInfo := *info
	if len(info.RequestConversionChain) > 0 {
		batchInfo.RequestConversionChain = append([]types.RelayFormat(nil), info.RequestConversionChain...)
	}
	batchReq := *request
	batchReq.Input = inputs
	embeddings, usage, err := doAutoRouteEmbeddingSingleRequest(c, &batchInfo, &batchReq, 1, len(inputs))
	if err != nil {
		return nil, nil, err
	}
	if len(embeddings) != len(inputs) {
		if len(embeddings) == 1 && len(embeddings[0]) > 0 {
			return embeddings, usage, nil
		}
		return nil, nil, fmt.Errorf("embedding scoring model returned %d embeddings for %d tier inputs", len(embeddings), len(inputs))
	}
	for index, embedding := range embeddings {
		if len(embedding) == 0 {
			return nil, nil, fmt.Errorf("embedding scoring model returned empty embedding for input %d", index+1)
		}
	}
	return embeddings, usage, nil
}

func doAutoRouteEmbeddingInput(c *gin.Context, info *relaycommon.RelayInfo, request *dto.EmbeddingRequest, input string, itemIndex int, itemCount int) ([]float64, *dto.Usage, error) {
	singleReq := *request
	singleReq.Input = input
	embeddings, usage, err := doAutoRouteEmbeddingSingleRequest(c, info, &singleReq, itemIndex, itemCount)
	if err != nil {
		return nil, nil, err
	}
	if len(embeddings) == 0 || len(embeddings[0]) == 0 {
		return nil, nil, fmt.Errorf("embedding scoring model returned no embedding for input %d", itemIndex)
	}
	return embeddings[0], usage, nil
}

func autoRouteEmbeddingTierCacheKey(info *relaycommon.RelayInfo, request *dto.EmbeddingRequest) string {
	if info == nil {
		return request.Model
	}
	return fmt.Sprintf("%d:%d:%s:%s:%s", info.ChannelType, info.ChannelId, info.UpstreamModelName, info.OriginModelName, request.Model)
}

func doAutoRouteEmbeddingSingleRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.EmbeddingRequest, _ int, _ int) ([][]float64, *dto.Usage, error) {
	adaptor := relay.GetAdaptor(info.ApiType)
	if adaptor == nil {
		return nil, nil, fmt.Errorf("invalid embedding scoring model api type: %d", info.ApiType)
	}
	adaptor.Init(info)

	convertedRequest, err := adaptor.ConvertEmbeddingRequest(c, info, *request)
	if err != nil {
		return nil, nil, err
	}
	relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)
	jsonData, err := common.Marshal(convertedRequest)
	if err != nil {
		return nil, nil, err
	}
	if len(info.ParamOverride) > 0 {
		jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
		if err != nil {
			return nil, nil, err
		}
	}

	reqCtx, cancel := context.WithTimeout(c.Request.Context(), autoRouteScorerTimeout)
	defer cancel()
	scoreCtx := c.Copy()
	scoreCtx.Request = c.Request.Clone(reqCtx)
	respAny, err := adaptor.DoRequest(scoreCtx, info, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, nil, err
	}
	httpResp, ok := respAny.(*http.Response)
	if !ok || httpResp == nil {
		return nil, nil, fmt.Errorf("invalid embedding scoring model response")
	}
	defer service.CloseResponseBodyGracefully(httpResp)
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, nil, err
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("embedding scoring model status %d: %s", httpResp.StatusCode, strings.TrimSpace(string(body)))
	}
	embeddings, usage, err := parseAutoRouteEmbeddingResponse(body)
	if err != nil {
		return nil, nil, err
	}
	if usage.PromptTokens == 0 && usage.TotalTokens == 0 {
		usage.PromptTokens = info.GetEstimatePromptTokens()
		usage.TotalTokens = usage.PromptTokens
	}
	return embeddings, &usage, nil
}

func addAutoRouteEmbeddingUsage(total *dto.Usage, usage dto.Usage) {
	total.PromptTokens += usage.PromptTokens
	total.CompletionTokens += usage.CompletionTokens
	total.TotalTokens += usage.TotalTokens
	total.PromptCacheHitTokens += usage.PromptCacheHitTokens
	total.PromptTokensDetails.CachedTokens += usage.PromptTokensDetails.CachedTokens
	total.PromptTokensDetails.CachedCreationTokens += usage.PromptTokensDetails.CachedCreationTokens
	total.PromptTokensDetails.TextTokens += usage.PromptTokensDetails.TextTokens
	total.PromptTokensDetails.AudioTokens += usage.PromptTokensDetails.AudioTokens
	total.PromptTokensDetails.ImageTokens += usage.PromptTokensDetails.ImageTokens
	total.CompletionTokenDetails.TextTokens += usage.CompletionTokenDetails.TextTokens
	total.CompletionTokenDetails.AudioTokens += usage.CompletionTokenDetails.AudioTokens
	total.CompletionTokenDetails.ReasoningTokens += usage.CompletionTokenDetails.ReasoningTokens
	total.InputTokens += usage.InputTokens
	total.OutputTokens += usage.OutputTokens
	if usage.InputTokensDetails != nil {
		if total.InputTokensDetails == nil {
			total.InputTokensDetails = &dto.InputTokenDetails{}
		}
		total.InputTokensDetails.CachedTokens += usage.InputTokensDetails.CachedTokens
		total.InputTokensDetails.CachedCreationTokens += usage.InputTokensDetails.CachedCreationTokens
		total.InputTokensDetails.TextTokens += usage.InputTokensDetails.TextTokens
		total.InputTokensDetails.AudioTokens += usage.InputTokensDetails.AudioTokens
		total.InputTokensDetails.ImageTokens += usage.InputTokensDetails.ImageTokens
	}
}

func parseAutoRouteEmbeddingResponse(body []byte) ([][]float64, dto.Usage, error) {
	var openAI dto.OpenAIEmbeddingResponse
	if err := common.Unmarshal(body, &openAI); err == nil && len(openAI.Data) > 0 {
		embeddings := make([][]float64, 0, len(openAI.Data))
		sort.SliceStable(openAI.Data, func(i, j int) bool {
			return openAI.Data[i].Index < openAI.Data[j].Index
		})
		for _, item := range openAI.Data {
			if len(item.Embedding) > 0 {
				embeddings = append(embeddings, item.Embedding)
			}
		}
		if len(embeddings) > 0 {
			return embeddings, openAI.Usage, nil
		}
	}

	var generic struct {
		Data  any       `json:"data"`
		Usage dto.Usage `json:"usage"`
	}
	if err := common.Unmarshal(body, &generic); err == nil {
		if embeddings := extractAutoRouteEmbeddingsFromGenericData(generic.Data); len(embeddings) > 0 {
			return embeddings, generic.Usage, nil
		}
	}

	var flexible dto.FlexibleEmbeddingResponse
	if err := common.Unmarshal(body, &flexible); err == nil && len(flexible.Data) > 0 {
		embeddings := make([][]float64, 0, len(flexible.Data))
		sort.SliceStable(flexible.Data, func(i, j int) bool {
			return flexible.Data[i].Index < flexible.Data[j].Index
		})
		for _, item := range flexible.Data {
			if embedding := extractAutoRouteEmbeddingVector(item.Embedding); len(embedding) > 0 {
				embeddings = append(embeddings, embedding)
			}
		}
		if len(embeddings) > 0 {
			return embeddings, flexible.Usage, nil
		}
	}

	var geminiBatch dto.GeminiBatchEmbeddingResponse
	if err := common.Unmarshal(body, &geminiBatch); err == nil && len(geminiBatch.Embeddings) > 0 {
		embeddings := make([][]float64, 0, len(geminiBatch.Embeddings))
		for _, item := range geminiBatch.Embeddings {
			if item != nil && len(item.Values) > 0 {
				embeddings = append(embeddings, item.Values)
			}
		}
		if len(embeddings) > 0 {
			return embeddings, dto.Usage{}, nil
		}
	}

	var gemini dto.GeminiEmbeddingResponse
	if err := common.Unmarshal(body, &gemini); err == nil && len(gemini.Embedding.Values) > 0 {
		return [][]float64{gemini.Embedding.Values}, dto.Usage{}, nil
	}

	return nil, dto.Usage{}, fmt.Errorf("embedding scoring model returned no embeddings")
}

type autoRouteIndexedEmbedding struct {
	index     int
	embedding []float64
}

func extractAutoRouteEmbeddingsFromGenericData(data any) [][]float64 {
	switch value := data.(type) {
	case []any:
		indexed := make([]autoRouteIndexedEmbedding, 0, len(value))
		for fallbackIndex, item := range value {
			itemMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			embedding := extractAutoRouteEmbeddingVector(itemMap["embedding"])
			if len(embedding) == 0 {
				continue
			}
			index := fallbackIndex
			if rawIndex, ok := autoRouteNumberToFloat64(itemMap["index"]); ok {
				index = int(rawIndex)
			}
			indexed = append(indexed, autoRouteIndexedEmbedding{index: index, embedding: embedding})
		}
		sort.SliceStable(indexed, func(i, j int) bool {
			return indexed[i].index < indexed[j].index
		})
		embeddings := make([][]float64, 0, len(indexed))
		for _, item := range indexed {
			embeddings = append(embeddings, item.embedding)
		}
		return embeddings
	case map[string]any:
		if embedding := extractAutoRouteEmbeddingVector(value["embedding"]); len(embedding) > 0 {
			return [][]float64{embedding}
		}
	}
	return nil
}

func extractAutoRouteEmbeddingVector(value any) []float64 {
	switch embedding := value.(type) {
	case []float64:
		return embedding
	case [][]float64:
		if len(embedding) > 0 {
			return embedding[0]
		}
	case [][]any:
		if len(embedding) > 0 {
			return anySliceToFloatVector(embedding[0])
		}
	case []any:
		if vector := anySliceToFloatVector(embedding); len(vector) > 0 {
			return vector
		}
		for _, item := range embedding {
			if vector := extractAutoRouteEmbeddingVector(item); len(vector) > 0 {
				return vector
			}
		}
	case map[string]any:
		for _, key := range []string{"dense", "float", "vector", "values", "embedding"} {
			if vector := extractAutoRouteEmbeddingVector(embedding[key]); len(vector) > 0 {
				return vector
			}
		}
	}
	return nil
}

func anySliceToFloatVector(values []any) []float64 {
	vector := make([]float64, 0, len(values))
	for _, value := range values {
		number, ok := autoRouteNumberToFloat64(value)
		if !ok {
			return nil
		}
		vector = append(vector, number)
	}
	return vector
}

func autoRouteNumberToFloat64(value any) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, true
	case float32:
		return float64(number), true
	case int:
		return float64(number), true
	case int64:
		return float64(number), true
	case json.Number:
		parsed, err := number.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

func scoreAutoRouteFromEmbeddings(c *gin.Context, embeddings [][]float64, features autoRouteFeatures) (autoRouteScoringResult, error) {
	if len(embeddings) < len(autoRouteEmbeddingTiers)+1 {
		return autoRouteScoringResult{}, fmt.Errorf("embedding scoring model returned %d vectors, want %d", len(embeddings), len(autoRouteEmbeddingTiers)+1)
	}
	query := embeddings[0]
	sims := make([]float64, 0, len(autoRouteEmbeddingTiers))
	for i := range autoRouteEmbeddingTiers {
		sims = append(sims, cosineSimilarity(query, embeddings[i+1]))
	}
	bestIdx := 0
	secondIdx := -1
	for i, sim := range sims {
		if sim > sims[bestIdx] {
			secondIdx = bestIdx
			bestIdx = i
			continue
		}
		if i != bestIdx && (secondIdx == -1 || sim > sims[secondIdx]) {
			secondIdx = i
		}
	}
	baseScore := autoRouteEmbeddingTiers[bestIdx].Score
	secondScore := 0
	secondSim := 0.0
	if secondIdx >= 0 {
		secondScore = autoRouteEmbeddingTiers[secondIdx].Score
		secondSim = sims[secondIdx]
	}
	margin := sims[bestIdx] - secondSim
	ambiguous := secondIdx >= 0 && margin < 0.03
	if ambiguous && secondScore < baseScore {
		baseScore = secondScore
	}
	continuationMinScore := autoRouteContinuationMinScore(features)
	if continuationMinScore > baseScore {
		baseScore = continuationMinScore
	}
	difficulty := clampDifficulty(baseScore)
	confidence := margin / 0.08
	if confidence < 0 {
		confidence = 0
	} else if confidence > 1 {
		confidence = 1
	}
	return autoRouteScoringResult{
		Difficulty: difficulty,
		Confidence: confidence,
	}, nil
}

func cosineSimilarity(a []float64, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var dot, normA, normB float64
	for i := 0; i < n; i++ {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func meanNormalizedAutoRouteEmbeddings(vectors [][]float64) []float64 {
	if len(vectors) == 0 || len(vectors[0]) == 0 {
		return nil
	}
	dim := len(vectors[0])
	centroid := make([]float64, dim)
	valid := 0
	for _, vector := range vectors {
		if len(vector) != dim {
			continue
		}
		normalized := normalizeAutoRouteVector(vector)
		if len(normalized) == 0 {
			continue
		}
		for i, value := range normalized {
			centroid[i] += value
		}
		valid++
	}
	if valid == 0 {
		return nil
	}
	for i := range centroid {
		centroid[i] /= float64(valid)
	}
	return normalizeAutoRouteVector(centroid)
}

func normalizeAutoRouteVector(vector []float64) []float64 {
	norm := 0.0
	for _, value := range vector {
		norm += value * value
	}
	if norm == 0 {
		return nil
	}
	normalized := make([]float64, len(vector))
	scale := math.Sqrt(norm)
	for i, value := range vector {
		normalized[i] = value / scale
	}
	return normalized
}

func softmax(values []float64, temperature float64) []float64 {
	if len(values) == 0 {
		return nil
	}
	maxValue := values[0]
	for _, value := range values[1:] {
		if value > maxValue {
			maxValue = value
		}
	}
	probs := make([]float64, len(values))
	sum := 0.0
	for i, value := range values {
		prob := math.Exp((value - maxValue) * temperature)
		probs[i] = prob
		sum += prob
	}
	if sum == 0 {
		return probs
	}
	for i := range probs {
		probs[i] /= sum
	}
	return probs
}

func scoreAutoRouteDifficulty(c *gin.Context, scorerModel string, usingGroup string, features autoRouteFeatures) (autoRouteScoringResult, error) {
	userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
	groups := []string{usingGroup}
	if usingGroup == "auto" {
		groups = service.GetUserAutoGroup(userGroup)
	}
	group, channel, ok := firstAvailableGroupChannelForModel(groups, scorerModel)
	if !ok {
		return autoRouteScoringResult{}, fmt.Errorf("scoring model has no available channel")
	}

	scoreCtx := c.Copy()
	scoreCtx.Request = c.Request.Clone(c.Request.Context())
	scoreCtx.Request.URL.Path = "/v1/chat/completions"
	scoreCtx.Request.URL.RawPath = ""
	scoreCtx.Request.URL.RawQuery = ""
	scoreCtx.Set("use_channel", []string{fmt.Sprintf("%d", channel.Id)})
	common.SetContextKey(scoreCtx, constant.ContextKeyAutoRouteScoring, true)
	common.SetContextKey(scoreCtx, constant.ContextKeyAutoRouteRequestedModel, constant.AutoRouteModelName)
	common.SetContextKey(scoreCtx, constant.ContextKeyAutoRouteScorerModel, scorerModel)
	if usingGroup == "auto" {
		common.SetContextKey(scoreCtx, constant.ContextKeyAutoGroup, group)
	}
	if newAPIError := SetupContextForSelectedChannel(scoreCtx, channel, scorerModel); newAPIError != nil {
		return autoRouteScoringResult{}, errors.New(newAPIError.Error())
	}

	stream := false
	temperature := 0.0
	scoreReq := &dto.GeneralOpenAIRequest{
		Model:       scorerModel,
		Stream:      &stream,
		Temperature: &temperature,
		MaxTokens:   common.GetPointer(autoRouteScoringMaxTokens),
		Messages: []dto.Message{
			{Role: "system", Content: autoRouteScoringSystemPrompt()},
			{Role: "user", Content: autoRouteScoringUserPrompt(features)},
		},
	}
	relayInfo := relaycommon.GenRelayInfoOpenAI(scoreCtx, scoreReq)
	relayInfo.InitRequestConversionChain()
	relayInfo.InitChannelMeta(scoreCtx)
	applyAutoRouteScoringThinkingParams(scoreReq, relayInfo.ApiType, relayInfo.ChannelType)
	if err := helper.ModelMappedHelper(scoreCtx, relayInfo, scoreReq); err != nil {
		return autoRouteScoringResult{}, err
	}

	meta := scoreReq.GetTokenCountMeta()
	tokens, err := service.EstimateRequestToken(scoreCtx, meta, relayInfo)
	if err != nil {
		return autoRouteScoringResult{}, err
	}
	relayInfo.SetEstimatePromptTokens(tokens)
	priceData, err := helper.ModelPriceHelper(scoreCtx, relayInfo, tokens, meta)
	if err != nil {
		return autoRouteScoringResult{}, err
	}
	if !priceData.FreeModel {
		if apiErr := service.PreConsumeBilling(scoreCtx, priceData.QuotaToPreConsume, relayInfo); apiErr != nil {
			return autoRouteScoringResult{}, errors.New(apiErr.Error())
		}
	}

	response, usage, err := doAutoRouteScoringRequest(scoreCtx, relayInfo, scoreReq)
	if err != nil {
		if relayInfo.Billing != nil {
			relayInfo.Billing.Refund(scoreCtx)
		}
		return autoRouteScoringResult{}, err
	}
	service.PostTextConsumeQuota(scoreCtx, relayInfo, usage, []string{"AFB Auto 路由打分"})

	result, err := parseAutoRouteScoringResult(response)
	if err != nil {
		return autoRouteScoringResult{}, err
	}
	result.Difficulty = clampDifficulty(result.Difficulty)
	return result, nil
}

func applyAutoRouteScoringThinkingParams(request *dto.GeneralOpenAIRequest, apiType int, channelType int) string {
	if request == nil {
		return "none"
	}

	switch channelType {
	case constant.ChannelTypeVolcEngine:
		request.THINKING = []byte(`{"type":"disabled"}`)
		return "volcengine_thinking_disabled"
	case constant.ChannelTypeAli:
		request.EnableThinking = []byte("false")
		return "ali_enable_thinking_false"
	case constant.ChannelTypeOllama:
		request.Think = []byte("false")
		return "ollama_think_false"
	case constant.ChannelTypeOpenRouter:
		request.Reasoning = []byte(`{"enabled":false,"exclude":true}`)
		return "openrouter_reasoning_disabled"
	case constant.ChannelTypeZhipu_v4:
		request.THINKING = []byte(`{"type":"disabled"}`)
		return "zhipu_v4_thinking_disabled"
	}

	switch apiType {
	case constant.APITypeVolcEngine:
		request.THINKING = []byte(`{"type":"disabled"}`)
		return "volcengine_thinking_disabled"
	case constant.APITypeAli:
		request.EnableThinking = []byte("false")
		return "ali_enable_thinking_false"
	case constant.APITypeOllama:
		request.Think = []byte("false")
		return "ollama_think_false"
	case constant.APITypeOpenRouter:
		request.Reasoning = []byte(`{"enabled":false,"exclude":true}`)
		return "openrouter_reasoning_disabled"
	case constant.APITypeZhipuV4:
		request.THINKING = []byte(`{"type":"disabled"}`)
		return "zhipu_v4_thinking_disabled"
	default:
		return "unsupported"
	}
}

func doAutoRouteScoringRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (string, *dto.Usage, error) {
	adaptor := relay.GetAdaptor(info.ApiType)
	if adaptor == nil {
		return "", nil, fmt.Errorf("invalid scoring model api type: %d", info.ApiType)
	}
	adaptor.Init(info)

	convertedRequest, err := adaptor.ConvertOpenAIRequest(c, info, request)
	if err != nil {
		return "", nil, err
	}
	relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)
	jsonData, err := common.Marshal(convertedRequest)
	if err != nil {
		return "", nil, err
	}
	jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
	if err != nil {
		return "", nil, err
	}
	if len(info.ParamOverride) > 0 {
		jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
		if err != nil {
			return "", nil, err
		}
	}

	reqCtx, cancel := context.WithTimeout(c.Request.Context(), autoRouteScorerTimeout)
	defer cancel()
	scoreCtx := c.Copy()
	scoreCtx.Request = c.Request.Clone(reqCtx)
	respAny, err := adaptor.DoRequest(scoreCtx, info, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", nil, err
	}
	httpResp, ok := respAny.(*http.Response)
	if !ok || httpResp == nil {
		return "", nil, fmt.Errorf("invalid scoring model response")
	}
	defer service.CloseResponseBodyGracefully(httpResp)
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(httpResp.Body, 1024))
		return "", nil, fmt.Errorf("scoring model status %d: %s", httpResp.StatusCode, strings.TrimSpace(string(body)))
	}
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return "", nil, err
	}

	content := ""
	usage := dto.Usage{}
	var simple dto.OpenAITextResponse
	if err := common.Unmarshal(body, &simple); err == nil {
		if oaiError := simple.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
			return "", nil, errors.New(oaiError.Message)
		}
		if len(simple.Choices) > 0 || simple.Usage.TotalTokens > 0 {
			for _, choice := range simple.Choices {
				if text := choice.Message.StringContent(); text != "" {
					content = text
					break
				}
			}
			usage = simple.Usage
		}
	}
	if content == "" {
		content = extractAutoRouteScoringTextFromBody(body)
	}
	if usage.PromptTokens == 0 && usage.TotalTokens == 0 {
		usage.PromptTokens = info.GetEstimatePromptTokens()
		usage.CompletionTokens = service.CountTextToken(content, info.UpstreamModelName)
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	return content, &usage, nil
}

func extractAutoRouteScoringTextFromBody(body []byte) string {
	var claude struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := common.Unmarshal(body, &claude); err == nil {
		for _, part := range claude.Content {
			if strings.TrimSpace(part.Text) != "" {
				return part.Text
			}
		}
	}

	var gemini struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := common.Unmarshal(body, &gemini); err == nil {
		for _, candidate := range gemini.Candidates {
			for _, part := range candidate.Content.Parts {
				if strings.TrimSpace(part.Text) != "" {
					return part.Text
				}
			}
		}
	}

	return string(body)
}

func parseAutoRouteScoringResult(content string) (autoRouteScoringResult, error) {
	jsonText := extractJSONObject(content)
	if jsonText == "" {
		return autoRouteScoringResult{}, fmt.Errorf("scoring model returned no JSON")
	}
	var fields map[string]interface{}
	if err := common.Unmarshal([]byte(jsonText), &fields); err != nil {
		return autoRouteScoringResult{}, err
	}
	if _, ok := fields["difficulty"]; !ok {
		return autoRouteScoringResult{}, fmt.Errorf("scoring model JSON missing difficulty")
	}
	var result autoRouteScoringResult
	if err := common.Unmarshal([]byte(jsonText), &result); err != nil {
		return autoRouteScoringResult{}, err
	}
	if result.Difficulty < 0 || result.Difficulty > 100 {
		result.Difficulty = clampDifficulty(result.Difficulty)
	}
	return result, nil
}

func extractJSONObject(content string) string {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start == -1 || end == -1 || end < start {
		return ""
	}
	return content[start : end+1]
}

func autoRouteScoringSystemPrompt() string {
	return "你是模型路由评分器，只评估请求难度，不答题。只输出 JSON：difficulty(0-100整数)、confidence(0-1)。简单闲聊/翻译/改写低分；代码排错、复杂分析、数学推理、多工具/跨上下文整合高分。"
}

func autoRouteScoringUserPrompt(features autoRouteFeatures) string {
	latestText := strings.TrimSpace(features.ScoringText)
	if latestText == "" {
		latestText = strings.TrimSpace(features.LatestText)
	}
	if latestText == "" {
		latestText = strings.TrimSpace(features.Text)
	}
	return fmt.Sprintf(
		"请为下面请求评分。\n历史轮数=%d\n估算总输入tokens=%d\n工具数=%d\n有图片=%t\nJSON Schema字节=%d\n评分输入：\n%s",
		features.HistoryTurns,
		features.PromptTokens,
		features.ToolsCount,
		features.HasImage,
		features.SchemaBytes,
		truncateRunes(latestText, autoRouteMaxScoringRunes),
	)
}

func truncateRunes(s string, limit int) string {
	if limit <= 0 || utf8.RuneCountInString(s) <= limit {
		return s
	}
	runes := []rune(s)
	return string(runes[:limit]) + "\n...[truncated]"
}

func estimateTextTokens(text string) int {
	if text == "" {
		return 0
	}
	return utf8.RuneCountInString(text)/4 + 1
}

func clampDifficulty(score int) int {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func containsAny(text string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
