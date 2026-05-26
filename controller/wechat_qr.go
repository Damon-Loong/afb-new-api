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

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const (
	wechatQRUpdateToken          = "mbm-dev-token"
	wechatQRScanTTL              = 30 * time.Minute
	wechatQRSessionTTL           = time.Hour
	wechatQRHeartbeatOfflineTime = time.Minute
	wechatQRAssignTimeout        = 30 * time.Second
	wechatQRGenerateTimeout      = time.Minute
	wechatQRWaitScanTimeout      = 2 * time.Minute

	wechatQRStatusIdle        = "idle"
	wechatQRStatusGenerating  = "generating"
	wechatQRStatusWaitingScan = "waiting_scan"
	wechatQRStatusWaiting     = "waiting"
	wechatQRStatusActive      = "active"
	wechatQRStatusEnding      = "ending"
	wechatQRStatusOffline     = "offline"
	wechatQRStatusError       = "error"
	wechatQRStatusExpired     = "expired"

	wechatQRRequestStatusPending    = "pending"
	wechatQRRequestStatusAssigned   = "assigned"
	wechatQRRequestStatusGenerating = "generating"
	wechatQRRequestStatusQRReady    = "qr_ready"
	wechatQRRequestStatusActive     = "active"
	wechatQRRequestStatusTimeout    = "timeout"
	wechatQRRequestStatusFailed     = "failed"

	wechatQRTaskActionGenerateQR = "generate_qr"

	wechatQRTaskStatusPending    = "pending"
	wechatQRTaskStatusGenerating = "generating"
	wechatQRTaskStatusDone       = "done"
	wechatQRTaskStatusFailed     = "failed"
	wechatQRTaskStatusTimeout    = "timeout"
)

type wechatQRUpdateRequest struct {
	DeviceID   string `json:"deviceId"`
	DeviceName string `json:"deviceName"`
	RequestID  string `json:"requestId"`
	TaskID     string `json:"taskId"`
	QRURL      string `json:"qrUrl"`
}

type wechatQRDeviceRequest struct {
	DeviceID  string `json:"deviceId"`
	RequestID string `json:"requestId"`
	Reason    string `json:"reason"`
}

type wechatQRHeartbeatRequest struct {
	DeviceID   string `json:"deviceId"`
	DeviceName string `json:"deviceName"`
	Status     string `json:"status"`
}

type wechatQRDeviceRecord struct {
	DeviceID         string
	DeviceName       string
	QRURL            string
	Status           string
	CurrentRequestID string
	CurrentTaskID    string
	UpdatedAt        time.Time
	QRExpireAt       time.Time
	LastHeartbeatAt  time.Time
	ActivatedAt      *time.Time
	SessionID        string
	SessionExpireAt  *time.Time
}

type wechatQRRequestRecord struct {
	RequestID       string
	DeviceID        string
	DeviceName      string
	TaskID          string
	UserID          int
	TokenID         int
	Status          string
	QRURL           string
	SessionID       string
	Message         string
	CreatedAt       time.Time
	AssignedAt      time.Time
	GeneratingAt    time.Time
	QRReadyAt       time.Time
	ActivatedAt     time.Time
	SessionExpireAt *time.Time
	FailedAt        time.Time
	TimeoutAt       time.Time
}

type wechatQRTaskRecord struct {
	TaskID    string
	RequestID string
	DeviceID  string
	UserID    int
	TokenID   int
	Action    string
	Status    string
	CreatedAt time.Time
	ClaimedAt time.Time
	DoneAt    time.Time
	FailedAt  time.Time
	TimeoutAt time.Time
}

var (
	wechatQRRecordsMu sync.RWMutex
	wechatQRRecords   = map[string]wechatQRDeviceRecord{}
	wechatQRRequests  = map[string]wechatQRRequestRecord{}
	wechatQRTasks     = map[string]wechatQRTaskRecord{}
)

func RequestWechatQR(c *gin.Context) {
	now := time.Now()
	userID := c.GetInt("id")
	if userID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "请先登录",
		})
		return
	}

	userToken, ok := getWechatQRUsableUserToken(userID)
	if !ok {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "请先配置 OpenClaw API Key",
		})
		return
	}

	wechatQRRecordsMu.Lock()
	applyWechatQRTimeoutsLocked(now)

	candidates := make([]wechatQRDeviceRecord, 0)
	for _, record := range wechatQRRecords {
		if isWechatQRDeviceAssignable(record, now) {
			candidates = append(candidates, record)
		}
	}

	if len(candidates) == 0 {
		wechatQRRecordsMu.Unlock()
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "当前暂无空闲电脑，请稍后再试",
		})
		return
	}

	record := candidates[mathrand.Intn(len(candidates))]
	requestID := newWechatQRID("req")
	taskID := newWechatQRID("task")

	request := wechatQRRequestRecord{
		RequestID:  requestID,
		DeviceID:   record.DeviceID,
		DeviceName: record.DeviceName,
		TaskID:     taskID,
		UserID:     userID,
		TokenID:    userToken.Id,
		Status:     wechatQRRequestStatusAssigned,
		Message:    "已分配空闲电脑",
		CreatedAt:  now,
		AssignedAt: now,
	}
	task := wechatQRTaskRecord{
		TaskID:    taskID,
		RequestID: requestID,
		DeviceID:  record.DeviceID,
		UserID:    userID,
		TokenID:   userToken.Id,
		Action:    wechatQRTaskActionGenerateQR,
		Status:    wechatQRTaskStatusPending,
		CreatedAt: now,
	}

	record.Status = wechatQRStatusGenerating
	record.CurrentRequestID = requestID
	record.CurrentTaskID = taskID
	record.QRURL = ""
	record.QRExpireAt = time.Time{}
	record.UpdatedAt = now

	wechatQRRequests[requestID] = request
	wechatQRTasks[taskID] = task
	wechatQRRecords[record.DeviceID] = record
	wechatQRRecordsMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "已分配空闲电脑",
		"data": gin.H{
			"requestId":  requestID,
			"taskId":     taskID,
			"deviceId":   record.DeviceID,
			"deviceName": record.DeviceName,
			"status":     wechatQRRequestStatusAssigned,
		},
	})
}

func GetWechatQRRequestStatus(c *gin.Context) {
	requestID := strings.TrimSpace(c.Query("requestId"))
	if requestID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "requestId 不能为空",
		})
		return
	}

	now := time.Now()

	wechatQRRecordsMu.Lock()
	applyWechatQRTimeoutsLocked(now)
	request, ok := wechatQRRequests[requestID]
	if !ok {
		wechatQRRecordsMu.Unlock()
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "请求不存在",
		})
		return
	}
	record := wechatQRRecords[request.DeviceID]
	data := wechatQRRequestResponseData(request, record, now)
	wechatQRRecordsMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

func GetWechatQRTask(c *gin.Context) {
	if !checkWechatQRToken(c) {
		return
	}

	deviceID := strings.TrimSpace(c.Query("deviceId"))
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "deviceId 不能为空",
		})
		return
	}

	now := time.Now()

	wechatQRRecordsMu.Lock()
	applyWechatQRTimeoutsLocked(now)

	record, ok := wechatQRRecords[deviceID]
	if !ok {
		wechatQRRecordsMu.Unlock()
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "暂无任务",
			"data":    nil,
		})
		return
	}

	task, taskOK := wechatQRTasks[record.CurrentTaskID]
	request, requestOK := wechatQRRequests[record.CurrentRequestID]
	if !taskOK || !requestOK || task.Status != wechatQRTaskStatusPending || task.Action != wechatQRTaskActionGenerateQR {
		wechatQRRecordsMu.Unlock()
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "暂无任务",
			"data":    nil,
		})
		return
	}

	task.Status = wechatQRTaskStatusGenerating
	task.ClaimedAt = now
	request.Status = wechatQRRequestStatusGenerating
	request.GeneratingAt = now
	request.Message = "正在生成二维码"
	record.Status = wechatQRStatusGenerating
	record.UpdatedAt = now

	wechatQRTasks[task.TaskID] = task
	wechatQRRequests[request.RequestID] = request
	wechatQRRecords[deviceID] = record
	wechatQRRecordsMu.Unlock()

	userToken, ok := getWechatQRTaskUserToken(task.UserID, task.TokenID)
	if !ok {
		wechatQRRecordsMu.Lock()
		failWechatQRRequestLocked(request.RequestID, wechatQRRequestStatusFailed, time.Now(), "用户 OpenClaw API Key 不可用")
		wechatQRRecordsMu.Unlock()
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "用户 OpenClaw API Key 不可用",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"taskId":         task.TaskID,
			"requestId":      task.RequestID,
			"action":         task.Action,
			"openclawApiKey": userToken.GetFullKey(),
		},
	})
}

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
	requestID := strings.TrimSpace(req.RequestID)
	taskID := strings.TrimSpace(req.TaskID)
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
	applyWechatQRTimeoutsLocked(now)

	record := wechatQRRecords[deviceID]
	record.DeviceID = deviceID
	if deviceName != "" {
		record.DeviceName = deviceName
	}
	record.QRURL = qrURL
	record.UpdatedAt = now
	record.QRExpireAt = now.Add(wechatQRScanTTL)
	record.LastHeartbeatAt = now
	record.ActivatedAt = nil
	record.SessionID = ""
	record.SessionExpireAt = nil

	if requestID != "" || taskID != "" {
		request, requestOK := wechatQRRequests[requestID]
		task, taskOK := wechatQRTasks[taskID]
		if requestID == "" || taskID == "" || !requestOK || !taskOK {
			wechatQRRecordsMu.Unlock()
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "requestId 或 taskId 无效",
			})
			return
		}
		if request.DeviceID != deviceID || task.DeviceID != deviceID || task.RequestID != requestID {
			wechatQRRecordsMu.Unlock()
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "任务与设备不匹配",
			})
			return
		}

		request.Status = wechatQRRequestStatusQRReady
		request.QRURL = qrURL
		request.DeviceName = record.DeviceName
		request.QRReadyAt = now
		request.Message = "二维码已生成"
		task.Status = wechatQRTaskStatusDone
		task.DoneAt = now
		record.Status = wechatQRStatusWaitingScan
		record.CurrentRequestID = requestID
		record.CurrentTaskID = taskID

		wechatQRRequests[requestID] = request
		wechatQRTasks[taskID] = task
	} else {
		record.Status = wechatQRStatusWaiting
		record.CurrentRequestID = ""
		record.CurrentTaskID = ""
	}

	wechatQRRecords[deviceID] = record
	data := wechatQRDeviceResponseData(record, now)
	wechatQRRecordsMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "二维码已更新",
		"data":    data,
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

	now := time.Now()

	wechatQRRecordsMu.Lock()
	applyWechatQRTimeoutsLocked(now)
	record, ok := wechatQRRecords[deviceID]
	data := wechatQRDeviceResponseData(record, now)
	wechatQRRecordsMu.Unlock()

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
		"data":    data,
	})
}

func GetAvailableWechatQR(c *gin.Context) {
	now := time.Now()
	candidates := make([]wechatQRDeviceRecord, 0)

	wechatQRRecordsMu.Lock()
	applyWechatQRTimeoutsLocked(now)
	for _, record := range wechatQRRecords {
		status := effectiveWechatQRStatus(record, now)
		if (status == wechatQRStatusWaiting || status == wechatQRStatusWaitingScan) && record.QRURL != "" {
			candidates = append(candidates, record)
		}
	}
	wechatQRRecordsMu.Unlock()

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
	requestID := strings.TrimSpace(req.RequestID)
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
	applyWechatQRTimeoutsLocked(now)

	record, ok := wechatQRRecords[deviceID]
	if !ok {
		wechatQRRecordsMu.Unlock()
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "设备不存在",
		})
		return
	}
	if requestID == "" {
		requestID = record.CurrentRequestID
	}

	sessionID := newWechatQRID("wqs")
	record.Status = wechatQRStatusActive
	record.ActivatedAt = &now
	record.SessionID = sessionID
	record.SessionExpireAt = &sessionExpireAt
	record.LastHeartbeatAt = now

	if requestID != "" {
		request, requestOK := wechatQRRequests[requestID]
		if !requestOK || request.DeviceID != deviceID {
			wechatQRRecordsMu.Unlock()
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "requestId 无效",
			})
			return
		}
		request.Status = wechatQRRequestStatusActive
		request.SessionID = sessionID
		request.ActivatedAt = now
		request.SessionExpireAt = &sessionExpireAt
		request.Message = "连接成功，请在微信中继续使用 AI 助手"
		wechatQRRequests[requestID] = request
		record.CurrentRequestID = requestID
		record.CurrentTaskID = request.TaskID
	}

	wechatQRRecords[deviceID] = record
	data := wechatQRDeviceResponseData(record, now)
	wechatQRRecordsMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "设备已激活",
		"data":    data,
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
	requestID := strings.TrimSpace(req.RequestID)
	reason := strings.TrimSpace(req.Reason)
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "deviceId 不能为空",
		})
		return
	}

	now := time.Now()

	wechatQRRecordsMu.Lock()
	applyWechatQRTimeoutsLocked(now)

	record, ok := wechatQRRecords[deviceID]
	if !ok {
		wechatQRRecordsMu.Unlock()
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "设备不存在",
		})
		return
	}
	if requestID == "" {
		requestID = record.CurrentRequestID
	}

	if requestID != "" {
		request, requestOK := wechatQRRequests[requestID]
		if requestOK && request.DeviceID == deviceID {
			switch reason {
			case wechatQRRequestStatusTimeout:
				request.Status = wechatQRRequestStatusTimeout
				request.TimeoutAt = now
				request.Message = "请求已超时"
			case wechatQRRequestStatusFailed, wechatQRStatusError:
				request.Status = wechatQRRequestStatusFailed
				request.FailedAt = now
				request.Message = "请求已失败"
			}
			wechatQRRequests[requestID] = request
		}
	}

	if record.CurrentTaskID != "" {
		task, taskOK := wechatQRTasks[record.CurrentTaskID]
		if taskOK && task.DeviceID == deviceID && task.Status != wechatQRTaskStatusDone {
			if reason == "timeout" {
				task.Status = wechatQRTaskStatusTimeout
				task.TimeoutAt = now
			} else {
				task.Status = wechatQRTaskStatusFailed
				task.FailedAt = now
			}
			wechatQRTasks[task.TaskID] = task
		}
	}

	record.ActivatedAt = nil
	record.SessionID = ""
	record.SessionExpireAt = nil
	record.CurrentRequestID = ""
	record.CurrentTaskID = ""
	record.QRURL = ""
	record.QRExpireAt = time.Time{}
	record.UpdatedAt = now
	if reason == wechatQRStatusError || reason == "error" {
		record.Status = wechatQRStatusError
	} else if isWechatQRHeartbeatFresh(record, now) {
		record.Status = wechatQRStatusIdle
	} else {
		record.Status = wechatQRStatusOffline
	}

	wechatQRRecords[deviceID] = record
	data := wechatQRDeviceResponseData(record, now)
	wechatQRRecordsMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "设备已释放",
		"data":    data,
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
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "deviceId 不能为空",
		})
		return
	}

	now := time.Now()

	wechatQRRecordsMu.Lock()
	applyWechatQRTimeoutsLocked(now)

	record := wechatQRRecords[deviceID]
	record.DeviceID = deviceID
	if deviceName != "" {
		record.DeviceName = deviceName
	}
	record.LastHeartbeatAt = now
	if record.Status == "" || record.Status == wechatQRStatusOffline || record.Status == wechatQRStatusExpired || record.Status == wechatQRStatusWaiting || record.Status == wechatQRStatusEnding {
		record.Status = wechatQRStatusIdle
	}
	if record.CurrentRequestID == "" && record.SessionID == "" && record.Status != wechatQRStatusError {
		record.Status = wechatQRStatusIdle
	}

	wechatQRRecords[deviceID] = record
	data := wechatQRDeviceResponseData(record, now)
	wechatQRRecordsMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "心跳已更新",
		"data":    data,
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

	now := time.Now()

	wechatQRRecordsMu.Lock()
	applyWechatQRTimeoutsLocked(now)
	record, ok := wechatQRRecords[deviceID]
	data := wechatQRSessionStatusData(record, now)
	wechatQRRecordsMu.Unlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "设备不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

func GetWechatQRDevices(c *gin.Context) {
	now := time.Now()
	devices := make([]gin.H, 0)

	wechatQRRecordsMu.Lock()
	applyWechatQRTimeoutsLocked(now)
	for _, record := range wechatQRRecords {
		devices = append(devices, wechatQRDeviceResponseData(record, now))
	}
	wechatQRRecordsMu.Unlock()

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

func getWechatQRUsableUserToken(userID int) (*model.Token, bool) {
	if userID <= 0 {
		return nil, false
	}
	tokens, err := model.GetAllUserTokens(userID, 0, 100)
	if err != nil {
		return nil, false
	}
	for _, token := range tokens {
		if isWechatQRUsableToken(token) {
			return token, true
		}
	}
	return nil, false
}

func getWechatQRTaskUserToken(userID int, tokenID int) (*model.Token, bool) {
	if userID <= 0 || tokenID <= 0 {
		return nil, false
	}
	token, err := model.GetTokenByIds(tokenID, userID)
	if err != nil || !isWechatQRUsableToken(token) {
		return nil, false
	}
	return token, true
}

func isWechatQRUsableToken(token *model.Token) bool {
	if token == nil {
		return false
	}
	if strings.TrimSpace(token.GetFullKey()) == "" {
		return false
	}
	if token.Status != common.TokenStatusEnabled {
		return false
	}
	if token.ExpiredTime != -1 && token.ExpiredTime < common.GetTimestamp() {
		return false
	}
	if !token.UnlimitedQuota && token.RemainQuota <= 0 {
		return false
	}
	return true
}

func isValidWechatQRURL(qrURL string) bool {
	return strings.HasPrefix(qrURL, "https://liteapp.weixin.qq.com/") || strings.Contains(qrURL, "qrcode=")
}

func effectiveWechatQRStatus(record wechatQRDeviceRecord, now time.Time) string {
	if !isWechatQRHeartbeatFresh(record, now) {
		return wechatQRStatusOffline
	}
	if record.Status == wechatQRStatusActive && record.SessionExpireAt != nil && !now.Before(*record.SessionExpireAt) {
		return wechatQRStatusEnding
	}
	if record.Status == "" {
		return wechatQRStatusIdle
	}
	return record.Status
}

func isWechatQRHeartbeatFresh(record wechatQRDeviceRecord, now time.Time) bool {
	return !record.LastHeartbeatAt.IsZero() && now.Sub(record.LastHeartbeatAt) <= wechatQRHeartbeatOfflineTime
}

func isWechatQRDeviceAssignable(record wechatQRDeviceRecord, now time.Time) bool {
	return effectiveWechatQRStatus(record, now) == wechatQRStatusIdle &&
		record.CurrentRequestID == "" &&
		record.CurrentTaskID == "" &&
		record.SessionID == ""
}

func applyWechatQRTimeoutsLocked(now time.Time) {
	for requestID, request := range wechatQRRequests {
		switch request.Status {
		case wechatQRRequestStatusAssigned:
			baseTime := request.AssignedAt
			if baseTime.IsZero() {
				baseTime = request.CreatedAt
			}
			if !baseTime.IsZero() && now.Sub(baseTime) > wechatQRAssignTimeout {
				failWechatQRRequestLocked(requestID, wechatQRRequestStatusFailed, now, "任务领取超时")
			}
		case wechatQRRequestStatusGenerating:
			baseTime := request.GeneratingAt
			if baseTime.IsZero() {
				baseTime = request.AssignedAt
			}
			if !baseTime.IsZero() && now.Sub(baseTime) > wechatQRGenerateTimeout {
				failWechatQRRequestLocked(requestID, wechatQRRequestStatusFailed, now, "二维码生成超时")
			}
		case wechatQRRequestStatusQRReady:
			baseTime := request.QRReadyAt
			if !baseTime.IsZero() && now.Sub(baseTime) > wechatQRWaitScanTimeout {
				failWechatQRRequestLocked(requestID, wechatQRRequestStatusTimeout, now, "二维码等待扫码超时")
			}
		}
	}
}

func failWechatQRRequestLocked(requestID string, status string, now time.Time, message string) {
	request, ok := wechatQRRequests[requestID]
	if !ok {
		return
	}
	request.Status = status
	request.Message = message
	if status == wechatQRRequestStatusTimeout {
		request.TimeoutAt = now
	} else {
		request.FailedAt = now
	}
	wechatQRRequests[requestID] = request

	if request.TaskID != "" {
		task, taskOK := wechatQRTasks[request.TaskID]
		if taskOK && task.Status != wechatQRTaskStatusDone {
			if status == wechatQRRequestStatusTimeout {
				task.Status = wechatQRTaskStatusTimeout
				task.TimeoutAt = now
			} else {
				task.Status = wechatQRTaskStatusFailed
				task.FailedAt = now
			}
			wechatQRTasks[task.TaskID] = task
		}
	}

	record, recordOK := wechatQRRecords[request.DeviceID]
	if !recordOK || record.CurrentRequestID != requestID {
		return
	}
	record.CurrentRequestID = ""
	record.CurrentTaskID = ""
	record.QRURL = ""
	record.QRExpireAt = time.Time{}
	record.ActivatedAt = nil
	record.SessionID = ""
	record.SessionExpireAt = nil
	record.UpdatedAt = now
	if isWechatQRHeartbeatFresh(record, now) {
		record.Status = wechatQRStatusIdle
	} else {
		record.Status = wechatQRStatusOffline
	}
	wechatQRRecords[record.DeviceID] = record
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
		"currentRequestId":      record.CurrentRequestID,
		"currentTaskId":         record.CurrentTaskID,
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

func wechatQRRequestResponseData(request wechatQRRequestRecord, record wechatQRDeviceRecord, now time.Time) gin.H {
	qrURL := request.QRURL
	if qrURL == "" && request.Status == wechatQRRequestStatusQRReady {
		qrURL = record.QRURL
	}

	sessionExpireAt := request.SessionExpireAt
	if sessionExpireAt == nil {
		sessionExpireAt = record.SessionExpireAt
	}

	remainingSeconds := 0
	if sessionExpireAt != nil {
		remainingSeconds = remainingSecondsUntil(*sessionExpireAt, now)
	}

	message := request.Message
	if request.Status == wechatQRRequestStatusActive && message == "" {
		message = "连接成功，请在微信中继续使用 AI 助手"
	}

	return gin.H{
		"requestId":        request.RequestID,
		"taskId":           request.TaskID,
		"deviceId":         request.DeviceID,
		"deviceName":       request.DeviceName,
		"status":           request.Status,
		"qrUrl":            qrURL,
		"sessionId":        request.SessionID,
		"sessionExpireAt":  timePtrToAny(sessionExpireAt),
		"remainingSeconds": remainingSeconds,
		"message":          message,
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

func newWechatQRID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err == nil {
		return prefix + "_" + hex.EncodeToString(buf)
	}
	return prefix + "_" + time.Now().Format("20060102150405.000000000")
}
