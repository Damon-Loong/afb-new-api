package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertSubscriptionOverdraftPlan(t *testing.T, id int, resetPeriod string) {
	t.Helper()
	plan := &SubscriptionPlan{
		Id:               id,
		Title:            "Overdraft Plan",
		PriceAmount:      1,
		Currency:         "USD",
		DurationUnit:     SubscriptionDurationMonth,
		DurationValue:    1,
		Enabled:          true,
		TotalAmount:      10000,
		QuotaResetPeriod: resetPeriod,
	}
	require.NoError(t, DB.Create(plan).Error)
}

func insertUserSubscriptionForOverdraftTest(t *testing.T, id int, userId int, planId int, used int64, nextReset int64) {
	t.Helper()
	sub := &UserSubscription{
		Id:            id,
		UserId:        userId,
		PlanId:        planId,
		AmountTotal:   10000,
		AmountUsed:    used,
		Status:        "active",
		StartTime:     time.Now().Add(-25 * time.Hour).Unix(),
		EndTime:       time.Now().Add(24 * time.Hour).Unix(),
		LastResetTime: time.Now().Add(-25 * time.Hour).Unix(),
		NextResetTime: nextReset,
	}
	require.NoError(t, DB.Create(sub).Error)
}

func getUserSubscriptionForOverdraftTest(t *testing.T, id int) UserSubscription {
	t.Helper()
	var sub UserSubscription
	require.NoError(t, DB.Where("id = ?", id).First(&sub).Error)
	return sub
}

func TestPreConsumeUserSubscriptionAllowsResettableFivePercentOverdraft(t *testing.T) {
	truncateTables(t)

	insertSubscriptionOverdraftPlan(t, 1001, SubscriptionResetDaily)
	insertUserSubscriptionForOverdraftTest(t, 2001, 3001, 1001, 9700, time.Now().Add(time.Hour).Unix())

	res, err := PreConsumeUserSubscription("req-reset-overdraft", 3001, "gpt-test", 0, 700)
	require.NoError(t, err)
	assert.Equal(t, 2001, res.UserSubscriptionId)
	assert.Equal(t, int64(700), res.PreConsumed)

	sub := getUserSubscriptionForOverdraftTest(t, 2001)
	assert.Equal(t, int64(10000), sub.AmountUsed)
	assert.Equal(t, int64(400), sub.OverdraftQuota)
}

func TestPreConsumeUserSubscriptionAllowsNeverResetOnePercentOverdraft(t *testing.T) {
	truncateTables(t)

	insertSubscriptionOverdraftPlan(t, 1002, SubscriptionResetNever)
	insertUserSubscriptionForOverdraftTest(t, 2002, 3002, 1002, 9900, 0)

	_, err := PreConsumeUserSubscription("req-never-overdraft", 3002, "gpt-test", 0, 200)
	require.NoError(t, err)

	sub := getUserSubscriptionForOverdraftTest(t, 2002)
	assert.Equal(t, int64(10000), sub.AmountUsed)
	assert.Equal(t, int64(100), sub.OverdraftQuota)
}

func TestPreConsumeUserSubscriptionRejectsBeyondOverdraftAllowance(t *testing.T) {
	truncateTables(t)

	insertSubscriptionOverdraftPlan(t, 1003, SubscriptionResetNever)
	insertUserSubscriptionForOverdraftTest(t, 2003, 3003, 1003, 9900, 0)

	_, err := PreConsumeUserSubscription("req-over-allowance", 3003, "gpt-test", 0, 201)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "subscription quota insufficient")
}

func TestPreConsumeUserSubscriptionPrefersNonOverdraftCandidate(t *testing.T) {
	truncateTables(t)

	insertSubscriptionOverdraftPlan(t, 1006, SubscriptionResetDaily)
	now := time.Now()
	insertUserSubscriptionForOverdraftTest(t, 2006, 3006, 1006, 9700, now.Add(time.Hour).Unix())
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", 2006).Update("end_time", now.Add(time.Hour).Unix()).Error)
	insertUserSubscriptionForOverdraftTest(t, 2007, 3006, 1006, 9000, now.Add(time.Hour).Unix())
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", 2007).Update("end_time", now.Add(2*time.Hour).Unix()).Error)

	res, err := PreConsumeUserSubscription("req-prefer-non-overdraft", 3006, "gpt-test", 0, 700)
	require.NoError(t, err)
	assert.Equal(t, 2007, res.UserSubscriptionId)

	first := getUserSubscriptionForOverdraftTest(t, 2006)
	assert.Equal(t, int64(9700), first.AmountUsed)
	assert.Equal(t, int64(0), first.OverdraftQuota)

	second := getUserSubscriptionForOverdraftTest(t, 2007)
	assert.Equal(t, int64(9700), second.AmountUsed)
	assert.Equal(t, int64(0), second.OverdraftQuota)
}

func TestRefundSubscriptionPreConsumeRefundsOverdraftFirst(t *testing.T) {
	truncateTables(t)

	insertSubscriptionOverdraftPlan(t, 1004, SubscriptionResetDaily)
	insertUserSubscriptionForOverdraftTest(t, 2004, 3004, 1004, 9700, time.Now().Add(time.Hour).Unix())

	_, err := PreConsumeUserSubscription("req-refund-overdraft", 3004, "gpt-test", 0, 700)
	require.NoError(t, err)
	require.NoError(t, RefundSubscriptionPreConsume("req-refund-overdraft"))

	sub := getUserSubscriptionForOverdraftTest(t, 2004)
	assert.Equal(t, int64(9700), sub.AmountUsed)
	assert.Equal(t, int64(0), sub.OverdraftQuota)
}

func TestResetDueSubscriptionsRepaysOverdraftFirst(t *testing.T) {
	truncateTables(t)

	insertSubscriptionOverdraftPlan(t, 1005, SubscriptionResetDaily)
	insertUserSubscriptionForOverdraftTest(t, 2005, 3005, 1005, 10000, time.Now().Add(-time.Minute).Unix())
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", 2005).Update("overdraft_quota", 600).Error)

	count, err := ResetDueSubscriptions(10)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	sub := getUserSubscriptionForOverdraftTest(t, 2005)
	assert.Equal(t, int64(600), sub.AmountUsed)
	assert.Equal(t, int64(0), sub.OverdraftQuota)
}
