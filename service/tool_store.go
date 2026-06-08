package service

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

const (
	toolMaxUploadSize = 5 * 1024 * 1024
)

var supportedToolCategories = map[string]bool{
	"商业": true,
	"工具": true,
	"开发": true,
	"媒体": true,
	"生活": true,
}

type ToolAppError struct {
	Code    string
	Message string
}

func (e *ToolAppError) Error() string {
	return e.Message
}

func NewToolAppError(code string, message string) *ToolAppError {
	return &ToolAppError{Code: code, Message: message}
}

type ToolSummary struct {
	ID             string `json:"id"`
	Slug           string `json:"slug"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Version        string `json:"version"`
	Type           string `json:"type"`
	AuthType       string `json:"auth_type"`
	ServerURL      string `json:"server_url"`
	ActionCount    int    `json:"action_count"`
	Status         string `json:"status"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
	DownloadCount  int64  `json:"download_count"`
	CreatedBy      int    `json:"created_by,omitempty"`
	CreatedByName  string `json:"created_by_name,omitempty"`
	Category       string `json:"category,omitempty"`
	Visibility     string `json:"visibility,omitempty"`
	SourceURL      string `json:"source_url,omitempty"`
	CallPrice      int    `json:"call_price,omitempty"`
	CurrentEarning int    `json:"current_earning"`
	Installed      bool   `json:"installed,omitempty"`
}

type ToolDetail struct {
	ToolSummary
	OpenAPIVersion string              `json:"openapi_version"`
	SourceFormat   string              `json:"source_format"`
	Actions        []ToolAction        `json:"actions"`
	Warnings       []ValidationWarning `json:"warnings"`
	Category       string              `json:"category,omitempty"`
	Visibility     string              `json:"visibility,omitempty"`
	APIKeyLocation string              `json:"api_key_location,omitempty"`
	APIKeyName     string              `json:"api_key_name,omitempty"`
	CommonHeaders  []ToolHeader        `json:"common_headers,omitempty"`
	CanEdit        bool                `json:"can_edit,omitempty"`
}

type ToolHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ToolAction struct {
	ID            string         `json:"id"`
	ToolID        string         `json:"tool_id"`
	Name          string         `json:"name"`
	DisplayName   string         `json:"display_name"`
	Description   string         `json:"description"`
	OperationID   string         `json:"operation_id"`
	Method        string         `json:"method"`
	Path          string         `json:"path"`
	InputSchema   map[string]any `json:"input_schema"`
	OutputSchema  any            `json:"output_schema,omitempty"`
	Enabled       bool           `json:"enabled"`
	RiskLevel     string         `json:"risk_level"`
	ParameterHint string         `json:"parameter_hint,omitempty"`
	ResponseHint  string         `json:"response_hint,omitempty"`
}

type ValidationWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

type ToolIndex struct {
	UpdatedAt int64         `json:"updated_at"`
	Tools     []ToolSummary `json:"tools"`
	Total     int           `json:"total"`
	Limit     int           `json:"limit"`
	Offset    int           `json:"offset"`
	HasMore   bool          `json:"has_more"`
}

type ToolListOptions struct {
	Keyword      string
	Category     string
	UserID       int
	CreatedBy    int
	AcquiredOnly bool
	Limit        int
	Offset       int
}

type ToolParseResult struct {
	Name           string              `json:"name"`
	Description    string              `json:"description"`
	Version        string              `json:"version"`
	ServerURL      string              `json:"server_url"`
	OpenAPIVersion string              `json:"openapi_version"`
	SourceFormat   string              `json:"source_format"`
	Actions        []ToolAction        `json:"actions"`
	Warnings       []ValidationWarning `json:"warnings"`
	OpenAPI        map[string]any      `json:"-"`
	Raw            []byte              `json:"-"`
	SourceExt      string              `json:"-"`
}

type ToolUploadOptions struct {
	Category       string
	Visibility     string
	Publish        bool
	AuthType       string
	APIKeyLocation string
	APIKeyName     string
	APIKeyValue    string
	CreatedBy      int
	CommonHeaders  []ToolHeader
	SourceURL      string
	CallPrice      int
}

type ToolManualCreateOptions struct {
	Name           string
	Description    string
	ServerURL      string
	Category       string
	Visibility     string
	Publish        bool
	AuthType       string
	APIKeyLocation string
	APIKeyName     string
	APIKeyValue    string
	CreatedBy      int
	CommonHeaders  []ToolHeader
	SourceURL      string
	CallPrice      int
	Action         ToolManualActionOptions
	Actions        []ToolManualActionOptions
}

type ToolManualActionOptions struct {
	DisplayName  string
	Description  string
	OperationID  string
	Method       string
	Path         string
	InputSchema  map[string]any
	OutputSchema any
	Enabled      bool
	RiskLevel    string
}

type ToolUpdateConfigOptions struct {
	UserID         int
	IsAdmin        bool
	Name           string
	Description    string
	ServerURL      string
	Category       string
	Visibility     string
	AuthType       string
	APIKeyLocation string
	APIKeyName     string
	APIKeyValue    string
	CommonHeaders  []ToolHeader
}

type ToolActionUpdateConfigOptions struct {
	UserID       int
	IsAdmin      bool
	DisplayName  string
	Description  string
	OperationID  string
	Method       string
	Path         string
	InputSchema  map[string]any
	OutputSchema any
	Enabled      bool
	RiskLevel    string
}

func ListTools(keyword string, category string) (ToolIndex, error) {
	return ListToolsWithOptions(ToolListOptions{Keyword: keyword, Category: category})
}

func toolSummaryFromModel(row model.Tool) ToolSummary {
	return ToolSummary{
		ID:             row.ID,
		Slug:           row.Slug,
		Name:           row.Name,
		Description:    row.Description,
		Version:        row.Version,
		Type:           row.Type,
		AuthType:       row.AuthType,
		ServerURL:      row.ServerURL,
		ActionCount:    row.ActionCount,
		Status:         row.Status,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		DownloadCount:  row.DownloadCount,
		CreatedBy:      row.CreatedBy,
		Category:       row.Category,
		Visibility:     row.Visibility,
		SourceURL:      row.SourceURL,
		CallPrice:      row.CallPrice,
		CurrentEarning: 0,
	}
}

func toolDetailFromModel(row model.Tool, actionRows []model.ToolAction) ToolDetail {
	actions := make([]ToolAction, 0, len(actionRows))
	for _, action := range actionRows {
		actions = append(actions, toolActionFromModel(action))
	}
	var headers []ToolHeader
	_ = json.Unmarshal([]byte(row.CommonHeaders), &headers)
	var warnings []ValidationWarning
	_ = json.Unmarshal([]byte(row.Warnings), &warnings)
	return ToolDetail{
		ToolSummary:    toolSummaryFromModel(row),
		OpenAPIVersion: row.OpenAPIVersion,
		SourceFormat:   row.SourceFormat,
		Actions:        actions,
		Warnings:       warnings,
		Category:       row.Category,
		Visibility:     row.Visibility,
		APIKeyLocation: row.APIKeyLocation,
		APIKeyName:     row.APIKeyName,
		CommonHeaders:  headers,
		CanEdit:        false,
	}
}

func toolActionFromModel(row model.ToolAction) ToolAction {
	inputSchema := map[string]any{}
	if row.InputSchema != "" {
		_ = json.Unmarshal([]byte(row.InputSchema), &inputSchema)
	}
	var outputSchema any
	if row.OutputSchema != "" {
		_ = json.Unmarshal([]byte(row.OutputSchema), &outputSchema)
	}
	return ToolAction{
		ID:            row.ActionID,
		ToolID:        row.ToolID,
		Name:          row.Name,
		DisplayName:   row.DisplayName,
		Description:   row.Description,
		OperationID:   row.OperationID,
		Method:        row.Method,
		Path:          row.Path,
		InputSchema:   inputSchema,
		OutputSchema:  outputSchema,
		Enabled:       row.Enabled,
		RiskLevel:     row.RiskLevel,
		ParameterHint: row.ParameterHint,
		ResponseHint:  row.ResponseHint,
	}
}

func mustJSONString(value any) string {
	if value == nil {
		return ""
	}
	content, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(content)
}

func saveToolDetailSQL(detail ToolDetail, parsed *ToolParseResult) error {
	if model.DB == nil {
		return NewToolAppError("tool_database_unavailable", "工具数据库不可用")
	}
	now := time.Now().Unix()
	if detail.CreatedAt == 0 {
		detail.CreatedAt = now
	}
	if detail.UpdatedAt == 0 {
		detail.UpdatedAt = now
	}
	openAPISpec := ""
	rawSpec := ""
	if parsed != nil {
		openAPISpec = mustJSONString(parsed.OpenAPI)
		rawSpec = string(parsed.Raw)
	}
	return model.DB.Transaction(func(tx *gorm.DB) error {
		if parsed == nil {
			var existing model.Tool
			if err := tx.Where("id = ?", detail.ID).First(&existing).Error; err == nil {
				openAPISpec = existing.OpenAPISpec
				rawSpec = existing.RawSpec
			}
		}
		row := model.Tool{
			ID:             detail.ID,
			Slug:           detail.Slug,
			Name:           detail.Name,
			Description:    detail.Description,
			Version:        detail.Version,
			Type:           detail.Type,
			AuthType:       detail.AuthType,
			ServerURL:      detail.ServerURL,
			ActionCount:    len(detail.Actions),
			Status:         detail.Status,
			DownloadCount:  detail.DownloadCount,
			CreatedBy:      detail.CreatedBy,
			Category:       detail.Category,
			Visibility:     detail.Visibility,
			SourceURL:      detail.SourceURL,
			CallPrice:      detail.CallPrice,
			SourceFormat:   detail.SourceFormat,
			OpenAPIVersion: detail.OpenAPIVersion,
			APIKeyLocation: detail.APIKeyLocation,
			APIKeyName:     detail.APIKeyName,
			CommonHeaders:  mustJSONString(detail.CommonHeaders),
			Warnings:       mustJSONString(detail.Warnings),
			OpenAPISpec:    openAPISpec,
			RawSpec:        rawSpec,
			CreatedAt:      detail.CreatedAt,
			UpdatedAt:      detail.UpdatedAt,
		}
		if err := tx.Save(&row).Error; err != nil {
			return err
		}
		if err := tx.Where("tool_id = ?", detail.ID).Delete(&model.ToolAction{}).Error; err != nil {
			return err
		}
		for i, action := range detail.Actions {
			actionID := strings.TrimSpace(action.ID)
			if actionID == "" {
				actionID = "action_" + action.Name
			}
			actionRow := model.ToolAction{
				ToolID:        detail.ID,
				ActionID:      actionID,
				Name:          action.Name,
				DisplayName:   action.DisplayName,
				Description:   action.Description,
				OperationID:   action.OperationID,
				Method:        action.Method,
				Path:          action.Path,
				InputSchema:   mustJSONString(action.InputSchema),
				OutputSchema:  mustJSONString(action.OutputSchema),
				Enabled:       action.Enabled,
				RiskLevel:     action.RiskLevel,
				ParameterHint: action.ParameterHint,
				ResponseHint:  action.ResponseHint,
				SortOrder:     i,
			}
			if err := tx.Create(&actionRow).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func ListToolsWithOptions(opts ToolListOptions) (ToolIndex, error) {
	keyword := strings.ToLower(strings.TrimSpace(opts.Keyword))
	category := strings.ToLower(strings.TrimSpace(opts.Category))
	limit := normalizeListLimit(opts.Limit)
	offset := normalizeListOffset(opts.Offset)
	installed := map[string]bool{}
	if opts.UserID > 0 {
		var installedErr error
		installed, installedErr = GetInstalledToolIDs(opts.UserID)
		if installedErr != nil {
			return ToolIndex{}, installedErr
		}
	}
	if model.DB == nil {
		return ToolIndex{}, NewToolAppError("tool_database_unavailable", "工具数据库不可用")
	}
	query := model.DB.Model(&model.Tool{})
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("LOWER(name) LIKE ? OR LOWER(description) LIKE ?", like, like)
	}
	if category != "" {
		query = query.Where("LOWER(category) = ?", category)
	}
	if opts.CreatedBy != 0 {
		query = query.Where("created_by = ?", opts.CreatedBy)
	} else {
		query = query.Where("status = ? AND visibility = ?", "published", "public")
	}
	var rows []model.Tool
	if err := query.Order("updated_at desc, created_at desc").Find(&rows).Error; err != nil {
		return ToolIndex{}, err
	}
	filtered := make([]ToolSummary, 0, len(rows))
	for _, row := range rows {
		tool := toolSummaryFromModel(row)
		tool.Installed = installed[tool.ID]
		if opts.AcquiredOnly && !tool.Installed {
			continue
		}
		filtered = append(filtered, tool)
	}
	if err := enrichToolEarnings(filtered); err != nil {
		return ToolIndex{}, err
	}
	total := len(filtered)
	end := offset + limit
	if offset > total {
		offset = total
	}
	if end > total {
		end = total
	}
	index := ToolIndex{
		UpdatedAt: time.Now().Unix(),
		Tools:     filtered[offset:end],
		Total:     total,
		Limit:     limit,
		Offset:    offset,
		HasMore:   end < total,
	}
	enrichToolCreatorNames(index.Tools)
	return index, nil
}

func GetToolDetail(toolID string) (ToolDetail, error) {
	toolID = sanitizeID(toolID)
	if toolID == "" {
		return ToolDetail{}, NewToolAppError("invalid_request", "工具 ID 无效")
	}
	if model.DB == nil {
		return ToolDetail{}, NewToolAppError("tool_database_unavailable", "工具数据库不可用")
	}
	var tool model.Tool
	if err := model.DB.Where("id = ?", toolID).First(&tool).Error; err != nil {
		return ToolDetail{}, NewToolAppError("tool_not_found", "工具不存在")
	}
	var actions []model.ToolAction
	if err := model.DB.Where("tool_id = ?", toolID).Order("sort_order asc, id asc").Find(&actions).Error; err != nil {
		return ToolDetail{}, err
	}
	return toolDetailFromModel(tool, actions), nil
}

func ParseOpenAPIUpload(filename string, reader io.Reader, size int64) (ToolParseResult, error) {
	sourceFormat, sourceExt, err := detectUploadFormat(filename)
	if err != nil {
		return ToolParseResult{}, err
	}
	if size > toolMaxUploadSize {
		return ToolParseResult{}, NewToolAppError("tool_file_too_large", "文件不能超过 5MB")
	}
	raw, err := io.ReadAll(io.LimitReader(reader, toolMaxUploadSize+1))
	if err != nil {
		return ToolParseResult{}, NewToolAppError("invalid_request", "读取上传文件失败")
	}
	if len(raw) == 0 {
		return ToolParseResult{}, NewToolAppError("invalid_openapi", "文件为空")
	}
	if len(raw) > toolMaxUploadSize {
		return ToolParseResult{}, NewToolAppError("tool_file_too_large", "文件不能超过 5MB")
	}
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	openapi, err := parseOpenAPIBytes(sourceFormat, raw)
	if err != nil {
		return ToolParseResult{}, err
	}
	result, err := validateAndBuildTool(openapi)
	if err != nil {
		return ToolParseResult{}, err
	}
	result.SourceFormat = sourceFormat
	result.SourceExt = sourceExt
	result.Raw = raw
	result.OpenAPI = openapi
	return result, nil
}

func UploadTool(filename string, reader io.Reader, size int64, opts ToolUploadOptions) (ToolDetail, error) {
	parsed, err := ParseOpenAPIUpload(filename, reader, size)
	if err != nil {
		return ToolDetail{}, err
	}
	slug := slugify(parsed.Name)
	if slug == "" {
		slug = "tool"
	}
	if hasToolConflictSQL(parsed.Name, slug, "") {
		return ToolDetail{}, NewToolAppError("tool_name_conflict", "工具名称或 slug 已存在")
	}
	now := time.Now().Unix()
	toolID := "tool_" + slug + "_" + randomHex(4)
	status := "published"
	if !opts.Publish {
		status = "draft"
	}
	authType := normalizeAuthType(opts.AuthType)
	visibility := normalizeVisibility(opts.Visibility)
	category, err := normalizeToolCategory(opts.Category)
	if err != nil {
		return ToolDetail{}, err
	}
	for i := range parsed.Actions {
		parsed.Actions[i].ToolID = toolID
		parsed.Actions[i].ID = "action_" + parsed.Actions[i].Name
	}
	detail := ToolDetail{
		ToolSummary: ToolSummary{
			ID:            toolID,
			Slug:          slug,
			Name:          parsed.Name,
			Description:   parsed.Description,
			Version:       parsed.Version,
			Type:          "openapi",
			AuthType:      authType,
			ServerURL:     parsed.ServerURL,
			ActionCount:   len(parsed.Actions),
			Status:        status,
			CreatedAt:     now,
			UpdatedAt:     now,
			DownloadCount: 0,
			CreatedBy:     opts.CreatedBy,
			Category:      category,
			Visibility:    normalizeVisibility(opts.Visibility),
			SourceURL:     strings.TrimSpace(opts.SourceURL),
			CallPrice:     normalizeToolCallPrice(opts.CallPrice),
		},
		OpenAPIVersion: parsed.OpenAPIVersion,
		SourceFormat:   parsed.SourceFormat,
		Actions:        parsed.Actions,
		Warnings:       parsed.Warnings,
		Category:       category,
		Visibility:     visibility,
		APIKeyLocation: normalizeAPIKeyLocation(opts.APIKeyLocation),
		APIKeyName:     strings.TrimSpace(opts.APIKeyName),
		CommonHeaders:  normalizeToolHeaders(opts.CommonHeaders),
	}
	if err := persistTool(detail, parsed); err != nil {
		return ToolDetail{}, err
	}
	if detail.AuthType == "api_key" && strings.TrimSpace(opts.APIKeyValue) != "" {
		if err := persistToolSecret(toolID, detail.APIKeyLocation, detail.APIKeyName, opts.APIKeyValue); err != nil {
			return ToolDetail{}, err
		}
	}
	return detail, nil
}

func CreateManualTool(opts ToolManualCreateOptions) (ToolDetail, error) {
	name := strings.TrimSpace(opts.Name)
	if name == "" {
		return ToolDetail{}, NewToolAppError("invalid_request", "工具名称不能为空")
	}
	description := strings.TrimSpace(opts.Description)
	if description == "" {
		return ToolDetail{}, NewToolAppError("invalid_request", "工具描述不能为空")
	}
	serverURL := strings.TrimSpace(opts.ServerURL)
	if err := validateServerURL(serverURL); err != nil {
		return ToolDetail{}, err
	}
	manualActions := opts.Actions
	if len(manualActions) == 0 {
		manualActions = []ToolManualActionOptions{opts.Action}
	}
	operationIDs := map[string]bool{}
	functionNames := map[string]bool{}
	methodPaths := map[string]bool{}
	for i := range manualActions {
		displayName := strings.TrimSpace(manualActions[i].DisplayName)
		if displayName == "" {
			return ToolDetail{}, NewToolAppError("invalid_request", "Action 名称不能为空")
		}
		actionDescription := strings.TrimSpace(manualActions[i].Description)
		if actionDescription == "" {
			actionDescription = displayName
		}
		operationID := strings.TrimSpace(manualActions[i].OperationID)
		if operationID == "" {
			operationID = displayName
		}
		functionName := normalizeFunctionName(operationID)
		if functionName == "" {
			return ToolDetail{}, NewToolAppError("invalid_request", "operationId 无法转换为可用 function name")
		}
		if operationIDs[operationID] || functionNames[functionName] {
			return ToolDetail{}, NewToolAppError("tool_name_conflict", "同一工具内 operationId/function name 不允许重复")
		}
		method := normalizeToolActionMethod(manualActions[i].Method)
		if method == "" {
			return ToolDetail{}, NewToolAppError("invalid_request", "HTTP Method 无效")
		}
		actionPath := strings.TrimSpace(manualActions[i].Path)
		if !strings.HasPrefix(actionPath, "/") {
			return ToolDetail{}, NewToolAppError("invalid_request", "请求路径必须以 / 开头")
		}
		methodPathKey := method + " " + actionPath
		if methodPaths[methodPathKey] {
			return ToolDetail{}, NewToolAppError("tool_name_conflict", "同一工具内 Method + Path 不允许重复")
		}
		outputSchema := manualActions[i].OutputSchema
		if outputSchema == nil {
			outputSchema = map[string]any{"type": "object"}
		}
		manualActions[i].DisplayName = displayName
		manualActions[i].Description = actionDescription
		manualActions[i].OperationID = operationID
		manualActions[i].Method = method
		manualActions[i].Path = actionPath
		manualActions[i].InputSchema = normalizeToolActionInputSchema(manualActions[i].InputSchema)
		manualActions[i].OutputSchema = outputSchema
		operationIDs[operationID] = true
		functionNames[functionName] = true
		methodPaths[methodPathKey] = true
	}

	slug := slugify(name)
	if slug == "" {
		slug = "tool"
	}
	if hasToolConflictSQL(name, slug, "") {
		return ToolDetail{}, NewToolAppError("tool_name_conflict", "工具名称或 slug 已存在")
	}
	category, err := normalizeToolCategory(opts.Category)
	if err != nil {
		return ToolDetail{}, err
	}
	authType := normalizeAuthType(opts.AuthType)
	apiKeyLocation := normalizeAPIKeyLocation(opts.APIKeyLocation)
	apiKeyName := strings.TrimSpace(opts.APIKeyName)
	if authType == "api_key" && apiKeyName == "" {
		return ToolDetail{}, NewToolAppError("invalid_request", "API Key 参数名不能为空")
	}

	openAPI := buildManualOpenAPI(name, description, serverURL, manualActions)
	parsed, err := validateAndBuildTool(openAPI)
	if err != nil {
		return ToolDetail{}, err
	}
	raw, err := json.MarshalIndent(openAPI, "", "  ")
	if err != nil {
		return ToolDetail{}, NewToolAppError("storage_write_failed", "序列化工具数据失败")
	}
	parsed.SourceFormat = "json"
	parsed.SourceExt = ".json"
	parsed.Raw = raw
	parsed.OpenAPI = openAPI

	now := time.Now().Unix()
	toolID := "tool_" + slug + "_" + randomHex(4)
	status := "published"
	if !opts.Publish {
		status = "draft"
	}
	for i := range parsed.Actions {
		parsed.Actions[i].ToolID = toolID
		parsed.Actions[i].ID = "action_" + parsed.Actions[i].Name
		for _, manualAction := range manualActions {
			if parsed.Actions[i].OperationID == manualAction.OperationID {
				parsed.Actions[i].Enabled = manualAction.Enabled
				parsed.Actions[i].RiskLevel = normalizeRiskLevel(manualAction.RiskLevel, manualAction.Method)
				break
			}
		}
	}
	detail := ToolDetail{
		ToolSummary: ToolSummary{
			ID:            toolID,
			Slug:          slug,
			Name:          parsed.Name,
			Description:   parsed.Description,
			Version:       parsed.Version,
			Type:          "openapi",
			AuthType:      authType,
			ServerURL:     parsed.ServerURL,
			ActionCount:   len(parsed.Actions),
			Status:        status,
			CreatedAt:     now,
			UpdatedAt:     now,
			DownloadCount: 0,
			CreatedBy:     opts.CreatedBy,
			Category:      category,
			Visibility:    normalizeVisibility(opts.Visibility),
			SourceURL:     strings.TrimSpace(opts.SourceURL),
			CallPrice:     normalizeToolCallPrice(opts.CallPrice),
		},
		OpenAPIVersion: parsed.OpenAPIVersion,
		SourceFormat:   parsed.SourceFormat,
		Actions:        parsed.Actions,
		Warnings:       parsed.Warnings,
		Category:       category,
		Visibility:     normalizeVisibility(opts.Visibility),
		APIKeyLocation: apiKeyLocation,
		APIKeyName:     apiKeyName,
		CommonHeaders:  normalizeToolHeaders(opts.CommonHeaders),
	}
	if err := persistTool(detail, parsed); err != nil {
		return ToolDetail{}, err
	}
	if detail.AuthType == "api_key" && strings.TrimSpace(opts.APIKeyValue) != "" {
		if err := persistToolSecret(toolID, detail.APIKeyLocation, detail.APIKeyName, opts.APIKeyValue); err != nil {
			return ToolDetail{}, err
		}
	}
	return detail, nil
}

func UpdateToolConfig(toolID string, opts ToolUpdateConfigOptions) (ToolDetail, error) {
	toolID = sanitizeID(toolID)
	if toolID == "" {
		return ToolDetail{}, NewToolAppError("invalid_request", "工具 ID 无效")
	}
	detail, err := GetToolDetail(toolID)
	if err != nil {
		return ToolDetail{}, err
	}
	if !canEditTool(detail, opts.UserID, opts.IsAdmin) {
		return ToolDetail{}, NewToolAppError("permission_denied", "只能编辑自己发布的工具")
	}
	name := strings.TrimSpace(opts.Name)
	if name == "" {
		return ToolDetail{}, NewToolAppError("invalid_request", "工具名称不能为空")
	}
	description := strings.TrimSpace(opts.Description)
	if description == "" {
		return ToolDetail{}, NewToolAppError("invalid_request", "工具描述不能为空")
	}
	serverURL := strings.TrimSpace(opts.ServerURL)
	if err := validateToolServerURL(serverURL); err != nil {
		return ToolDetail{}, err
	}
	authType := normalizeAuthType(opts.AuthType)
	apiKeyLocation := normalizeAPIKeyLocation(opts.APIKeyLocation)
	apiKeyName := strings.TrimSpace(opts.APIKeyName)
	if authType == "api_key" && apiKeyName == "" {
		return ToolDetail{}, NewToolAppError("invalid_request", "API Key 参数名不能为空")
	}
	category, err := normalizeToolCategory(opts.Category)
	if err != nil {
		return ToolDetail{}, err
	}
	now := time.Now().Unix()
	detail.Name = name
	detail.Description = description
	detail.ServerURL = serverURL
	detail.Category = category
	detail.Visibility = normalizeVisibility(opts.Visibility)
	detail.ToolSummary.Category = detail.Category
	detail.ToolSummary.Visibility = detail.Visibility
	detail.AuthType = authType
	detail.APIKeyLocation = apiKeyLocation
	detail.APIKeyName = apiKeyName
	detail.CommonHeaders = normalizeToolHeaders(opts.CommonHeaders)
	detail.UpdatedAt = now

	if authType == "api_key" {
		if strings.TrimSpace(opts.APIKeyValue) != "" {
			if err := persistToolSecret(toolID, apiKeyLocation, apiKeyName, opts.APIKeyValue); err != nil {
				return ToolDetail{}, err
			}
		}
	} else {
		_ = model.DB.Where("tool_id = ?", toolID).Delete(&model.ToolSecret{}).Error
		detail.APIKeyLocation = ""
		detail.APIKeyName = ""
	}

	if err := saveToolDetailSQL(detail, nil); err != nil {
		return ToolDetail{}, err
	}
	return detail, nil
}

func UpdateToolActionConfig(toolID string, actionID string, opts ToolActionUpdateConfigOptions) (ToolDetail, error) {
	toolID = sanitizeID(toolID)
	actionID = strings.TrimSpace(actionID)
	if toolID == "" || actionID == "" {
		return ToolDetail{}, NewToolAppError("invalid_request", "工具或函数 ID 无效")
	}
	detail, err := GetToolDetail(toolID)
	if err != nil {
		return ToolDetail{}, err
	}
	if !canEditTool(detail, opts.UserID, opts.IsAdmin) {
		return ToolDetail{}, NewToolAppError("permission_denied", "只能编辑自己发布的工具")
	}
	actionIndex := -1
	for i, action := range detail.Actions {
		if action.ID == actionID || action.OperationID == actionID || action.Name == actionID {
			actionIndex = i
			break
		}
	}
	if actionIndex < 0 {
		return ToolDetail{}, NewToolAppError("tool_action_not_found", "工具函数不存在")
	}

	displayName := strings.TrimSpace(opts.DisplayName)
	if displayName == "" {
		return ToolDetail{}, NewToolAppError("invalid_request", "函数名称不能为空")
	}
	description := strings.TrimSpace(opts.Description)
	if description == "" {
		description = displayName
	}
	operationID := strings.TrimSpace(opts.OperationID)
	if operationID == "" {
		return ToolDetail{}, NewToolAppError("invalid_request", "operationId 不能为空")
	}
	functionName := normalizeFunctionName(operationID)
	if functionName == "" {
		return ToolDetail{}, NewToolAppError("invalid_request", "operationId 无法转换为可用 function name")
	}
	method := normalizeToolActionMethod(opts.Method)
	if method == "" {
		return ToolDetail{}, NewToolAppError("invalid_request", "HTTP Method 无效")
	}
	actionPath := strings.TrimSpace(opts.Path)
	if !strings.HasPrefix(actionPath, "/") {
		return ToolDetail{}, NewToolAppError("invalid_request", "请求路径必须以 / 开头")
	}
	inputSchema := normalizeToolActionInputSchema(opts.InputSchema)
	outputSchema := opts.OutputSchema
	if outputSchema == nil {
		outputSchema = detail.Actions[actionIndex].OutputSchema
	}

	for i, action := range detail.Actions {
		if i == actionIndex {
			continue
		}
		if action.OperationID == operationID || action.Name == functionName {
			return ToolDetail{}, NewToolAppError("tool_name_conflict", "同一工具内 operationId/function name 不允许重复")
		}
		if strings.EqualFold(action.Method, method) && action.Path == actionPath {
			return ToolDetail{}, NewToolAppError("tool_name_conflict", "同一工具内 Method + Path 不允许重复")
		}
	}

	oldAction := detail.Actions[actionIndex]
	nextAction := oldAction
	nextAction.DisplayName = displayName
	nextAction.Description = description
	nextAction.OperationID = operationID
	nextAction.Name = functionName
	nextAction.Method = method
	nextAction.Path = actionPath
	nextAction.InputSchema = inputSchema
	nextAction.OutputSchema = outputSchema
	nextAction.Enabled = opts.Enabled
	nextAction.RiskLevel = normalizeRiskLevel(opts.RiskLevel, method)
	detail.Actions[actionIndex] = nextAction
	detail.UpdatedAt = time.Now().Unix()
	detail.ToolSummary.UpdatedAt = detail.UpdatedAt

	if err := saveToolDetailSQL(detail, nil); err != nil {
		return ToolDetail{}, err
	}
	return detail, nil
}

func CheckToolName(name string) (map[string]any, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, NewToolAppError("invalid_request", "工具名称不能为空")
	}
	slug := slugify(name)
	available := !hasToolConflictSQL(name, slug, "")
	return map[string]any{
		"available":      available,
		"reason":         map[bool]string{true: "", false: "tool_name_conflict"}[available],
		"suggested_slug": suggestSlugSQL(slug),
	}, nil
}

func DeleteTool(toolID string) error {
	toolID = sanitizeID(toolID)
	if toolID == "" {
		return NewToolAppError("invalid_request", "工具 ID 无效")
	}
	if model.DB == nil {
		return NewToolAppError("tool_database_unavailable", "工具数据库不可用")
	}
	result := model.DB.Where("id = ?", toolID).Delete(&model.Tool{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return NewToolAppError("tool_not_found", "工具不存在")
	}
	_ = model.DB.Where("tool_id = ?", toolID).Delete(&model.ToolAction{}).Error
	_ = model.DB.Where("tool_id = ?", toolID).Delete(&model.ToolSecret{}).Error
	_ = model.DB.Where("tool_id = ?", toolID).Delete(&model.UserTool{}).Error
	return nil
}

func BuildToolDownload(toolID string) (string, error) {
	detail, err := GetToolDetail(toolID)
	if err != nil {
		return "", err
	}
	downloadDir := filepath.Join(os.TempDir(), "new-api-tool-downloads", sanitizeID(toolID))
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		return "", NewToolAppError("storage_write_failed", "创建下载目录失败")
	}
	zipPath := filepath.Join(downloadDir, fmt.Sprintf("tool-%s.zip", toolID))
	file, err := os.Create(zipPath)
	if err != nil {
		return "", NewToolAppError("storage_write_failed", "创建下载包失败")
	}
	zipWriter := zip.NewWriter(file)
	writeErr := addToolZipFiles(zipWriter, detail)
	closeZipErr := zipWriter.Close()
	closeFileErr := file.Close()
	if writeErr != nil {
		return "", writeErr
	}
	if closeZipErr != nil || closeFileErr != nil {
		return "", NewToolAppError("storage_write_failed", "写入下载包失败")
	}
	if err := incrementDownloadCount(toolID); err != nil {
		return "", err
	}
	return zipPath, nil
}

func addToolZipFiles(zipWriter *zip.Writer, detail ToolDetail) error {
	var row model.Tool
	if model.DB != nil {
		_ = model.DB.Where("id = ?", detail.ID).First(&row).Error
	}
	toolJSON, err := json.MarshalIndent(detail, "", "  ")
	if err != nil {
		return NewToolAppError("storage_write_failed", "序列化工具数据失败")
	}
	actionsJSON, err := json.MarshalIndent(detail.Actions, "", "  ")
	if err != nil {
		return NewToolAppError("storage_write_failed", "序列化工具函数失败")
	}
	if err := addZipFile(zipWriter, "tool.json", toolJSON); err != nil {
		return err
	}
	if err := addZipFile(zipWriter, "actions.json", actionsJSON); err != nil {
		return err
	}
	if row.OpenAPISpec != "" {
		if err := addZipFile(zipWriter, "openapi.json", []byte(row.OpenAPISpec)); err != nil {
			return err
		}
	}
	if row.RawSpec != "" {
		sourceName := "source.openapi.json"
		if detail.SourceFormat == "yaml" {
			sourceName = "source.openapi.yaml"
		}
		if err := addZipFile(zipWriter, sourceName, []byte(row.RawSpec)); err != nil {
			return err
		}
	}
	if detail.SourceURL != "" {
		if err := addZipFile(zipWriter, "source_url.txt", []byte(detail.SourceURL+"\n")); err != nil {
			return err
		}
	}
	readme := []byte("# OpenAPI 工具定义包\n\n本包包含工具元信息、规范化 OpenAPI、原始 OpenAPI 文件和 Action 列表，不包含 API Key 或密钥。\n")
	return addZipFile(zipWriter, "README.md", readme)
}

func addZipFile(zipWriter *zip.Writer, name string, content []byte) error {
	writer, err := zipWriter.Create(name)
	if err != nil {
		return NewToolAppError("storage_write_failed", "写入下载包失败")
	}
	if _, err := writer.Write(content); err != nil {
		return NewToolAppError("storage_write_failed", "写入下载包失败")
	}
	return nil
}

func persistTool(detail ToolDetail, parsed ToolParseResult) error {
	return saveToolDetailSQL(detail, &parsed)
}

func persistToolSecret(toolID string, location string, name string, value string) error {
	if model.DB == nil {
		return NewToolAppError("tool_database_unavailable", "工具数据库不可用")
	}
	return model.DB.Save(&model.ToolSecret{
		ToolID:         toolID,
		APIKeyLocation: location,
		APIKeyName:     name,
		APIKeyValue:    value,
	}).Error
}

func incrementDownloadCount(toolID string) error {
	if model.DB == nil {
		return NewToolAppError("tool_database_unavailable", "工具数据库不可用")
	}
	return model.DB.Model(&model.Tool{}).Where("id = ?", toolID).Updates(map[string]interface{}{
		"download_count": gorm.Expr("download_count + ?", 1),
		"updated_at":     time.Now().Unix(),
	}).Error
}

func enrichToolCreatorNames(tools []ToolSummary) {
	if model.DB == nil {
		return
	}
	ids := make([]int, 0)
	seen := map[int]bool{}
	for _, tool := range tools {
		if tool.CreatedBy > 0 && !seen[tool.CreatedBy] {
			seen[tool.CreatedBy] = true
			ids = append(ids, tool.CreatedBy)
		}
	}
	if len(ids) == 0 {
		return
	}
	var users []model.User
	if err := model.DB.Select("id, username, phone").Where("id IN ?", ids).Find(&users).Error; err != nil {
		return
	}
	names := map[int]string{}
	for _, user := range users {
		names[user.Id] = normalizeIncomeUserName(user.Id, strings.TrimSpace(user.Username), user.Phone)
	}
	for i := range tools {
		if name := names[tools[i].CreatedBy]; name != "" {
			tools[i].CreatedByName = name
		}
	}
}

func enrichToolEarnings(tools []ToolSummary) error {
	if len(tools) == 0 {
		return nil
	}
	if err := ensureToolDB(); err != nil {
		return err
	}
	ids := make([]string, 0, len(tools))
	for _, tool := range tools {
		if tool.ID != "" {
			ids = append(ids, tool.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	var rows []struct {
		ToolID string
		Quota  int
	}
	if err := model.ToolDB.Model(&model.ToolRun{}).
		Select("tool_id, COALESCE(SUM(reward_quota), 0) AS quota").
		Where("tool_id IN ? AND billing_status = ? AND reward_quota > 0", ids, "charged").
		Group("tool_id").
		Scan(&rows).Error; err != nil {
		return err
	}
	earnings := map[string]int{}
	for _, row := range rows {
		earnings[row.ToolID] = row.Quota
	}
	for i := range tools {
		tools[i].CurrentEarning = earnings[tools[i].ID]
	}
	return nil
}

func CanEditTool(detail ToolDetail, userID int, isAdmin bool) bool {
	return canEditTool(detail, userID, isAdmin)
}

func canEditTool(detail ToolDetail, userID int, isAdmin bool) bool {
	if isAdmin {
		return true
	}
	if userID <= 0 {
		return false
	}
	return detail.CreatedBy == 0 || detail.CreatedBy == userID
}

func validateToolServerURL(value string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return NewToolAppError("invalid_request", "工具 URL 无效")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return NewToolAppError("invalid_request", "工具 URL 仅支持 http/https")
	}
	return nil
}

func validateAndBuildTool(openapi map[string]any) (ToolParseResult, error) {
	version, _ := openapi["openapi"].(string)
	if version == "" {
		if _, ok := openapi["swagger"]; ok {
			return ToolParseResult{}, NewToolAppError("openapi_version_not_supported", "不支持 OpenAPI 2.0 / Swagger 2.0")
		}
		return ToolParseResult{}, NewToolAppError("invalid_openapi", "OpenAPI 缺少 openapi 版本")
	}
	if !strings.HasPrefix(version, "3.") {
		return ToolParseResult{}, NewToolAppError("openapi_version_not_supported", "只支持 OpenAPI 3.x")
	}
	info, ok := asMap(openapi["info"])
	if !ok {
		return ToolParseResult{}, NewToolAppError("invalid_openapi", "OpenAPI 缺少 info")
	}
	title := strings.TrimSpace(asString(info["title"]))
	if title == "" {
		return ToolParseResult{}, NewToolAppError("invalid_openapi", "OpenAPI 缺少 info.title")
	}
	description := strings.TrimSpace(asString(info["description"]))
	if description == "" {
		return ToolParseResult{}, NewToolAppError("invalid_openapi", "OpenAPI 缺少 info.description")
	}
	toolVersion := strings.TrimSpace(asString(info["version"]))
	if toolVersion == "" {
		toolVersion = "1.0.0"
	}
	servers, ok := openapi["servers"].([]any)
	if !ok || len(servers) == 0 {
		return ToolParseResult{}, NewToolAppError("invalid_openapi", "OpenAPI 缺少 servers")
	}
	firstServer, ok := asMap(servers[0])
	if !ok {
		return ToolParseResult{}, NewToolAppError("invalid_openapi", "OpenAPI server 格式无效")
	}
	serverURL := strings.TrimSpace(asString(firstServer["url"]))
	if err := validateServerURL(serverURL); err != nil {
		return ToolParseResult{}, err
	}
	paths, ok := asMap(openapi["paths"])
	if !ok || len(paths) == 0 {
		return ToolParseResult{}, NewToolAppError("invalid_openapi", "OpenAPI 缺少 paths")
	}
	warnings := make([]ValidationWarning, 0)
	if len(servers) > 1 {
		warnings = append(warnings, ValidationWarning{Code: "multiple_servers", Message: "存在多个 servers，当前仅使用第一个"})
	}
	actions, err := buildActions(paths, &warnings)
	if err != nil {
		return ToolParseResult{}, err
	}
	return ToolParseResult{
		Name:           title,
		Description:    description,
		Version:        toolVersion,
		ServerURL:      serverURL,
		OpenAPIVersion: version,
		Actions:        actions,
		Warnings:       warnings,
	}, nil
}

var supportedMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true,
}

func buildActions(paths map[string]any, warnings *[]ValidationWarning) ([]ToolAction, error) {
	actions := make([]ToolAction, 0)
	operationIDs := map[string]bool{}
	functionNames := map[string]bool{}
	for path, rawPathItem := range paths {
		pathItem, ok := asMap(rawPathItem)
		if !ok {
			continue
		}
		pathParameters := asArray(pathItem["parameters"])
		for methodKey, rawOperation := range pathItem {
			method := strings.ToUpper(methodKey)
			if !supportedMethods[method] {
				continue
			}
			operation, ok := asMap(rawOperation)
			if !ok {
				return nil, NewToolAppError("invalid_openapi", fmt.Sprintf("%s %s operation 格式无效", method, path))
			}
			operationID := strings.TrimSpace(asString(operation["operationId"]))
			if operationID == "" {
				return nil, NewToolAppError("invalid_openapi", fmt.Sprintf("%s %s 缺少 operationId", method, path))
			}
			if operationIDs[operationID] {
				return nil, NewToolAppError("tool_name_conflict", "同一工具内 operationId 不允许重复")
			}
			operationIDs[operationID] = true
			functionName := normalizeFunctionName(operationID)
			if functionName == "" {
				return nil, NewToolAppError("invalid_openapi", "operationId 无法转换为可用 function name")
			}
			if functionNames[functionName] {
				return nil, NewToolAppError("tool_name_conflict", "同一工具内 function name 不允许重复")
			}
			functionNames[functionName] = true
			summary := strings.TrimSpace(asString(operation["summary"]))
			desc := strings.TrimSpace(asString(operation["description"]))
			if summary == "" && desc == "" {
				return nil, NewToolAppError("invalid_openapi", fmt.Sprintf("%s %s 缺少 summary 或 description", method, path))
			}
			if len([]rune(desc)) > 0 && len([]rune(desc)) < 8 {
				*warnings = append(*warnings, ValidationWarning{Code: "description_too_short", Message: "Action 描述较短", Path: path})
			}
			if summary == "" {
				summary = desc
			}
			if desc == "" {
				desc = summary
			}
			parameters := append([]any{}, pathParameters...)
			parameters = append(parameters, asArray(operation["parameters"])...)
			inputSchema := buildInputSchema(parameters, operation["requestBody"], warnings, path)
			outputSchema := pickOutputSchema(operation["responses"])
			if outputSchema == nil {
				*warnings = append(*warnings, ValidationWarning{Code: "response_schema_missing", Message: "未找到 2xx response schema", Path: path})
			}
			actions = append(actions, ToolAction{
				Name:         functionName,
				DisplayName:  summary,
				Description:  desc,
				OperationID:  operationID,
				Method:       method,
				Path:         path,
				InputSchema:  inputSchema,
				OutputSchema: outputSchema,
				Enabled:      true,
				RiskLevel:    riskLevel(method),
			})
		}
	}
	if len(actions) == 0 {
		return nil, NewToolAppError("invalid_openapi", "OpenAPI paths 中没有可用 Action")
	}
	sort.SliceStable(actions, func(i, j int) bool {
		if actions[i].Path == actions[j].Path {
			return actions[i].Method < actions[j].Method
		}
		return actions[i].Path < actions[j].Path
	})
	return actions, nil
}

func buildInputSchema(parameters []any, requestBody any, warnings *[]ValidationWarning, path string) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": map[string]any{},
		"required":   []string{},
	}
	props := schema["properties"].(map[string]any)
	required := make([]string, 0)
	for _, rawParameter := range parameters {
		parameter, ok := asMap(rawParameter)
		if !ok {
			continue
		}
		name := strings.TrimSpace(asString(parameter["name"]))
		if name == "" {
			continue
		}
		paramSchema, ok := asMap(parameter["schema"])
		if !ok {
			paramSchema = map[string]any{"type": "string"}
		}
		prop := cloneMap(paramSchema)
		prop["description"] = strings.TrimSpace(asString(parameter["description"]))
		prop["x-openapi-in"] = strings.TrimSpace(asString(parameter["in"]))
		props[name] = prop
		if requiredFlag, ok := parameter["required"].(bool); ok && requiredFlag {
			required = append(required, name)
		}
	}
	bodySchema := extractJSONSchemaFromContent(requestBody, "content")
	if bodySchema != nil {
		if bodyMap, ok := asMap(bodySchema); ok {
			if bodyMap["type"] != nil && bodyMap["type"] != "object" {
				*warnings = append(*warnings, ValidationWarning{Code: "request_body_not_object", Message: "requestBody 不是 object schema", Path: path})
			}
			props["body"] = bodyMap
			required = append(required, "body")
		}
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func extractJSONSchemaFromContent(parent any, contentKey string) any {
	parentMap, ok := asMap(parent)
	if !ok {
		return nil
	}
	content, ok := asMap(parentMap[contentKey])
	if !ok {
		return nil
	}
	jsonContent, ok := asMap(content["application/json"])
	if !ok {
		for key, value := range content {
			if strings.HasSuffix(key, "+json") {
				jsonContent, _ = asMap(value)
				break
			}
		}
	}
	if jsonContent == nil {
		return nil
	}
	return jsonContent["schema"]
}

func pickOutputSchema(responses any) any {
	responseMap, ok := asMap(responses)
	if !ok {
		return nil
	}
	if response, ok := responseMap["200"]; ok {
		if schema := extractJSONSchemaFromContent(response, "content"); schema != nil {
			return schema
		}
	}
	keys := make([]string, 0, len(responseMap))
	for key := range responseMap {
		if len(key) == 3 && strings.HasPrefix(key, "2") {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		if schema := extractJSONSchemaFromContent(responseMap[key], "content"); schema != nil {
			return schema
		}
	}
	return nil
}

func detectUploadFormat(filename string) (string, string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".json":
		return "json", ext, nil
	case ".yaml", ".yml":
		return "yaml", ext, nil
	default:
		return "", "", NewToolAppError("unsupported_file_type", "仅支持 .json/.yaml/.yml OpenAPI 文件")
	}
}

func parseOpenAPIBytes(format string, raw []byte) (map[string]any, error) {
	var parsed any
	if format == "json" {
		decoder := json.NewDecoder(bytes.NewReader(raw))
		if err := decoder.Decode(&parsed); err != nil {
			return nil, NewToolAppError("invalid_json", "JSON 解析失败")
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			return nil, NewToolAppError("invalid_json", "JSON 包含多余内容")
		}
	} else {
		decoder := yaml.NewDecoder(bytes.NewReader(raw))
		if err := decoder.Decode(&parsed); err != nil {
			return nil, NewToolAppError("invalid_yaml", "YAML 解析失败")
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			return nil, NewToolAppError("invalid_yaml", "YAML 不能包含多个文档")
		}
	}
	normalized, err := normalizeToMap(parsed)
	if err != nil {
		return nil, err
	}
	return normalized, nil
}

func normalizeToMap(value any) (map[string]any, error) {
	if value == nil {
		return nil, NewToolAppError("invalid_openapi", "文件内容不能为空")
	}
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return nil, NewToolAppError("invalid_openapi", "OpenAPI 内容无法规范化")
	}
	var result map[string]any
	if err := json.Unmarshal(jsonBytes, &result); err != nil || result == nil {
		return nil, NewToolAppError("invalid_openapi", "OpenAPI 内容必须是对象")
	}
	return result, nil
}

func validateServerURL(raw string) error {
	if raw == "" {
		return NewToolAppError("invalid_openapi", "server URL 不能为空")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return NewToolAppError("invalid_openapi", "server URL 格式无效")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return NewToolAppError("unsafe_server_url", "server URL 必须是 http 或 https")
	}
	if isProductionMode() && isUnsafeHost(parsed.Hostname()) {
		return NewToolAppError("unsafe_server_url", "生产环境禁止 localhost 或内网地址")
	}
	return nil
}

func isProductionMode() bool {
	return os.Getenv("GIN_MODE") == "release" || os.Getenv("NODE_ENV") == "production"
}

func isUnsafeHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
}

var nonFunctionNameChars = regexp.MustCompile(`[^A-Za-z0-9_]+`)

func normalizeFunctionName(value string) string {
	name := nonFunctionNameChars.ReplaceAllString(value, "_")
	name = strings.Trim(name, "_")
	if name == "" {
		return ""
	}
	if name[0] >= '0' && name[0] <= '9' {
		name = "_" + name
	}
	if len(name) > 64 {
		name = name[:64]
	}
	return name
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func hasToolConflictSQL(name string, slug string, excludeID string) bool {
	if model.DB == nil {
		return false
	}
	query := model.DB.Model(&model.Tool{})
	conditions := make([]string, 0, 2)
	args := make([]any, 0, 2)
	if normalizedName := strings.ToLower(strings.TrimSpace(name)); normalizedName != "" {
		conditions = append(conditions, "LOWER(name) = ?")
		args = append(args, normalizedName)
	}
	if normalizedSlug := strings.ToLower(strings.TrimSpace(slug)); normalizedSlug != "" {
		conditions = append(conditions, "LOWER(slug) = ?")
		args = append(args, normalizedSlug)
	}
	if len(conditions) == 0 {
		return false
	}
	query = query.Where(strings.Join(conditions, " OR "), args...)
	if strings.TrimSpace(excludeID) != "" {
		query = query.Where("id <> ?", strings.TrimSpace(excludeID))
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return true
	}
	return count > 0
}

func suggestSlugSQL(base string) string {
	if base == "" {
		base = "tool"
	}
	if !hasToolConflictSQL("", base, "") {
		return base
	}
	for i := 2; i < 1000; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if !hasToolConflictSQL("", candidate, "") {
			return candidate
		}
	}
	return base + "-" + randomHex(3)
}

func sanitizeID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.Contains(value, "..") || strings.ContainsAny(value, `/\`) {
		return ""
	}
	return value
}

func randomHex(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func normalizeAuthType(value string) string {
	if strings.TrimSpace(value) == "api_key" {
		return "api_key"
	}
	return "none"
}

func normalizeAPIKeyLocation(value string) string {
	if strings.TrimSpace(value) == "query" {
		return "query"
	}
	return "header"
}

func normalizeVisibility(value string) string {
	value = strings.TrimSpace(value)
	if value == "private" {
		return "private"
	}
	return "public"
}

func normalizeToolCategory(value string) (string, error) {
	category := strings.TrimSpace(value)
	if supportedToolCategories[category] {
		return category, nil
	}
	return "", NewToolAppError("invalid_request", "工具分类必须是商业、工具、开发、媒体、生活之一")
}

func normalizeToolHeaders(headers []ToolHeader) []ToolHeader {
	normalized := make([]ToolHeader, 0, len(headers))
	seen := map[string]bool{}
	for _, header := range headers {
		name := strings.TrimSpace(header.Name)
		value := strings.TrimSpace(header.Value)
		if name == "" || value == "" {
			continue
		}
		lowerName := strings.ToLower(name)
		if seen[lowerName] || lowerName == "authorization" {
			continue
		}
		seen[lowerName] = true
		normalized = append(normalized, ToolHeader{Name: name, Value: value})
	}
	return normalized
}

func normalizeToolCallPrice(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func normalizeListLimit(value int) int {
	if value <= 0 {
		return 20
	}
	if value > 50 {
		return 50
	}
	return value
}

func normalizeListOffset(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func normalizeToolActionMethod(value string) string {
	method := strings.ToUpper(strings.TrimSpace(value))
	if supportedMethods[method] {
		return method
	}
	return ""
}

func normalizeToolActionInputSchema(schema map[string]any) map[string]any {
	if schema == nil {
		return map[string]any{
			"type":       "object",
			"properties": map[string]any{},
			"required":   []any{},
		}
	}
	normalized := cloneMap(schema)
	if strings.TrimSpace(asString(normalized["type"])) == "" {
		normalized["type"] = "object"
	}
	if _, ok := normalized["properties"].(map[string]any); !ok {
		normalized["properties"] = map[string]any{}
	}
	if _, ok := normalized["required"].([]any); !ok {
		if values, ok := normalized["required"].([]string); ok {
			required := make([]any, 0, len(values))
			for _, value := range values {
				if strings.TrimSpace(value) != "" {
					required = append(required, strings.TrimSpace(value))
				}
			}
			normalized["required"] = required
		} else {
			normalized["required"] = []any{}
		}
	}
	return normalized
}

func normalizeRiskLevel(value string, method string) string {
	switch strings.TrimSpace(value) {
	case "read", "write", "dangerous":
		return strings.TrimSpace(value)
	default:
		return riskLevel(method)
	}
}

func buildManualOpenAPI(
	name string,
	description string,
	serverURL string,
	actions []ToolManualActionOptions,
) map[string]any {
	paths := map[string]any{}
	for _, action := range actions {
		operation := map[string]any{
			"operationId": action.OperationID,
			"summary":     action.DisplayName,
			"description": action.Description,
			"parameters":  buildOpenAPIParametersFromInputSchema(action.InputSchema),
			"responses": map[string]any{
				"200": map[string]any{
					"description": "OK",
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": action.OutputSchema,
						},
					},
				},
			},
		}
		if requestBody := buildOpenAPIRequestBodyFromInputSchema(action.InputSchema); requestBody != nil {
			operation["requestBody"] = requestBody
		}
		pathItem, _ := paths[action.Path].(map[string]any)
		if pathItem == nil {
			pathItem = map[string]any{}
			paths[action.Path] = pathItem
		}
		pathItem[strings.ToLower(action.Method)] = operation
	}
	return map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":       name,
			"description": description,
			"version":     "1.0.0",
		},
		"servers": []any{
			map[string]any{"url": serverURL},
		},
		"paths": paths,
	}
}

func buildOpenAPIParametersFromInputSchema(schema map[string]any) []any {
	props, _ := schema["properties"].(map[string]any)
	requiredNames := requiredNameSet(schema["required"])
	parameters := make([]any, 0, len(props))
	for name, rawProp := range props {
		prop, _ := rawProp.(map[string]any)
		location := strings.TrimSpace(asString(prop["x-openapi-in"]))
		if location == "" || location == "body" {
			continue
		}
		paramSchema := cloneMap(prop)
		delete(paramSchema, "description")
		delete(paramSchema, "x-openapi-in")
		parameters = append(parameters, map[string]any{
			"name":        name,
			"in":          location,
			"description": strings.TrimSpace(asString(prop["description"])),
			"required":    requiredNames[name],
			"schema":      paramSchema,
		})
	}
	return parameters
}

func buildOpenAPIRequestBodyFromInputSchema(schema map[string]any) any {
	props, _ := schema["properties"].(map[string]any)
	bodyProps := map[string]any{}
	requiredNames := requiredNameSet(schema["required"])
	required := make([]any, 0)
	for name, rawProp := range props {
		prop, _ := rawProp.(map[string]any)
		location := strings.TrimSpace(asString(prop["x-openapi-in"]))
		if location != "" && location != "body" {
			continue
		}
		bodyProp := cloneMap(prop)
		delete(bodyProp, "x-openapi-in")
		bodyProps[name] = bodyProp
		if requiredNames[name] {
			required = append(required, name)
		}
	}
	if len(bodyProps) == 0 {
		return nil
	}
	bodySchema := map[string]any{
		"type":       "object",
		"properties": bodyProps,
	}
	if len(required) > 0 {
		bodySchema["required"] = required
	}
	return map[string]any{
		"required": true,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": bodySchema,
			},
		},
	}
}

func requiredNameSet(value any) map[string]bool {
	result := map[string]bool{}
	switch values := value.(type) {
	case []any:
		for _, value := range values {
			name := strings.TrimSpace(fmt.Sprint(value))
			if name != "" {
				result[name] = true
			}
		}
	case []string:
		for _, value := range values {
			name := strings.TrimSpace(value)
			if name != "" {
				result[name] = true
			}
		}
	}
	return result
}

func riskLevel(method string) string {
	switch method {
	case "GET":
		return "read"
	case "DELETE":
		return "dangerous"
	default:
		return "write"
	}
}

func sortTools(tools []ToolSummary) {
	sort.SliceStable(tools, func(i, j int) bool {
		return tools[i].UpdatedAt > tools[j].UpdatedAt
	})
}

func asMap(value any) (map[string]any, bool) {
	result, ok := value.(map[string]any)
	return result, ok
}

func asArray(value any) []any {
	if array, ok := value.([]any); ok {
		return array
	}
	return nil
}

func asString(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func cloneMap(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}
