package common

import (
	"sync"
	"time"
)

// SMS verification storage (in-memory).
// Purpose: keep short-lived OTP for login/bind, with basic anti-bruteforce.

type smsCodeValue struct {
	code      string
	expiresAt time.Time

	// attempt tracking
	failCount   int
	lockedUntil time.Time
}

var (
	smsCodeMu  sync.Mutex
	smsCodeMap = map[string]*smsCodeValue{}
)

const (
	SMSCodeValidMinutes        = 5
	SMSVerificationCodeLength  = 4
	smsMaxFailCount            = 8
	smsLockDuration            = 10 * time.Minute
)

func smsKey(purpose, phone string) string { return purpose + ":" + phone }

func SetSMSCode(purpose, phone, code string) {
	smsCodeMu.Lock()
	defer smsCodeMu.Unlock()
	smsCodeMap[smsKey(purpose, phone)] = &smsCodeValue{
		code:      code,
		expiresAt: time.Now().Add(SMSCodeValidMinutes * time.Minute),
	}
}

func VerifySMSCode(purpose, phone, code string) (ok bool, locked bool) {
	smsCodeMu.Lock()
	defer smsCodeMu.Unlock()

	key := smsKey(purpose, phone)
	v, exists := smsCodeMap[key]
	if !exists || v == nil {
		return false, false
	}

	now := time.Now()
	if !v.lockedUntil.IsZero() && now.Before(v.lockedUntil) {
		return false, true
	}
	if now.After(v.expiresAt) {
		delete(smsCodeMap, key)
		return false, false
	}
	if v.code != code {
		v.failCount++
		if v.failCount >= smsMaxFailCount {
			v.lockedUntil = now.Add(smsLockDuration)
		}
		return false, false
	}
	// success
	delete(smsCodeMap, key)
	return true, false
}

