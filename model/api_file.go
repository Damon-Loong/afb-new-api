package model

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	ApiFileStorageLocal = "local"
	ApiFileStorageOSS   = "oss"

	ApiFileStatusProcessed = "processed"
	ApiFileStatusError     = "error"
)

type ApiFile struct {
	ID         int    `json:"-" gorm:"primaryKey;autoIncrement"`
	FileID     string `json:"-" gorm:"column:file_id;type:varchar(64);uniqueIndex"`
	UserID     int    `json:"-" gorm:"index"`
	FileName   string `json:"filename" gorm:"column:filename;type:varchar(255)"`
	Purpose    string `json:"purpose" gorm:"type:varchar(64);index"`
	FileSize   int64  `json:"bytes" gorm:"column:bytes"`
	MimeType   string `json:"-" gorm:"type:varchar(128)"`
	Storage    string `json:"-" gorm:"type:varchar(16)"`
	StorageKey string `json:"-" gorm:"column:storage_path;type:text"`
	FileURL    string `json:"-" gorm:"type:text"`
	Status     string `json:"status" gorm:"type:varchar(32)"`
	CreatedAt  int64  `json:"created_at"`
}

func (ApiFile) TableName() string {
	return "api_files"
}

func (f *ApiFile) BeforeCreate(tx *gorm.DB) error {
	if f.CreatedAt == 0 {
		f.CreatedAt = time.Now().Unix()
	}
	if strings.TrimSpace(f.Status) == "" {
		f.Status = ApiFileStatusProcessed
	}
	return nil
}

func CreateApiFile(file *ApiFile) error {
	return DB.Create(file).Error
}

func GetApiFileByFileID(fileID string) (*ApiFile, error) {
	var file ApiFile
	err := DB.Where("file_id = ?", fileID).First(&file).Error
	if err != nil {
		return nil, err
	}
	return &file, nil
}

func ListApiFilesByUser(userID int, afterID string, limit int) ([]ApiFile, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var files []ApiFile
	db := DB.Where("user_id = ?", userID).Order("id DESC").Limit(limit)
	if strings.TrimSpace(afterID) != "" {
		var cursor ApiFile
		if err := DB.Where("file_id = ? AND user_id = ?", afterID, userID).First(&cursor).Error; err == nil {
			db = db.Where("id < ?", cursor.ID)
		}
	}
	err := db.Find(&files).Error
	return files, err
}

func DeleteApiFile(fileID string, userID int) error {
	return DB.Where("file_id = ? AND user_id = ?", fileID, userID).Delete(&ApiFile{}).Error
}
