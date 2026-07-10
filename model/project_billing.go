package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	ProjectBillingStatusSuccess = "success"
)

var (
	ErrProjectBillingDuplicate         = errors.New("project billing duplicate")
	ErrProjectBillingInsufficientQuota = errors.New("project billing insufficient quota")
)

type ProjectBillingRecord struct {
	ID             int    `json:"id" gorm:"primaryKey"`
	ProjectKey     string `json:"project_key" gorm:"type:varchar(64);not null;uniqueIndex:idx_project_billing_idempotency,priority:1;index"`
	UserID         int    `json:"user_id" gorm:"not null;index"`
	IdempotencyKey string `json:"idempotency_key" gorm:"type:varchar(128);not null;uniqueIndex:idx_project_billing_idempotency,priority:2"`
	TokenAmount    int    `json:"token_amount" gorm:"not null"`
	Description    string `json:"description" gorm:"type:varchar(255)"`
	Metadata       string `json:"metadata" gorm:"type:text"`
	Status         string `json:"status" gorm:"type:varchar(32);not null;default:'success';index"`
	RemainingQuota int    `json:"remaining_quota" gorm:"not null;default:0"`
	CreatedAt      int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

type ProjectBillingChargeParams struct {
	ProjectKey     string
	UserID         int
	UserGroup      string
	IdempotencyKey string
	TokenAmount    int
	Description    string
	Metadata       map[string]interface{}
}

func ChargeProjectQuota(params ProjectBillingChargeParams) (*ProjectBillingRecord, bool, error) {
	projectKey := strings.TrimSpace(params.ProjectKey)
	idempotencyKey := strings.TrimSpace(params.IdempotencyKey)
	description := strings.TrimSpace(params.Description)
	if description == "" {
		description = fmt.Sprintf("Project %s billing charge", projectKey)
	}

	if projectKey == "" || idempotencyKey == "" || params.UserID <= 0 || params.TokenAmount <= 0 {
		return nil, false, errors.New("invalid project billing params")
	}

	metadata := params.Metadata
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	metadata["project_key"] = projectKey
	metadata["idempotency_key"] = idempotencyKey
	metadata["source"] = "project_billing"
	metadataJSON := common.MapToJsonStr(metadata)

	record := &ProjectBillingRecord{
		ProjectKey:     projectKey,
		UserID:         params.UserID,
		IdempotencyKey: idempotencyKey,
		TokenAmount:    params.TokenAmount,
		Description:    description,
		Metadata:       metadataJSON,
		Status:         ProjectBillingStatusSuccess,
		CreatedAt:      time.Now().Unix(),
		UpdatedAt:      time.Now().Unix(),
	}

	err := DB.Transaction(func(tx *gorm.DB) error {
		var existing ProjectBillingRecord
		if err := tx.Where("project_key = ? AND idempotency_key = ?", projectKey, idempotencyKey).First(&existing).Error; err == nil {
			if existing.UserID != params.UserID || existing.TokenAmount != params.TokenAmount {
				return errors.New("project billing idempotency key conflict")
			}
			*record = existing
			return ErrProjectBillingDuplicate
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if err := tx.Create(record).Error; err != nil {
			return err
		}

		result := tx.Model(&User{}).
			Where("id = ? AND quota >= ?", params.UserID, params.TokenAmount).
			Updates(map[string]interface{}{
				"quota":         gorm.Expr("quota - ?", params.TokenAmount),
				"used_quota":    gorm.Expr("used_quota + ?", params.TokenAmount),
				"request_count": gorm.Expr("request_count + ?", 1),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrProjectBillingInsufficientQuota
		}

		var remainingQuota int
		if err := tx.Model(&User{}).Where("id = ?", params.UserID).Select("quota").Scan(&remainingQuota).Error; err != nil {
			return err
		}
		record.RemainingQuota = remainingQuota
		record.UpdatedAt = time.Now().Unix()
		return tx.Model(record).Updates(map[string]interface{}{
			"remaining_quota": record.RemainingQuota,
			"updated_at":      record.UpdatedAt,
		}).Error
	})

	if errors.Is(err, ErrProjectBillingDuplicate) {
		return record, true, nil
	}
	if err != nil {
		return nil, false, err
	}

	if cacheErr := cacheDecrUserQuota(params.UserID, int64(params.TokenAmount)); cacheErr != nil {
		common.SysLog("failed to decrease project billing user quota cache: " + cacheErr.Error())
	}
	RecordTaskBillingLog(RecordTaskBillingLogParams{
		UserId:    params.UserID,
		LogType:   LogTypeProjectConsume,
		Content:   description,
		ModelName: "project:" + projectKey,
		Quota:     params.TokenAmount,
		Group:     params.UserGroup,
		Other:     metadata,
	})

	return record, false, nil
}
