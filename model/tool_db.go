package model

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var ToolDB *gorm.DB

func InitToolDB() error {
	db, err := openToolDB()
	if err != nil {
		return err
	}
	ToolDB = db
	sqlDB, err := ToolDB.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Minute)
	return ToolDB.AutoMigrate(&UserTool{}, &ToolRun{})
}

func openToolDB() (*gorm.DB, error) {
	dsn := strings.TrimSpace(os.Getenv("TOOL_SQL_DSN"))
	if dsn != "" {
		if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
			return gorm.Open(postgres.New(postgres.Config{
				DSN:                  dsn,
				PreferSimpleProtocol: true,
			}), &gorm.Config{PrepareStmt: true})
		}
		if strings.HasPrefix(dsn, "local") {
			return openToolSQLite(toolSQLiteDSN())
		}
		if !strings.Contains(dsn, "parseTime") {
			if strings.Contains(dsn, "?") {
				dsn += "&parseTime=true"
			} else {
				dsn += "?parseTime=true"
			}
		}
		return gorm.Open(mysql.Open(dsn), &gorm.Config{PrepareStmt: true})
	}
	return openToolSQLite(toolSQLiteDSN())
}

func toolSQLiteDSN() string {
	path := strings.TrimSpace(os.Getenv("TOOL_SQLITE_PATH"))
	if path == "" {
		path = filepath.Join("data", "tool-local-dev.db")
	}
	if strings.Contains(path, "?") {
		return path
	}
	return path + "?_busy_timeout=30000"
}

func openToolSQLite(dsn string) (*gorm.DB, error) {
	path := dsn
	if idx := strings.Index(path, "?"); idx >= 0 {
		path = path[:idx]
	}
	if path != "" && path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, err
		}
	}
	return gorm.Open(sqlite.Open(dsn), &gorm.Config{PrepareStmt: true})
}
