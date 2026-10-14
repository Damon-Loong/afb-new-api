package model

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBillExportScopeAndFilters(t *testing.T) {
	const owner = 91001
	const other = 91002
	const start = 1700000000
	t.Cleanup(func() { LOG_DB.Where("user_id IN ?", []int{owner, other}).Delete(&Log{}) })
	logs := []Log{
		{UserId: owner, Username: "bill_owner", CreatedAt: start, Type: LogTypeConsume, ModelName: "model_a", TokenName: "bill_token", Group: "paid", ChannelId: 3, RequestId: "req_1", Content: "private content", Ip: "127.0.0.1"},
		{UserId: owner, Username: "bill_owner", CreatedAt: start + 1, Type: LogTypeRefund, ModelName: "model_a", TokenName: "bill_token", Group: "paid", ChannelId: 3},
		{UserId: owner, CreatedAt: start, Type: LogTypeMarketConsume},
		{UserId: owner, CreatedAt: start, Type: LogTypeProjectConsume},
		{UserId: owner, CreatedAt: start, Type: LogTypeMarketReward},
		{UserId: owner, CreatedAt: start, Type: LogTypeTopup},
		{UserId: owner, CreatedAt: start, Type: LogTypeError},
		{UserId: owner, CreatedAt: start - 1, Type: LogTypeConsume},
		{UserId: other, Username: "bill_other", CreatedAt: start, Type: LogTypeConsume, ChannelId: 5},
	}
	require.NoError(t, LOG_DB.Create(&logs).Error)
	filter := BillExportFilter{StartTimestamp: start, EndTimestamp: start + 1, Username: "bill_other", Channel: 5}
	// Supplying another username/channel must never escape a self export scope.
	result, err := GetBillExportLogs(context.Background(), filter, owner, false)
	require.NoError(t, err)
	require.Len(t, result, 4)
	for _, log := range result {
		require.Equal(t, owner, log.UserId)
		require.Empty(t, log.Content)
		require.Empty(t, log.Ip)
	}
	result, err = GetBillExportLogs(context.Background(), filter, owner, true)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, other, result[0].UserId)
	filter = BillExportFilter{StartTimestamp: start, EndTimestamp: start + 1, ModelName: "model_a", TokenName: "bill_token", Group: "paid", RequestID: "req_1", Channel: 3, Username: "bill_owner"}
	result, err = GetBillExportLogs(context.Background(), filter, owner, true)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, logs[0].Id, result[0].Id)
	filter.RequestID = ""
	filter.LogType = LogTypeRefund
	result, err = GetBillExportLogs(context.Background(), filter, owner, false)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, LogTypeRefund, result[0].Type)
	filter.LogType = LogTypeTopup
	_, err = GetBillExportLogs(context.Background(), filter, owner, true)
	require.Error(t, err)
	_, err = GetBillExportLogs(context.Background(), filter, 0, false)
	require.Error(t, err)
}

func TestBillExportDoesNotTruncate(t *testing.T) {
	const owner = 91003
	t.Cleanup(func() { LOG_DB.Where("user_id = ?", owner).Delete(&Log{}) })
	// Insert through batches to stay below SQLite's placeholder limit.
	logs := make([]Log, MaxBillExportRows+1)
	for i := range logs {
		logs[i] = Log{UserId: owner, CreatedAt: 1700000000, Type: LogTypeConsume}
	}
	require.NoError(t, LOG_DB.CreateInBatches(logs, 200).Error)
	result, err := GetBillExportLogs(context.Background(), BillExportFilter{StartTimestamp: 1700000000, EndTimestamp: 1700000000}, owner, false)
	require.ErrorIs(t, err, ErrBillExportTooLarge)
	require.Empty(t, result)
}
