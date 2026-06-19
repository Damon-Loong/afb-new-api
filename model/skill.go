package model

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	SkillStatusPublished = "published"
	SkillStatusDraft     = "draft"
)

type Skill struct {
	ID              int    `json:"id" gorm:"primary_key;AUTO_INCREMENT"`
	UserID          int    `json:"user_id" gorm:"index;not null"`
	Title           string `json:"title" gorm:"type:varchar(191);not null"`
	Description     string `json:"description" gorm:"type:text"`
	PackageURL      string `json:"package_url,omitempty" gorm:"type:text;not null"`
	SkillMarkdown   string `json:"skill_md,omitempty" gorm:"type:longtext"`
	ContentHash     string `json:"content_hash" gorm:"type:varchar(128);index"`
	SkillMDTokens   int    `json:"skill_md_tokens" gorm:"default:0"`
	TokenMultiplier int    `json:"token_multiplier" gorm:"default:1"`
	PromotionMode   string `json:"promotion_mode" gorm:"type:varchar(32);index;default:platform_auto"`
	Visibility      string `json:"visibility" gorm:"type:varchar(32);index;default:public"`
	Status          string `json:"status" gorm:"type:varchar(32);index;default:published"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
}

func (s *Skill) BeforeCreate(tx *gorm.DB) error {
	now := time.Now().Unix()
	s.CreatedAt = now
	s.UpdatedAt = now
	if strings.TrimSpace(s.Status) == "" {
		s.Status = SkillStatusPublished
	}
	if strings.TrimSpace(s.Visibility) == "" {
		s.Visibility = "public"
	}
	if strings.TrimSpace(s.PromotionMode) == "" {
		s.PromotionMode = "platform_auto"
	}
	return nil
}

func (s *Skill) BeforeUpdate(tx *gorm.DB) error {
	s.UpdatedAt = time.Now().Unix()
	return nil
}
