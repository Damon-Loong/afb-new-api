package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func setupAccountSecurityTest(t *testing.T) {
	t.Helper()
	require.NoError(t, model.DB.AutoMigrate(&model.AccountSecurity{}, &model.AccountLoginState{}, &model.AccountAlertState{}, &model.AccountSecurityEvent{}))
	t.Cleanup(func() {
		model.DB.Exec("DELETE FROM account_securities")
		model.DB.Exec("DELETE FROM account_login_states")
		model.DB.Exec("DELETE FROM account_alert_states")
		model.DB.Exec("DELETE FROM account_security_events")
	})
}

func saveSecurityTestPolicy(t *testing.T, id int, p model.AccountSecurityPolicy) {
	b, err := json.Marshal(p)
	require.NoError(t, err)
	require.NoError(t, model.DB.Save(&model.AccountSecurity{UserID: id, Document: string(b)}).Error)
}

func TestAccountLoginIPAndSingleSession(t *testing.T) {
	setupAccountSecurityTest(t)
	p := model.DefaultAccountSecurityPolicy()
	p.SingleSession = true
	p.LoginIPs = "203.0.113.0/24"
	saveSecurityTestPolicy(t, 81, p)
	_, _, _, err := BeginAccountLogin(81, "198.51.100.4:5000", "203.0.113.4")
	require.Error(t, err, "untrusted forwarding headers must not bypass allowlist")
	a, _, _, err := BeginAccountLogin(81, "203.0.113.4:5000", "")
	require.NoError(t, err)
	require.NoError(t, CheckAccountSession(81, a, "203.0.113.4:5000", ""))
	b, _, _, err := BeginAccountLogin(81, "203.0.113.5:6000", "")
	require.NoError(t, err)
	require.Error(t, CheckAccountSession(81, a, "203.0.113.4:5000", ""))
	require.NoError(t, CheckAccountSession(81, b, "203.0.113.5:6000", ""))
	require.Error(t, CheckAccountSession(81, "", "203.0.113.5:6000", ""), "management bearer must not bypass single session")
	require.NoError(t, EndAccountSession(81, a))
	require.NoError(t, CheckAccountSession(81, b, "203.0.113.5:6000", ""))
	require.NoError(t, EndAccountSession(81, b))
	require.Error(t, CheckAccountSession(81, b, "203.0.113.5:6000", ""))
	p.SingleSession = false
	saveSecurityTestPolicy(t, 81, p)
	require.NoError(t, CheckAccountSession(81, a, "203.0.113.4:5000", ""))
	require.NoError(t, CheckAccountSession(82, "", "198.51.100.4:5000", ""), "default policy preserves existing sessions")
}

func TestAccountPolicyValidationAndTrustedProxies(t *testing.T) {
	p := model.DefaultAccountSecurityPolicy()
	require.NoError(t, p.Validate(true))
	p.LoginIPs = "nonsense"
	require.Error(t, p.Validate(false))
	p.LoginIPs = "2001:db8::/32,203.0.113.5"
	require.NoError(t, p.Validate(false))
	require.True(t, model.SecurityIPAllowed("2001:db8::1", p.LoginIPs))
	p.AdminEmails = "admin@example.test"
	require.Error(t, p.Validate(false))
	require.NoError(t, p.Validate(true))
	p.AdminEmails = "bad email"
	require.Error(t, p.Validate(true))
	require.Equal(t, "203.0.113.2", model.SecurityClientIP("127.0.0.1:4444", "fake, 203.0.113.2, 10.0.0.2", "127.0.0.1,10.0.0.0/8"))
	require.Equal(t, "198.51.100.1", model.SecurityClientIP("198.51.100.1:4444", "203.0.113.2", "127.0.0.1"))
}

func TestAccountAlertCooldownRecoveryAndEscalation(t *testing.T) {
	setupAccountSecurityTest(t)
	require.True(t, claimAccountAlert("test", 1, 1000, 60))
	require.False(t, claimAccountAlert("test", 1, 1001, 60))
	require.True(t, claimAccountAlert("test", 2, 1002, 60), "escalation is immediate")
	require.False(t, claimAccountAlert("test", 2, 1003, 60))
	require.False(t, claimAccountAlert("test", 0, 1004, 60))
	require.True(t, claimAccountAlert("test", 1, 1005, 60), "recovered and low again")
}

func TestAccountUsageVideoSettlementNotReserve(t *testing.T) {
	logs := []model.Log{
		{UserId: 8, Type: model.LogTypeConsume, Quota: 735000, Other: `{"billing_mode":"video_tokens","task_id":"video1"}`},
		{UserId: 8, Type: model.LogTypeRefund, Quota: 639733, Other: `{"billing_mode":"video_tokens","task_id":"video1","actual_quota":95267}`},
		{UserId: 8, Type: model.LogTypeRefund, Quota: 639733, Other: `{"billing_mode":"video_tokens","task_id":"video1","actual_quota":95267}`},
		{UserId: 9, Type: model.LogTypeConsume, Quota: 1000},
		{UserId: 9, Type: model.LogTypeRefund, Quota: 100},
	}
	u := AccountUsageFromLogs(logs)
	require.EqualValues(t, 95267, u[8].Quota)
	require.Equal(t, 1, u[8].Requests)
	require.EqualValues(t, 900, u[9].Quota)
	r := model.DefaultAccountSecurityPolicy().Rules
	r.UsageEnabled = true
	r.SpendUSD = 10
	r.Requests = 0
	require.False(t, AccountUsageTriggered(*u[8], AccountUsage{}, r, 500000))
	r.SpendUSD = 0
	r.SpikeRatio = 3
	r.SpikeMinUSD = 1
	require.True(t, AccountUsageTriggered(AccountUsage{Quota: 1000000}, AccountUsage{Quota: 700000}, r, 500000))
	require.False(t, AccountUsageTriggered(AccountUsage{Quota: 1000000}, AccountUsage{}, r, 500000))
}

func TestAccountMonitorMailDedupAndBalanceRecovery(t *testing.T) {
	setupAccountSecurityTest(t)
	truncate(t)
	seedUser(t, 88, 500000)
	oldEmail, oldSMS := accountEmailSender, accountSMSSender
	t.Cleanup(func() { accountEmailSender = oldEmail; accountSMSSender = oldSMS })
	sent := 0
	accountEmailSender = func(subject, to, body string) error { sent++; require.Equal(t, "admin@example.test", to); return nil }
	accountSMSSender = func(phone, template, user string) error { t.Fatal("unexpected SMS"); return nil }
	p := model.DefaultAccountSecurityPolicy()
	p.AdminEmails = "admin@example.test"
	p.Rules.LowEnabled = true
	saveSecurityTestPolicy(t, 0, p)
	now := time.Unix(1800000000, 0)
	require.NoError(t, ScanAccountAlerts(now))
	require.Equal(t, 1, sent)
	require.NoError(t, ScanAccountAlerts(now.Add(time.Minute)))
	require.Equal(t, 1, sent)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 88).Update("quota", 100000000).Error)
	require.NoError(t, ScanAccountAlerts(now.Add(2*time.Minute)))
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 88).Update("quota", 500000).Error)
	require.NoError(t, ScanAccountAlerts(now.Add(3*time.Minute)))
	require.Equal(t, 2, sent)
	var events []model.AccountSecurityEvent
	model.DB.Find(&events)
	require.Len(t, events, 2)
	require.Equal(t, "sent", events[0].Delivery)
}

func TestBillVideoRefundReconciliation(t *testing.T) {
	logs := []model.Log{
		{Id: 1, UserId: 8, CreatedAt: 1700000000, Type: model.LogTypeConsume, Quota: 735000, Other: `{"billing_source":"wallet","billing_mode":"video_tokens","task_id":"v1"}`},
		{Id: 2, UserId: 8, CreatedAt: 1700000001, Type: model.LogTypeRefund, Quota: 639733, Other: `{"billing_source":"wallet","billing_mode":"video_tokens","task_id":"v1","actual_quota":95267,"completion_tokens":38830,"resolution":"480p","duration_seconds":4,"video_pricing_snapshot":{"pricing":{"low_no_video":70,"exchange_rate":7},"group_ratio":0.49,"resolution":"480p"}}`},
	}
	var output bytes.Buffer
	require.NoError(t, WriteBillExport(context.Background(), &output, logs, billTestOptions(), false, 500000))
	book, err := excelize.OpenReader(bytes.NewReader(output.Bytes()))
	require.NoError(t, err)
	defer book.Close()
	rows, err := book.GetRows("账单概览")
	require.NoError(t, err)
	values := map[string]string{}
	for _, row := range rows {
		if len(row) == 2 {
			values[row[0]] = row[1]
		}
	}
	require.Equal(t, "0.190534", values["钱包净扣折算(USD)"])
	require.Equal(t, "38830", values["视频结算Token"])
	details, err := book.GetRows("消费明细")
	require.NoError(t, err)
	cols := map[string]int{}
	for i, v := range details[0] {
		cols[v] = i
	}
	require.Equal(t, "视频预扣（非最终费用）", details[1][cols["计费阶段"]])
	require.Equal(t, "38830", details[2][cols["视频结算Token"]])
	require.Equal(t, "70", details[2][cols["视频单价(CNY/百万Token)"]])
	require.Equal(t, "0.49", details[2][cols["快照分组倍率"]])
}

func TestAccountMonitorUsageAndLoginNotifications(t *testing.T) {
	setupAccountSecurityTest(t)
	truncate(t)
	seedUser(t, 89, 100000000)
	oldEmail := accountEmailSender
	t.Cleanup(func() { accountEmailSender = oldEmail })
	sent := 0
	accountEmailSender = func(subject, to, body string) error { sent++; return nil }
	p := model.DefaultAccountSecurityPolicy()
	p.AdminEmails = "admin@example.test"
	p.Rules.UsageEnabled = true
	p.Rules.Requests = 1
	p.Rules.SpendUSD = 0
	saveSecurityTestPolicy(t, 0, p)
	now := time.Unix(1800000000, 0)
	require.NoError(t, model.LOG_DB.Create(&model.Log{UserId: 89, CreatedAt: now.Unix() - 60, Type: model.LogTypeConsume, Quota: 1000}).Error)
	require.NoError(t, ScanAccountAlerts(now))
	require.Equal(t, 1, sent)
	require.NoError(t, ScanAccountAlerts(now.Add(time.Minute)))
	require.Equal(t, 1, sent)
	p.LoginNotify = true
	u := &model.User{Id: 89, Username: "tester", Email: "user@example.test"}
	NotifyAccountLogin(u, "203.0.113.1", "test-browser", p)
	NotifyAccountLogin(u, "203.0.113.1", "test-browser", p)
	require.Equal(t, 2, sent)
	NotifyAccountLogin(u, "203.0.113.2", "test-browser", p)
	require.Equal(t, 3, sent)
}

func TestAccountNotificationFailureIsRecordedAndThrottled(t *testing.T) {
	setupAccountSecurityTest(t)
	oldEmail := accountEmailSender
	t.Cleanup(func() { accountEmailSender = oldEmail })
	sent := 0
	accountEmailSender = func(subject, to, body string) error {
		sent++
		return errors.New("simulated transport failure")
	}
	p := model.DefaultAccountSecurityPolicy()
	p.LoginNotify = true
	u := &model.User{Id: 90, Username: "tester", Email: "user@example.test"}
	NotifyAccountLogin(u, "203.0.113.1", "test", p)
	NotifyAccountLogin(u, "203.0.113.1", "test", p)
	require.Equal(t, 1, sent)
	var event model.AccountSecurityEvent
	require.NoError(t, model.DB.Where("user_id = ?", 90).First(&event).Error)
	require.Equal(t, "failed", event.Delivery)
}
