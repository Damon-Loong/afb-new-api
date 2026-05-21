package controller

import (
	"crypto/rand"
	"encoding/hex"
	"math"
	mathrand "math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	wechatQRUpdateToken          = "mbm-dev-token"
	wechatQRScanTTL              = 30 * time.Minute
	wechatQRSessionTTL           = time.Hour
	wechatQRHeartbeatOfflineTime = time.Minute

	wechatQRStatusWaiting = "waiting"
	wechatQRStatusActive  = "active"
	wechatQRStatusEnding  = "ending"
	wechatQRStatusOffline = "offline"
	wechatQRStatusError   = "error"
	wechatQRStatusExpired = "expired"
)

type wechatQRUpdateRequest struct {
	DeviceID   string `json:"deviceId"`
	DeviceName string `json:"deviceName"`
	QRURL      string `json:"qrUrl"`
}

type wechatQRDeviceRequest struct {
	DeviceID string `json:"deviceId"`
}

type wechatQRHeartbeatRequest struct {
	DeviceID   string `json:"deviceId"`
	DeviceName string `json:"deviceName"`
	Status     string `json:"status"`
}

type wechatQRDeviceRecord struct {
	DeviceID        string
	DeviceName      string
	QRURL           string
	Status          string
	UpdatedAt       time.Time
	QRExpireAt      time.Time
	LastHeartbeatAt time.Time
	ActivatedAt     *time.Time
	SessionID       string
	SessionExpireAt *time.Time
}

var (
	wechatQRRecordsMu sync.RWMutex
	wechatQRRecords   = map[string]wechatQRDeviceRecord{}
)

func UpdateWechatQR(c *gin.Context) {
	if !checkWechatQRToken(c) {
		return
	}

	var req wechatQRUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求体格式错误",
		})
		return
	}

	deviceID := strings.TrimSpace(req.DeviceID)
	deviceName := strings.TrimSpace(req.DeviceName)
	qrURL := strings.TrimSpace(req.QRURL)
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "deviceId 不能为空",
		})
		return
	}
	if qrURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "qrUrl 不能为空",
		})
		return
	}
	if !isValidWechatQRURL(qrURL) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "qrUrl 格式不正确",
		})
		return
	}

	now := time.Now()

	wechatQRRecordsMu.Lock()
	record := wechatQRRecords[deviceID]
	record.DeviceID = deviceID
	if deviceName != "" {
		record.DeviceName = deviceName
	}
	record.QRURL = qrURL
	record.Status = wechatQRStatusWaiting
	record.UpdatedAt = now
	record.QRExpireAt = now.Add(wechatQRScanTTL)
	record.LastHeartbeatAt = now
	record.ActivatedAt = nil
	record.SessionID = ""
	record.SessionExpireAt = nil
	wechatQRRecords[deviceID] = record
	wechatQRRecordsMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "二维码已更新",
		"data":    wechatQRDeviceResponseData(record, now),
	})
}

func GetCurrentWechatQR(c *gin.Context) {
	deviceID := strings.TrimSpace(c.Query("deviceId"))
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "deviceId 不能为空",
		})
		return
	}

	wechatQRRecordsMu.RLock()
	record, ok := wechatQRRecords[deviceID]
	wechatQRRecordsMu.RUnlock()

	if !ok {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "暂无二维码，请先启动 MClaw 微信通道",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "二维码已获取",
		"data":    wechatQRDeviceResponseData(record, time.Now()),
	})
}

func GetAvailableWechatQR(c *gin.Context) {
	now := time.Now()
	candidates := make([]wechatQRDeviceRecord, 0)

	wechatQRRecordsMu.RLock()
	for _, record := range wechatQRRecords {
		if effectiveWechatQRStatus(record, now) == wechatQRStatusWaiting && record.QRURL != "" {
			candidates = append(candidates, record)
		}
	}
	wechatQRRecordsMu.RUnlock()

	if len(candidates) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "当前暂无空闲电脑，请稍后再试",
		})
		return
	}

	record := candidates[mathrand.Intn(len(candidates))]
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "二维码已获取",
		"data":    wechatQRDeviceResponseData(record, now),
	})
}

func ActivateWechatQR(c *gin.Context) {
	if !checkWechatQRToken(c) {
		return
	}

	var req wechatQRDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求体格式错误",
		})
		return
	}

	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "deviceId 不能为空",
		})
		return
	}

	now := time.Now()
	sessionExpireAt := now.Add(wechatQRSessionTTL)

	wechatQRRecordsMu.Lock()
	record, ok := wechatQRRecords[deviceID]
	if !ok {
		wechatQRRecordsMu.Unlock()
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "设备不存在",
		})
		return
	}
	sessionID := newWechatQRSessionID()
	record.Status = wechatQRStatusActive
	record.ActivatedAt = &now
	record.SessionID = sessionID
	record.SessionExpireAt = &sessionExpireAt
	record.LastHeartbeatAt = now
	wechatQRRecords[deviceID] = record
	wechatQRRecordsMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "设备已激活",
		"data":    wechatQRDeviceResponseData(record, now),
	})
}

func ReleaseWechatQR(c *gin.Context) {
	if !checkWechatQRToken(c) {
		return
	}

	var req wechatQRDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求体格式错误",
		})
		return
	}

	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "deviceId 不能为空",
		})
		return
	}

	now := time.Now()

	wechatQRRecordsMu.Lock()
	record, ok := wechatQRRecords[deviceID]
	if !ok {
		wechatQRRecordsMu.Unlock()
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "设备不存在",
		})
		return
	}
	record.ActivatedAt = nil
	record.SessionID = ""
	record.SessionExpireAt = nil
	if record.QRURL != "" && now.Before(record.QRExpireAt) {
		record.Status = wechatQRStatusWaiting
	} else {
		record.Status = wechatQRStatusExpired
	}
	wechatQRRecords[deviceID] = record
	wechatQRRecordsMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "设备已释放",
		"data":    wechatQRDeviceResponseData(record, now),
	})
}

func HeartbeatWechatQR(c *gin.Context) {
	if !checkWechatQRToken(c) {
		return
	}

	var req wechatQRHeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求体格式错误",
		})
		return
	}

	deviceID := strings.TrimSpace(req.DeviceID)
	deviceName := strings.TrimSpace(req.DeviceName)
	status := strings.TrimSpace(req.Status)
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "deviceId 不能为空",
		})
		return
	}

	now := time.Now()

	wechatQRRecordsMu.Lock()
	record := wechatQRRecords[deviceID]
	record.DeviceID = deviceID
	if deviceName != "" {
		record.DeviceName = deviceName
	}
	if status != "" && isValidWechatQRStatus(status) {
		record.Status = status
	}
	record.LastHeartbeatAt = now
	wechatQRRecords[deviceID] = record
	wechatQRRecordsMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "心跳已更新",
		"data":    wechatQRDeviceResponseData(record, now),
	})
}

func GetWechatQRSessionStatus(c *gin.Context) {
	deviceID := strings.TrimSpace(c.Query("deviceId"))
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "deviceId 不能为空",
		})
		return
	}

	wechatQRRecordsMu.RLock()
	record, ok := wechatQRRecords[deviceID]
	wechatQRRecordsMu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "设备不存在",
		})
		return
	}

	now := time.Now()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    wechatQRSessionStatusData(record, now),
	})
}

func GetWechatQRDevices(c *gin.Context) {
	now := time.Now()
	devices := make([]gin.H, 0)

	wechatQRRecordsMu.RLock()
	for _, record := range wechatQRRecords {
		devices = append(devices, wechatQRDeviceResponseData(record, now))
	}
	wechatQRRecordsMu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    devices,
	})
}

func checkWechatQRToken(c *gin.Context) bool {
	if strings.TrimSpace(c.GetHeader("Authorization")) != "Bearer "+wechatQRUpdateToken {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "unauthorized",
		})
		return false
	}
	return true
}

func isValidWechatQRURL(qrURL string) bool {
	return strings.HasPrefix(qrURL, "https://liteapp.weixin.qq.com/") || strings.Contains(qrURL, "qrcode=")
}

func isValidWechatQRStatus(status string) bool {
	switch status {
	case wechatQRStatusWaiting, wechatQRStatusActive, wechatQRStatusEnding, wechatQRStatusOffline, wechatQRStatusError, wechatQRStatusExpired:
		return true
	default:
		return false
	}
}

func effectiveWechatQRStatus(record wechatQRDeviceRecord, now time.Time) string {
	if !record.LastHeartbeatAt.IsZero() && now.Sub(record.LastHeartbeatAt) > wechatQRHeartbeatOfflineTime {
		return wechatQRStatusOffline
	}
	if record.Status == wechatQRStatusActive && record.SessionExpireAt != nil && !now.Before(*record.SessionExpireAt) {
		return wechatQRStatusEnding
	}
	if record.Status == wechatQRStatusWaiting && !record.QRExpireAt.IsZero() && !now.Before(record.QRExpireAt) {
		return wechatQRStatusExpired
	}
	if record.Status == "" {
		return wechatQRStatusExpired
	}
	return record.Status
}

func wechatQRDeviceResponseData(record wechatQRDeviceRecord, now time.Time) gin.H {
	status := effectiveWechatQRStatus(record, now)
	remainingSeconds := remainingSecondsUntil(record.QRExpireAt, now)
	activeDurationSeconds := 0
	if record.ActivatedAt != nil {
		activeDurationSeconds = int(math.Floor(now.Sub(*record.ActivatedAt).Seconds()))
		if activeDurationSeconds < 0 {
			activeDurationSeconds = 0
		}
	}

	return gin.H{
		"deviceId":              record.DeviceID,
		"deviceName":            record.DeviceName,
		"qrUrl":                 record.QRURL,
		"hasQrUrl":              record.QRURL != "",
		"status":                status,
		"updatedAt":             zeroTimeToNil(record.UpdatedAt),
		"qrExpireAt":            zeroTimeToNil(record.QRExpireAt),
		"expireAt":              zeroTimeToNil(record.QRExpireAt),
		"lastHeartbeatAt":       zeroTimeToNil(record.LastHeartbeatAt),
		"activatedAt":           timePtrToAny(record.ActivatedAt),
		"sessionId":             record.SessionID,
		"sessionExpireAt":       timePtrToAny(record.SessionExpireAt),
		"remainingSeconds":      remainingSeconds,
		"activeDurationSeconds": activeDurationSeconds,
	}
}

func wechatQRSessionStatusData(record wechatQRDeviceRecord, now time.Time) gin.H {
	status := effectiveWechatQRStatus(record, now)
	remainingSeconds := 0
	expired := false
	if record.SessionExpireAt != nil {
		remainingSeconds = remainingSecondsUntil(*record.SessionExpireAt, now)
		expired = !now.Before(*record.SessionExpireAt)
	}

	return gin.H{
		"deviceId":         record.DeviceID,
		"status":           status,
		"sessionId":        record.SessionID,
		"sessionExpireAt":  timePtrToAny(record.SessionExpireAt),
		"remainingSeconds": remainingSeconds,
		"expired":          expired,
	}
}

func remainingSecondsUntil(t time.Time, now time.Time) int {
	if t.IsZero() {
		return 0
	}
	remainingSeconds := int(math.Ceil(t.Sub(now).Seconds()))
	if remainingSeconds < 0 {
		return 0
	}
	return remainingSeconds
}

func zeroTimeToNil(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t
}

func timePtrToAny(t *time.Time) interface{} {
	if t == nil || t.IsZero() {
		return nil
	}
	return *t
}

func newWechatQRSessionID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err == nil {
		return "wqs_" + hex.EncodeToString(buf)
	}
	return "wqs_" + time.Now().Format("20060102150405.000000000")
}
