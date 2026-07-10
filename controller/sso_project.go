package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const ssoTicketTTL = 60 * time.Second

const ssoTicketPurposeLogin = "login"

type ssoTicketPayload struct {
	ProjectKey string `json:"project_key"`
	UserID     int    `json:"user_id"`
	ReturnTo   string `json:"return_to"`
	Origin     string `json:"origin"`
	State      string `json:"state"`
	Purpose    string `json:"purpose"`
	ExpiresAt  int64  `json:"expires_at"`
}

var (
	ssoTicketMemoryMu sync.Mutex
	ssoTicketMemory   = map[string]ssoTicketPayload{}
)

type ssoProjectPayload struct {
	Key           string `json:"key"`
	Name          string `json:"name"`
	OfficialURL   string `json:"official_url"`
	Description   string `json:"description"`
	IconURL       string `json:"icon_url"`
	BillingSecret string `json:"billing_secret"`
	Enabled       *bool  `json:"enabled"`
	SSOEnabled    *bool  `json:"sso_enabled"`
	Sort          int    `json:"sort"`
}

type ssoExchangeRequest struct {
	Project string `json:"project"`
	Ticket  string `json:"ticket"`
	State   string `json:"state"`
}

type ssoTicketRequest struct {
	Project  string `json:"project"`
	ReturnTo string `json:"return_to"`
}

func normalizeSSOProjectPayload(payload ssoProjectPayload, partial bool) (*model.SSOProject, map[string]interface{}, error) {
	key := strings.TrimSpace(payload.Key)
	name := strings.TrimSpace(payload.Name)
	officialURL := strings.TrimSpace(payload.OfficialURL)
	description := strings.TrimSpace(payload.Description)
	iconURL := strings.TrimSpace(payload.IconURL)
	billingSecret := strings.TrimSpace(payload.BillingSecret)

	if !partial || key != "" {
		if !model.IsProjectKeyValid(key) {
			return nil, nil, errors.New("项目标识只能包含小写字母、数字、短横线或下划线，长度 1-64")
		}
	}
	if !partial || name != "" {
		if name == "" || len([]rune(name)) > 128 {
			return nil, nil, errors.New("项目名称不能为空，且不能超过 128 个字符")
		}
	}
	if !partial || officialURL != "" {
		if _, err := model.URLOrigin(officialURL); err != nil {
			return nil, nil, errors.New("官网 URL 必须是有效的 http(s) 地址")
		}
	}
	if iconURL != "" {
		if _, err := model.URLOrigin(iconURL); err != nil {
			return nil, nil, errors.New("图标 URL 必须是有效的 http(s) 地址")
		}
	}

	enabled := true
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	}
	ssoEnabled := true
	if payload.SSOEnabled != nil {
		ssoEnabled = *payload.SSOEnabled
	}

	project := &model.SSOProject{
		Key:           key,
		Name:          name,
		OfficialURL:   officialURL,
		Description:   description,
		IconURL:       iconURL,
		BillingSecret: billingSecret,
		Enabled:       enabled,
		SSOEnabled:    ssoEnabled,
		Sort:          payload.Sort,
	}

	updates := map[string]interface{}{}
	if key != "" {
		updates["project_key"] = key
	}
	if name != "" {
		updates["name"] = name
	}
	if officialURL != "" {
		updates["official_url"] = officialURL
	}
	updates["description"] = description
	updates["icon_url"] = iconURL
	updates["billing_secret"] = billingSecret
	if payload.Enabled != nil {
		updates["enabled"] = *payload.Enabled
	}
	if payload.SSOEnabled != nil {
		updates["sso_enabled"] = *payload.SSOEnabled
	}
	updates["sort"] = payload.Sort

	return project, updates, nil
}

func AdminListSSOProjects(c *gin.Context) {
	projects, err := model.ListSSOProjects(true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": projects})
}

func ListAvailableSSOProjects(c *gin.Context) {
	projects, err := model.ListSSOProjects(false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	available := make([]gin.H, 0, len(projects))
	for _, project := range projects {
		if project.SSOEnabled {
			available = append(available, gin.H{
				"id":           project.ID,
				"key":          project.Key,
				"name":         project.Name,
				"official_url": project.OfficialURL,
				"description":  project.Description,
				"icon_url":     project.IconURL,
				"enabled":      project.Enabled,
				"sso_enabled":  project.SSOEnabled,
				"sort":         project.Sort,
				"created_at":   project.CreatedAt,
				"updated_at":   project.UpdatedAt,
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": available})
}

func AdminCreateSSOProject(c *gin.Context) {
	var payload ssoProjectPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		common.ApiError(c, err)
		return
	}
	project, _, err := normalizeSSOProjectPayload(payload, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	exists, err := model.SSOProjectExistsWithKey(project.Key, 0)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if exists {
		common.ApiError(c, errors.New("项目标识已存在"))
		return
	}
	if err := model.CreateSSOProject(project); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": project})
}

func AdminUpdateSSOProject(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiError(c, errors.New("项目 ID 无效"))
		return
	}
	var payload ssoProjectPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		common.ApiError(c, err)
		return
	}
	_, updates, err := normalizeSSOProjectPayload(payload, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if key, ok := updates["project_key"].(string); ok && key != "" {
		exists, err := model.SSOProjectExistsWithKey(key, id)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if exists {
			common.ApiError(c, errors.New("项目标识已存在"))
			return
		}
	}
	if err := model.UpdateSSOProject(id, updates); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func AdminDeleteSSOProject(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiError(c, errors.New("项目 ID 无效"))
		return
	}
	if err := model.DeleteSSOProject(id); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func HandleSSOLogin(c *gin.Context) {
	projectKey := strings.TrimSpace(c.Query("project"))
	returnTo := strings.TrimSpace(c.Query("return_to"))
	if projectKey == "" || returnTo == "" {
		c.String(http.StatusBadRequest, "missing project or return_to")
		return
	}

	project, err := validateSSOReturn(projectKey, returnTo)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	session := sessions.Default(c)
	userIDRaw := session.Get("id")
	userID, ok := userIDRaw.(int)
	if !ok || userID <= 0 {
		redirect := "/login?redirect=" + url.QueryEscape(c.Request.URL.RequestURI())
		c.Redirect(http.StatusFound, redirect)
		return
	}

	redirectURL, err := createSSORedirect(project, userID, returnTo)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Redirect(http.StatusFound, redirectURL)
}

func CreateSSOTicket(c *gin.Context) {
	var req ssoTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	projectKey := strings.TrimSpace(req.Project)
	returnTo := strings.TrimSpace(req.ReturnTo)
	if projectKey == "" || returnTo == "" {
		common.ApiError(c, errors.New("project 和 return_to 不能为空"))
		return
	}
	project, err := validateSSOReturn(projectKey, returnTo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	redirectURL, err := createSSORedirect(project, c.GetInt("id"), returnTo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"redirect_url": redirectURL,
		},
	})
}

func ExchangeSSOTicket(c *gin.Context) {
	var req ssoExchangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	projectKey := strings.TrimSpace(req.Project)
	ticket := strings.TrimSpace(req.Ticket)
	if projectKey == "" || ticket == "" {
		common.ApiError(c, errors.New("project 和 ticket 不能为空"))
		return
	}

	payload, err := consumeSSOTicket(ticket)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if payload.ProjectKey != projectKey {
		common.ApiError(c, errors.New("ticket 项目不匹配"))
		return
	}
	if payload.Purpose != "" && payload.Purpose != ssoTicketPurposeLogin {
		common.ApiError(c, errors.New("ticket purpose mismatch"))
		return
	}
	if req.State != "" && payload.State != "" && req.State != payload.State {
		common.ApiError(c, errors.New("state 不匹配"))
		return
	}
	if time.Now().Unix() > payload.ExpiresAt {
		common.ApiError(c, errors.New("ticket 已过期"))
		return
	}

	user, err := model.GetUserById(payload.UserID, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	accessToken, err := ensureUserAccessToken(user)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":           user.Id,
			"username":     user.Username,
			"display_name": user.DisplayName,
			"phone":        user.Phone,
			"role":         user.Role,
			"status":       user.Status,
			"group":        user.Group,
			"access_token": accessToken,
		},
	})
}

func validateSSOReturn(projectKey, returnTo string) (*model.SSOProject, error) {
	project, err := model.GetSSOProjectByKey(projectKey)
	if err != nil {
		if model.IsRecordNotFound(err) {
			return nil, errors.New("项目不存在")
		}
		return nil, err
	}
	if !project.Enabled {
		return nil, errors.New("项目未启用")
	}
	if !project.SSOEnabled {
		return nil, errors.New("项目未开启 SSO")
	}
	projectOrigin, err := project.Origin()
	if err != nil {
		return nil, errors.New("项目官网 URL 配置无效")
	}
	returnOrigin, err := model.URLOrigin(returnTo)
	if err != nil {
		return nil, errors.New("return_to 无效")
	}
	if projectOrigin != returnOrigin {
		return nil, errors.New("return_to 不在项目白名单中")
	}
	return project, nil
}

func createSSORedirect(project *model.SSOProject, userID int, returnTo string) (string, error) {
	ticket, err := common.GenerateRandomKey(32)
	if err != nil {
		return "", err
	}
	state, err := common.GenerateRandomKey(16)
	if err != nil {
		return "", err
	}
	origin, err := model.URLOrigin(returnTo)
	if err != nil {
		return "", err
	}
	payload := ssoTicketPayload{
		ProjectKey: project.Key,
		UserID:     userID,
		ReturnTo:   returnTo,
		Origin:     origin,
		State:      state,
		Purpose:    ssoTicketPurposeLogin,
		ExpiresAt:  time.Now().Add(ssoTicketTTL).Unix(),
	}
	if err := storeSSOTicket(ticket, payload); err != nil {
		return "", err
	}
	parsed, err := url.Parse(returnTo)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("ticket", ticket)
	query.Set("state", state)
	query.Set("project", project.Key)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func storeSSOTicket(ticket string, payload ssoTicketPayload) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if common.RedisEnabled && common.RDB != nil {
		return common.RDB.Set(context.Background(), "sso:ticket:"+ticket, string(raw), ssoTicketTTL).Err()
	}
	ssoTicketMemoryMu.Lock()
	defer ssoTicketMemoryMu.Unlock()
	ssoTicketMemory[ticket] = payload
	return nil
}

func consumeSSOTicket(ticket string) (ssoTicketPayload, error) {
	key := "sso:ticket:" + ticket
	if common.RedisEnabled && common.RDB != nil {
		ctx := context.Background()
		raw, err := common.RDB.Get(ctx, key).Result()
		if err != nil {
			return ssoTicketPayload{}, errors.New("ticket 无效或已过期")
		}
		_ = common.RDB.Del(ctx, key).Err()
		var payload ssoTicketPayload
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return ssoTicketPayload{}, fmt.Errorf("ticket 数据异常: %w", err)
		}
		return payload, nil
	}

	ssoTicketMemoryMu.Lock()
	defer ssoTicketMemoryMu.Unlock()
	payload, ok := ssoTicketMemory[ticket]
	if !ok {
		return ssoTicketPayload{}, errors.New("ticket 无效或已过期")
	}
	delete(ssoTicketMemory, ticket)
	return payload, nil
}
