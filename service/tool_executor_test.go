package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
)

func TestRunUserToolActionRecordsRun(t *testing.T) {
	setupUserToolStoreTest(t)

	var gotSecret string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSecret = r.Header.Get("X-Tool-Key")
		if r.URL.Path != "/weather" || r.URL.Query().Get("city") != "shanghai" {
			t.Fatalf("unexpected request: path=%s query=%s", r.URL.Path, r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"temp": 21, "city": "shanghai"})
	}))
	defer server.Close()

	openAPI := strings.Replace(validOpenAPIJSON, "https://api.example.com", server.URL, 1)
	detail, err := UploadTool("weather.json", strings.NewReader(openAPI), int64(len(openAPI)), ToolUploadOptions{
		Publish:        true,
		AuthType:       "api_key",
		APIKeyLocation: "header",
		APIKeyName:     "X-Tool-Key",
		APIKeyValue:    "secret-value",
	})
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}
	if _, err := InstallUserTool(1001, detail.ID); err != nil {
		t.Fatalf("install failed: %v", err)
	}

	result, err := RunUserToolAction(1001, false, detail.ID, "getWeather", ToolActionRunRequest{
		Arguments: map[string]any{"city": "shanghai"},
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if gotSecret != "secret-value" {
		t.Fatalf("expected secret header, got %q", gotSecret)
	}
	if result.Status != "success" || result.HTTPStatus != http.StatusOK || result.RunID == 0 {
		t.Fatalf("unexpected result: %+v", result)
	}

	var runs []model.ToolRun
	if err := model.ToolDB.Find(&runs).Error; err != nil {
		t.Fatalf("list tool runs failed: %v", err)
	}
	if len(runs) != 1 || runs[0].Status != "success" || runs[0].FunctionName != "getWeather" {
		t.Fatalf("unexpected tool runs: %+v", runs)
	}
}
