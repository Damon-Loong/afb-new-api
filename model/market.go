package model

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	MarketActivityStatusDraft     = "draft"
	MarketActivityStatusPublished = "published"
	MarketActivityStatusEnded     = "ended"
	MarketActivityStatusArchived  = "archived"

	MarketSubmissionStatusPending  = "pending"
	MarketSubmissionStatusApproved = "approved"
	MarketSubmissionStatusRejected = "rejected"
)

type MarketActivity struct {
	ID                  int                    `json:"id" gorm:"primary_key;AUTO_INCREMENT"`
	Title               string                 `json:"title" gorm:"type:varchar(191);not null"`
	Subtitle            string                 `json:"subtitle" gorm:"type:varchar(255)"`
	PrizeSummary        string                 `json:"prize_summary" gorm:"type:varchar(255)"`
	DetailContent       string                 `json:"detail_content" gorm:"type:longtext"`
	ExternalURL         string                 `json:"external_url" gorm:"type:text"`
	SourceName          string                 `json:"source_name" gorm:"type:varchar(64)"`
	SourceKey           string                 `json:"source_key" gorm:"type:varchar(191);index"`
	SourceStatus        string                 `json:"source_status" gorm:"type:varchar(64);index"`
	CoverURL            string                 `json:"cover_url" gorm:"type:text"`
	Status              string                 `json:"status" gorm:"type:varchar(32);index;default:draft"`
	Category            string                 `json:"category" gorm:"type:varchar(64);default:image"`
	SubmissionCount     int                    `json:"submission_count" gorm:"default:0"`
	SortWeight          int                    `json:"sort_weight" gorm:"default:0;index"`
	StartTime           int64                  `json:"start_time" gorm:"index"`
	EndTime             int64                  `json:"end_time" gorm:"index"`
	SubmissionStartTime int64                  `json:"submission_start_time" gorm:"index"`
	SubmissionEndTime   int64                  `json:"submission_end_time" gorm:"index"`
	CreatedBy           int                    `json:"created_by" gorm:"index"`
	CreatedAt           int64                  `json:"created_at"`
	UpdatedAt           int64                  `json:"updated_at"`
	Policies            []MarketActivityPolicy `json:"policies" gorm:"foreignKey:ActivityID"`
	CanSubmit           bool                   `json:"can_submit" gorm:"-"`
	SubmissionMode      string                 `json:"submission_mode" gorm:"-"`
	SubmitURL           string                 `json:"submit_url" gorm:"-"`
}

type MarketActivityPolicy struct {
	ID          int    `json:"id" gorm:"primary_key;AUTO_INCREMENT"`
	ActivityID  int    `json:"activity_id" gorm:"index;not null"`
	RegionName  string `json:"region_name" gorm:"type:varchar(64)"`
	PolicyName  string `json:"policy_name" gorm:"type:varchar(128)"`
	Description string `json:"description" gorm:"type:text"`
	SortOrder   int    `json:"sort_order" gorm:"default:0"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

type MarketSubmission struct {
	ID           int             `json:"id" gorm:"primary_key;AUTO_INCREMENT"`
	ActivityID   int             `json:"activity_id" gorm:"index;not null"`
	UserID       int             `json:"user_id" gorm:"index;not null"`
	Title        string          `json:"title" gorm:"type:varchar(191);not null"`
	Description  string          `json:"description" gorm:"type:text"`
	WorkURL      string          `json:"work_url" gorm:"type:text"`
	CoverURL     string          `json:"cover_url" gorm:"type:text"`
	Attachments  string          `json:"attachments" gorm:"type:longtext"`
	Status       string          `json:"status" gorm:"type:varchar(32);index;default:pending"`
	RejectReason string          `json:"reject_reason" gorm:"type:text"`
	IsFeatured   bool            `json:"is_featured" gorm:"default:false;index"`
	SortWeight   int             `json:"sort_weight" gorm:"default:0;index"`
	CreatedAt    int64           `json:"created_at"`
	UpdatedAt    int64           `json:"updated_at"`
	Activity     *MarketActivity `json:"activity,omitempty" gorm:"foreignKey:ActivityID"`
}

type MarketUpload struct {
	ID         int    `json:"id" gorm:"primary_key;AUTO_INCREMENT"`
	UserID     int    `json:"user_id" gorm:"index"`
	FileName   string `json:"file_name" gorm:"type:varchar(255)"`
	FileURL    string `json:"file_url" gorm:"type:text"`
	FileType   string `json:"file_type" gorm:"type:varchar(128)"`
	FileSize   int64  `json:"file_size"`
	StorageKey string `json:"storage_key" gorm:"type:varchar(191);uniqueIndex"`
	UsageType  string `json:"usage_type" gorm:"type:varchar(64);index"`
	CreatedAt  int64  `json:"created_at"`
}

func (a *MarketActivity) BeforeCreate(tx *gorm.DB) error {
	now := time.Now().Unix()
	a.CreatedAt = now
	a.UpdatedAt = now
	if strings.TrimSpace(a.Status) == "" {
		a.Status = MarketActivityStatusDraft
	}
	if strings.TrimSpace(a.Category) == "" {
		a.Category = "image"
	}
	return nil
}

func (a *MarketActivity) BeforeUpdate(tx *gorm.DB) error {
	a.UpdatedAt = time.Now().Unix()
	return nil
}

func (p *MarketActivityPolicy) BeforeCreate(tx *gorm.DB) error {
	now := time.Now().Unix()
	p.CreatedAt = now
	p.UpdatedAt = now
	return nil
}

func (p *MarketActivityPolicy) BeforeUpdate(tx *gorm.DB) error {
	p.UpdatedAt = time.Now().Unix()
	return nil
}

func (s *MarketSubmission) BeforeCreate(tx *gorm.DB) error {
	now := time.Now().Unix()
	s.CreatedAt = now
	s.UpdatedAt = now
	if strings.TrimSpace(s.Status) == "" {
		s.Status = MarketSubmissionStatusPending
	}
	return nil
}

func (s *MarketSubmission) BeforeUpdate(tx *gorm.DB) error {
	s.UpdatedAt = time.Now().Unix()
	return nil
}

func (u *MarketUpload) BeforeCreate(tx *gorm.DB) error {
	u.CreatedAt = time.Now().Unix()
	return nil
}

func IsValidMarketActivityStatus(status string) bool {
	switch status {
	case MarketActivityStatusDraft, MarketActivityStatusPublished, MarketActivityStatusEnded, MarketActivityStatusArchived:
		return true
	default:
		return false
	}
}

func IsValidMarketSubmissionStatus(status string) bool {
	switch status {
	case MarketSubmissionStatusPending, MarketSubmissionStatusApproved, MarketSubmissionStatusRejected:
		return true
	default:
		return false
	}
}

func ListMarketActivities(offset, limit int, admin bool, status string) ([]MarketActivity, int64, error) {
	var activities []MarketActivity
	var total int64
	db := DB.Model(&MarketActivity{})
	if !admin {
		db = db.Where("status = ?", MarketActivityStatusPublished)
	} else if strings.TrimSpace(status) != "" {
		db = db.Where("status = ?", status)
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := db.Preload("Policies", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC, id ASC")
	}).Order(`sort_weight DESC,
		CASE source_status
			WHEN 'in_progress' THEN 3
			WHEN 'in_evaluation' THEN 2
			WHEN 'awarded' THEN 1
			ELSE 0
		END DESC,
		start_time DESC, end_time DESC, id DESC`).Offset(offset).Limit(limit).Find(&activities).Error
	return activities, total, err
}

func GetMarketActivity(id int, admin bool) (*MarketActivity, error) {
	var activity MarketActivity
	db := DB.Preload("Policies", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC, id ASC")
	})
	if !admin {
		db = db.Where("status = ?", MarketActivityStatusPublished)
	}
	if err := db.First(&activity, id).Error; err != nil {
		return nil, err
	}
	return &activity, nil
}

func SaveMarketActivity(activity *MarketActivity) error {
	if strings.TrimSpace(activity.Title) == "" {
		return errors.New("活动标题不能为空")
	}
	if !IsValidMarketActivityStatus(activity.Status) {
		return errors.New("活动状态无效")
	}
	if activity.StartTime > 0 && activity.EndTime > 0 && activity.EndTime <= activity.StartTime {
		return errors.New("活动结束时间必须晚于开始时间")
	}
	if activity.SubmissionStartTime > 0 && activity.SubmissionEndTime > 0 && activity.SubmissionEndTime <= activity.SubmissionStartTime {
		return errors.New("投稿结束时间必须晚于投稿开始时间")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		policies := activity.Policies
		activity.Policies = nil
		if activity.ID == 0 {
			if err := tx.Create(activity).Error; err != nil {
				return err
			}
		} else {
			updates := map[string]interface{}{
				"title":                 activity.Title,
				"subtitle":              activity.Subtitle,
				"prize_summary":         activity.PrizeSummary,
				"detail_content":        activity.DetailContent,
				"external_url":          activity.ExternalURL,
				"source_name":           activity.SourceName,
				"source_key":            activity.SourceKey,
				"source_status":         activity.SourceStatus,
				"cover_url":             activity.CoverURL,
				"status":                activity.Status,
				"category":              activity.Category,
				"submission_count":      activity.SubmissionCount,
				"sort_weight":           activity.SortWeight,
				"start_time":            activity.StartTime,
				"end_time":              activity.EndTime,
				"submission_start_time": activity.SubmissionStartTime,
				"submission_end_time":   activity.SubmissionEndTime,
				"updated_at":            time.Now().Unix(),
			}
			if err := tx.Model(&MarketActivity{}).Where("id = ?", activity.ID).Updates(updates).Error; err != nil {
				return err
			}
			if err := tx.Where("activity_id = ?", activity.ID).Delete(&MarketActivityPolicy{}).Error; err != nil {
				return err
			}
		}
		for i := range policies {
			policies[i].ActivityID = activity.ID
			policies[i].SortOrder = i
		}
		if len(policies) > 0 {
			if err := tx.Create(&policies).Error; err != nil {
				return err
			}
		}
		activity.Policies = policies
		return nil
	})
}

func DeleteMarketActivity(id int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("activity_id = ?", id).Delete(&MarketActivityPolicy{}).Error; err != nil {
			return err
		}
		if err := tx.Where("activity_id = ?", id).Delete(&MarketSubmission{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&MarketActivity{}, id).Error; err != nil {
			return err
		}
		return nil
	})
}

func CreateMarketSubmission(submission *MarketSubmission) error {
	if strings.TrimSpace(submission.Title) == "" {
		return errors.New("作品标题不能为空")
	}
	attachments := strings.TrimSpace(submission.Attachments)
	if strings.TrimSpace(submission.WorkURL) == "" && (attachments == "" || attachments == "[]") {
		return errors.New("请填写作品链接或上传作品附件")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(submission).Error; err != nil {
			return err
		}
		return tx.Model(&MarketActivity{}).Where("id = ?", submission.ActivityID).
			UpdateColumn("submission_count", gorm.Expr("submission_count + ?", 1)).Error
	})
}

func ListMarketSubmissionsByUser(userID, offset, limit int) ([]MarketSubmission, int64, error) {
	var submissions []MarketSubmission
	var total int64
	db := DB.Model(&MarketSubmission{}).Where("user_id = ?", userID)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := db.Preload("Activity").Order("id DESC").Offset(offset).Limit(limit).Find(&submissions).Error
	return submissions, total, err
}

func ListMarketWorks(activityID, offset, limit int) ([]MarketSubmission, int64, error) {
	var submissions []MarketSubmission
	var total int64
	db := DB.Model(&MarketSubmission{}).Where("activity_id = ? AND status = ?", activityID, MarketSubmissionStatusApproved)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := db.Order("is_featured DESC, sort_weight DESC, id DESC").Offset(offset).Limit(limit).Find(&submissions).Error
	return submissions, total, err
}

func AdminListMarketSubmissions(activityID int, status string, offset, limit int) ([]MarketSubmission, int64, error) {
	var submissions []MarketSubmission
	var total int64
	db := DB.Model(&MarketSubmission{})
	if activityID > 0 {
		db = db.Where("activity_id = ?", activityID)
	}
	if strings.TrimSpace(status) != "" {
		db = db.Where("status = ?", status)
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := db.Preload("Activity").Order("id DESC").Offset(offset).Limit(limit).Find(&submissions).Error
	return submissions, total, err
}

func UpdateMarketSubmissionStatus(id int, status, rejectReason string) error {
	if !IsValidMarketSubmissionStatus(status) {
		return errors.New("投稿状态无效")
	}
	return DB.Model(&MarketSubmission{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":        status,
		"reject_reason": rejectReason,
		"updated_at":    time.Now().Unix(),
	}).Error
}

func UpdateMarketSubmissionFeature(id int, featured bool, sortWeight int) error {
	return DB.Model(&MarketSubmission{}).Where("id = ?", id).Updates(map[string]interface{}{
		"is_featured": featured,
		"sort_weight": sortWeight,
		"updated_at":  time.Now().Unix(),
	}).Error
}

func DeleteMarketSubmission(id int) error {
	return DB.Delete(&MarketSubmission{}, id).Error
}

func CreateMarketUpload(upload *MarketUpload) error {
	return DB.Create(upload).Error
}
