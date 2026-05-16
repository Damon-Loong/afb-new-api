package service

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validOpenAPIJSON = `{
  "openapi": "3.0.3",
  "info": {
    "title": "Weather Tool",
    "description": "查询城市天气",
    "version": "1.0.0"
  },
  "servers": [{"url": "https://api.example.com"}],
  "paths": {
    "/weather": {
      "get": {
        "operationId": "getWeather",
        "summary": "查询天气",
        "description": "根据城市查询天气",
        "parameters": [
          {"name": "city", "in": "query", "required": true, "schema": {"type": "string"}}
        ],
        "responses": {
          "200": {
            "description": "ok",
            "content": {
              "application/json": {
                "schema": {"type": "object", "properties": {"temp": {"type": "number"}}}
              }
            }
          }
        }
      }
    }
  }
}`

const validOpenAPIYAML = `
openapi: 3.0.3
info:
  title: Calendar Tool
  description: 查询日程信息
  version: 1.2.0
servers:
  - url: https://calendar.example.com
paths:
  /events:
    post:
      operationId: createEvent
      summary: 创建日程
      description: 创建新的日程
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                title:
                  type: string
      responses:
        "201":
          description: created
`

func setupToolStoreTest(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv(toolDataDirEnv, filepath.Join(root, "data", "tools"))
	t.Setenv(toolSecretDirEnv, filepath.Join(root, "data", "tool-secrets"))
	return root
}

func TestParseOpenAPIUploadJSONAndYAML(t *testing.T) {
	setupToolStoreTest(t)

	jsonResult, err := ParseOpenAPIUpload("weather.json", strings.NewReader(validOpenAPIJSON), int64(len(validOpenAPIJSON)))
	if err != nil {
		t.Fatalf("parse json failed: %v", err)
	}
	if jsonResult.Name != "Weather Tool" || jsonResult.SourceFormat != "json" || len(jsonResult.Actions) != 1 {
		t.Fatalf("unexpected json parse result: %+v", jsonResult)
	}
	if jsonResult.Actions[0].Name != "getWeather" || jsonResult.Actions[0].RiskLevel != "read" {
		t.Fatalf("unexpected action: %+v", jsonResult.Actions[0])
	}

	yamlResult, err := ParseOpenAPIUpload("calendar.yaml", strings.NewReader(validOpenAPIYAML), int64(len(validOpenAPIYAML)))
	if err != nil {
		t.Fatalf("parse yaml failed: %v", err)
	}
	if yamlResult.Name != "Calendar Tool" || yamlResult.SourceFormat != "yaml" || yamlResult.Actions[0].RiskLevel != "write" {
		t.Fatalf("unexpected yaml parse result: %+v", yamlResult)
	}
}

func TestParseOpenAPIValidationFailures(t *testing.T) {
	setupToolStoreTest(t)

	cases := []struct {
		name string
		file string
		body string
		code string
	}{
		{name: "bad json", file: "bad.json", body: `{`, code: "invalid_json"},
		{name: "unsupported extension", file: "bad.txt", body: validOpenAPIJSON, code: "unsupported_file_type"},
		{name: "swagger", file: "swagger.json", body: `{"swagger":"2.0"}`, code: "openapi_version_not_supported"},
		{name: "missing title", file: "missing.json", body: `{"openapi":"3.0.0","info":{"description":"desc"},"servers":[{"url":"https://api.example.com"}],"paths":{"/x":{"get":{"operationId":"x","summary":"x"}}}}`, code: "invalid_openapi"},
		{name: "duplicate operation", file: "dup.json", body: `{"openapi":"3.0.0","info":{"title":"Dup","description":"desc"},"servers":[{"url":"https://api.example.com"}],"paths":{"/a":{"get":{"operationId":"same","summary":"same"}},"/b":{"get":{"operationId":"same","summary":"same"}}}}`, code: "tool_name_conflict"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseOpenAPIUpload(tc.file, strings.NewReader(tc.body), int64(len(tc.body)))
			var appErr *ToolAppError
			if err == nil {
				t.Fatalf("expected error")
			}
			if !errors.As(err, &appErr) || appErr.Code != tc.code {
				t.Fatalf("expected %s, got %#v", tc.code, err)
			}
		})
	}
}

func TestUploadToolPersistsIndexAndDetectsNameConflict(t *testing.T) {
	root := setupToolStoreTest(t)

	detail, err := UploadTool("weather.json", strings.NewReader(validOpenAPIJSON), int64(len(validOpenAPIJSON)), ToolUploadOptions{Publish: true, Category: "工具"})
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}
	if detail.ID == "" || detail.ActionCount != 1 || detail.Status != "published" {
		t.Fatalf("unexpected detail: %+v", detail)
	}
	for _, name := range []string{"tool.json", "openapi.json", "actions.json", "source.openapi.json"} {
		if _, err := os.Stat(filepath.Join(root, "data", "tools", detail.ID, name)); err != nil {
			t.Fatalf("expected %s to be persisted: %v", name, err)
		}
	}
	index, err := ListTools("", "")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(index.Tools) != 1 || index.Tools[0].Name != "Weather Tool" {
		t.Fatalf("unexpected index: %+v", index)
	}
	if index.Tools[0].Category != "工具" {
		t.Fatalf("expected category to be persisted, got %+v", index.Tools[0])
	}
	businessIndex, err := ListTools("", "商业")
	if err != nil {
		t.Fatalf("list by business failed: %v", err)
	}
	if len(businessIndex.Tools) != 0 {
		t.Fatalf("expected no business tools, got %+v", businessIndex.Tools)
	}
	toolIndex, err := ListTools("", "工具")
	if err != nil {
		t.Fatalf("list by tool category failed: %v", err)
	}
	if len(toolIndex.Tools) != 1 || toolIndex.Tools[0].ID != detail.ID {
		t.Fatalf("expected tool category result, got %+v", toolIndex.Tools)
	}
	_, err = UploadTool("weather.json", strings.NewReader(validOpenAPIJSON), int64(len(validOpenAPIJSON)), ToolUploadOptions{Publish: true, Category: "工具"})
	var appErr *ToolAppError
	if !errors.As(err, &appErr) || appErr.Code != "tool_name_conflict" {
		t.Fatalf("expected name conflict, got %#v", err)
	}
}

func TestCreateManualToolBuildsOpenAPIAndPersistsIndex(t *testing.T) {
	root := setupToolStoreTest(t)

	detail, err := CreateManualTool(ToolManualCreateOptions{
		Name:        "Manual Weather",
		Description: "manually created weather tool",
		ServerURL:   "https://api.example.com",
		Category:    "工具",
		Visibility:  "public",
		Publish:     true,
		CreatedBy:   18,
		Actions: []ToolManualActionOptions{
			{
				DisplayName: "Get weather",
				Description: "Get weather by city",
				OperationID: "getWeather",
				Method:      "GET",
				Path:        "/weather",
				Enabled:     true,
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"city": map[string]any{
							"type":         "string",
							"x-openapi-in": "query",
						},
					},
					"required": []any{"city"},
				},
				OutputSchema: map[string]any{"type": "object"},
			},
			{
				DisplayName:  "Create weather alert",
				Description:  "Create a weather alert",
				OperationID:  "createWeatherAlert",
				Method:       "POST",
				Path:         "/weather/alerts",
				Enabled:      true,
				InputSchema:  map[string]any{"type": "object", "properties": map[string]any{}},
				OutputSchema: map[string]any{"type": "object"},
			},
		},
	})
	if err != nil {
		t.Fatalf("manual create failed: %v", err)
	}
	if detail.ID == "" || detail.ActionCount != 2 || detail.CreatedBy != 18 {
		t.Fatalf("unexpected detail: %+v", detail)
	}
	for _, name := range []string{"tool.json", "openapi.json", "actions.json", "source.openapi.json"} {
		if _, err := os.Stat(filepath.Join(root, "data", "tools", detail.ID, name)); err != nil {
			t.Fatalf("expected %s to be persisted: %v", name, err)
		}
	}
	var openAPI map[string]any
	if err := readJSON(filepath.Join(root, "data", "tools", detail.ID, "openapi.json"), &openAPI); err != nil {
		t.Fatalf("read openapi failed: %v", err)
	}
	paths, _ := openAPI["paths"].(map[string]any)
	pathItem, _ := paths["/weather"].(map[string]any)
	operation, _ := pathItem["get"].(map[string]any)
	if operation["operationId"] != "getWeather" {
		t.Fatalf("manual openapi not built: %+v", operation)
	}
	alertPathItem, _ := paths["/weather/alerts"].(map[string]any)
	alertOperation, _ := alertPathItem["post"].(map[string]any)
	if alertOperation["operationId"] != "createWeatherAlert" {
		t.Fatalf("second manual openapi action not built: %+v", alertOperation)
	}
	toolIndex, err := ListTools("", "工具")
	if err != nil {
		t.Fatalf("list by category failed: %v", err)
	}
	if len(toolIndex.Tools) != 1 || toolIndex.Tools[0].ID != detail.ID {
		t.Fatalf("expected manual tool in category result, got %+v", toolIndex.Tools)
	}

	_, err = CreateManualTool(ToolManualCreateOptions{
		Name:        "Bad Category",
		Description: "bad category tool",
		ServerURL:   "https://api.example.com",
		Category:    "other",
		Publish:     true,
		Action: ToolManualActionOptions{
			DisplayName: "Run",
			OperationID: "run",
			Method:      "GET",
			Path:        "/run",
			Enabled:     true,
		},
	})
	var appErr *ToolAppError
	if !errors.As(err, &appErr) || appErr.Code != "invalid_request" {
		t.Fatalf("expected invalid category, got %#v", err)
	}
}

func TestDownloadToolExcludesSecretsAndIncrementsCount(t *testing.T) {
	setupToolStoreTest(t)

	detail, err := UploadTool("weather.json", strings.NewReader(validOpenAPIJSON), int64(len(validOpenAPIJSON)), ToolUploadOptions{
		Publish:        true,
		Category:       "工具",
		AuthType:       "api_key",
		APIKeyLocation: "header",
		APIKeyName:     "X-API-Key",
		APIKeyValue:    "secret-value",
	})
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}
	zipPath, err := BuildToolDownload(detail.ID)
	if err != nil {
		t.Fatalf("download build failed: %v", err)
	}
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open zip failed: %v", err)
	}
	defer reader.Close()
	names := map[string]bool{}
	combined := bytes.Buffer{}
	for _, file := range reader.File {
		names[file.Name] = true
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open zip file failed: %v", err)
		}
		_, _ = combined.ReadFrom(rc)
		_ = rc.Close()
	}
	for _, name := range []string{"tool.json", "openapi.json", "actions.json", "source.openapi.json", "README.md"} {
		if !names[name] {
			t.Fatalf("zip missing %s", name)
		}
	}
	if strings.Contains(combined.String(), "secret-value") {
		t.Fatalf("zip leaked api key")
	}
	refreshed, err := GetToolDetail(detail.ID)
	if err != nil {
		t.Fatalf("get detail failed: %v", err)
	}
	if refreshed.DownloadCount != 1 {
		t.Fatalf("expected download count 1, got %d", refreshed.DownloadCount)
	}
	if _, err := json.Marshal(refreshed); err != nil {
		t.Fatalf("detail should be json serializable: %v", err)
	}
}

func TestUpdateToolConfigUpdatesAuthAndHeaders(t *testing.T) {
	root := setupToolStoreTest(t)

	detail, err := UploadTool("weather.json", strings.NewReader(validOpenAPIJSON), int64(len(validOpenAPIJSON)), ToolUploadOptions{
		Publish:        true,
		Category:       "工具",
		CreatedBy:      18,
		AuthType:       "api_key",
		APIKeyLocation: "header",
		APIKeyName:     "X-Old-Key",
		APIKeyValue:    "old-secret",
	})
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}

	updated, err := UpdateToolConfig(detail.ID, ToolUpdateConfigOptions{
		UserID:         18,
		Name:           "Weather Tool Updated",
		Description:    "updated description",
		ServerURL:      "https://weather.example.com",
		Category:       "商业",
		Visibility:     "private",
		AuthType:       "api_key",
		APIKeyLocation: "query",
		APIKeyName:     "token",
		APIKeyValue:    "new-secret",
		CommonHeaders:  []ToolHeader{{Name: "User-Agent", Value: "Coze/1.0"}},
	})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if updated.Name != "Weather Tool Updated" || updated.AuthType != "api_key" || updated.APIKeyLocation != "query" || updated.APIKeyName != "token" {
		t.Fatalf("unexpected updated detail: %+v", updated)
	}
	if len(updated.CommonHeaders) != 1 || updated.CommonHeaders[0].Name != "User-Agent" {
		t.Fatalf("unexpected headers: %+v", updated.CommonHeaders)
	}
	var secret map[string]string
	if err := readJSON(filepath.Join(root, "data", "tool-secrets", detail.ID+".json"), &secret); err != nil {
		t.Fatalf("read secret failed: %v", err)
	}
	if secret["api_key_value"] != "new-secret" || secret["api_key_name"] != "token" || secret["api_key_location"] != "query" {
		t.Fatalf("unexpected secret: %+v", secret)
	}

	_, err = UpdateToolConfig(detail.ID, ToolUpdateConfigOptions{
		UserID:      18,
		Name:        "Invalid Category",
		Description: "updated description",
		ServerURL:   "https://weather.example.com",
		Category:    "other",
	})
	var appErr *ToolAppError
	if !errors.As(err, &appErr) || appErr.Code != "invalid_request" {
		t.Fatalf("expected invalid category, got %#v", err)
	}

	_, err = UpdateToolConfig(detail.ID, ToolUpdateConfigOptions{
		UserID:      19,
		Name:        "Nope",
		Description: "updated description",
		ServerURL:   "https://weather.example.com",
		Category:    "工具",
	})
	if !errors.As(err, &appErr) || appErr.Code != "permission_denied" {
		t.Fatalf("expected permission denied, got %#v", err)
	}
}

func TestUpdateToolActionConfigUpdatesActionAndOpenAPI(t *testing.T) {
	setupToolStoreTest(t)

	detail, err := UploadTool("weather.json", strings.NewReader(validOpenAPIJSON), int64(len(validOpenAPIJSON)), ToolUploadOptions{
		Publish:   true,
		Category:  "工具",
		CreatedBy: 18,
	})
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}

	inputSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"city": map[string]any{
				"type":         "string",
				"description":  "city name",
				"x-openapi-in": "query",
			},
		},
		"required": []any{"city"},
	}
	updated, err := UpdateToolActionConfig(detail.ID, "getWeather", ToolActionUpdateConfigOptions{
		UserID:      18,
		DisplayName: "Get weather by city",
		Description: "Get weather by city",
		OperationID: "getWeatherByCity",
		Method:      "POST",
		Path:        "/weather/by-city",
		InputSchema: inputSchema,
		OutputSchema: map[string]any{
			"type": "object",
		},
		Enabled:   true,
		RiskLevel: "write",
	})
	if err != nil {
		t.Fatalf("update action failed: %v", err)
	}
	if len(updated.Actions) != 1 {
		t.Fatalf("expected one action, got %+v", updated.Actions)
	}
	action := updated.Actions[0]
	if action.OperationID != "getWeatherByCity" || action.Name != "getWeatherByCity" || action.Method != "POST" || action.Path != "/weather/by-city" {
		t.Fatalf("unexpected action: %+v", action)
	}

	var openAPI map[string]any
	if err := readJSON(filepath.Join(toolDir(detail.ID), "openapi.json"), &openAPI); err != nil {
		t.Fatalf("read openapi failed: %v", err)
	}
	paths, _ := openAPI["paths"].(map[string]any)
	pathItem, _ := paths["/weather/by-city"].(map[string]any)
	operation, _ := pathItem["post"].(map[string]any)
	if operation["operationId"] != "getWeatherByCity" {
		t.Fatalf("openapi not synced: %+v", operation)
	}
}
