package controller

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"
)

type SubscriptionWeChatPayRequest struct {
	PlanId int `json:"plan_id"`
}

// 微信 out_trade_no：6–32 位，仅数字、大小写字母及 _-*。旧版 SUBWXPAY-{user}-{plan}-{ms}-{rand} 易超过 32 导致下单失败。
func newSubscriptionWeChatPayTradeNo() (string, error) {
	const prefix = "SWX" // subscription + wechat native
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(buf), nil // 3 + 24 = 27
}

// SubscriptionRequestWeChatPay 订阅套餐 — 微信 Native 支付（与余额充值共用微信商户与 webhook）
func SubscriptionRequestWeChatPay(c *gin.Context) {
	if !isWeChatPayTopUpEnabled() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "微信支付未启用或配置不完整"})
		return
	}

	var req SubscriptionWeChatPayRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanId <= 0 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}

	plan, err := model.GetSubscriptionPlanById(req.PlanId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !plan.Enabled {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "套餐未启用"})
		return
	}
	if plan.PriceAmount < 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "套餐金额过低"})
		return
	}

	userId := c.GetInt("id")
	if plan.MaxPurchasePerUser > 0 {
		count, err := model.CountUserSubscriptionsByPlan(userId, plan.Id)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if count >= int64(plan.MaxPurchasePerUser) {
			c.JSON(http.StatusOK, gin.H{"message": "error", "data": "已达到该套餐购买上限"})
			return
		}
	}

	tradeNo, err := newSubscriptionWeChatPayTradeNo()
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("订阅微信支付 生成订单号失败 user_id=%d plan_id=%d error=%q", userId, plan.Id, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}

	order := &model.SubscriptionOrder{
		UserId:        userId,
		PlanId:        plan.Id,
		Money:         plan.PriceAmount,
		TradeNo:       tradeNo,
		PaymentMethod: model.PaymentMethodWeChatPay,
		CreateTime:    time.Now().Unix(),
		Status:        common.TopUpStatusPending,
	}
	if err := order.Insert(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}

	totalFen := int64(math.Round(plan.PriceAmount * 100))
	if totalFen < 1 {
		totalFen = 1
	}

	notifyURL := strings.TrimSpace(setting.WeChatPayNotifyURL)
	if notifyURL == "" {
		callbackAddr := service.GetCallbackAddress()
		notifyURL = strings.TrimRight(callbackAddr, "/") + "/api/wechatpay/webhook"
	}
	returnURL := strings.TrimSpace(setting.WeChatPayReturnURL)
	if returnURL == "" {
		returnURL = strings.TrimRight(system_setting.ServerAddress, "/") + "/console/topup?show_history=true"
	}

	client, err := newWeChatPayClient(c.Request.Context())
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("订阅微信支付 client 初始化失败 trade_no=%s error=%q", tradeNo, err.Error()))
		_ = model.ExpireSubscriptionOrder(tradeNo, model.PaymentMethodWeChatPay)
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "支付配置错误"})
		return
	}

	svc := native.NativeApiService{Client: client}
	resp, result, err := svc.Prepay(c.Request.Context(), native.PrepayRequest{
		Appid:       core.String(strings.TrimSpace(setting.WeChatPayAppID)),
		Mchid:       core.String(strings.TrimSpace(setting.WeChatPayMchID)),
		Description: core.String(fmt.Sprintf("Subscription:%s", plan.Title)),
		OutTradeNo:  core.String(tradeNo),
		NotifyUrl:   core.String(notifyURL),
		Amount: &native.Amount{
			Total: core.Int64(totalFen),
		},
	})
	if err != nil || result == nil || result.Response == nil || result.Response.StatusCode/100 != 2 {
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		logger.LogError(c.Request.Context(), fmt.Sprintf("订阅微信支付 下单失败 trade_no=%s plan_id=%d error=%q", tradeNo, plan.Id, errMsg))
		_ = model.ExpireSubscriptionOrder(tradeNo, model.PaymentMethodWeChatPay)
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}

	codeURL := ""
	if resp != nil && resp.CodeUrl != nil {
		codeURL = *resp.CodeUrl
	}
	if strings.TrimSpace(codeURL) == "" {
		logger.LogError(c.Request.Context(), fmt.Sprintf("订阅微信支付 缺少 code_url trade_no=%s", tradeNo))
		_ = model.ExpireSubscriptionOrder(tradeNo, model.PaymentMethodWeChatPay)
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("订阅微信支付 订单创建成功 trade_no=%s plan_id=%d money=%.2f fen=%d", tradeNo, plan.Id, plan.PriceAmount, totalFen))
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"code_url":   codeURL,
			"order_id":   tradeNo,
			"return_url": returnURL,
		},
	})
}
