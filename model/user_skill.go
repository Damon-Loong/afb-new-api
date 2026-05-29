package model

import (
	"time"

	"gorm.io/gorm"
)

type UserSkill struct {
	ID         int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID     int    `json:"user_id" gorm:"not null;uniqueIndex:idx_user_skill;index"`
	SkillID    int    `json:"skill_id" gorm:"not null;uniqueIndex:idx_user_skill;index"`
	PaidQuota  int    `json:"paid_quota" gorm:"default:0"`
	PackageURL string `json:"package_url" gorm:"type:text"`
	AcquiredAt int64  `json:"acquired_at" gorm:"bigint"`
	UpdatedAt  int64  `json:"updated_at" gorm:"bigint"`
}

func (u *UserSkill) BeforeCreate(tx *gorm.DB) error {
	now := time.Now().Unix()
	if u.AcquiredAt == 0 {
		u.AcquiredAt = now
	}
	u.UpdatedAt = now
	return nil
}

func (u *UserSkill) BeforeUpdate(tx *gorm.DB) error {
	u.UpdatedAt = time.Now().Unix()
	return nil
}
