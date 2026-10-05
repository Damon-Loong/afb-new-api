package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

func TestGetUserModelsByGroup(t *testing.T) {
	db, router := setupSMSAuthTest(t)
	// Initialize the database dialect's quoted column names without migrating.
	previousPath, previousMaster := common.SQLitePath, common.IsMasterNode
	common.SQLitePath = filepath.Join(t.TempDir(), "dialect.db")
	common.IsMasterNode = false
	t.Setenv("SQL_DSN", "local")
	err := model.InitDB()
	common.SQLitePath, common.IsMasterNode = previousPath, previousMaster
	if err != nil {
		t.Fatal(err)
	}
	initializedDB, err := model.DB.DB()
	model.DB = db
	if err != nil {
		t.Fatal(err)
	}
	_ = initializedDB.Close()
	groupsBefore := setting.UserUsableGroups2JSONString()
	autoBefore := setting.AutoGroups2JsonString()
	t.Cleanup(func() {
		_ = setting.UpdateUserUsableGroupsByJSONString(groupsBefore)
		_ = setting.UpdateAutoGroupsByJsonString(autoBefore)
	})
	if err := setting.UpdateUserUsableGroupsByJSONString(`{"default":"","vip":"","empty":"","auto":""}`); err != nil {
		t.Fatal(err)
	}
	if err := setting.UpdateAutoGroupsByJsonString(`["default","secret"]`); err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Ability{}); err != nil {
		t.Fatal(err)
	}
	user := model.User{Username: "models_by_group", Group: "default"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	for i, group := range []string{"default", "vip", "secret"} {
		if err := db.Create(&model.Ability{Group: group, Model: group + "-model", ChannelId: i + 1, Enabled: true}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&model.Ability{Group: "default", Model: "disabled-model", ChannelId: 4, Enabled: false}).Error; err != nil {
		t.Fatal(err)
	}
	router.GET("/api/user/models", func(c *gin.Context) {
		c.Set("id", user.Id)
		GetUserModels(c)
	})
	for _, tc := range []struct {
		group   string
		want    []string
		success bool
	}{
		{"default", []string{"default-model"}, true},
		{"vip", []string{"vip-model"}, true},
		{"empty", []string{}, true},
		{"auto", []string{"default-model"}, true},
		{"secret", nil, false},
		{"", []string{"default-model", "vip-model"}, true},
	} {
		t.Run(tc.group, func(t *testing.T) {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/user/models?group="+tc.group, nil))
			var response struct {
				Success bool     `json:"success"`
				Data    []string `json:"data"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.Success != tc.success || len(response.Data) != len(tc.want) {
				t.Fatalf("unexpected response: %s", w.Body.String())
			}
			if tc.success && response.Data == nil {
				t.Fatal("model list must be an array, including for empty groups")
			}
			for _, name := range tc.want {
				found := false
				for _, got := range response.Data {
					if got == name {
						found = true
					}
				}
				if !found {
					t.Fatalf("missing model %s: %v", name, response.Data)
				}
			}
		})
	}
}
