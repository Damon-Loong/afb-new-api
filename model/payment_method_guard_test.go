package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertUserForPaymentGuardTest(t *testing.T, id int, quota int) {
	t.Helper()
	user := &User{
		Id:       id,
		Username: "payment_guard_user",
		Status:   common.UserStatusEnabled,
		Quota:    quota,
	}
	require.NoError(t, DB.Create(user).Error)
}

func insertSubscriptionPlanForPaymentGuardTest(t *testing.T, id int) *SubscriptionPlan {
	t.Helper()
	plan := &SubscriptionPlan{
		Id:            id,
		Title:         "Guard Plan",
		PriceAmount:   9.99,
		Currency:      "USD",
		DurationUnit:  SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
		TotalAmount:   1000,
	}
	require.NoError(t, DB.Create(plan).Error)
	return plan
}

func insertSubscriptionOrderForPaymentGuardTest(t *testing.T, tradeNo string, userID int, planID int, paymentMethod string) {
	t.Helper()
	order := &SubscriptionOrder{
		UserId:        userID,
		PlanId:        planID,
		Money:         9.99,
		TradeNo:       tradeNo,
		PaymentMethod: paymentMethod,
		Status:        common.TopUpStatusPending,
		CreateTime:    time.Now().Unix(),
	}
	require.NoError(t, order.Insert())
}

func insertTopUpForPaymentGuardTest(t *testing.T, tradeNo string, userID int, paymentMethod string) {
	t.Helper()
	topUp := &TopUp{
		UserId:        userID,
		Amount:        2,
		Money:         9.99,
		TradeNo:       tradeNo,
		PaymentMethod: paymentMethod,
		Status:        common.TopUpStatusPending,
		CreateTime:    time.Now().Unix(),
	}
	require.NoError(t, topUp.Insert())
}

func getTopUpStatusForPaymentGuardTest(t *testing.T, tradeNo string) string {
	t.Helper()
	topUp := GetTopUpByTradeNo(tradeNo)
	require.NotNil(t, topUp)
	return topUp.Status
}

func countUserSubscriptionsForPaymentGuardTest(t *testing.T, userID int) int64 {
	t.Helper()
	var count int64
	require.NoError(t, DB.Model(&UserSubscription{}).Where("user_id = ?", userID).Count(&count).Error)
	return count
}

func getUserQuotaForPaymentGuardTest(t *testing.T, userID int) int {
	t.Helper()
	var user User
	require.NoError(t, DB.Select("quota").Where("id = ?", userID).First(&user).Error)
	return user.Quota
}

func TestRechargeWaffoPancake_RejectsMismatchedPaymentMethod(t *testing.T) {
	truncateTables(t)

	insertUserForPaymentGuardTest(t, 101, 0)
	insertTopUpForPaymentGuardTest(t, "waffo-pancake-guard", 101, PaymentMethodStripe)

	err := RechargeWaffoPancake("waffo-pancake-guard")
	require.Error(t, err)

	topUp := GetTopUpByTradeNo("waffo-pancake-guard")
	require.NotNil(t, topUp)
	assert.Equal(t, common.TopUpStatusPending, topUp.Status)
	assert.Equal(t, 0, getUserQuotaForPaymentGuardTest(t, 101))
}

func TestUpdatePendingTopUpStatus_RejectsMismatchedPaymentMethod(t *testing.T) {
	testCases := []struct {
		name                  string
		tradeNo               string
		storedPaymentMethod   string
		expectedPaymentMethod string
		targetStatus          string
	}{
		{
			name:                  "stripe expire",
			tradeNo:               "stripe-expire-guard",
			storedPaymentMethod:   PaymentMethodCreem,
			expectedPaymentMethod: PaymentMethodStripe,
			targetStatus:          common.TopUpStatusExpired,
		},
		{
			name:                  "waffo failed",
			tradeNo:               "waffo-failed-guard",
			storedPaymentMethod:   PaymentMethodStripe,
			expectedPaymentMethod: PaymentMethodWaffo,
			targetStatus:          common.TopUpStatusFailed,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			truncateTables(t)
			insertUserForPaymentGuardTest(t, 150, 0)
			insertTopUpForPaymentGuardTest(t, tc.tradeNo, 150, tc.storedPaymentMethod)

			err := UpdatePendingTopUpStatus(tc.tradeNo, tc.expectedPaymentMethod, tc.targetStatus)
			require.ErrorIs(t, err, ErrPaymentMethodMismatch)
			assert.Equal(t, common.TopUpStatusPending, getTopUpStatusForPaymentGuardTest(t, tc.tradeNo))
		})
	}
}

func TestCompleteSubscriptionOrder_RejectsMismatchedPaymentMethod(t *testing.T) {
	truncateTables(t)

	insertUserForPaymentGuardTest(t, 202, 0)
	plan := insertSubscriptionPlanForPaymentGuardTest(t, 301)
	insertSubscriptionOrderForPaymentGuardTest(t, "sub-guard-order", 202, plan.Id, PaymentMethodStripe)

	err := CompleteSubscriptionOrder("sub-guard-order", `{"provider":"epay"}`, "alipay")
	require.ErrorIs(t, err, ErrPaymentMethodMismatch)

	order := GetSubscriptionOrderByTradeNo("sub-guard-order")
	require.NotNil(t, order)
	assert.Equal(t, common.TopUpStatusPending, order.Status)
	assert.Zero(t, countUserSubscriptionsForPaymentGuardTest(t, 202))

	topUp := GetTopUpByTradeNo("sub-guard-order")
	assert.Nil(t, topUp)
}

func TestCompleteWeChatPaySubscriptionOrder_ValidatesAmountAndIsIdempotent(t *testing.T) {
	truncateTables(t)

	insertUserForPaymentGuardTest(t, 203, 0)
	plan := insertSubscriptionPlanForPaymentGuardTest(t, 302)
	order := &SubscriptionOrder{
		UserId:          203,
		PlanId:          plan.Id,
		Money:           9.99,
		TradeNo:         "wechat-subscription-success",
		PaymentMethod:   PaymentMethodWeChatPay,
		PaymentProvider: PaymentProviderWeChatPay,
		Status:          common.TopUpStatusPending,
		CreateTime:      time.Now().Unix(),
	}
	require.NoError(t, order.Insert())

	require.NoError(t, CompleteWeChatPaySubscriptionOrder(order.TradeNo, "{\"provider\":\"wechatpay\"}", 999))
	assert.Equal(t, common.TopUpStatusSuccess, GetSubscriptionOrderByTradeNo(order.TradeNo).Status)
	assert.EqualValues(t, 1, countUserSubscriptionsForPaymentGuardTest(t, 203))

	require.NoError(t, CompleteWeChatPaySubscriptionOrder(order.TradeNo, "{\"duplicate\":true}", 999))
	assert.EqualValues(t, 1, countUserSubscriptionsForPaymentGuardTest(t, 203))
}

func TestCompleteWeChatPaySubscriptionOrder_RejectsAmountMismatch(t *testing.T) {
	truncateTables(t)

	insertUserForPaymentGuardTest(t, 204, 0)
	plan := insertSubscriptionPlanForPaymentGuardTest(t, 303)
	order := &SubscriptionOrder{
		UserId:          204,
		PlanId:          plan.Id,
		Money:           9.99,
		TradeNo:         "wechat-subscription-amount-mismatch",
		PaymentMethod:   PaymentMethodWeChatPay,
		PaymentProvider: PaymentProviderWeChatPay,
		Status:          common.TopUpStatusPending,
		CreateTime:      time.Now().Unix(),
	}
	require.NoError(t, order.Insert())

	err := CompleteWeChatPaySubscriptionOrder(order.TradeNo, "{\"provider\":\"wechatpay\"}", 998)
	require.ErrorIs(t, err, ErrPaymentAmountMismatch)
	assert.Equal(t, common.TopUpStatusPending, GetSubscriptionOrderByTradeNo(order.TradeNo).Status)
	assert.Zero(t, countUserSubscriptionsForPaymentGuardTest(t, 204))
	assert.Nil(t, GetTopUpByTradeNo(order.TradeNo))
}

func TestExpireSubscriptionOrder_RejectsMismatchedPaymentMethod(t *testing.T) {
	truncateTables(t)

	insertUserForPaymentGuardTest(t, 303, 0)
	plan := insertSubscriptionPlanForPaymentGuardTest(t, 401)
	insertSubscriptionOrderForPaymentGuardTest(t, "sub-expire-guard", 303, plan.Id, PaymentMethodStripe)

	err := ExpireSubscriptionOrder("sub-expire-guard", PaymentMethodCreem)
	require.ErrorIs(t, err, ErrPaymentMethodMismatch)

	order := GetSubscriptionOrderByTradeNo("sub-expire-guard")
	require.NotNil(t, order)
	assert.Equal(t, common.TopUpStatusPending, order.Status)
}

func TestCompleteEpayTopUp_AtomicallyCreditsAndIsIdempotent(t *testing.T) {
	truncateTables(t)
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 100
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })

	insertUserForPaymentGuardTest(t, 501, 10)
	topUp := &TopUp{
		UserId:          501,
		Amount:          2,
		Money:           2,
		TradeNo:         "epay-atomic-success",
		PaymentMethod:   "alipay",
		PaymentProvider: PaymentProviderEpay,
		Status:          common.TopUpStatusPending,
		CreateTime:      time.Now().Unix(),
	}
	require.NoError(t, topUp.Insert())

	result, err := CompleteEpayTopUp(topUp.TradeNo, "alipay")
	require.NoError(t, err)
	require.True(t, result.NewlyCredited)
	assert.Equal(t, 200, result.QuotaToAdd)
	assert.Equal(t, 210, getUserQuotaForPaymentGuardTest(t, 501))
	assert.Equal(t, common.TopUpStatusSuccess, getTopUpStatusForPaymentGuardTest(t, topUp.TradeNo))

	result, err = CompleteEpayTopUp(topUp.TradeNo, "alipay")
	require.NoError(t, err)
	assert.False(t, result.NewlyCredited)
	assert.Equal(t, 210, getUserQuotaForPaymentGuardTest(t, 501))
}

func TestCompleteEpayTopUp_RejectsCrossGatewayAndPaymentMethod(t *testing.T) {
	tests := []struct {
		name           string
		provider       string
		storedMethod   string
		callbackMethod string
	}{
		{name: "cross gateway", provider: PaymentProviderStripe, storedMethod: PaymentMethodStripe, callbackMethod: "alipay"},
		{name: "wrong epay method", provider: PaymentProviderEpay, storedMethod: "wxpay", callbackMethod: "alipay"},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			truncateTables(t)
			userID := 520 + index
			insertUserForPaymentGuardTest(t, userID, 5)
			topUp := &TopUp{
				UserId: userID, Amount: 1, Money: 1,
				TradeNo:       fmt.Sprintf("epay-reject-%d", index),
				PaymentMethod: test.storedMethod, PaymentProvider: test.provider,
				Status: common.TopUpStatusPending, CreateTime: time.Now().Unix(),
			}
			require.NoError(t, topUp.Insert())
			_, err := CompleteEpayTopUp(topUp.TradeNo, test.callbackMethod)
			require.ErrorIs(t, err, ErrPaymentMethodMismatch)
			assert.Equal(t, 5, getUserQuotaForPaymentGuardTest(t, userID))
			assert.Equal(t, common.TopUpStatusPending, getTopUpStatusForPaymentGuardTest(t, topUp.TradeNo))
		})
	}
}

func TestCompleteEpayTopUp_RollsBackWhenUserMissing(t *testing.T) {
	truncateTables(t)
	topUp := &TopUp{
		UserId: 9999, Amount: 1, Money: 1, TradeNo: "epay-missing-user",
		PaymentMethod: "alipay", PaymentProvider: PaymentProviderEpay,
		Status: common.TopUpStatusPending, CreateTime: time.Now().Unix(),
	}
	require.NoError(t, topUp.Insert())

	_, err := CompleteEpayTopUp(topUp.TradeNo, "alipay")
	require.Error(t, err)
	assert.Equal(t, common.TopUpStatusPending, getTopUpStatusForPaymentGuardTest(t, topUp.TradeNo))
}

func TestRechargeWeChatPay_AtomicallyCreditsAndIsIdempotent(t *testing.T) {
	truncateTables(t)
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 100
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })

	insertUserForPaymentGuardTest(t, 601, 10)
	topUp := &TopUp{
		UserId: 601, Amount: 25000, AmountDenom: 10000, Money: 5.27,
		TradeNo: "wechat-atomic-success", PaymentMethod: PaymentMethodWeChatPay,
		PaymentProvider: PaymentProviderWeChatPay, Status: common.TopUpStatusPending,
		CreateTime: time.Now().Unix(),
	}
	require.NoError(t, topUp.Insert())

	require.NoError(t, RechargeWeChatPay(topUp.TradeNo, "127.0.0.1", 527))
	assert.Equal(t, 260, getUserQuotaForPaymentGuardTest(t, 601))
	assert.Equal(t, common.TopUpStatusSuccess, getTopUpStatusForPaymentGuardTest(t, topUp.TradeNo))

	require.NoError(t, RechargeWeChatPay(topUp.TradeNo, "127.0.0.1", 527))
	assert.Equal(t, 260, getUserQuotaForPaymentGuardTest(t, 601))
}

func TestRechargeWeChatPay_RejectsAmountMismatch(t *testing.T) {
	truncateTables(t)
	insertUserForPaymentGuardTest(t, 602, 10)
	topUp := &TopUp{
		UserId: 602, Amount: 1, AmountDenom: 1, Money: 9.99,
		TradeNo: "wechat-amount-mismatch", PaymentMethod: PaymentMethodWeChatPay,
		PaymentProvider: PaymentProviderWeChatPay, Status: common.TopUpStatusPending,
		CreateTime: time.Now().Unix(),
	}
	require.NoError(t, topUp.Insert())

	err := RechargeWeChatPay(topUp.TradeNo, "127.0.0.1", 998)
	require.ErrorIs(t, err, ErrPaymentAmountMismatch)
	assert.Equal(t, 10, getUserQuotaForPaymentGuardTest(t, 602))
	assert.Equal(t, common.TopUpStatusPending, getTopUpStatusForPaymentGuardTest(t, topUp.TradeNo))
}

func TestRechargeWeChatPay_RejectsCrossProvider(t *testing.T) {
	truncateTables(t)
	insertUserForPaymentGuardTest(t, 603, 10)
	topUp := &TopUp{
		UserId: 603, Amount: 1, AmountDenom: 1, Money: 9.99,
		TradeNo: "wechat-cross-provider", PaymentMethod: "wxpay",
		PaymentProvider: PaymentProviderEpay, Status: common.TopUpStatusPending,
		CreateTime: time.Now().Unix(),
	}
	require.NoError(t, topUp.Insert())

	err := RechargeWeChatPay(topUp.TradeNo, "127.0.0.1", 999)
	require.ErrorIs(t, err, ErrPaymentMethodMismatch)
	assert.Equal(t, 10, getUserQuotaForPaymentGuardTest(t, 603))
	assert.Equal(t, common.TopUpStatusPending, getTopUpStatusForPaymentGuardTest(t, topUp.TradeNo))
}

func TestRechargeWeChatPay_RollsBackWhenUserMissing(t *testing.T) {
	truncateTables(t)
	topUp := &TopUp{
		UserId: 9998, Amount: 1, AmountDenom: 1, Money: 9.99,
		TradeNo: "wechat-missing-user", PaymentMethod: PaymentMethodWeChatPay,
		PaymentProvider: PaymentProviderWeChatPay, Status: common.TopUpStatusPending,
		CreateTime: time.Now().Unix(),
	}
	require.NoError(t, topUp.Insert())

	err := RechargeWeChatPay(topUp.TradeNo, "127.0.0.1", 999)
	require.Error(t, err)
	assert.Equal(t, common.TopUpStatusPending, getTopUpStatusForPaymentGuardTest(t, topUp.TradeNo))
}
