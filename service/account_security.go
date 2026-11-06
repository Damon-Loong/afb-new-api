package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm/clause"
)

func securityHash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

var accountEmailSender = common.SendEmail
var accountSMSSender = sendAccountAlertSMS

func BeginAccountLogin(userID int, remote, forwarded string) (string, string, model.AccountSecurityPolicy, error) {
	p, err := model.GetAccountSecurityPolicy(userID)
	if err != nil {
		return "", "", p, err
	}
	g, err := model.GetAccountSecurityPolicy(0)
	if err != nil {
		return "", "", p, err
	}
	ip := model.SecurityClientIP(remote, forwarded, g.TrustedProxies)
	if !model.SecurityIPAllowed(ip, p.LoginIPs) {
		return "", ip, p, errors.New("当前IP不在账号允许登录范围，请联系管理员")
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", ip, p, err
	}
	token := hex.EncodeToString(b)
	state := model.AccountLoginState{UserID: userID, SessionHash: securityHash(token)}
	err = model.DB.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}}, DoUpdates: clause.AssignmentColumns([]string{"session_hash"})}).Create(&state).Error
	return token, ip, p, err
}

// Management access tokens cannot bypass single-browser-session restrictions.
// Model API keys (sk-...) use TokenAuth and remain independent.
func CheckAccountSession(userID int, nonce, remote, forwarded string) error {
	p, err := model.GetAccountSecurityPolicy(userID)
	if err != nil {
		return errors.New("无法读取登录安全策略，请稍后重试")
	}
	if p.LoginIPs != "" {
		g, err := model.GetAccountSecurityPolicy(0)
		if err != nil {
			return errors.New("无法读取可信代理配置")
		}
		if !model.SecurityIPAllowed(model.SecurityClientIP(remote, forwarded, g.TrustedProxies), p.LoginIPs) {
			return errors.New("当前IP不在账号允许登录范围")
		}
	}
	if p.SingleSession {
		var state model.AccountLoginState
		if nonce == "" || model.DB.First(&state, "user_id = ?", userID).Error != nil || state.SessionHash != securityHash(nonce) {
			return errors.New("登录已失效或账号已在其他设备登录，请重新登录")
		}
	}
	return nil
}

func EndAccountSession(userID int, nonce string) error {
	if nonce == "" {
		return nil
	}
	// An old browser logging out must not revoke the newer browser's session.
	return model.DB.Where("user_id = ? AND session_hash = ?", userID, securityHash(nonce)).Delete(&model.AccountLoginState{}).Error
}

func claimAccountAlert(key string, level int, now int64, cooldown int) bool {
	s := model.AccountAlertState{Key: key}
	if model.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&s).Error != nil {
		return false
	}
	if level == 0 {
		model.DB.Model(&s).Where("alert_key = ?", key).Updates(map[string]interface{}{"level": 0, "next_at": 0})
		return false
	}
	// An atomic compare-and-set prevents simultaneous replicas from mailing twice.
	r := model.DB.Model(&s).Where("alert_key = ? AND (next_at <= ? OR level < ?)", key, now, level).Updates(map[string]interface{}{"level": level, "next_at": now + int64(cooldown)*60})
	return r.Error == nil && r.RowsAffected == 1
}

func recordAccountEvent(userID int, kind, message, delivery string) {
	model.DB.Create(&model.AccountSecurityEvent{UserID: userID, Kind: kind, Message: message, Delivery: delivery, CreatedAt: time.Now().Unix()})
}

func NotifyAccountLogin(user *model.User, ip, agent string, p model.AccountSecurityPolicy) {
	if !p.LoginNotify || user.Email == "" {
		return
	}
	if len(agent) > 256 {
		agent = agent[:256]
	}
	key := fmt.Sprintf("login:%d:%s", user.Id, securityHash(ip+"|"+agent))
	if !claimAccountAlert(key, 1, time.Now().Unix(), 1440) {
		return
	}
	message := fmt.Sprintf("账号 %s 于 %s 登录。IP：%s；浏览器：%s。如非本人操作，请立即联系管理员。", user.Username, time.Now().UTC().Format(time.RFC3339), ip, agent)
	status := "sent"
	if err := accountEmailSender("账号登录提醒", user.Email, html.EscapeString(message)); err != nil {
		status = "failed"
	}
	recordAccountEvent(user.Id, "login", message, status)
}

type AccountUsage struct {
	Quota    int64
	Requests int
	Models   map[string]int
}

// Video reserve/refund logs are cash movements, not two separate requests.
// Count a video only on settlement, using actual_quota from its frozen snapshot.
func AccountUsageFromLogs(logs []model.Log) map[int]*AccountUsage {
	out := map[int]*AccountUsage{}
	seen := map[string]bool{}
	for _, l := range logs {
		var m struct {
			Mode   string `json:"billing_mode"`
			Task   string `json:"task_id"`
			Actual *int64 `json:"actual_quota"`
		}
		if err := json.Unmarshal([]byte(l.Other), &m); err != nil && l.Other != "" {
			continue
		}
		quota, count := int64(l.Quota), 0
		if m.Mode == "video_tokens" {
			if m.Actual == nil || m.Task == "" {
				continue
			}
			key := fmt.Sprintf("%d:%s", l.UserId, m.Task)
			if seen[key] {
				continue
			}
			seen[key] = true
			quota = *m.Actual
			if quota > 0 {
				count = 1
			}
		} else if l.Type == model.LogTypeRefund {
			quota = -quota
		} else if model.IsConsumeLogType(l.Type) {
			count = 1
		} else {
			continue
		}
		if out[l.UserId] == nil {
			out[l.UserId] = &AccountUsage{Models: map[string]int{}}
		}
		u := out[l.UserId]
		u.Quota += quota
		u.Requests += count
		u.Models[l.ModelName] += count
	}
	return out
}

func AccountLowLevel(balance float64, r model.AccountAlertRules) int {
	if !r.LowEnabled || balance >= r.LowUSD {
		return 0
	}
	if balance < r.CriticalUSD {
		return 2
	}
	return 1
}

func AccountUsageTriggered(current, baseline AccountUsage, r model.AccountAlertRules, unit float64) bool {
	if !r.UsageEnabled || unit <= 0 {
		return false
	}
	spend := float64(current.Quota) / unit
	return (r.SpendUSD > 0 && spend >= r.SpendUSD) || (r.Requests > 0 && current.Requests >= r.Requests) ||
		(r.SpikeRatio > 1 && baseline.Quota > 0 && spend >= r.SpikeMinUSD && float64(current.Quota) >= float64(baseline.Quota)/7*r.SpikeRatio)
}

func sendAccountAdminAlert(g model.AccountSecurityPolicy, user model.User, kind, message string) {
	status := "sent"
	count := 0
	for _, email := range model.SecurityList(g.AdminEmails) {
		count++
		if accountEmailSender("平台账户告警", email, html.EscapeString(message)) != nil {
			status = "failed"
		}
	}
	if kind == "balance" {
		for _, phone := range model.SecurityList(g.AdminPhones) {
			count++
			if accountSMSSender(phone, g.SMSTemplate, user.Username) != nil {
				status = "failed"
			}
		}
	}
	if count == 0 {
		status = "not_configured"
	}
	recordAccountEvent(user.Id, kind, message, status)
}

// Run on the master node; persistent claim keys additionally cover rolling overlap.
// A bounded query fails visibly rather than sending alerts from truncated data.
func ScanAccountAlerts(now time.Time) error {
	g, err := model.GetAccountSecurityPolicy(0)
	if err != nil {
		return err
	}
	if g.AdminEmails == "" && g.AdminPhones == "" {
		return nil
	}
	var policies []model.AccountSecurity
	if err = model.DB.Find(&policies).Error; err != nil {
		return err
	}
	overrides := map[int]model.AccountSecurityPolicy{}
	for _, row := range policies {
		var p model.AccountSecurityPolicy
		if json.Unmarshal([]byte(row.Document), &p) == nil && p.OverrideAlerts {
			overrides[row.UserID] = p
		}
	}
	var users []model.User
	lastID := 0
	// Each distinct window is queried once, rather than scanning all logs per user.
	type windowUsage struct{ current, baseline map[int]*AccountUsage }
	windows := map[int]windowUsage{}
	for {
		users = nil
		if err = model.DB.Select("id", "username", "quota").Where("id > ? AND status = ?", lastID, common.UserStatusEnabled).Order("id").Limit(200).Find(&users).Error; err != nil {
			return err
		}
		if len(users) == 0 {
			break
		}
		for _, u := range users {
			lastID = u.Id
			r := g.Rules
			if p, ok := overrides[u.Id]; ok {
				r = p.Rules
			}
			if r.LowEnabled {
				// Redis carries pending batch debits; SQL alone can be stale.
				q, err := model.GetUserQuota(u.Id, false)
				if err != nil {
					return err
				}
				balance := float64(q) / common.QuotaPerUnit
				if claimAccountAlert(fmt.Sprintf("balance:%d", u.Id), AccountLowLevel(balance, r), now.Unix(), r.CooldownMinutes) {
					sendAccountAdminAlert(g, u, "balance", fmt.Sprintf("用户 %s（ID %d）余额不足，当前可用余额 USD %.6f，普通/紧急阈值 USD %.2f / %.2f。余额可能包含尚未结算的任务预扣，请在用户管理和使用日志核对。", u.Username, u.Id, balance, r.LowUSD, r.CriticalUSD))
				}
			}
			if !r.UsageEnabled {
				continue
			}
			w, ok := windows[r.WindowMinutes]
			if !ok {
				var logs []model.Log
				start := now.Add(-time.Duration(r.WindowMinutes) * time.Minute).Unix()
				periods := model.LOG_DB.Where("created_at >= ? AND created_at < ?", start, now.Unix())
				for day := int64(1); day <= 7; day++ {
					periods = periods.Or("created_at >= ? AND created_at < ?", start-day*86400, now.Unix()-day*86400)
				}
				err = model.LOG_DB.Where(periods).Where("type IN ?", []int{model.LogTypeConsume, model.LogTypeRefund, model.LogTypeMarketConsume, model.LogTypeProjectConsume}).Order("id").Limit(100001).Find(&logs).Error
				if err != nil {
					return err
				}
				if len(logs) > 100000 {
					return errors.New("用量监测超过10万条窗口上限，请缩小窗口或配置聚合存储；未发送截断数据告警")
				}
				var current, baseline []model.Log
				for _, l := range logs {
					if l.CreatedAt >= start {
						current = append(current, l)
						continue
					}
					// Seven previous same-time windows, not an all-day average.
					for day := int64(1); day <= 7; day++ {
						if l.CreatedAt >= start-day*86400 && l.CreatedAt < now.Unix()-day*86400 {
							baseline = append(baseline, l)
							break
						}
					}
				}
				w = windowUsage{AccountUsageFromLogs(current), AccountUsageFromLogs(baseline)}
				windows[r.WindowMinutes] = w
			}
			cur, base := AccountUsage{}, AccountUsage{}
			if w.current[u.Id] != nil {
				cur = *w.current[u.Id]
			}
			if w.baseline[u.Id] != nil {
				base = *w.baseline[u.Id]
			}
			if AccountUsageTriggered(cur, base, r, common.QuotaPerUnit) && claimAccountAlert(fmt.Sprintf("usage:%d", u.Id), 1, now.Unix(), r.CooldownMinutes) {
				models, _ := json.Marshal(cur.Models)
				sendAccountAdminAlert(g, u, "usage", fmt.Sprintf("用户 %s（ID %d）用量告警：截至 %s，最近%d分钟净结算用量 USD %.6f、计费请求记录%d条。模型分布：%s。视频仅计结算任务；订阅为额度折算，不是现金扣款。请在使用日志按用户和时间核对。", u.Username, u.Id, now.UTC().Format(time.RFC3339), r.WindowMinutes, float64(cur.Quota)/common.QuotaPerUnit, cur.Requests, string(models)))
			}
		}
	}
	return nil
}

func StartAccountSecurityMonitor() {
	if !common.IsMasterNode {
		return
	}
	go func() {
		for range time.NewTicker(time.Minute).C {
			if err := ScanAccountAlerts(time.Now()); err != nil {
				common.SysLog("account alert scan failed: " + err.Error())
				if claimAccountAlert("monitor-error", 1, time.Now().Unix(), 60) {
					recordAccountEvent(0, "monitor", err.Error(), "failed")
				}
			}
			// Retain operational records for 90 days, prune a bounded batch.
			var ids []int64
			if model.DB.Model(&model.AccountSecurityEvent{}).Where("created_at < ?", time.Now().AddDate(0, 0, -90).Unix()).Limit(500).Pluck("id", &ids).Error == nil && len(ids) > 0 {
				model.DB.Delete(&model.AccountSecurityEvent{}, ids)
			}
		}
	}()
}
