package service

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestVideoTaskSettlement(t *testing.T) {
	for _, tc := range []struct {
		name   string
		tokens int
		status model.TaskStatus
		actual int
	}{
		{"refund_difference", 10000, model.TaskStatusSuccess, 50000},
		{"additional_charge", 50000, model.TaskStatusSuccess, 250000},
		{"exact_reserve", 20000, model.TaskStatusSuccess, 100000},
		{"failed", 0, model.TaskStatusFailure, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			truncate(t)
			seedUser(t, 1, 900000)
			seedToken(t, 1, 1, "video-test", 900000)
			seedChannel(t, 1)
			task := makeTask(1, 1, 100000, 1, BillingSourceWallet, 0)
			task.PrivateData.BillingContext = &model.TaskBillingContext{OriginModelName: "seedance-test", VideoBilling: &types.VideoBillingSnapshot{
				Pricing: types.VideoTokenPricing{ExchangeRate: 7, LowNoVideo: 70, LowWithVideo: 42, HighNoVideo: 77, HighWithVideo: 46, ReserveTokens: 20000}, GroupRatio: 1, QuotaPerUnit: 500000,
			}}
			require.NoError(t, task.Insert())
			task.Status = tc.status
			result := &relaycommon.TaskInfo{CompletionTokens: tc.tokens, Resolution: "720p", Duration: 4.6}
			require.NoError(t, completeVideoBilling(context.Background(), task, model.TaskStatusInProgress, result))
			var stored model.Task
			require.NoError(t, model.DB.First(&stored, task.ID).Error)
			require.Equal(t, tc.actual, stored.Quota)
			var user model.User
			require.NoError(t, model.DB.First(&user, 1).Error)
			require.Equal(t, 1000000-tc.actual, user.Quota)
			var token model.Token
			require.NoError(t, model.DB.First(&token, 1).Error)
			require.Equal(t, user.Quota, token.RemainQuota)
			// Simulate a competing worker with stale unfinished state.
			require.NoError(t, completeVideoBilling(context.Background(), task, model.TaskStatusInProgress, result))
			require.NoError(t, model.DB.First(&user, 1).Error)
			require.Equal(t, 1000000-tc.actual, user.Quota)
			var count int64
			model.DB.Model(&model.Log{}).Count(&count)
			require.EqualValues(t, 1, count)
		})
	}
}

func TestVideoMissingUsageLeavesTaskUnfinished(t *testing.T) {
	truncate(t)
	seedUser(t, 1, 900000)
	seedChannel(t, 1)
	task := makeTask(1, 1, 100000, 0, BillingSourceWallet, 0)
	task.PrivateData.BillingContext = &model.TaskBillingContext{VideoBilling: &types.VideoBillingSnapshot{Pricing: types.VideoTokenPricing{ExchangeRate: 7, LowNoVideo: 70, LowWithVideo: 42, HighNoVideo: 77, HighWithVideo: 46, ReserveTokens: 20000}, GroupRatio: 1, QuotaPerUnit: 500000}}
	require.NoError(t, task.Insert())
	task.Status = model.TaskStatusSuccess
	require.Error(t, completeVideoBilling(context.Background(), task, model.TaskStatusInProgress, &relaycommon.TaskInfo{Resolution: "720p"}))
	var stored model.Task
	require.NoError(t, model.DB.First(&stored, task.ID).Error)
	require.Equal(t, model.TaskStatus(model.TaskStatusInProgress), stored.Status)
	require.Equal(t, 100000, stored.Quota)
}

func TestVideoSettlementRollbackAndSubscription(t *testing.T) {
	t.Run("rollback_missing_user", func(t *testing.T) {
		truncate(t)
		seedChannel(t, 1)
		task := makeTask(999, 1, 100000, 0, BillingSourceWallet, 0)
		require.NoError(t, task.Insert())
		task.Status = model.TaskStatusSuccess
		won, err := model.CompleteVideoTask(task, model.TaskStatusInProgress, 50000)
		require.Error(t, err)
		require.False(t, won)
		var stored model.Task
		require.NoError(t, model.DB.First(&stored, task.ID).Error)
		require.Equal(t, model.TaskStatus(model.TaskStatusInProgress), stored.Status)
		require.Equal(t, 100000, stored.Quota)
	})
	t.Run("subscription_delta", func(t *testing.T) {
		truncate(t)
		seedUser(t, 1, 900000)
		seedChannel(t, 1)
		seedSubscription(t, 1, 1, 1000000, 100000)
		task := makeTask(1, 1, 100000, 0, BillingSourceSubscription, 1)
		require.NoError(t, task.Insert())
		task.Status = model.TaskStatusSuccess
		won, err := model.CompleteVideoTask(task, model.TaskStatusInProgress, 50000)
		require.NoError(t, err)
		require.True(t, won)
		var sub model.UserSubscription
		require.NoError(t, model.DB.First(&sub, 1).Error)
		require.EqualValues(t, 50000, sub.AmountUsed)
		var user model.User
		require.NoError(t, model.DB.First(&user, 1).Error)
		require.Equal(t, 900000, user.Quota)
	})
}
