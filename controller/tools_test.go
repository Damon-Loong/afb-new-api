package controller

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

const toolControllerOpenAPIJSON = `{
  "openapi": "3.0.3",
  "info": {"title": "Controller Weather", "description": "查询城市天气", "version": "1.0.0"},
  "servers": [{"url": "https://api.example.com"}],
  "paths": {
    "/weather": {
      "get": {
        "operationId": "getWeather",
        "summary": "查询天气",
        "description": "根据城市查询天气",
        "parameters": [{"name": "city", "in": "query", "required": true, "schema": {"type": "string"}}],
        "responses": {"200": {"description": "ok", "content": {"application/json": {"schema": {"type": "object"}}}}}
      }
    }
  }
}`

type toolControllerAPIResponse struct {
	Success bool            `json:"success"`
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func setupToolControllerRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	t.Setenv("TOOL_DATA_DIR", filepath.Join(root, "data", "tools"))
	t.Setenv("TOOL_SECRET_DIR", filepath.Join(root, "data", "tool-secrets"))
	router := gin.New()
	tools := router.Group("/api/tools")
	tools.GET("", GetTools)
	tools.POST("/parse", ParseToolOpenAPI)
	tools.POST("/upload", UploadToolOpenAPI)
	tools.POST("/manual", CreateManualTool)
	tools.GET("/check-name", CheckToolName)
	tools.GET("/:tool_id/download", DownloadTool)
	tools.GET("/:tool_id", GetTool)
	return router
}

func multipartRequest(t *testing.T, method string, path string, field string, fileName string, content string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(field, fileName)
	if err != nil {
		t.Fatalf("create form file failed: %v", err)
	}
	if _, err := io.WriteString(part, content); err != nil {
		t.Fatalf("write form file failed: %v", err)
	}
	if err := writer.WriteField("category", "工具"); err != nil {
		t.Fatalf("write category field failed: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart failed: %v", err)
	}
	req := httptest.NewRequest(method, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func decodeToolControllerResponse(t *testing.T, recorder *httptest.ResponseRecorder) toolControllerAPIResponse {
	t.Helper()
	var response toolControllerAPIResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response failed: %v, body=%s", err, recorder.Body.String())
	}
	return response
}

func TestToolControllerUploadListDetailDownload(t *testing.T) {
	router := setupToolControllerRouter(t)

	parseRecorder := httptest.NewRecorder()
	router.ServeHTTP(parseRecorder, multipartRequest(t, http.MethodPost, "/api/tools/parse", "file", "weather.json", toolControllerOpenAPIJSON))
	parseResponse := decodeToolControllerResponse(t, parseRecorder)
	if !parseResponse.Success {
		t.Fatalf("parse failed: %+v", parseResponse)
	}

	uploadRecorder := httptest.NewRecorder()
	router.ServeHTTP(uploadRecorder, multipartRequest(t, http.MethodPost, "/api/tools/upload", "file", "weather.json", toolControllerOpenAPIJSON))
	uploadResponse := decodeToolControllerResponse(t, uploadRecorder)
	if !uploadResponse.Success {
		t.Fatalf("upload failed: %+v", uploadResponse)
	}
	var uploadData struct {
		ID          string `json:"id"`
		ActionCount int    `json:"action_count"`
	}
	if err := json.Unmarshal(uploadResponse.Data, &uploadData); err != nil {
		t.Fatalf("decode upload data failed: %v", err)
	}
	if uploadData.ID == "" || uploadData.ActionCount != 1 {
		t.Fatalf("unexpected upload data: %+v", uploadData)
	}

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, httptest.NewRequest(http.MethodGet, "/api/tools?scope=recommended", nil))
	listResponse := decodeToolControllerResponse(t, listRecorder)
	if !listResponse.Success || !bytes.Contains(listResponse.Data, []byte("Controller Weather")) {
		t.Fatalf("unexpected list response: %+v", listResponse)
	}

	detailRecorder := httptest.NewRecorder()
	router.ServeHTTP(detailRecorder, httptest.NewRequest(http.MethodGet, "/api/tools/"+uploadData.ID, nil))
	detailResponse := decodeToolControllerResponse(t, detailRecorder)
	if !detailResponse.Success || bytes.Contains(detailResponse.Data, []byte("api_key_value")) {
		t.Fatalf("unexpected detail response: %+v", detailResponse)
	}

	downloadRecorder := httptest.NewRecorder()
	router.ServeHTTP(downloadRecorder, httptest.NewRequest(http.MethodGet, "/api/tools/"+uploadData.ID+"/download", nil))
	if downloadRecorder.Code != http.StatusOK {
		t.Fatalf("download status = %d body=%s", downloadRecorder.Code, downloadRecorder.Body.String())
	}
	zipReader, err := zip.NewReader(bytes.NewReader(downloadRecorder.Body.Bytes()), int64(downloadRecorder.Body.Len()))
	if err != nil {
		t.Fatalf("download is not zip: %v", err)
	}
	names := map[string]bool{}
	for _, file := range zipReader.File {
		names[file.Name] = true
	}
	for _, name := range []string{"tool.json", "openapi.json", "actions.json", "source.openapi.json", "README.md"} {
		if !names[name] {
			t.Fatalf("download missing %s", name)
		}
	}
}

func TestToolControllerRejectsDuplicateName(t *testing.T) {
	router := setupToolControllerRouter(t)
	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, multipartRequest(t, http.MethodPost, "/api/tools/upload", "file", "weather.json", toolControllerOpenAPIJSON))
		response := decodeToolControllerResponse(t, recorder)
		if i == 0 && !response.Success {
			t.Fatalf("first upload failed: %+v", response)
		}
		if i == 1 && (response.Success || response.Code != "tool_name_conflict") {
			t.Fatalf("expected duplicate name rejection, got %+v", response)
		}
	}
}

func TestToolControllerManualCreate(t *testing.T) {
	router := setupToolControllerRouter(t)
	payload := map[string]any{
		"name":        "Manual Weather",
		"description": "manually created weather tool",
		"server_url":  "https://api.example.com",
		"category":    "工具",
		"visibility":  "public",
		"publish":     true,
		"auth_type":   "none",
		"actions": []any{
			map[string]any{
				"display_name": "Get weather",
				"description":  "Get weather by city",
				"operation_id": "getWeather",
				"method":       "GET",
				"path":         "/weather",
				"enabled":      true,
				"input_schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"city": map[string]any{
							"type":         "string",
							"x-openapi-in": "query",
						},
					},
					"required": []any{"city"},
				},
				"output_schema": map[string]any{"type": "object"},
			},
			map[string]any{
				"display_name":  "Create alert",
				"description":   "Create weather alert",
				"operation_id":  "createAlert",
				"method":        "POST",
				"path":          "/weather/alerts",
				"enabled":       true,
				"input_schema":  map[string]any{"type": "object", "properties": map[string]any{}},
				"output_schema": map[string]any{"type": "object"},
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload failed: %v", err)
	}
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/tools/manual", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)
	response := decodeToolControllerResponse(t, recorder)
	if !response.Success {
		t.Fatalf("manual create failed: %+v", response)
	}
	var data struct {
		ID          string `json:"id"`
		ActionCount int    `json:"action_count"`
	}
	if err := json.Unmarshal(response.Data, &data); err != nil {
		t.Fatalf("decode manual create data failed: %v", err)
	}
	if data.ID == "" || data.ActionCount != 2 {
		t.Fatalf("unexpected manual create data: %+v", data)
	}
}
