package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type smsAuthTestResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type smsAuthLoginData struct {
	ID          int    `json:"id"`
	DisplayName string `json:"display_name"`
	AccessToken string `json:"access_token"`
}

func setupSMSAuthTest(t *testing.T) (*gorm.DB, *gin.Engine) {
	t.Helper()

	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousSMSLoginEnabled := common.SMSLoginEnabled
	previousUsingSQLite := common.UsingSQLite
	previousUsingMySQL := common.UsingMySQL
	previousUsingPostgreSQL := common.UsingPostgreSQL
	previousRedisEnabled := common.RedisEnabled
	previousQuotaForNewUser := common.QuotaForNewUser
	previousGenerateDefaultToken := constant.GenerateDefaultToken
	previousDefaultUseAutoGroup := setting.DefaultUseAutoGroup

	gin.SetMode(gin.TestMode)
	common.SMSLoginEnabled = true
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.QuotaForNewUser = 0
	constant.GenerateDefaultToken = true
	setting.DefaultUseAutoGroup = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Token{}); err != nil {
		t.Fatalf("migrate sms auth tables: %v", err)
	}
	model.DB = db
	model.LOG_DB = db

	router := gin.New()
	router.Use(sessions.Sessions("sms-auth-test", cookie.NewStore([]byte("sms-auth-test-secret"))))
	router.POST("/api/auth/sms/send", SendSMSCode)
	router.POST("/api/auth/sms/login", SMSLogin)

	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.SMSLoginEnabled = previousSMSLoginEnabled
		common.UsingSQLite = previousUsingSQLite
		common.UsingMySQL = previousUsingMySQL
		common.UsingPostgreSQL = previousUsingPostgreSQL
		common.RedisEnabled = previousRedisEnabled
		common.QuotaForNewUser = previousQuotaForNewUser
		constant.GenerateDefaultToken = previousGenerateDefaultToken
		setting.DefaultUseAutoGroup = previousDefaultUseAutoGroup
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	})

	return db, router
}

func callSMSAuthEndpoint(t *testing.T, router *gin.Engine, path string, body any) smsAuthTestResponse {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", recorder.Code, recorder.Body.String())
	}

	var response smsAuthTestResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, recorder.Body.String())
	}
	return response
}

func TestAppStoreReviewSMSDoesNotSendOrCreateOTP(t *testing.T) {
	_, router := setupSMSAuthTest(t)
	t.Setenv(appStoreReviewLoginEnabledEnv, "true")

	response := callSMSAuthEndpoint(t, router, "/api/auth/sms/send", map[string]string{
		"phone":   "13800000000",
		"purpose": smsPurposeLogin,
	})
	if !response.Success {
		t.Fatalf("expected review SMS request to succeed, got %q", response.Message)
	}

	if ok, _ := common.VerifySMSCode(smsPurposeLogin, appStoreReviewPhoneE164, appStoreReviewCode); ok {
		t.Fatal("review SMS request must not create a regular OTP")
	}
}

func TestAppStoreReviewLoginCreatesUserAndReturnsAccessToken(t *testing.T) {
	db, router := setupSMSAuthTest(t)
	t.Setenv(appStoreReviewLoginEnabledEnv, "true")

	response := callSMSAuthEndpoint(t, router, "/api/auth/sms/login", map[string]string{
		"phone": "13800000000",
		"code":  appStoreReviewCode,
	})
	if !response.Success {
		t.Fatalf("expected review login to succeed, got %q", response.Message)
	}

	var data smsAuthLoginData
	if err := json.Unmarshal(response.Data, &data); err != nil {
		t.Fatalf("decode login data: %v", err)
	}
	if data.ID == 0 || data.AccessToken == "" {
		t.Fatalf("expected user ID and access token, got %+v", data)
	}
	if data.DisplayName != appStoreReviewDisplayName {
		t.Fatalf("expected display name %q, got %q", appStoreReviewDisplayName, data.DisplayName)
	}

	var user model.User
	if err := db.Where("phone = ?", appStoreReviewPhoneE164).First(&user).Error; err != nil {
		t.Fatalf("load review user: %v", err)
	}
	if user.Role != common.RoleCommonUser || user.Status != common.UserStatusEnabled {
		t.Fatalf("expected enabled common user, got role=%d status=%d", user.Role, user.Status)
	}
	if user.GetAccessToken() == "" {
		t.Fatal("expected persisted access token")
	}

	var tokenCount int64
	if err := db.Model(&model.Token{}).Where("user_id = ?", user.Id).Count(&tokenCount).Error; err != nil {
		t.Fatalf("count default API token: %v", err)
	}
	if tokenCount != 1 {
		t.Fatalf("expected one default API token, got %d", tokenCount)
	}
}

func TestAppStoreReviewLoginRejectsWrongCodeAndDisabledFlag(t *testing.T) {
	db, router := setupSMSAuthTest(t)
	t.Setenv(appStoreReviewLoginEnabledEnv, "true")

	wrongCode := callSMSAuthEndpoint(t, router, "/api/auth/sms/login", map[string]string{
		"phone": "13800000000",
		"code":  "0000",
	})
	if wrongCode.Success {
		t.Fatal("expected review login with wrong code to fail")
	}

	t.Setenv(appStoreReviewLoginEnabledEnv, "false")
	disabled := callSMSAuthEndpoint(t, router, "/api/auth/sms/login", map[string]string{
		"phone": "13800000000",
		"code":  appStoreReviewCode,
	})
	if disabled.Success {
		t.Fatal("expected review login to fail when disabled")
	}

	var count int64
	if err := db.Model(&model.User{}).Where("phone = ?", appStoreReviewPhoneE164).Count(&count).Error; err != nil {
		t.Fatalf("count review users: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no review user after failed logins, got %d", count)
	}
}

func TestSMSLoginStillRequiresOTPForRegularPhone(t *testing.T) {
	_, router := setupSMSAuthTest(t)
	t.Setenv(appStoreReviewLoginEnabledEnv, "true")

	regularPhone := "+8613900000000"
	failed := callSMSAuthEndpoint(t, router, "/api/auth/sms/login", map[string]string{
		"phone": "13900000000",
		"code":  appStoreReviewCode,
	})
	if failed.Success {
		t.Fatal("expected regular phone to reject the review code without an OTP")
	}

	common.SetSMSCode(smsPurposeLogin, regularPhone, "5678")
	success := callSMSAuthEndpoint(t, router, "/api/auth/sms/login", map[string]string{
		"phone": "13900000000",
		"code":  "5678",
	})
	if !success.Success {
		t.Fatalf("expected regular phone login with a valid OTP to succeed, got %q", success.Message)
	}
}
