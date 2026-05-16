package service

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
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
)

const (
	toolDataDirEnv    = "TOOL_DATA_DIR"
	toolSecretDirEnv  = "TOOL_SECRET_DIR"
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
	ID            string `json:"id"`
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Version       string `json:"version"`
	Type          string `json:"type"`
	AuthType      string `json:"auth_type"`
	ServerURL     string `json:"server_url"`
	ActionCount   int    `json:"action_count"`
	Status        string `json:"status"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
	DownloadCount int64  `json:"download_count"`
	CreatedBy     int    `json:"created_by,omitempty"`
	CreatedByName string `json:"created_by_name,omitempty"`
	Category      string `json:"category,omitempty"`
	Visibility    string `json:"visibility,omitempty"`
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
	index, err := readToolIndex()
	if err != nil {
		return ToolIndex{}, err
	}
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	category = strings.ToLower(strings.TrimSpace(category))
	filtered := make([]ToolSummary, 0, len(index.Tools))
	indexChanged := false
	for i, tool := range index.Tools {
		hydrated := hydrateToolSummary(tool)
		if hydrated != tool {
			index.Tools[i] = hydrated
			indexChanged = true
		}
		tool = hydrated
		haystack := strings.ToLower(tool.Name + " " + tool.Description)
		if keyword != "" && !strings.Contains(haystack, keyword) {
			continue
		}
		if category != "" && strings.ToLower(strings.TrimSpace(tool.Category)) != category {
			continue
		}
		filtered = append(filtered, tool)
	}
	if indexChanged {
		index.UpdatedAt = time.Now().Unix()
		sortTools(index.Tools)
		_ = writeToolIndex(index)
	}
	index.Tools = filtered
	enrichToolCreatorNames(index.Tools)
	return index, nil
}

func GetToolDetail(toolID string) (ToolDetail, error) {
	toolID = sanitizeID(toolID)
	if toolID == "" {
		return ToolDetail{}, NewToolAppError("invalid_request", "工具 ID 无效")
	}
	var detail ToolDetail
	if err := readJSON(filepath.Join(toolDir(toolID), "tool.json"), &detail); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ToolDetail{}, NewToolAppError("tool_not_found", "工具不存在")
		}
		return ToolDetail{}, NewToolAppError("storage_read_failed", "读取工具失败")
	}
	var actions []ToolAction
	if err := readJSON(filepath.Join(toolDir(toolID), "actions.json"), &actions); err == nil {
		detail.Actions = actions
	}
	return detail, nil
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
	index, err := readToolIndex()
	if err != nil {
		return ToolDetail{}, err
	}
	slug := slugify(parsed.Name)
	if slug == "" {
		slug = "tool"
	}
	if hasToolConflict(index.Tools, parsed.Name, slug) {
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
	index.Tools = append(index.Tools, detail.ToolSummary)
	sortTools(index.Tools)
	index.UpdatedAt = now
	if err := writeToolIndex(index); err != nil {
		return ToolDetail{}, err
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
		_ = os.Remove(filepath.Join(toolSecretDir(), toolID+".json"))
		detail.APIKeyLocation = ""
		detail.APIKeyName = ""
	}

	if err := writeJSON(filepath.Join(toolDir(toolID), "tool.json"), detail); err != nil {
		return ToolDetail{}, err
	}
	if err := updateToolIndexSummary(detail.ToolSummary); err != nil {
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

	if err := writeJSON(filepath.Join(toolDir(toolID), "actions.json"), detail.Actions); err != nil {
		return ToolDetail{}, err
	}
	if err := writeJSON(filepath.Join(toolDir(toolID), "tool.json"), detail); err != nil {
		return ToolDetail{}, err
	}
	if err := syncToolActionToOpenAPI(toolID, oldAction, nextAction); err != nil {
		return ToolDetail{}, err
	}
	if err := updateToolIndexSummary(detail.ToolSummary); err != nil {
		return ToolDetail{}, err
	}
	return detail, nil
}

func CheckToolName(name string) (map[string]any, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, NewToolAppError("invalid_request", "工具名称不能为空")
	}
	index, err := readToolIndex()
	if err != nil {
		return nil, err
	}
	slug := slugify(name)
	available := !hasToolConflict(index.Tools, name, slug)
	return map[string]any{
		"available":      available,
		"reason":         map[bool]string{true: "", false: "tool_name_conflict"}[available],
		"suggested_slug": suggestSlug(index.Tools, slug),
	}, nil
}

func DeleteTool(toolID string) error {
	toolID = sanitizeID(toolID)
	index, err := readToolIndex()
	if err != nil {
		return err
	}
	found := false
	tools := make([]ToolSummary, 0, len(index.Tools))
	for _, tool := range index.Tools {
		if tool.ID == toolID {
			found = true
			continue
		}
		tools = append(tools, tool)
	}
	if !found {
		return NewToolAppError("tool_not_found", "工具不存在")
	}
	index.Tools = tools
	index.UpdatedAt = time.Now().Unix()
	if err := writeToolIndex(index); err != nil {
		return err
	}
	_ = os.RemoveAll(toolDir(toolID))
	_ = os.Remove(filepath.Join(toolSecretDir(), toolID+".json"))
	return nil
}

func BuildToolDownload(toolID string) (string, error) {
	detail, err := GetToolDetail(toolID)
	if err != nil {
		return "", err
	}
	downloadDir := filepath.Join(toolDir(toolID), "download")
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		return "", NewToolAppError("storage_write_failed", "创建下载目录失败")
	}
	zipPath := filepath.Join(downloadDir, fmt.Sprintf("tool-%s.zip", toolID))
	file, err := os.Create(zipPath)
	if err != nil {
		return "", NewToolAppError("storage_write_failed", "创建下载包失败")
	}
	zipWriter := zip.NewWriter(file)
	writeErr := addToolZipFiles(zipWriter, toolID, detail.SourceFormat)
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

func addToolZipFiles(zipWriter *zip.Writer, toolID string, sourceFormat string) error {
	sourceName := "source.openapi.json"
	if sourceFormat == "yaml" {
		sourceName = "source.openapi.yaml"
	}
	files := []string{"tool.json", "openapi.json", "actions.json", sourceName}
	for _, name := range files {
		content, err := os.ReadFile(filepath.Join(toolDir(toolID), name))
		if err != nil {
			return NewToolAppError("storage_read_failed", "读取下载包内容失败")
		}
		writer, err := zipWriter.Create(name)
		if err != nil {
			return NewToolAppError("storage_write_failed", "写入下载包失败")
		}
		if _, err := writer.Write(content); err != nil {
			return NewToolAppError("storage_write_failed", "写入下载包失败")
		}
	}
	readme := []byte("# OpenAPI 工具定义包\n\n本包包含工具元信息、规范化 OpenAPI、原始 OpenAPI 文件和 Action 列表，不包含 API Key 或密钥。\n")
	writer, err := zipWriter.Create("README.md")
	if err != nil {
		return NewToolAppError("storage_write_failed", "写入下载包失败")
	}
	if _, err := writer.Write(readme); err != nil {
		return NewToolAppError("storage_write_failed", "写入下载包失败")
	}
	return nil
}

func persistTool(detail ToolDetail, parsed ToolParseResult) error {
	dir := toolDir(detail.ID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return NewToolAppError("storage_write_failed", "创建工具目录失败")
	}
	sourceName := "source.openapi.json"
	if parsed.SourceFormat == "yaml" {
		sourceName = "source.openapi.yaml"
	}
	if err := os.WriteFile(filepath.Join(dir, sourceName), parsed.Raw, 0644); err != nil {
		return NewToolAppError("storage_write_failed", "保存原始 OpenAPI 失败")
	}
	if err := writeJSON(filepath.Join(dir, "openapi.json"), parsed.OpenAPI); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(dir, "actions.json"), detail.Actions); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(dir, "tool.json"), detail); err != nil {
		return err
	}
	return nil
}

func persistToolSecret(toolID string, location string, name string, value string) error {
	if err := os.MkdirAll(toolSecretDir(), 0700); err != nil {
		return NewToolAppError("storage_write_failed", "创建密钥目录失败")
	}
	secret := map[string]string{
		"api_key_location": location,
		"api_key_name":     name,
		"api_key_value":    value,
	}
	if err := writeJSON(filepath.Join(toolSecretDir(), toolID+".json"), secret); err != nil {
		return err
	}
	return nil
}

func incrementDownloadCount(toolID string) error {
	index, err := readToolIndex()
	if err != nil {
		return err
	}
	detail, err := GetToolDetail(toolID)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	detail.DownloadCount++
	detail.UpdatedAt = now
	for i := range index.Tools {
		if index.Tools[i].ID == toolID {
			index.Tools[i].DownloadCount = detail.DownloadCount
			index.Tools[i].UpdatedAt = now
			break
		}
	}
	index.UpdatedAt = now
	sortTools(index.Tools)
	if err := writeJSON(filepath.Join(toolDir(toolID), "tool.json"), detail); err != nil {
		return err
	}
	return writeToolIndex(index)
}

func updateToolIndexSummary(summary ToolSummary) error {
	index, err := readToolIndex()
	if err != nil {
		return err
	}
	found := false
	for i := range index.Tools {
		if index.Tools[i].ID == summary.ID {
			index.Tools[i] = summary
			found = true
			break
		}
	}
	if !found {
		index.Tools = append(index.Tools, summary)
	}
	index.UpdatedAt = summary.UpdatedAt
	sortTools(index.Tools)
	return writeToolIndex(index)
}

func hydrateToolSummary(summary ToolSummary) ToolSummary {
	if summary.Category != "" && summary.Visibility != "" {
		return summary
	}
	var detail ToolDetail
	if err := readJSON(filepath.Join(toolDir(summary.ID), "tool.json"), &detail); err != nil {
		return summary
	}
	if detail.Category != "" {
		summary.Category = detail.Category
	}
	if detail.Visibility != "" {
		summary.Visibility = detail.Visibility
	}
	if detail.CreatedBy != 0 {
		summary.CreatedBy = detail.CreatedBy
	}
	return summary
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
	if err := model.DB.Select("id, username, display_name").Where("id IN ?", ids).Find(&users).Error; err != nil {
		return
	}
	names := map[int]string{}
	for _, user := range users {
		name := strings.TrimSpace(user.DisplayName)
		if name == "" {
			name = strings.TrimSpace(user.Username)
		}
		names[user.Id] = name
	}
	for i := range tools {
		if name := names[tools[i].CreatedBy]; name != "" {
			tools[i].CreatedByName = name
		}
	}
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

func hasToolConflict(tools []ToolSummary, name string, slug string) bool {
	normalizedName := strings.ToLower(strings.TrimSpace(name))
	for _, tool := range tools {
		if strings.ToLower(strings.TrimSpace(tool.Name)) == normalizedName || tool.Slug == slug {
			return true
		}
	}
	return false
}

func suggestSlug(tools []ToolSummary, base string) string {
	if base == "" {
		base = "tool"
	}
	used := map[string]bool{}
	for _, tool := range tools {
		used[tool.Slug] = true
	}
	if !used[base] {
		return base
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if !used[candidate] {
			return candidate
		}
	}
}

func readToolIndex() (ToolIndex, error) {
	path := filepath.Join(toolDataDir(), "index.json")
	var index ToolIndex
	if err := readJSON(path, &index); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ToolIndex{UpdatedAt: time.Now().Unix(), Tools: []ToolSummary{}}, nil
		}
		return ToolIndex{}, NewToolAppError("storage_read_failed", "读取工具索引失败")
	}
	if index.Tools == nil {
		index.Tools = []ToolSummary{}
	}
	sortTools(index.Tools)
	return index, nil
}

func writeToolIndex(index ToolIndex) error {
	if err := os.MkdirAll(toolDataDir(), 0755); err != nil {
		return NewToolAppError("storage_write_failed", "创建工具数据目录失败")
	}
	return writeJSON(filepath.Join(toolDataDir(), "index.json"), index)
}

func readJSON(path string, value any) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(content, value)
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return NewToolAppError("storage_write_failed", "创建数据目录失败")
	}
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return NewToolAppError("storage_write_failed", "序列化工具数据失败")
	}
	if err := os.WriteFile(path, content, 0644); err != nil {
		return NewToolAppError("storage_write_failed", "写入工具数据失败")
	}
	return nil
}

func toolDataDir() string {
	if dir := strings.TrimSpace(os.Getenv(toolDataDirEnv)); dir != "" {
		return dir
	}
	return filepath.Join("data", "tools")
}

func toolSecretDir() string {
	if dir := strings.TrimSpace(os.Getenv(toolSecretDirEnv)); dir != "" {
		return dir
	}
	return filepath.Join("data", "tool-secrets")
}

func toolDir(toolID string) string {
	return filepath.Join(toolDataDir(), toolID)
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

func syncToolActionToOpenAPI(toolID string, oldAction ToolAction, nextAction ToolAction) error {
	openAPIPath := filepath.Join(toolDir(toolID), "openapi.json")
	var openAPI map[string]any
	if err := readJSON(openAPIPath, &openAPI); err != nil {
		return nil
	}
	paths, ok := openAPI["paths"].(map[string]any)
	if !ok {
		paths = map[string]any{}
		openAPI["paths"] = paths
	}
	oldPathItem, _ := paths[oldAction.Path].(map[string]any)
	oldMethodKey := strings.ToLower(oldAction.Method)
	nextMethodKey := strings.ToLower(nextAction.Method)
	operation, _ := oldPathItem[oldMethodKey].(map[string]any)
	if operation == nil {
		operation = map[string]any{}
	}
	if oldPathItem != nil {
		delete(oldPathItem, oldMethodKey)
		if len(oldPathItem) == 0 {
			delete(paths, oldAction.Path)
		}
	}
	operation["operationId"] = nextAction.OperationID
	operation["summary"] = nextAction.DisplayName
	operation["description"] = nextAction.Description
	operation["parameters"] = buildOpenAPIParametersFromInputSchema(nextAction.InputSchema)
	if requestBody := buildOpenAPIRequestBodyFromInputSchema(nextAction.InputSchema); requestBody != nil {
		operation["requestBody"] = requestBody
	} else {
		delete(operation, "requestBody")
	}
	if nextAction.OutputSchema != nil {
		operation["responses"] = map[string]any{
			"200": map[string]any{
				"description": "OK",
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": nextAction.OutputSchema,
					},
				},
			},
		}
	}
	nextPathItem, _ := paths[nextAction.Path].(map[string]any)
	if nextPathItem == nil {
		nextPathItem = map[string]any{}
		paths[nextAction.Path] = nextPathItem
	}
	nextPathItem[nextMethodKey] = operation
	return writeJSON(openAPIPath, openAPI)
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
