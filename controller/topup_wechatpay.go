package controller

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
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
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/thanhpk/randstr"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/downloader"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
)

type WeChatPayTopUpRequest struct {
	Amount json.RawMessage `json:"amount"`
}

const wechatPayAmountScale int64 = 10000 // 充值数量保留 4 位小数存入 Amount

func parseWeChatPayAmountRaw(raw json.RawMessage) (decimal.Decimal, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return decimal.Zero, errors.New("empty amount")
	}
	var i int64
	if err := common.Unmarshal(raw, &i); err == nil {
		return decimal.NewFromInt(i), nil
	}
	var f float64
	if err := common.Unmarshal(raw, &f); err == nil {
		return decimal.NewFromFloat(f), nil
	}
	var s string
	if err := common.Unmarshal(raw, &s); err == nil {
		return decimal.NewFromString(strings.TrimSpace(s))
	}
	return decimal.Zero, errors.New("invalid amount")
}

func parseRSAPrivateKeyFromPEM(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("invalid pem")
	}

	// PKCS8
	if keyAny, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if k, ok := keyAny.(*rsa.PrivateKey); ok {
			return k, nil
		}
	}

	// PKCS1
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return k, nil
	}

	return nil, errors.New("unsupported private key type")
}

func wechatPayPrivateKey() (*rsa.PrivateKey, error) {
	key := strings.TrimSpace(setting.WeChatPayPrivateKey)
	if key == "" {
		return nil, errors.New("private key empty")
	}
	// allow the user to paste file path by mistake; try load with utils when it looks like a path
	if !strings.Contains(key, "BEGIN") && (strings.Contains(key, ":\\") || strings.HasPrefix(key, "/")) {
		return utils.LoadPrivateKeyWithPath(key)
	}
	return parseRSAPrivateKeyFromPEM(key)
}

func newWeChatPayClient(ctx context.Context) (*core.Client, error) {
	privateKey, err := wechatPayPrivateKey()
	if err != nil {
		return nil, err
	}
	opts := []core.ClientOption{
		option.WithWechatPayAutoAuthCipher(
			strings.TrimSpace(setting.WeChatPayMchID),
			strings.TrimSpace(setting.WeChatPayMchCertSerialNo),
			privateKey,
			strings.TrimSpace(setting.WeChatPayAPIv3Key),
		),
	}
	return core.NewClient(ctx, opts...)
}

func ensureWeChatPayNotifyHandler(ctx context.Context) (*notify.Handler, error) {
	privateKey, err := wechatPayPrivateKey()
	if err != nil {
		return nil, err
	}
	mchID := strings.TrimSpace(setting.WeChatPayMchID)
	serial := strings.TrimSpace(setting.WeChatPayMchCertSerialNo)
	apiV3Key := strings.TrimSpace(setting.WeChatPayAPIv3Key)

	// Register downloader (idempotent inside mgr).
	_ = downloader.MgrInstance().RegisterDownloaderWithPrivateKey(ctx, privateKey, serial, mchID, apiV3Key)
	certVisitor := downloader.MgrInstance().GetCertificateVisitor(mchID)
	if certVisitor == nil {
		return nil, errors.New("wechatpay cert visitor not ready")
	}
	return notify.NewNotifyHandler(apiV3Key, verifiers.NewSHA256WithRSAVerifier(certVisitor)), nil
}

func getWeChatPayMinTopupDecimal() decimal.Decimal {
	dMin := decimal.NewFromFloat(setting.WeChatPayMinTopUp)
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		dMin = dMin.Mul(decimal.NewFromFloat(common.QuotaPerUnit))
	}
	return dMin
}

func getWeChatPayMoney(amountDec decimal.Decimal, group string) float64 {
	dAmount := amountDec
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		dAmount = dAmount.Div(decimal.NewFromFloat(common.QuotaPerUnit))
	}

	topupGroupRatio := common.GetTopupGroupRatio(group)
	if topupGroupRatio == 0 {
		topupGroupRatio = 1
	}

	discount := 1.0
	if ds, ok := operation_setting.GetPaymentSetting().AmountDiscount[int(amountDec.IntPart())]; ok && ds > 0 {
		discount = ds
	}

	payMoney := dAmount.
		Mul(decimal.NewFromFloat(setting.WeChatPayUnitPrice)).
		Mul(decimal.NewFromFloat(topupGroupRatio)).
		Mul(decimal.NewFromFloat(discount))

	return payMoney.InexactFloat64()
}

func RequestWeChatPayAmount(c *gin.Context) {
	var req WeChatPayTopUpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}

	if !isWeChatPayTopUpEnabled() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "微信支付未启用"})
		return
	}

	amountDec, err := parseWeChatPayAmountRaw(req.Amount)
	if err != nil || amountDec.Sign() <= 0 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值数量无效"})
		return
	}

	minDec := getWeChatPayMinTopupDecimal()
	if amountDec.Cmp(minDec) < 0 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值数量不能小于 %s", minDec.String())})
		return
	}

	id := c.GetInt("id")
	group, err := model.GetUserGroup(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "获取用户分组失败"})
		return
	}

	payMoney := getWeChatPayMoney(amountDec, group)
	if payMoney <= 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值金额过低"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": decimal.NewFromFloat(payMoney).StringFixed(2)})
}

func RequestWeChatPayPay(c *gin.Context) {
	if !isWeChatPayTopUpEnabled() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "微信支付未启用或配置不完整"})
		return
	}

	var req WeChatPayTopUpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}

	amountDec, err := parseWeChatPayAmountRaw(req.Amount)
	if err != nil || amountDec.Sign() <= 0 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值数量无效"})
		return
	}

	minDec := getWeChatPayMinTopupDecimal()
	if amountDec.Cmp(minDec) < 0 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值数量不能小于 %s", minDec.String())})
		return
	}

	id := c.GetInt("id")
	group, err := model.GetUserGroup(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "获取用户分组失败"})
		return
	}

	payMoney := getWeChatPayMoney(amountDec, group)
	if payMoney < 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值金额过低"})
		return
	}

	totalFen := int64(math.Round(payMoney * 100))
	if totalFen < 1 {
		totalFen = 1
	}

	tradeNo := fmt.Sprintf("WXPAY-%d-%d-%s", id, time.Now().UnixMilli(), randstr.String(6))

	var amountToStore int64
	var amountDenom int64 = 1
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		tokenInt := amountDec.IntPart()
		if tokenInt < 1 {
			c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值数量无效"})
			return
		}
		dTok := decimal.NewFromInt(tokenInt)
		normalized := dTok.Div(decimal.NewFromFloat(common.QuotaPerUnit)).Floor().IntPart()
		if normalized < 1 {
			normalized = 1
		}
		amountToStore = normalized
		amountDenom = 1
	} else {
		amountToStore = amountDec.Mul(decimal.NewFromInt(wechatPayAmountScale)).Round(0).IntPart()
		if amountToStore < 1 {
			c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值数量过低"})
			return
		}
		amountDenom = wechatPayAmountScale
	}

	topUp := &model.TopUp{
		UserId:        id,
		Amount:        amountToStore,
		AmountDenom:   amountDenom,
		Money:         payMoney,
		TradeNo:       tradeNo,
		PaymentMethod: model.PaymentMethodWeChatPay,
		CreateTime:    time.Now().Unix(),
		Status:        common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("微信支付 创建充值订单失败 user_id=%d trade_no=%s amount=%s error=%q", id, tradeNo, amountDec.String(), err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
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
		logger.LogError(c.Request.Context(), fmt.Sprintf("微信支付 client 初始化失败 user_id=%d trade_no=%s error=%q", id, tradeNo, err.Error()))
		topUp.Status = common.TopUpStatusFailed
		_ = topUp.Update()
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "支付配置错误"})
		return
	}

	svc := native.NativeApiService{Client: client}
	resp, result, err := svc.Prepay(c.Request.Context(), native.PrepayRequest{
		Appid:       core.String(strings.TrimSpace(setting.WeChatPayAppID)),
		Mchid:       core.String(strings.TrimSpace(setting.WeChatPayMchID)),
		Description: core.String(fmt.Sprintf("TopUp %s", amountDec.String())),
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
		logger.LogError(c.Request.Context(), fmt.Sprintf("微信支付 下单失败 user_id=%d trade_no=%s amount=%s money=%.2f status=%d error=%q", id, tradeNo, amountDec.String(), payMoney, func() int {
			if result != nil && result.Response != nil {
				return result.Response.StatusCode
			}
			return 0
		}(), errMsg))
		topUp.Status = common.TopUpStatusFailed
		_ = topUp.Update()
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}

	codeURL := ""
	if resp != nil && resp.CodeUrl != nil {
		codeURL = *resp.CodeUrl
	}
	if strings.TrimSpace(codeURL) == "" {
		logger.LogError(c.Request.Context(), fmt.Sprintf("微信支付 下单成功但缺少 code_url user_id=%d trade_no=%s", id, tradeNo))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("微信支付 充值订单创建成功 user_id=%d trade_no=%s amount=%s money=%.2f fen=%d notify_url=%q", id, tradeNo, amountDec.String(), payMoney, totalFen, notifyURL))
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"code_url":   codeURL,
			"order_id":   tradeNo,
			"return_url": returnURL,
		},
	})
}

// WeChatPayWebhook handles WeChat Pay payment notifications (API v3).
func WeChatPayWebhook(c *gin.Context) {
	ctx := c.Request.Context()
	if !isWeChatPayWebhookEnabled() {
		logger.LogWarn(ctx, fmt.Sprintf("微信支付 webhook 被拒绝 reason=webhook_disabled path=%q client_ip=%s", c.Request.RequestURI, c.ClientIP()))
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	handler, err := ensureWeChatPayNotifyHandler(ctx)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("微信支付 webhook 初始化 notify handler 失败 path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	transaction := new(payments.Transaction)
	notifyReq, err := handler.ParseNotifyRequest(ctx, c.Request, transaction)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("微信支付 webhook 验签/解密失败 path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	tradeNo := ""
	if transaction.OutTradeNo != nil {
		tradeNo = *transaction.OutTradeNo
	}
	tradeState := ""
	if transaction.TradeState != nil {
		tradeState = *transaction.TradeState
	}
	logger.LogInfo(ctx, fmt.Sprintf("微信支付 webhook 收到通知 trade_no=%s trade_state=%s summary=%q client_ip=%s", tradeNo, tradeState, notifyReq.Summary, c.ClientIP()))

	if tradeNo == "" {
		c.Status(http.StatusOK)
		return
	}

	if strings.EqualFold(tradeState, "SUCCESS") {
		LockOrder(tradeNo)
		defer UnlockOrder(tradeNo)
		if subOrder := model.GetSubscriptionOrderByTradeNo(tradeNo); subOrder != nil &&
			subOrder.PaymentMethod == model.PaymentMethodWeChatPay &&
			subOrder.Status == common.TopUpStatusPending {
			payloadBytes, mErr := common.Marshal(transaction)
			payload := ""
			if mErr == nil {
				payload = string(payloadBytes)
			}
			if err := model.CompleteSubscriptionOrder(tradeNo, payload, model.PaymentMethodWeChatPay); err != nil {
				logger.LogError(ctx, fmt.Sprintf("微信支付 订阅订单履约失败 trade_no=%s client_ip=%s error=%q", tradeNo, c.ClientIP(), err.Error()))
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			c.Status(http.StatusOK)
			return
		}
		if err := model.RechargeWeChatPay(tradeNo, c.ClientIP()); err != nil {
			logger.LogError(ctx, fmt.Sprintf("微信支付 充值处理失败 trade_no=%s client_ip=%s error=%q", tradeNo, c.ClientIP(), err.Error()))
			// Return 500 to trigger retry.
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
		return
	}

	// Non-success terminal states => mark failed to avoid keeping pending forever.
	if strings.EqualFold(tradeState, "PAYERROR") ||
		strings.EqualFold(tradeState, "CLOSED") ||
		strings.EqualFold(tradeState, "REVOKED") ||
		strings.EqualFold(tradeState, "REFUND") {
		if subOrder := model.GetSubscriptionOrderByTradeNo(tradeNo); subOrder != nil &&
			subOrder.PaymentMethod == model.PaymentMethodWeChatPay &&
			subOrder.Status == common.TopUpStatusPending {
			if err := model.ExpireSubscriptionOrder(tradeNo, model.PaymentMethodWeChatPay); err != nil &&
				!errors.Is(err, model.ErrSubscriptionOrderNotFound) {
				logger.LogError(ctx, fmt.Sprintf("微信支付 订阅订单标记失败 trade_no=%s error=%q", tradeNo, err.Error()))
			}
		} else if err := model.UpdatePendingTopUpStatus(tradeNo, model.PaymentMethodWeChatPay, common.TopUpStatusFailed); err != nil &&
			!errors.Is(err, model.ErrTopUpNotFound) &&
			!errors.Is(err, model.ErrTopUpStatusInvalid) &&
			!errors.Is(err, model.ErrPaymentMethodMismatch) {
			logger.LogError(ctx, fmt.Sprintf("微信支付 标记失败订单状态失败 trade_no=%s error=%q", tradeNo, err.Error()))
		}
	}

	c.Status(http.StatusOK)
}

