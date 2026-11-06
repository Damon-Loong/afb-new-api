package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/mail"
	"net/netip"
	"strings"
)

// Separate from User.Setting: ordinary profile updates must not alter admin policy.
type AccountSecurity struct {
	UserID   int    `gorm:"primaryKey;autoIncrement:false"`
	Document string `gorm:"type:text"`
}

type AccountLoginState struct {
	UserID      int    `gorm:"primaryKey;autoIncrement:false"`
	SessionHash string `gorm:"size:64"`
}

type AccountAlertState struct {
	Key    string `gorm:"column:alert_key;primaryKey;size:160"`
	Level  int
	NextAt int64
}

type AccountSecurityEvent struct {
	ID        int64  `json:"id" gorm:"primaryKey"`
	UserID    int    `json:"user_id" gorm:"index"`
	Kind      string `json:"kind" gorm:"size:32"`
	CreatedAt int64  `json:"created_at" gorm:"index"`
	Message   string `json:"message" gorm:"type:text"`
	Delivery  string `json:"delivery" gorm:"size:32"`
}

type AccountAlertRules struct {
	LowEnabled      bool    `json:"low_enabled"`
	LowUSD          float64 `json:"low_usd"`
	CriticalUSD     float64 `json:"critical_usd"`
	UsageEnabled    bool    `json:"usage_enabled"`
	WindowMinutes   int     `json:"window_minutes"`
	SpendUSD        float64 `json:"spend_usd"`
	Requests        int     `json:"requests"`
	SpikeRatio      float64 `json:"spike_ratio"`
	SpikeMinUSD     float64 `json:"spike_min_usd"`
	CooldownMinutes int     `json:"cooldown_minutes"`
}

type AccountSecurityPolicy struct {
	SingleSession  bool              `json:"single_session"`
	LoginIPs       string            `json:"login_ips"`
	LoginNotify    bool              `json:"login_notify"`
	OverrideAlerts bool              `json:"override_alerts"`
	Rules          AccountAlertRules `json:"rules"`
	// Only user_id=0 may supply notification destinations and trusted proxies.
	AdminEmails    string `json:"admin_emails"`
	AdminPhones    string `json:"admin_phones"`
	SMSTemplate    string `json:"sms_template"`
	TrustedProxies string `json:"trusted_proxies"`
}

func DefaultAccountSecurityPolicy() AccountSecurityPolicy {
	return AccountSecurityPolicy{Rules: AccountAlertRules{LowUSD: 20, CriticalUSD: 5, WindowMinutes: 10, SpendUSD: 10, Requests: 1000, SpikeRatio: 0, SpikeMinUSD: 5, CooldownMinutes: 60}}
}

func GetAccountSecurityPolicy(userID int) (AccountSecurityPolicy, error) {
	p := DefaultAccountSecurityPolicy()
	var row AccountSecurity
	r := DB.Where("user_id = ?", userID).Limit(1).Find(&row)
	err := r.Error
	if err != nil {
		return p, err
	}
	if r.RowsAffected == 0 {
		return p, nil
	}
	err = json.Unmarshal([]byte(row.Document), &p)
	return p, err
}

func SecurityList(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' || r == '\n' || r == '\r' || r == ' ' })
}

func ParseSecurityNetworks(value string) ([]netip.Prefix, error) {
	if len(value) > 4096 {
		return nil, errors.New("IP列表过长")
	}
	var out []netip.Prefix
	for _, item := range SecurityList(value) {
		if ip, err := netip.ParseAddr(item); err == nil {
			ip = ip.Unmap()
			out = append(out, netip.PrefixFrom(ip, ip.BitLen()))
			continue
		}
		p, err := netip.ParsePrefix(item)
		if err != nil {
			return nil, fmt.Errorf("IP或网段无效：%s", item)
		}
		out = append(out, p.Masked())
	}
	return out, nil
}

func SecurityIPAllowed(ip, allow string) bool {
	if strings.TrimSpace(allow) == "" {
		return true
	}
	nets, err := ParseSecurityNetworks(allow)
	addr, parseErr := netip.ParseAddr(ip)
	if err != nil || parseErr != nil {
		return false
	}
	for _, n := range nets {
		if n.Contains(addr.Unmap()) {
			return true
		}
	}
	return false
}

// Trust forwarding headers only when the direct peer is explicitly configured.
func SecurityClientIP(remote, forwarded, trusted string) string {
	peer := remote
	if ap, err := netip.ParseAddrPort(remote); err == nil {
		peer = ap.Addr().Unmap().String()
	}
	if trusted == "" || !SecurityIPAllowed(peer, trusted) {
		return peer
	}
	chain := strings.Split(forwarded, ",")
	for i := len(chain) - 1; i >= 0; i-- {
		ip, err := netip.ParseAddr(strings.TrimSpace(chain[i]))
		if err != nil {
			return peer
		}
		if !SecurityIPAllowed(ip.Unmap().String(), trusted) || i == 0 {
			return ip.Unmap().String()
		}
	}
	return peer
}

func (p AccountSecurityPolicy) Validate(global bool) error {
	if _, err := ParseSecurityNetworks(p.LoginIPs); err != nil {
		return err
	}
	proxies, err := ParseSecurityNetworks(p.TrustedProxies)
	if err != nil {
		return err
	}
	for _, proxy := range proxies {
		if proxy.Bits() == 0 {
			return errors.New("可信代理不能包含全网网段，请填写实际代理地址")
		}
	}
	if !global && (p.AdminEmails != "" || p.AdminPhones != "" || p.SMSTemplate != "" || p.TrustedProxies != "") {
		return errors.New("用户策略不能修改全局通知或代理配置")
	}
	r := p.Rules
	for _, v := range []float64{r.LowUSD, r.CriticalUSD, r.SpendUSD, r.SpikeRatio, r.SpikeMinUSD} {
		if v < 0 || v > 1e9 || math.IsNaN(v) || math.IsInf(v, 0) {
			return errors.New("阈值必须是有效的非负数")
		}
	}
	if r.CriticalUSD > r.LowUSD || r.WindowMinutes < 1 || r.WindowMinutes > 1440 || r.CooldownMinutes < 5 || r.CooldownMinutes > 10080 || r.Requests < 0 {
		return errors.New("请检查余额阈值、监测窗口（1～1440分钟）和通知间隔（5～10080分钟）")
	}
	if r.SpikeRatio != 0 && (r.SpikeRatio <= 1 || r.SpikeMinUSD <= 0) {
		return errors.New("激增倍数必须大于1且最低消费大于0；填0关闭历史比较")
	}
	if r.UsageEnabled && r.SpendUSD == 0 && r.Requests == 0 && r.SpikeRatio == 0 {
		return errors.New("用量告警至少设置一项阈值")
	}
	if len(SecurityList(p.AdminEmails)) > 10 || len(SecurityList(p.AdminPhones)) > 10 {
		return errors.New("通知收件人最多10个")
	}
	for _, s := range SecurityList(p.AdminEmails) {
		a, err := mail.ParseAddress(s)
		if err != nil || a.Address != s {
			return errors.New("通知邮箱格式无效")
		}
	}
	for _, s := range SecurityList(p.AdminPhones) {
		if len(s) != 11 || s[0] != '1' || strings.Trim(s, "0123456789") != "" {
			return errors.New("短信手机号须为11位大陆手机号")
		}
	}
	if len(p.SMSTemplate) > 64 || (p.AdminPhones != "" && !strings.HasPrefix(p.SMSTemplate, "SMS_")) {
		return errors.New("短信提醒须单独配置审核通过的SMS_通知模板，不能使用验证码模板")
	}
	return nil
}
