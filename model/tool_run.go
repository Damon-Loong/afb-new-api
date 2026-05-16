package model

import (
	"time"

	"gorm.io/gorm"
)

type ToolRun struct {
	ID              int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	ConversationID  string `json:"conversation_id" gorm:"type:varchar(191);index"`
	MessageID       string `json:"message_id" gorm:"type:varchar(191);index"`
	UserID          int    `json:"user_id" gorm:"not null;index"`
	ToolID          string `json:"tool_id" gorm:"type:varchar(191);not null;index"`
	ActionID        string `json:"action_id" gorm:"type:varchar(191);not null;index"`
	FunctionName    string `json:"function_name" gorm:"type:varchar(191);index"`
	Status          string `json:"status" gorm:"type:varchar(32);index"`
	RequestArgs     string `json:"request_args" gorm:"type:text"`
	ResponsePreview string `json:"response_preview" gorm:"type:text"`
	ErrorMessage    string `json:"error_message" gorm:"type:text"`
	StartedAt       int64  `json:"started_at" gorm:"bigint;index"`
	FinishedAt      int64  `json:"finished_at" gorm:"bigint"`
	DurationMS      int64  `json:"duration_ms" gorm:"bigint"`
	CreatedAt       int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt       int64  `json:"updated_at" gorm:"bigint"`
}

func (r *ToolRun) BeforeCreate(tx *gorm.DB) error {
	now := time.Now().Unix()
	if r.StartedAt == 0 {
		r.StartedAt = now
	}
	r.CreatedAt = now
	r.UpdatedAt = now
	return nil
}

func (r *ToolRun) BeforeUpdate(tx *gorm.DB) error {
	r.UpdatedAt = time.Now().Unix()
	return nil
}
