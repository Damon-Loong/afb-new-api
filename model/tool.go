package model

import (
	"time"

	"gorm.io/gorm"
)

type Tool struct {
	ID             string `json:"id" gorm:"type:varchar(191);primaryKey"`
	Slug           string `json:"slug" gorm:"type:varchar(191);uniqueIndex"`
	Name           string `json:"name" gorm:"type:varchar(191);index"`
	Description    string `json:"description" gorm:"type:text"`
	Version        string `json:"version" gorm:"type:varchar(64)"`
	Type           string `json:"type" gorm:"type:varchar(32);index"`
	AuthType       string `json:"auth_type" gorm:"type:varchar(32)"`
	ServerURL      string `json:"server_url" gorm:"type:text"`
	ActionCount    int    `json:"action_count" gorm:"default:0"`
	Status         string `json:"status" gorm:"type:varchar(32);index"`
	DownloadCount  int64  `json:"download_count" gorm:"default:0"`
	CreatedBy      int    `json:"created_by" gorm:"index"`
	Category       string `json:"category" gorm:"type:varchar(64);index"`
	Visibility     string `json:"visibility" gorm:"type:varchar(32);index"`
	SourceURL      string `json:"source_url" gorm:"type:text"`
	CallPrice      int    `json:"call_price" gorm:"default:0"`
	SourceFormat   string `json:"source_format" gorm:"type:varchar(32)"`
	OpenAPIVersion string `json:"openapi_version" gorm:"type:varchar(64)"`
	APIKeyLocation string `json:"api_key_location" gorm:"type:varchar(32)"`
	APIKeyName     string `json:"api_key_name" gorm:"type:varchar(191)"`
	CommonHeaders  string `json:"common_headers" gorm:"type:text"`
	Warnings       string `json:"warnings" gorm:"type:text"`
	OpenAPISpec    string `json:"openapi_spec" gorm:"type:text"`
	RawSpec        string `json:"raw_spec" gorm:"type:text"`
	CreatedAt      int64  `json:"created_at" gorm:"bigint;index"`
	UpdatedAt      int64  `json:"updated_at" gorm:"bigint;index"`
}

func (t *Tool) BeforeCreate(tx *gorm.DB) error {
	now := time.Now().Unix()
	if t.CreatedAt == 0 {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	return nil
}

func (t *Tool) BeforeUpdate(tx *gorm.DB) error {
	t.UpdatedAt = time.Now().Unix()
	return nil
}

type ToolAction struct {
	ID            int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	ToolID        string `json:"tool_id" gorm:"type:varchar(191);not null;uniqueIndex:idx_tool_action;index"`
	ActionID      string `json:"action_id" gorm:"type:varchar(191);not null;uniqueIndex:idx_tool_action"`
	Name          string `json:"name" gorm:"type:varchar(191);index"`
	DisplayName   string `json:"display_name" gorm:"type:varchar(191)"`
	Description   string `json:"description" gorm:"type:text"`
	OperationID   string `json:"operation_id" gorm:"type:varchar(191);index"`
	Method        string `json:"method" gorm:"type:varchar(16)"`
	Path          string `json:"path" gorm:"type:text"`
	InputSchema   string `json:"input_schema" gorm:"type:text"`
	OutputSchema  string `json:"output_schema" gorm:"type:text"`
	Enabled       bool   `json:"enabled" gorm:"default:true"`
	RiskLevel     string `json:"risk_level" gorm:"type:varchar(32)"`
	ParameterHint string `json:"parameter_hint" gorm:"type:text"`
	ResponseHint  string `json:"response_hint" gorm:"type:text"`
	SortOrder     int    `json:"sort_order" gorm:"index"`
	CreatedAt     int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt     int64  `json:"updated_at" gorm:"bigint"`
}

func (a *ToolAction) BeforeCreate(tx *gorm.DB) error {
	now := time.Now().Unix()
	if a.CreatedAt == 0 {
		a.CreatedAt = now
	}
	a.UpdatedAt = now
	return nil
}

func (a *ToolAction) BeforeUpdate(tx *gorm.DB) error {
	a.UpdatedAt = time.Now().Unix()
	return nil
}

type ToolSecret struct {
	ToolID         string `json:"tool_id" gorm:"type:varchar(191);primaryKey"`
	APIKeyLocation string `json:"api_key_location" gorm:"type:varchar(32)"`
	APIKeyName     string `json:"api_key_name" gorm:"type:varchar(191)"`
	APIKeyValue    string `json:"api_key_value" gorm:"type:text"`
	CreatedAt      int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt      int64  `json:"updated_at" gorm:"bigint"`
}

func (s *ToolSecret) BeforeCreate(tx *gorm.DB) error {
	now := time.Now().Unix()
	if s.CreatedAt == 0 {
		s.CreatedAt = now
	}
	s.UpdatedAt = now
	return nil
}

func (s *ToolSecret) BeforeUpdate(tx *gorm.DB) error {
	s.UpdatedAt = time.Now().Unix()
	return nil
}
