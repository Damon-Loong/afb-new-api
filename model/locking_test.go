package model

import (
	"testing"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/utils/tests"
)

func TestLockForUpdateEmitsRowLock(t *testing.T) {
	database, err := gorm.Open(tests.DummyDialector{}, &gorm.Config{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	originalSQLite := common.UsingSQLite
	t.Cleanup(func() { common.UsingSQLite = originalSQLite })

	common.UsingSQLite = false
	var rows []Redemption
	query := lockForUpdate(database).Where("id = ?", 1).Find(&rows)
	if query.Error != nil {
		t.Fatal(query.Error)
	}
	if sql := query.Statement.SQL.String(); !strings.Contains(sql, "FOR UPDATE") {
		t.Fatalf("expected FOR UPDATE in %q", sql)
	}

	common.UsingSQLite = true
	query = lockForUpdate(database.Session(&gorm.Session{NewDB: true})).Where("id = ?", 1).Find(&rows)
	if sql := query.Statement.SQL.String(); strings.Contains(sql, "FOR UPDATE") {
		t.Fatalf("did not expect FOR UPDATE in SQLite query %q", sql)
	}
}
