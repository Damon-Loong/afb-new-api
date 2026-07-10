package model

import (
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type SSOProject struct {
	ID            int    `json:"id" gorm:"primaryKey"`
	Key           string `json:"key" gorm:"column:project_key;type:varchar(64);uniqueIndex;not null"`
	Name          string `json:"name" gorm:"type:varchar(128);not null"`
	OfficialURL   string `json:"official_url" gorm:"type:varchar(512);not null"`
	Description   string `json:"description" gorm:"type:text"`
	IconURL       string `json:"icon_url" gorm:"type:varchar(512)"`
	BillingSecret string `json:"billing_secret,omitempty" gorm:"type:varchar(128)"`
	Enabled       bool   `json:"enabled" gorm:"default:true"`
	SSOEnabled    bool   `json:"sso_enabled" gorm:"default:true"`
	Sort          int    `json:"sort" gorm:"default:0;index"`
	CreatedAt     int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (p *SSOProject) Origin() (string, error) {
	return URLOrigin(p.OfficialURL)
}

func URLOrigin(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("url scheme must be http or https")
	}
	if parsed.Host == "" {
		return "", errors.New("url host is required")
	}
	return parsed.Scheme + "://" + parsed.Host, nil
}

func IsProjectKeyValid(key string) bool {
	if key == "" || len(key) > 64 {
		return false
	}
	for _, r := range key {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

func ListSSOProjects(includeDisabled bool) ([]SSOProject, error) {
	var projects []SSOProject
	query := DB.Model(&SSOProject{})
	if !includeDisabled {
		query = query.Where("enabled = ?", true)
	}
	err := query.Order("sort ASC, id DESC").Find(&projects).Error
	return projects, err
}

func GetSSOProjectByKey(key string) (*SSOProject, error) {
	var project SSOProject
	if err := DB.Where("project_key = ?", key).First(&project).Error; err != nil {
		return nil, err
	}
	return &project, nil
}

func CreateSSOProject(project *SSOProject) error {
	now := time.Now().Unix()
	project.CreatedAt = now
	project.UpdatedAt = now
	return DB.Create(project).Error
}

func UpdateSSOProject(id int, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now().Unix()
	return DB.Model(&SSOProject{}).Where("id = ?", id).Updates(updates).Error
}

func DeleteSSOProject(id int) error {
	return DB.Delete(&SSOProject{}, id).Error
}

func SSOProjectExistsWithKey(key string, excludeID int) (bool, error) {
	var count int64
	query := DB.Model(&SSOProject{}).Where("project_key = ?", key)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func IsSSOProjectOriginAllowed(origin string) bool {
	origin = strings.TrimRight(strings.TrimSpace(origin), "/")
	if origin == "" || DB == nil {
		return false
	}
	var projects []SSOProject
	if err := DB.Where("enabled = ? AND sso_enabled = ?", true, true).Find(&projects).Error; err != nil {
		return false
	}
	for _, project := range projects {
		projectOrigin, err := project.Origin()
		if err == nil && projectOrigin == origin {
			return true
		}
	}
	return false
}

func migrateSSOProjectLegacyKeyColumn() {
	if DB == nil || !DB.Migrator().HasTable("sso_projects") {
		return
	}
	if !ssoProjectColumnExists("key") {
		return
	}
	if ssoProjectColumnExists("project_key") {
		if common.UsingMySQL {
			_ = DB.Exec("UPDATE sso_projects SET project_key = `key` WHERE (project_key IS NULL OR project_key = '') AND `key` IS NOT NULL").Error
		} else {
			_ = DB.Exec("UPDATE sso_projects SET project_key = key WHERE (project_key IS NULL OR project_key = '') AND key IS NOT NULL").Error
		}
	}
	if common.UsingMySQL {
		if err := DB.Exec("ALTER TABLE sso_projects MODIFY COLUMN `key` varchar(64) NULL").Error; err != nil {
			common.SysLog("failed to relax legacy sso_projects.key column: " + err.Error())
		}
	}
}

func ssoProjectColumnExists(column string) bool {
	if DB == nil {
		return false
	}
	if common.UsingMySQL {
		var count int64
		err := DB.Raw(
			"SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?",
			"sso_projects",
			column,
		).Scan(&count).Error
		if err != nil {
			common.SysLog("failed to inspect sso_projects." + column + " column: " + err.Error())
			return false
		}
		return count > 0
	}
	defer func() {
		if r := recover(); r != nil {
			common.SysLog("failed to inspect sso_projects." + column + " column")
		}
	}()
	return DB.Migrator().HasColumn("sso_projects", column)
}

func IsRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
