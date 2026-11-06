package controller

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

func accountSecurityTarget(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 0 {
		common.ApiErrorMsg(c, "用户ID无效")
		return 0, false
	}
	role := c.GetInt("role")
	if id == 0 {
		if role < common.RoleRootUser {
			common.ApiErrorMsg(c, "全局策略仅限超级管理员")
			return 0, false
		}
		return 0, true
	}
	var user model.User
	if model.DB.Select("id", "role").First(&user, "id = ?", id).Error != nil {
		common.ApiErrorMsg(c, "用户不存在")
		return 0, false
	}
	if role < common.RoleRootUser && (user.Role >= role || id == c.GetInt("id")) {
		common.ApiErrorMsg(c, "不能修改同级或更高权限用户的安全策略")
		return 0, false
	}
	return id, true
}

func GetAccountSecurity(c *gin.Context) {
	id, ok := accountSecurityTarget(c)
	if !ok {
		return
	}
	p, err := model.GetAccountSecurityPolicy(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var events []model.AccountSecurityEvent
	q := model.DB.Order("id DESC").Limit(30)
	if id != 0 {
		q = q.Where("user_id = ?", id)
	}
	if err = q.Find(&events).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	common.ApiSuccess(c, gin.H{"policy": p, "events": events})
}

func PutAccountSecurity(c *gin.Context) {
	id, ok := accountSecurityTarget(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16384)
	p := model.DefaultAccountSecurityPolicy()
	if err := c.ShouldBindJSON(&p); err != nil {
		common.ApiErrorMsg(c, "策略格式无效")
		return
	}
	if err := p.Validate(id == 0); err != nil {
		common.ApiError(c, err)
		return
	}
	// Do not permit self-lockout from the same screen.
	if id == c.GetInt("id") && p.LoginIPs != "" {
		g, err := model.GetAccountSecurityPolicy(0)
		if err != nil || !model.SecurityIPAllowed(model.SecurityClientIP(c.Request.RemoteAddr, c.GetHeader("X-Forwarded-For"), g.TrustedProxies), p.LoginIPs) {
			common.ApiErrorMsg(c, "白名单必须包含当前登录IP，避免锁定自己")
			return
		}
	}
	doc, _ := json.Marshal(p)
	row := model.AccountSecurity{UserID: id, Document: string(doc)}
	if err := model.DB.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}}, DoUpdates: clause.AssignmentColumns([]string{"document"})}).Create(&row).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	model.DB.Create(&model.AccountSecurityEvent{UserID: id, Kind: "policy", CreatedAt: time.Now().Unix(), Message: "安全策略已由管理员 ID " + strconv.Itoa(c.GetInt("id")) + " 更新", Delivery: "audit"})
	common.ApiSuccess(c, nil)
}
