package controller

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

func adminCanManageUser(c *gin.Context, user *model.User) bool {
	return user != nil && (c.GetInt("role") == common.RoleRootUser || user.Id == c.GetInt("id") || user.Role < c.GetInt("role"))
}

func getAdminManagedUser(c *gin.Context, userId int) (*model.User, bool) {
	user, err := model.GetUserById(userId, false)
	if err != nil {
		common.ApiError(c, err)
		return nil, false
	}
	if !adminCanManageUser(c, user) {
		common.ApiErrorI18n(c, i18n.MsgUserNoPermissionSameLevel)
		return nil, false
	}
	return user, true
}

func getAdminManagedToken(c *gin.Context, tokenId int) (*model.Token, *model.User, bool) {
	token, err := model.GetTokenById(tokenId)
	if err != nil {
		common.ApiError(c, err)
		return nil, nil, false
	}
	user, ok := getAdminManagedUser(c, token.UserId)
	return token, user, ok
}

func maskAdminTokens(tokens []*model.TokenWithUser) []*model.TokenWithUser {
	for _, token := range tokens {
		token.Key = token.GetMaskedKey()
	}
	return tokens
}

func AdminGetAllTokens(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	tokens, total, err := model.GetAdminTokens(c.GetInt("id"), c.GetInt("role"), c.Query("keyword"), c.Query("token"), c.Query("username"), pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(maskAdminTokens(tokens))
	common.ApiSuccess(c, pageInfo)
}

func AdminGetToken(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	token, user, ok := getAdminManagedToken(c, id)
	if !ok {
		return
	}
	result := &model.TokenWithUser{Token: *buildMaskedTokenResponse(token), Username: user.Username, UserRole: user.Role}
	common.ApiSuccess(c, result)
}

func AdminGetTokenKey(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	token, user, ok := getAdminManagedToken(c, id)
	if !ok {
		return
	}
	model.RecordLogWithAdminInfo(user.Id, model.LogTypeManage, fmt.Sprintf("管理员查看令牌密钥（令牌ID: %d）", token.Id), map[string]interface{}{"admin_id": c.GetInt("id"), "admin_username": c.GetString("username")})
	common.ApiSuccess(c, gin.H{"key": token.GetFullKey()})
}

func validateAdminToken(c *gin.Context, token *model.Token) bool {
	if len(token.Name) > 50 {
		common.ApiErrorI18n(c, i18n.MsgTokenNameTooLong)
		return false
	}
	if !token.UnlimitedQuota {
		maxQuota := int(1000000000 * common.QuotaPerUnit)
		if token.RemainQuota < 0 {
			common.ApiErrorI18n(c, i18n.MsgTokenQuotaNegative)
			return false
		}
		if token.RemainQuota > maxQuota {
			common.ApiErrorI18n(c, i18n.MsgTokenQuotaExceedMax, map[string]any{"Max": maxQuota})
			return false
		}
	}
	return true
}

func validateAdminTokenGroup(c *gin.Context, user *model.User, group string) bool {
	if group == "" {
		return true
	}
	if _, ok := service.GetUserUsableGroups(user.Group)[group]; !ok {
		common.ApiErrorMsg(c, fmt.Sprintf("目标用户无权使用 %s 分组", group))
		return false
	}
	return true
}

func AdminAddToken(c *gin.Context) {
	var input model.Token
	if err := c.ShouldBindJSON(&input); err != nil || input.UserId <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	user, ok := getAdminManagedUser(c, input.UserId)
	if !ok || !validateAdminToken(c, &input) || !validateAdminTokenGroup(c, user, input.Group) {
		return
	}
	count, err := model.CountUserTokens(user.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if int(count) >= operation_setting.GetMaxUserTokens() {
		common.ApiErrorMsg(c, fmt.Sprintf("目标用户已达到最大令牌数量限制 (%d)", operation_setting.GetMaxUserTokens()))
		return
	}
	key, err := common.GenerateKey()
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgTokenGenerateFailed)
		return
	}
	cleanToken := model.Token{UserId: user.Id, Name: input.Name, Key: key, CreatedTime: common.GetTimestamp(), AccessedTime: common.GetTimestamp(), ExpiredTime: input.ExpiredTime, RemainQuota: input.RemainQuota, UnlimitedQuota: input.UnlimitedQuota, ModelLimitsEnabled: input.ModelLimitsEnabled, ModelLimits: input.ModelLimits, AllowIps: input.AllowIps, Group: input.Group, CrossGroupRetry: input.CrossGroupRetry}
	if err := cleanToken.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	model.RecordLogWithAdminInfo(user.Id, model.LogTypeManage, fmt.Sprintf("管理员创建令牌（令牌ID: %d）", cleanToken.Id), map[string]interface{}{"admin_id": c.GetInt("id"), "admin_username": c.GetString("username")})
	common.ApiSuccess(c, buildMaskedTokenResponse(&cleanToken))
}

func AdminUpdateToken(c *gin.Context) {
	var input model.Token
	if err := c.ShouldBindJSON(&input); err != nil || input.Id <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if !validateAdminToken(c, &input) {
		return
	}
	token, user, ok := getAdminManagedToken(c, input.Id)
	if !ok {
		return
	}
	if c.Query("status_only") != "" {
		if input.Status == common.TokenStatusEnabled {
			if token.Status == common.TokenStatusExpired && token.ExpiredTime <= common.GetTimestamp() && token.ExpiredTime != -1 {
				common.ApiErrorI18n(c, i18n.MsgTokenExpiredCannotEnable)
				return
			}
			if token.Status == common.TokenStatusExhausted && token.RemainQuota <= 0 && !token.UnlimitedQuota {
				common.ApiErrorI18n(c, i18n.MsgTokenExhaustedCannotEable)
				return
			}
		}
		token.Status = input.Status
	} else {
		if !validateAdminTokenGroup(c, user, input.Group) {
			return
		}
		token.Name, token.ExpiredTime, token.RemainQuota = input.Name, input.ExpiredTime, input.RemainQuota
		token.UnlimitedQuota, token.ModelLimitsEnabled, token.ModelLimits = input.UnlimitedQuota, input.ModelLimitsEnabled, input.ModelLimits
		token.AllowIps, token.Group, token.CrossGroupRetry = input.AllowIps, input.Group, input.CrossGroupRetry
	}
	if err := token.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	model.RecordLogWithAdminInfo(user.Id, model.LogTypeManage, fmt.Sprintf("管理员更新令牌（令牌ID: %d）", token.Id), map[string]interface{}{"admin_id": c.GetInt("id"), "admin_username": c.GetString("username")})
	common.ApiSuccess(c, buildMaskedTokenResponse(token))
}

func AdminDeleteToken(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	token, user, ok := getAdminManagedToken(c, id)
	if !ok {
		return
	}
	if err := token.Delete(); err != nil {
		common.ApiError(c, err)
		return
	}
	model.RecordLogWithAdminInfo(user.Id, model.LogTypeManage, fmt.Sprintf("管理员删除令牌（令牌ID: %d）", token.Id), map[string]interface{}{"admin_id": c.GetInt("id"), "admin_username": c.GetString("username")})
	common.ApiSuccess(c, nil)
}

func AdminDeleteTokenBatch(c *gin.Context) {
	var batch TokenBatch
	if err := c.ShouldBindJSON(&batch); err != nil || len(batch.Ids) == 0 || len(batch.Ids) > 100 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	for _, id := range batch.Ids {
		_, _, ok := getAdminManagedToken(c, id)
		if !ok {
			return
		}
	}
	count, err := model.BatchDeleteTokensByIds(batch.Ids)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, count)
}

func AdminGetTokenKeysBatch(c *gin.Context) {
	var batch TokenBatch
	if err := c.ShouldBindJSON(&batch); err != nil || len(batch.Ids) == 0 || len(batch.Ids) > 100 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	keys := make(map[int]string, len(batch.Ids))
	userIds := make(map[int]struct{})
	for _, id := range batch.Ids {
		token, user, ok := getAdminManagedToken(c, id)
		if !ok {
			return
		}
		keys[id] = token.GetFullKey()
		userIds[user.Id] = struct{}{}
	}
	for userId := range userIds {
		model.RecordLogWithAdminInfo(userId, model.LogTypeManage, fmt.Sprintf("管理员批量查看 %d 个令牌密钥", len(batch.Ids)), map[string]interface{}{"admin_id": c.GetInt("id"), "admin_username": c.GetString("username")})
	}
	common.ApiSuccess(c, gin.H{"keys": keys})
}

func AdminGetTokenUsers(c *gin.Context) {
	users, err := model.GetManageableUsers(c.GetInt("id"), c.GetInt("role"), c.Query("keyword"), 50)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, users)
}

func AdminGetUserTokenGroups(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("user_id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	user, ok := getAdminManagedUser(c, userId)
	if !ok {
		return
	}
	result := make(map[string]map[string]interface{})
	usable := service.GetUserUsableGroups(user.Group)
	for name := range ratio_setting.GetGroupRatioCopy() {
		if desc, exists := usable[name]; exists {
			result[name] = map[string]interface{}{"ratio": service.GetUserGroupRatio(user.Group, name), "desc": desc}
		}
	}
	if _, exists := usable["auto"]; exists {
		result["auto"] = map[string]interface{}{"ratio": "自动", "desc": setting.GetUsableGroupDescription("auto")}
	}
	common.ApiSuccess(c, result)
}

func AdminGetUserTokenModels(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("user_id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	user, ok := getAdminManagedUser(c, userId)
	if !ok {
		return
	}
	models := make([]string, 0)
	seen := make(map[string]bool)
	for group := range service.GetUserUsableGroups(user.Group) {
		for _, name := range model.GetGroupEnabledModels(group) {
			if !seen[name] {
				seen[name] = true
				models = append(models, name)
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": models})
}
