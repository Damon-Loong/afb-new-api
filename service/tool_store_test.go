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

	detail, err := UploadTool("weather.json", strings.NewReader(validOpenAPIJSON), int64(len(validOpenAPIJSON)), ToolUploadOptions{Publish: true})
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
	index, err := ListTools("")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(index.Tools) != 1 || index.Tools[0].Name != "Weather Tool" {
		t.Fatalf("unexpected index: %+v", index)
	}
	_, err = UploadTool("weather.json", strings.NewReader(validOpenAPIJSON), int64(len(validOpenAPIJSON)), ToolUploadOptions{Publish: true})
	var appErr *ToolAppError
	if !errors.As(err, &appErr) || appErr.Code != "tool_name_conflict" {
		t.Fatalf("expected name conflict, got %#v", err)
	}
}

func TestDownloadToolExcludesSecretsAndIncrementsCount(t *testing.T) {
	setupToolStoreTest(t)

	detail, err := UploadTool("weather.json", strings.NewReader(validOpenAPIJSON), int64(len(validOpenAPIJSON)), ToolUploadOptions{
		Publish:        true,
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
