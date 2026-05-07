package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/thanhpk/randstr"
	"github.com/waffo-com/waffo-go/types/order"
)

type SubscriptionWaffoPayRequest struct {
	PlanId         int  `json:"plan_id"`
	PayMethodIndex *int `json:"pay_method_index"`
}

// SubscriptionRequestWaffoPay 订阅套餐 — Waffo（与余额充值共用 webhook）
func SubscriptionRequestWaffoPay(c *gin.Context) {
	if !setting.WaffoEnabled {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Waffo 支付未启用"})
		return
	}

	var req SubscriptionWaffoPayRequest
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
	user, err := model.GetUserById(userId, false)
	if err != nil || user == nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "用户不存在"})
		return
	}

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

	var resolvedPayMethodType, resolvedPayMethodName string
	methods := setting.GetWaffoPayMethods()
	if req.PayMethodIndex != nil {
		idx := *req.PayMethodIndex
		if idx < 0 || idx >= len(methods) {
			c.JSON(http.StatusOK, gin.H{"message": "error", "data": "不支持的支付方式"})
			return
		}
		resolvedPayMethodType = methods[idx].PayMethodType
		resolvedPayMethodName = methods[idx].PayMethodName
	}

	tradeNo := fmt.Sprintf("SUBWAFFO-%d-%d-%s", userId, time.Now().UnixMilli(), randstr.String(6))
	paymentRequestId := tradeNo

	subOrder := &model.SubscriptionOrder{
		UserId:        userId,
		PlanId:        plan.Id,
		Money:         plan.PriceAmount,
		TradeNo:       tradeNo,
		PaymentMethod: model.PaymentMethodWaffo,
		CreateTime:    time.Now().Unix(),
		Status:        common.TopUpStatusPending,
	}
	if err := subOrder.Insert(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}

	sdk, err := getWaffoSDK()
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("订阅 Waffo SDK 初始化失败 trade_no=%s error=%q", tradeNo, err.Error()))
		_ = model.ExpireSubscriptionOrder(tradeNo, model.PaymentMethodWaffo)
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "支付配置错误"})
		return
	}

	callbackAddr := service.GetCallbackAddress()
	notifyUrl := callbackAddr + "/api/waffo/webhook"
	if setting.WaffoNotifyUrl != "" {
		notifyUrl = setting.WaffoNotifyUrl
	}
	returnUrl := system_setting.ServerAddress + "/console/topup?show_history=true"
	if setting.WaffoReturnUrl != "" {
		returnUrl = setting.WaffoReturnUrl
	}

	currency := getWaffoCurrency()
	payMoney := plan.PriceAmount

	createParams := &order.CreateOrderParams{
		PaymentRequestID: paymentRequestId,
		MerchantOrderID:  tradeNo,
		OrderAmount:      formatWaffoAmount(payMoney, currency),
		OrderCurrency:    currency,
		OrderDescription: fmt.Sprintf("Subscription:%s", plan.Title),
		OrderRequestedAt: time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		NotifyURL:        notifyUrl,
		MerchantInfo: &order.MerchantInfo{
			MerchantID: setting.WaffoMerchantId,
		},
		UserInfo: &order.UserInfo{
			UserID:       strconv.Itoa(user.Id),
			UserEmail:    getWaffoUserEmail(user),
			UserTerminal: "WEB",
		},
		PaymentInfo: &order.PaymentInfo{
			ProductName:   "ONE_TIME_PAYMENT",
			PayMethodType: resolvedPayMethodType,
			PayMethodName: resolvedPayMethodName,
		},
		SuccessRedirectURL: returnUrl,
		FailedRedirectURL:  returnUrl,
	}

	resp, err := sdk.Order().Create(c.Request.Context(), createParams, nil)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("订阅 Waffo 创建订单失败 trade_no=%s error=%q", tradeNo, err.Error()))
		_ = model.ExpireSubscriptionOrder(tradeNo, model.PaymentMethodWaffo)
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}
	if !resp.IsSuccess() {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("订阅 Waffo 业务失败 trade_no=%s code=%s msg=%q", tradeNo, resp.Code, resp.Message))
		_ = model.ExpireSubscriptionOrder(tradeNo, model.PaymentMethodWaffo)
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}

	orderData := resp.GetData()
	paymentUrl := orderData.FetchRedirectURL()
	if paymentUrl == "" {
		paymentUrl = orderData.OrderAction
	}
	if paymentUrl == "" {
		_ = model.ExpireSubscriptionOrder(tradeNo, model.PaymentMethodWaffo)
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("订阅 Waffo 订单创建成功 trade_no=%s plan_id=%d money=%.2f", tradeNo, plan.Id, payMoney))
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"payment_url": paymentUrl,
			"order_id":    tradeNo,
		},
	})
}
