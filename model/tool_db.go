package model

import (
	"errors"

	"gorm.io/gorm"
)

var ToolDB *gorm.DB

func InitToolDB() error {
	if DB == nil {
		return errors.New("main database is not initialized")
	}
	ToolDB = DB
	return ToolDB.AutoMigrate(&Tool{}, &ToolAction{}, &ToolSecret{}, &UserTool{}, &ToolRun{})
}
