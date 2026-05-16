package model

import (
	"time"

	"gorm.io/gorm"
)

type UserTool struct {
	ID                int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID            int    `json:"user_id" gorm:"not null;uniqueIndex:idx_user_tool;index"`
	ToolID            string `json:"tool_id" gorm:"type:varchar(191);not null;uniqueIndex:idx_user_tool;index"`
	Enabled           bool   `json:"enabled" gorm:"default:true"`
	SelectedActionIDs string `json:"selected_action_ids" gorm:"type:text"`
	InstalledAt       int64  `json:"installed_at" gorm:"bigint"`
	UpdatedAt         int64  `json:"updated_at" gorm:"bigint"`
}

func (u *UserTool) BeforeCreate(tx *gorm.DB) error {
	now := time.Now().Unix()
	if u.InstalledAt == 0 {
		u.InstalledAt = now
	}
	u.UpdatedAt = now
	return nil
}

func (u *UserTool) BeforeUpdate(tx *gorm.DB) error {
	u.UpdatedAt = time.Now().Unix()
	return nil
}
