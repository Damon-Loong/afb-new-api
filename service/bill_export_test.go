package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func billTestOptions() BillExportOptions {
	return BillExportOptions{BillExportFilter: model.BillExportFilter{StartTimestamp: 1700000000, EndTimestamp: 1700100000}, Format: "xlsx", Timezone: "Asia/Shanghai", Dimensions: []string{"user", "model", "day"}}
}

func TestBillExportAccountingAndExcelLayout(t *testing.T) {
	logs := []model.Log{
		{Id: 1, UserId: 10, Username: "=SUM(A1:A2)", TokenId: 1, RequestId: "0001234567890123456789", CreatedAt: 1700000000, Type: model.LogTypeConsume, Quota: 1000, PromptTokens: 10, CompletionTokens: 5, Other: `{"billing_source":"wallet","admin_info":{"key":"secret-admin"},"cache_tokens":3}`},
		{Id: 2, UserId: 10, CreatedAt: 1700000000, Type: model.LogTypeRefund, Quota: 200, Other: `{"billing_source":"wallet"}`},
		{Id: 3, UserId: 10, CreatedAt: 1700000000, Type: model.LogTypeConsume, Quota: 500, Other: `{"billing_source":"subscription","subscription_id":12,"subscription_plan_title":"Pro"}`},
		{Id: 4, UserId: 10, CreatedAt: 1700000000, Type: model.LogTypeRefund, Quota: 50, Other: `{"billing_source":"subscription"}`},
		{Id: 5, UserId: 10, CreatedAt: 1700000000, Type: model.LogTypeConsume, Quota: 70},
		{Id: 6, UserId: 10, CreatedAt: 1700000000, Type: model.LogTypeRefund, Quota: 10},
		{Id: 7, UserId: 10, CreatedAt: 1700000000, Type: model.LogTypeProjectConsume, Quota: 20, Other: `{"billing_source":"subscription"}`},
		{Id: 8, UserId: 10, CreatedAt: 1700000000, Type: model.LogTypeMarketConsume, Quota: 30},
	}
	var output bytes.Buffer
	require.NoError(t, WriteBillExport(context.Background(), &output, logs, billTestOptions(), false, 1000))
	book, err := excelize.OpenReader(bytes.NewReader(output.Bytes()))
	require.NoError(t, err)
	defer book.Close()
	require.Equal(t, []string{"账单概览", "用户汇总", "模型汇总", "每日汇总", "消费明细"}, book.GetSheetList())
	rows, err := book.GetRows("账单概览")
	require.NoError(t, err)
	values := map[string]string{}
	for _, row := range rows {
		if len(row) == 2 {
			values[row[0]] = row[1]
		}
	}
	for key, want := range map[string]string{"钱包扣除额度": "1050", "钱包退回额度": "200", "订阅消耗额度": "500", "订阅退回额度": "50", "来源未记录扣除额度": "70", "来源未记录退回额度": "10", "钱包净扣额度": "850", "订阅净消耗额度": "450", "钱包净扣折算(USD)": "0.85"} {
		require.Equal(t, want, values[key], key)
	}
	details, err := book.GetRows("消费明细")
	require.NoError(t, err)
	require.Len(t, details, 9)
	require.NotContains(t, details[0], "渠道ID")
	require.Equal(t, "0001234567890123456789", details[1][2])
	require.Equal(t, "=SUM(A1:A2)", details[1][5])
	formula, err := book.GetCellFormula("消费明细", "F2")
	require.NoError(t, err)
	require.Empty(t, formula)
	panes, err := book.GetPanes("消费明细")
	require.NoError(t, err)
	require.True(t, panes.Freeze)
	require.Equal(t, 1, panes.YSplit)
	users, err := book.GetRows("用户汇总")
	require.NoError(t, err)
	require.Len(t, users, 2)
	require.Equal(t, "1050", users[1][6])
	archive, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
	require.NoError(t, err)
	for _, file := range archive.File {
		reader, err := file.Open()
		require.NoError(t, err)
		data, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.NoError(t, reader.Close())
		require.NotContains(t, string(data), "secret-admin")
		if file.Name == "xl/worksheets/sheet5.xml" {
			require.Contains(t, string(data), `<autoFilter ref="$A$1:$AC$9"`)
		}
	}
}

func TestBillExportBackgroundJobLifecycle(t *testing.T) {
	truncate(t)
	log := model.Log{UserId: 92001, Type: model.LogTypeConsume, CreatedAt: 1700000000, Quota: 1, Other: `{"billing_source":"wallet"}`}
	require.NoError(t, model.LOG_DB.Create(&log).Error)
	job, err := StartBillExport(log.UserId, false, billTestOptions())
	require.NoError(t, err)
	require.Equal(t, "running", job.Status)
	var finished BillExportJob
	require.Eventually(t, func() bool {
		var ok bool
		finished, ok = GetBillExportJob(job.ID, log.UserId, false)
		return ok && finished.Status != "running"
	}, 3*time.Second, 10*time.Millisecond)
	require.Equal(t, "ready", finished.Status, finished.Message)
	require.Equal(t, 1, finished.Rows)
	file, _, err := OpenBillExport(job.ID, log.UserId, false)
	require.NoError(t, err)
	t.Cleanup(func() {
		file.Close()
		os.Remove(file.Name())
		billExports.mu.Lock()
		delete(billExports.jobs, job.ID)
		billExports.mu.Unlock()
	})
	book, err := excelize.OpenReader(file)
	require.NoError(t, err)
	require.NoError(t, book.Close())
	_, _, err = OpenBillExport(job.ID, log.UserId+1, false)
	require.Error(t, err)
}

func TestBillExportCSVTextSafetyAndTimezone(t *testing.T) {
	options := billTestOptions()
	options.Format = "csv"
	options.Dimensions = []string{"day", "token", "channel"}
	when := time.Date(2026, 1, 1, 18, 0, 0, 0, time.UTC).Unix()
	logs := []model.Log{{Id: 1, UserId: 10, Username: " \t=HYPERLINK(\"evil\")", CreatedAt: when, RequestId: "00123456789012345678", Type: model.LogTypeConsume, Quota: 1, Other: `{"billing_source":"wallet"}`}}
	var output bytes.Buffer
	require.NoError(t, WriteBillExport(context.Background(), &output, logs, options, true, 1500000))
	archive, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
	require.NoError(t, err)
	require.Len(t, archive.File, 5)
	for _, file := range archive.File {
		reader, err := file.Open()
		require.NoError(t, err)
		data, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.NoError(t, reader.Close())
		require.True(t, bytes.HasPrefix(data, []byte("\xef\xbb\xbf")))
		csvReader := csv.NewReader(bytes.NewReader(data[3:]))
		rows, err := csvReader.ReadAll()
		require.NoError(t, err)
		if file.Name == "消费明细.csv" {
			require.Equal(t, "'00123456789012345678", rows[1][2])
			require.True(t, strings.HasPrefix(rows[1][5], "'"))
			require.Equal(t, "渠道ID", rows[0][len(rows[0])-1])
			require.Equal(t, "2026-01-02 02:00:00", rows[1][0])
		}
		if file.Name == "每日汇总.csv" {
			require.Equal(t, "2026-01-02", rows[1][0])
		}
	}
}

func TestBillExportValidation(t *testing.T) {
	for _, change := range []func(*BillExportOptions){
		func(o *BillExportOptions) { o.StartTimestamp = 0 },
		func(o *BillExportOptions) { o.EndTimestamp = o.StartTimestamp - 1 },
		func(o *BillExportOptions) { o.EndTimestamp = o.StartTimestamp + 367*86400 },
		func(o *BillExportOptions) { o.Timezone = "invalid/location" },
		func(o *BillExportOptions) { o.Format = "pdf" },
		func(o *BillExportOptions) { o.LogType = model.LogTypeTopup },
		func(o *BillExportOptions) { o.Dimensions = []string{"channel"} },
		func(o *BillExportOptions) { o.Dimensions = []string{"user", "user"} },
		func(o *BillExportOptions) { o.Dimensions = []string{"bad"} },
	} {
		options := billTestOptions()
		change(&options)
		require.Error(t, options.Validate(false))
	}
	options := billTestOptions()
	options.Username, options.Channel = "someone_else", 123
	require.NoError(t, options.Validate(false))
	require.Empty(t, options.Username)
	require.Zero(t, options.Channel)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, WriteBillExport(ctx, io.Discard, []model.Log{{Type: model.LogTypeConsume}}, options, false, 1000), context.Canceled)
	require.Error(t, WriteBillExport(context.Background(), io.Discard, nil, options, false, 0))
}

func TestBillExportJobOwnershipAndExpiry(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "bill")
	require.NoError(t, err)
	require.NoError(t, file.Close())
	job := &BillExportJob{ID: "bill-test-owner", Status: "ready", ownerID: 91001, admin: true, path: file.Name(), ExpiresAt: time.Now().Add(time.Minute).Unix()}
	billExports.mu.Lock()
	billExports.jobs[job.ID] = job
	billExports.mu.Unlock()
	t.Cleanup(func() { billExports.mu.Lock(); delete(billExports.jobs, job.ID); billExports.mu.Unlock() })
	_, ok := GetBillExportJob(job.ID, 91002, true)
	require.False(t, ok)
	_, ok = GetBillExportJob(job.ID, job.ownerID, false)
	require.False(t, ok)
	_, _, err = OpenBillExport(job.ID, 91002, true)
	require.Error(t, err)
	opened, _, err := OpenBillExport(job.ID, job.ownerID, true)
	require.NoError(t, err)
	require.NoError(t, opened.Close())
	billExports.mu.Lock()
	job.ExpiresAt = time.Now().Add(-time.Second).Unix()
	billExports.mu.Unlock()
	_, ok = GetBillExportJob(job.ID, job.ownerID, true)
	require.False(t, ok)
}

func TestTaskBillingMetadataPreservesFundingSource(t *testing.T) {
	task := &model.Task{}
	require.Equal(t, BillingSourceWallet, taskBillingOther(task)["billing_source"])
	task.PrivateData.BillingSource = BillingSourceSubscription
	task.PrivateData.SubscriptionId = 12
	require.Equal(t, BillingSourceSubscription, taskBillingOther(task)["billing_source"])
	require.Equal(t, 12, taskBillingOther(task)["subscription_id"])
}
