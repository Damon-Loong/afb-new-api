package controller

import (
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const wechatQRUpdateToken = "mbm-dev-token"

type wechatQRUpdateRequest struct {
	DeviceID string `json:"deviceId"`
	QRURL    string `json:"qrUrl"`
}

type wechatQRRecord struct {
	DeviceID  string    `json:"deviceId"`
	QRURL     string    `json:"qrUrl"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updatedAt"`
	ExpireAt  time.Time `json:"expireAt"`
}

var (
	wechatQRRecordsMu sync.RWMutex
	wechatQRRecords   = map[string]wechatQRRecord{}
)

func UpdateWechatQR(c *gin.Context) {
	if strings.TrimSpace(c.GetHeader("Authorization")) != "Bearer "+wechatQRUpdateToken {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "unauthorized",
		})
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
	qrURL := strings.TrimSpace(req.QRURL)
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
	record := wechatQRRecord{
		DeviceID:  deviceID,
		QRURL:     qrURL,
		Status:    "waiting",
		UpdatedAt: now,
		ExpireAt:  now.Add(30 * time.Minute),
	}

	wechatQRRecordsMu.Lock()
	wechatQRRecords[deviceID] = record
	wechatQRRecordsMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "二维码已更新",
		"data":    wechatQRResponseData(record, now),
	})
}

func GetCurrentWechatQR(c *gin.Context) {
	deviceID := strings.TrimSpace(c.Query("deviceId"))

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
		"data":    wechatQRResponseData(record, time.Now()),
	})
}

func isValidWechatQRURL(qrURL string) bool {
	return strings.HasPrefix(qrURL, "https://liteapp.weixin.qq.com/") || strings.Contains(qrURL, "qrcode=")
}

func wechatQRResponseData(record wechatQRRecord, now time.Time) gin.H {
	status := record.Status
	remainingSeconds := int(math.Ceil(record.ExpireAt.Sub(now).Seconds()))
	if remainingSeconds < 0 {
		remainingSeconds = 0
	}
	if !now.Before(record.ExpireAt) {
		status = "expired"
	}

	return gin.H{
		"deviceId":         record.DeviceID,
		"qrUrl":            record.QRURL,
		"status":           status,
		"updatedAt":        record.UpdatedAt,
		"expireAt":         record.ExpireAt,
		"remainingSeconds": remainingSeconds,
	}
}
