package controller

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

const (
	smsPurposeLogin = "sms_login"
	smsPurposeBind  = "sms_bind"

	// App Store review credentials are intentionally limited to a reserved phone
	// number and can be disabled immediately after review through the environment.
	appStoreReviewLoginEnabledEnv = "APP_STORE_REVIEW_LOGIN_ENABLED"
	appStoreReviewPhoneE164       = "+8613800000000"
	appStoreReviewCode            = "1234"
	appStoreReviewDisplayName     = "App Store Review"
)

var (
	cnPhoneRe = regexp.MustCompile(`^(?:\+?86)?1\d{10}$`)

	smsSendMu      sync.Mutex
	smsSendByPhone = map[string]time.Time{}
	smsSendByIP    = map[string][]time.Time{}
)

func normalizeCNPhone(input string) (e164 string, phone11 string, ok bool) {
	raw := strings.TrimSpace(input)
	raw = strings.ReplaceAll(raw, " ", "")
	raw = strings.ReplaceAll(raw, "-", "")
	if raw == "" || !cnPhoneRe.MatchString(raw) {
		return "", "", false
	}
	// Strip leading + if present.
	raw = strings.TrimPrefix(raw, "+")
	// Strip leading 86 if present.
	raw = strings.TrimPrefix(raw, "86")
	if len(raw) != 11 || raw[0] != '1' {
		return "", "", false
	}
	return "+86" + raw, raw, true
}

func appStoreReviewLoginEnabled() bool {
	return common.GetEnvOrDefaultBool(appStoreReviewLoginEnabledEnv, false)
}

func isAppStoreReviewPhone(e164 string) bool {
	return e164 == appStoreReviewPhoneE164
}

func isAppStoreReviewLogin(e164, code string) bool {
	return appStoreReviewLoginEnabled() && isAppStoreReviewPhone(e164) && code == appStoreReviewCode
}

func canSendSMS(ip, phone string) (ok bool, msg string) {
	const (
		perPhoneMinInterval = 60 * time.Second
		ipWindow            = 10 * time.Minute
		ipMaxInWindow       = 10
	)
	now := time.Now()
	smsSendMu.Lock()
	defer smsSendMu.Unlock()

	if last, exists := smsSendByPhone[phone]; exists && now.Sub(last) < perPhoneMinInterval {
		remain := int(perPhoneMinInterval.Seconds() - now.Sub(last).Seconds())
		if remain < 1 {
			remain = 1
		}
		return false, fmt.Sprintf("请稍后再试（%d 秒后可重新发送）", remain)
	}

	h := smsSendByIP[ip]
	// purge
	kept := h[:0]
	for _, t := range h {
		if now.Sub(t) <= ipWindow {
			kept = append(kept, t)
		}
	}
	h = kept
	if len(h) >= ipMaxInWindow {
		return false, "请求过于频繁，请稍后再试"
	}
	// ok - record
	smsSendByPhone[phone] = now
	smsSendByIP[ip] = append(h, now)
	return true, ""
}

func smsSendFailureMessage(err error) string {
	if err == nil {
		return "短信发送失败"
	}
	errText := strings.ToUpper(err.Error())
	if strings.Contains(errText, "BUSINESS_LIMIT_CONTROL") {
		return "该手机号短信发送过于频繁，请稍后再试"
	}
	return "短信发送失败"
}

type smsSendRequest struct {
	Phone   string `json:"phone"`
	Purpose string `json:"purpose"`
}

// SendSMSCode sends login/bind verification code to CN phone (+86).
func SendSMSCode(c *gin.Context) {
	if !common.SMSLoginEnabled {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "短信登录未启用"})
		return
	}
	var req smsSendRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	e164, phone11, ok := normalizeCNPhone(req.Phone)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的手机号"})
		return
	}
	purpose := strings.TrimSpace(req.Purpose)
	if purpose == "" {
		purpose = smsPurposeLogin
	}
	if purpose != smsPurposeLogin && purpose != smsPurposeBind {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的用途"})
		return
	}
	if purpose == smsPurposeLogin && isAppStoreReviewPhone(e164) {
		if !appStoreReviewLoginEnabled() {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "审核登录未启用"})
			return
		}
		// App Store review account: never create an OTP or send an SMS.
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"is_new_user": false}})
		return
	}

	if allow, msg := canSendSMS(c.ClientIP(), e164); !allow {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": msg})
		return
	}

	code := common.GenerateNumericVerificationCode(common.SMSVerificationCodeLength)
	common.SetSMSCode(purpose, e164, code)
	if err := service.SendAliyunLoginSMS(c.Request.Context(), phone11, code); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("sms send failed phone=%s err=%v", maskPhone(phone11), err))
		msg := smsSendFailureMessage(err)
		if strings.HasPrefix(err.Error(), "aliyun sms not configured:") {
			// Tell admin what is missing; no secrets are included.
			missing := strings.TrimSpace(strings.TrimPrefix(err.Error(), "aliyun sms not configured:"))
			msg = fmt.Sprintf("短信未配置（%s），请在系统设置中完善短信签名/模板等配置", missing)
		}
		c.JSON(http.StatusOK, gin.H{"success": false, "message": msg})
		return
	}

	data := gin.H{}
	if purpose == smsPurposeLogin {
		data["is_new_user"] = !model.IsPhoneAlreadyTaken(e164)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": data})
}

type smsLoginRequest struct {
	Phone   string `json:"phone"`
	Code    string `json:"code"`
	AffCode string `json:"aff_code"`
}

// SMSLogin verifies code then login/register user automatically.
func SMSLogin(c *gin.Context) {
	if !common.SMSLoginEnabled {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "短信登录未启用"})
		return
	}
	var req smsLoginRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	e164, _, ok := normalizeCNPhone(req.Phone)
	code := strings.TrimSpace(req.Code)
	if !ok || code == "" {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	reviewPhone := isAppStoreReviewPhone(e164)
	if reviewPhone {
		if !isAppStoreReviewLogin(e164, code) {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "验证码错误或已过期"})
			return
		}
	} else {
		okCode, locked := common.VerifySMSCode(smsPurposeLogin, e164, code)
		if locked {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "验证码尝试次数过多，请稍后再试"})
			return
		}
		if !okCode {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "验证码错误或已过期"})
			return
		}
	}

	var user model.User
	err := model.DB.Where("phone = ?", e164).First(&user).Error
	if err != nil {
		inviterId := 0
		if !reviewPhone {
			affCode := strings.TrimSpace(req.AffCode)
			if affCode != "" {
				inviterId, _ = model.GetUserIdByAffCode(affCode)
			}
		}
		// auto register
		displayName := ""
		if reviewPhone {
			displayName = appStoreReviewDisplayName
		}
		u, err2 := createUserWithPhone(e164, inviterId, displayName)
		if err2 != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("sms auto register failed phone=%s err=%v", maskE164(e164), err2))
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "登录失败，请稍后再试"})
			return
		}
		if _, err2 = ensureUserAccessToken(u); err2 != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("sms auto register access token failed phone=%s err=%v", maskE164(e164), err2))
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "登录失败，请稍后再试"})
			return
		}
		tokenName := u.DisplayName + "的初始令牌"
		if tokenName == "的初始令牌" {
			tokenName = u.Username + "的初始令牌"
		}
		if err2 = createDefaultApiTokenIfEnabled(u.Id, tokenName); err2 != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("sms auto register default token failed phone=%s err=%v", maskE164(e164), err2))
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "登录失败，请稍后再试"})
			return
		}
		user = *u
	}

	if user.Status != common.UserStatusEnabled {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "用户已被禁用"})
		return
	}

	setupLogin(&user, c)
}

type smsBindRequest struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
}

// BindPhone binds a phone to current user. Not required for SMS login.
func BindPhone(c *gin.Context) {
	if !common.SMSLoginEnabled {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "短信登录未启用"})
		return
	}
	var req smsBindRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	e164, _, ok := normalizeCNPhone(req.Phone)
	if !ok || strings.TrimSpace(req.Code) == "" {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	okCode, locked := common.VerifySMSCode(smsPurposeBind, e164, strings.TrimSpace(req.Code))
	if locked {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "验证码尝试次数过多，请稍后再试"})
		return
	}
	if !okCode {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "验证码错误或已过期"})
		return
	}
	if model.IsPhoneAlreadyTaken(e164) {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "手机号已被占用"})
		return
	}

	id := c.GetInt("id")
	user := model.User{Id: id}
	if err := user.FillUserById(); err != nil || user.Id == 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "用户不存在"})
		return
	}
	user.Phone = &e164
	if err := user.Update(false); err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func createUserWithPhone(e164 string, inviterId int, displayName string) (*model.User, error) {
	last4 := e164
	if len(e164) >= 4 {
		last4 = e164[len(e164)-4:]
	}
	for i := 0; i < 5; i++ {
		username := fmt.Sprintf("m%s%s", last4, common.GetRandomString(4))
		username = strings.ToLower(username)
		display := fmt.Sprintf("M%s", last4)
		if strings.TrimSpace(displayName) != "" {
			display = displayName
		}
		pwd := common.GetRandomString(12) + "A1!"
		u := &model.User{
			Username:    username,
			Password:    pwd,
			DisplayName: display,
			Role:        common.RoleCommonUser,
			Status:      common.UserStatusEnabled,
			Group:       "default",
			Phone:       &e164,
		}
		if err := u.Insert(inviterId); err == nil {
			_ = model.DB.Where("id = ?", u.Id).First(u).Error
			return u, nil
		}
	}
	return nil, fmt.Errorf("failed to create user")
}

func maskPhone(phone11 string) string {
	if len(phone11) != 11 {
		return "***"
	}
	return phone11[:3] + "****" + phone11[7:]
}

func maskE164(e164 string) string {
	if len(e164) < 6 {
		return "***"
	}
	return e164[:4] + "****" + e164[len(e164)-2:]
}
