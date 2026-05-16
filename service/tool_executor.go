package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
)

const toolExecutionTimeout = 30 * time.Second

type ToolActionRunRequest struct {
	Arguments      map[string]any `json:"arguments"`
	ConversationID string         `json:"conversation_id,omitempty"`
	MessageID      string         `json:"message_id,omitempty"`
	ToolCallID     string         `json:"tool_call_id,omitempty"`
}

type ToolActionRunResult struct {
	RunID          int64          `json:"run_id"`
	ToolID         string         `json:"tool_id"`
	ActionID       string         `json:"action_id"`
	FunctionName   string         `json:"function_name"`
	Status         string         `json:"status"`
	HTTPStatus     int            `json:"http_status,omitempty"`
	Result         any            `json:"result,omitempty"`
	ResultText     string         `json:"result_text,omitempty"`
	ErrorMessage   string         `json:"error_message,omitempty"`
	DurationMS     int64          `json:"duration_ms"`
	Arguments      map[string]any `json:"arguments,omitempty"`
	ToolCallID     string         `json:"tool_call_id,omitempty"`
	ConversationID string         `json:"conversation_id,omitempty"`
	MessageID      string         `json:"message_id,omitempty"`
}

type toolSecret struct {
	APIKeyLocation string `json:"api_key_location"`
	APIKeyName     string `json:"api_key_name"`
	APIKeyValue    string `json:"api_key_value"`
}

func RunUserToolAction(userID int, isAdmin bool, toolID string, actionID string, req ToolActionRunRequest) (ToolActionRunResult, error) {
	if userID <= 0 {
		return ToolActionRunResult{}, NewToolAppError("invalid_request", "用户未登录")
	}
	if err := ensureToolDB(); err != nil {
		return ToolActionRunResult{}, err
	}
	toolID = strings.TrimSpace(toolID)
	actionID = strings.TrimSpace(actionID)
	if toolID == "" || actionID == "" {
		return ToolActionRunResult{}, NewToolAppError("invalid_request", "工具或函数无效")
	}

	var userTool model.UserTool
	if err := model.ToolDB.Where("user_id = ? AND tool_id = ? AND enabled = ?", userID, toolID, true).First(&userTool).Error; err != nil && !canRunUninstalledTool(userID, isAdmin, toolID) {
		return ToolActionRunResult{}, NewToolAppError("user_tool_not_found", "请先获取该工具")
	}

	detail, err := GetToolDetail(toolID)
	if err != nil {
		return ToolActionRunResult{}, err
	}
	action, ok := findToolAction(detail.Actions, actionID)
	if !ok || !action.Enabled {
		return ToolActionRunResult{}, NewToolAppError("tool_action_not_found", "工具函数不存在或未启用")
	}
	args := req.Arguments
	if args == nil {
		args = map[string]any{}
	}
	argText := marshalJSONString(args)
	run := model.ToolRun{
		ConversationID: req.ConversationID,
		MessageID:      req.MessageID,
		UserID:         userID,
		ToolID:         toolID,
		ActionID:       action.ID,
		FunctionName:   action.OperationID,
		Status:         "running",
		RequestArgs:    argText,
		StartedAt:      time.Now().Unix(),
	}
	if err := model.ToolDB.Create(&run).Error; err != nil {
		return ToolActionRunResult{}, err
	}

	start := time.Now()
	result, statusCode, execErr := executeOpenAPIToolAction(detail, action, args)
	duration := time.Since(start).Milliseconds()
	run.DurationMS = duration
	run.FinishedAt = time.Now().Unix()

	responseText := stringifyToolResult(result)
	run.ResponsePreview = truncateForToolRun(responseText, 4000)
	if execErr != nil {
		run.Status = "failed"
		run.ErrorMessage = execErr.Error()
		_ = model.ToolDB.Save(&run).Error
		return ToolActionRunResult{
			RunID:          run.ID,
			ToolID:         toolID,
			ActionID:       action.ID,
			FunctionName:   action.OperationID,
			Status:         run.Status,
			HTTPStatus:     statusCode,
			Result:         result,
			ResultText:     responseText,
			ErrorMessage:   execErr.Error(),
			DurationMS:     duration,
			Arguments:      args,
			ToolCallID:     req.ToolCallID,
			ConversationID: req.ConversationID,
			MessageID:      req.MessageID,
		}, execErr
	}

	run.Status = "success"
	if err := model.ToolDB.Save(&run).Error; err != nil {
		return ToolActionRunResult{}, err
	}
	return ToolActionRunResult{
		RunID:          run.ID,
		ToolID:         toolID,
		ActionID:       action.ID,
		FunctionName:   action.OperationID,
		Status:         run.Status,
		HTTPStatus:     statusCode,
		Result:         result,
		ResultText:     responseText,
		DurationMS:     duration,
		Arguments:      args,
		ToolCallID:     req.ToolCallID,
		ConversationID: req.ConversationID,
		MessageID:      req.MessageID,
	}, nil
}

func findToolAction(actions []ToolAction, actionID string) (ToolAction, bool) {
	for _, action := range actions {
		if action.ID == actionID || action.OperationID == actionID || action.Name == actionID {
			return action, true
		}
	}
	return ToolAction{}, false
}

func canRunUninstalledTool(userID int, isAdmin bool, toolID string) bool {
	detail, err := GetToolDetail(toolID)
	if err != nil {
		return false
	}
	return canEditTool(detail, userID, isAdmin)
}

func executeOpenAPIToolAction(detail ToolDetail, action ToolAction, args map[string]any) (any, int, error) {
	targetURL, err := buildToolActionURL(detail, action, args)
	if err != nil {
		return nil, 0, err
	}
	body := buildToolActionBody(action, args)
	var bodyReader io.Reader
	if body != nil && action.Method != http.MethodGet {
		content, err := json.Marshal(body)
		if err != nil {
			return nil, 0, NewToolAppError("tool_arguments_invalid", "工具参数无法序列化")
		}
		bodyReader = bytes.NewReader(content)
	}

	httpReq, err := http.NewRequest(strings.ToUpper(action.Method), targetURL, bodyReader)
	if err != nil {
		return nil, 0, NewToolAppError("tool_request_invalid", "工具请求创建失败")
	}
	httpReq.Header.Set("Accept", "application/json")
	if bodyReader != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	for _, header := range detail.CommonHeaders {
		name := strings.TrimSpace(header.Name)
		value := strings.TrimSpace(header.Value)
		if name != "" && value != "" {
			httpReq.Header.Set(name, value)
		}
	}
	if err := applyToolAuth(&httpReq.Header, &targetURL, detail); err != nil {
		return nil, 0, err
	}
	httpReq.URL, _ = url.Parse(targetURL)

	client := &http.Client{Timeout: toolExecutionTimeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, 0, NewToolAppError("tool_request_failed", "工具接口请求失败: "+err.Error())
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, resp.StatusCode, NewToolAppError("tool_response_read_failed", "读取工具响应失败")
	}
	parsed := parseToolResponse(content)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parsed, resp.StatusCode, NewToolAppError("tool_http_failed", fmt.Sprintf("工具接口返回 HTTP %d", resp.StatusCode))
	}
	return parsed, resp.StatusCode, nil
}

func buildToolActionURL(detail ToolDetail, action ToolAction, args map[string]any) (string, error) {
	base, err := url.Parse(strings.TrimSpace(detail.ServerURL))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return "", NewToolAppError("tool_server_invalid", "工具服务地址无效")
	}
	actionPath := strings.TrimSpace(action.Path)
	if actionPath == "" {
		return "", NewToolAppError("tool_action_invalid", "工具函数缺少请求路径")
	}
	base.Path = path.Join(strings.TrimRight(base.Path, "/"), strings.TrimLeft(actionPath, "/"))
	query := base.Query()
	props, _ := action.InputSchema["properties"].(map[string]any)
	for name, schema := range props {
		prop, _ := schema.(map[string]any)
		if prop["x-openapi-in"] != "query" {
			continue
		}
		if value, ok := args[name]; ok && value != nil {
			query.Set(name, fmt.Sprint(value))
		}
	}
	base.RawQuery = query.Encode()
	return base.String(), nil
}

func buildToolActionBody(action ToolAction, args map[string]any) any {
	if body, ok := args["body"]; ok {
		return body
	}
	body := map[string]any{}
	props, _ := action.InputSchema["properties"].(map[string]any)
	for name, value := range args {
		prop, _ := props[name].(map[string]any)
		if prop["x-openapi-in"] == "query" || prop["x-openapi-in"] == "header" || prop["x-openapi-in"] == "path" {
			continue
		}
		body[name] = value
	}
	if len(body) == 0 {
		return nil
	}
	return body
}

func applyToolAuth(headers *http.Header, targetURL *string, detail ToolDetail) error {
	if detail.AuthType != "api_key" {
		return nil
	}
	secret, err := readToolSecret(detail.ID)
	if err != nil {
		return err
	}
	location := strings.TrimSpace(detail.APIKeyLocation)
	name := strings.TrimSpace(detail.APIKeyName)
	if location == "" {
		location = secret.APIKeyLocation
	}
	if name == "" {
		name = secret.APIKeyName
	}
	value := strings.TrimSpace(secret.APIKeyValue)
	if name == "" || value == "" {
		return NewToolAppError("tool_auth_missing", "工具密钥未配置")
	}
	if location == "query" {
		parsed, err := url.Parse(*targetURL)
		if err != nil {
			return NewToolAppError("tool_request_invalid", "工具请求地址无效")
		}
		query := parsed.Query()
		query.Set(name, value)
		parsed.RawQuery = query.Encode()
		*targetURL = parsed.String()
		return nil
	}
	headers.Set(name, value)
	return nil
}

func readToolSecret(toolID string) (toolSecret, error) {
	var secret toolSecret
	if err := readJSON(filepath.Join(toolSecretDir(), sanitizeID(toolID)+".json"), &secret); err != nil {
		return toolSecret{}, NewToolAppError("tool_auth_missing", "工具密钥未配置")
	}
	return secret, nil
}

func parseToolResponse(content []byte) any {
	var parsed any
	if err := json.Unmarshal(content, &parsed); err == nil {
		return parsed
	}
	return string(content)
}

func stringifyToolResult(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	content, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(content)
}

func marshalJSONString(value any) string {
	content, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(content)
}

func truncateForToolRun(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
