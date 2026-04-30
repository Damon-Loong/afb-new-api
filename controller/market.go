package controller

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	marketUploadMaxSize = 100 * 1024 * 1024
	marketUploadDir     = "uploads/market"

	marketSubmissionModeInternal = "internal"
	marketSubmissionModeExternal = "external"
	marketSubmissionModeNone     = "none"
)

func GetMarketActivities(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	activities, total, err := model.ListMarketActivities(pageInfo.GetStartIdx(), pageInfo.GetPageSize(), false, "")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]marketActivityListItem, 0, len(activities))
	for i := range activities {
		applyMarketSubmissionInfo(&activities[i])
		items = append(items, toMarketActivityListItem(activities[i]))
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetMarketActivity(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "活动 ID 无效")
		return
	}
	activity, err := model.GetMarketActivity(id, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	applyMarketSubmissionInfo(activity)
	common.ApiSuccess(c, activity)
}

type marketActivityListItem struct {
	ID                  int                          `json:"id"`
	Title               string                       `json:"title"`
	Subtitle            string                       `json:"subtitle"`
	PrizeSummary        string                       `json:"prize_summary"`
	ExternalURL         string                       `json:"external_url"`
	SourceName          string                       `json:"source_name"`
	SourceStatus        string                       `json:"source_status"`
	CoverURL            string                       `json:"cover_url"`
	Status              string                       `json:"status"`
	Category            string                       `json:"category"`
	SubmissionCount     int                          `json:"submission_count"`
	SortWeight          int                          `json:"sort_weight"`
	StartTime           int64                        `json:"start_time"`
	EndTime             int64                        `json:"end_time"`
	SubmissionStartTime int64                        `json:"submission_start_time"`
	SubmissionEndTime   int64                        `json:"submission_end_time"`
	CreatedAt           int64                        `json:"created_at"`
	UpdatedAt           int64                        `json:"updated_at"`
	Policies            []model.MarketActivityPolicy `json:"policies"`
	CanSubmit           bool                         `json:"can_submit"`
	SubmissionMode      string                       `json:"submission_mode"`
	SubmitURL           string                       `json:"submit_url"`
}

func toMarketActivityListItem(activity model.MarketActivity) marketActivityListItem {
	return marketActivityListItem{
		ID:                  activity.ID,
		Title:               activity.Title,
		Subtitle:            activity.Subtitle,
		PrizeSummary:        activity.PrizeSummary,
		ExternalURL:         activity.ExternalURL,
		SourceName:          activity.SourceName,
		SourceStatus:        activity.SourceStatus,
		CoverURL:            activity.CoverURL,
		Status:              activity.Status,
		Category:            activity.Category,
		SubmissionCount:     activity.SubmissionCount,
		SortWeight:          activity.SortWeight,
		StartTime:           activity.StartTime,
		EndTime:             activity.EndTime,
		SubmissionStartTime: activity.SubmissionStartTime,
		SubmissionEndTime:   activity.SubmissionEndTime,
		CreatedAt:           activity.CreatedAt,
		UpdatedAt:           activity.UpdatedAt,
		Policies:            activity.Policies,
		CanSubmit:           activity.CanSubmit,
		SubmissionMode:      activity.SubmissionMode,
		SubmitURL:           activity.SubmitURL,
	}
}

func applyMarketSubmissionInfo(activity *model.MarketActivity) {
	activity.CanSubmit = false
	activity.SubmissionMode = marketSubmissionModeNone
	activity.SubmitURL = ""

	if activity.Status != model.MarketActivityStatusPublished {
		return
	}

	sourceName := strings.TrimSpace(activity.SourceName)
	externalURL := strings.TrimSpace(activity.ExternalURL)
	sourceStatus := strings.TrimSpace(activity.SourceStatus)
	if sourceName == "" || sourceName == "official" {
		activity.CanSubmit = true
		activity.SubmissionMode = marketSubmissionModeInternal
		return
	}

	if externalURL != "" && sourceStatus == "in_progress" {
		activity.CanSubmit = true
		activity.SubmissionMode = marketSubmissionModeExternal
		activity.SubmitURL = externalURL
	}
}

func GetMarketActivityWorks(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "活动 ID 无效")
		return
	}
	if _, err := model.GetMarketActivity(id, false); err != nil {
		common.ApiErrorMsg(c, "活动不存在或未发布")
		return
	}
	pageInfo := common.GetPageQuery(c)
	works, total, err := model.ListMarketWorks(id, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(works)
	common.ApiSuccess(c, pageInfo)
}

func UploadMarketFile(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, marketUploadMaxSize+1024)
	file, err := c.FormFile("file")
	if err != nil {
		common.ApiErrorMsg(c, "请选择要上传的文件")
		return
	}
	if file.Size > marketUploadMaxSize {
		common.ApiErrorMsg(c, "单个文件不能超过 100MB")
		return
	}
	if err := os.MkdirAll(marketUploadDir, 0755); err != nil {
		common.ApiError(c, err)
		return
	}
	originalName := sanitizeMarketFileName(file.Filename)
	storageKey := uuid.NewString() + filepath.Ext(originalName)
	dst := filepath.Join(marketUploadDir, storageKey)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		common.ApiError(c, err)
		return
	}
	upload := model.MarketUpload{
		UserID:     c.GetInt("id"),
		FileName:   originalName,
		FileURL:    "/api/market/uploads/" + storageKey,
		FileType:   file.Header.Get("Content-Type"),
		FileSize:   file.Size,
		StorageKey: storageKey,
		UsageType:  strings.TrimSpace(c.PostForm("usage_type")),
	}
	if err := model.CreateMarketUpload(&upload); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, upload)
}

func GetMarketUpload(c *gin.Context) {
	storageKey := filepath.Base(c.Param("key"))
	if storageKey == "." || storageKey == string(filepath.Separator) || storageKey != c.Param("key") {
		common.ApiErrorMsg(c, "文件地址无效")
		return
	}
	path := filepath.Join(marketUploadDir, storageKey)
	if _, err := os.Stat(path); err != nil {
		common.ApiErrorMsg(c, "文件不存在")
		return
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.File(path)
}

func CreateMarketSubmission(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "活动 ID 无效")
		return
	}
	if _, err := model.GetMarketActivity(id, false); err != nil {
		common.ApiErrorMsg(c, "活动不存在或未发布")
		return
	}
	var payload struct {
		Title       string                   `json:"title"`
		Description string                   `json:"description"`
		WorkURL     string                   `json:"work_url"`
		CoverURL    string                   `json:"cover_url"`
		Attachments []map[string]interface{} `json:"attachments"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		common.ApiError(c, err)
		return
	}
	attachmentsJSON := "[]"
	if len(payload.Attachments) > 0 {
		attachmentBytes, err := json.Marshal(payload.Attachments)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		attachmentsJSON = string(attachmentBytes)
	}
	submission := model.MarketSubmission{
		ActivityID:  id,
		UserID:      c.GetInt("id"),
		Title:       strings.TrimSpace(payload.Title),
		Description: strings.TrimSpace(payload.Description),
		WorkURL:     strings.TrimSpace(payload.WorkURL),
		CoverURL:    strings.TrimSpace(payload.CoverURL),
		Attachments: attachmentsJSON,
		Status:      model.MarketSubmissionStatusPending,
	}
	if err := model.CreateMarketSubmission(&submission); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, submission)
}

func GetMarketSubmissionsSelf(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	submissions, total, err := model.ListMarketSubmissionsByUser(c.GetInt("id"), pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(submissions)
	common.ApiSuccess(c, pageInfo)
}

func AdminGetMarketActivities(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	activities, total, err := model.ListMarketActivities(pageInfo.GetStartIdx(), pageInfo.GetPageSize(), true, c.Query("status"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(activities)
	common.ApiSuccess(c, pageInfo)
}

func AdminGetMarketActivity(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "活动 ID 无效")
		return
	}
	activity, err := model.GetMarketActivity(id, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, activity)
}

func AdminCreateMarketActivity(c *gin.Context) {
	var activity model.MarketActivity
	if err := c.ShouldBindJSON(&activity); err != nil {
		common.ApiError(c, err)
		return
	}
	activity.ID = 0
	activity.CreatedBy = c.GetInt("id")
	normalizeMarketActivity(&activity)
	if err := model.SaveMarketActivity(&activity); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, activity)
}

func AdminUpdateMarketActivity(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "活动 ID 无效")
		return
	}
	var activity model.MarketActivity
	if err := c.ShouldBindJSON(&activity); err != nil {
		common.ApiError(c, err)
		return
	}
	activity.ID = id
	normalizeMarketActivity(&activity)
	if err := model.SaveMarketActivity(&activity); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, activity)
}

func AdminUpdateMarketActivityStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "活动 ID 无效")
		return
	}
	var payload struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		common.ApiError(c, err)
		return
	}
	if !model.IsValidMarketActivityStatus(payload.Status) {
		common.ApiErrorMsg(c, "活动状态无效")
		return
	}
	if err := model.DB.Model(&model.MarketActivity{}).Where("id = ?", id).Update("status", payload.Status).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"id": id, "status": payload.Status})
}

func AdminDeleteMarketActivity(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "活动 ID 无效")
		return
	}
	if err := model.DeleteMarketActivity(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"id": id})
}

func AdminGetMarketSubmissions(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	activityID, _ := strconv.Atoi(c.Query("activity_id"))
	submissions, total, err := model.AdminListMarketSubmissions(activityID, c.Query("status"), pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(submissions)
	common.ApiSuccess(c, pageInfo)
}

func AdminUpdateMarketSubmissionStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "投稿 ID 无效")
		return
	}
	var payload struct {
		Status       string `json:"status"`
		RejectReason string `json:"reject_reason"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.UpdateMarketSubmissionStatus(id, payload.Status, payload.RejectReason); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"id": id, "status": payload.Status})
}

func AdminUpdateMarketSubmissionFeature(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "投稿 ID 无效")
		return
	}
	var payload struct {
		IsFeatured bool `json:"is_featured"`
		SortWeight int  `json:"sort_weight"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.UpdateMarketSubmissionFeature(id, payload.IsFeatured, payload.SortWeight); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"id": id, "is_featured": payload.IsFeatured})
}

func AdminDeleteMarketSubmission(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "投稿 ID 无效")
		return
	}
	if err := model.DeleteMarketSubmission(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"id": id})
}

func normalizeMarketActivity(activity *model.MarketActivity) {
	if strings.TrimSpace(activity.Status) == "" {
		activity.Status = model.MarketActivityStatusDraft
	}
	if strings.TrimSpace(activity.Category) == "" {
		activity.Category = "image"
	}
}

func sanitizeMarketFileName(name string) string {
	name = strings.TrimSpace(filepath.Base(name))
	if name == "" || name == "." {
		return "upload"
	}
	if len([]rune(name)) > 180 {
		ext := filepath.Ext(name)
		stem := strings.TrimSuffix(name, ext)
		stemRunes := []rune(stem)
		if len(stemRunes) > 160 {
			stem = string(stemRunes[:160])
		}
		name = stem + ext
	}
	return name
}
