package service

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
)

func setupUserToolStoreTest(t *testing.T) {
	t.Helper()
	root := setupToolStoreTest(t)
	t.Setenv("TOOL_SQL_DSN", "")
	t.Setenv("TOOL_SQLITE_PATH", filepath.Join(root, "data", "tool-local-test.db"))
	if model.ToolDB != nil {
		if sqlDB, err := model.ToolDB.DB(); err == nil {
			_ = sqlDB.Close()
		}
		model.ToolDB = nil
	}
	if err := model.InitToolDB(); err != nil {
		t.Fatalf("init tool db failed: %v", err)
	}
	t.Cleanup(func() {
		if model.ToolDB != nil {
			if sqlDB, err := model.ToolDB.DB(); err == nil {
				_ = sqlDB.Close()
			}
			model.ToolDB = nil
		}
	})
}

func TestInstallListAndUninstallUserTool(t *testing.T) {
	setupUserToolStoreTest(t)

	detail, err := UploadTool("weather.json", strings.NewReader(validOpenAPIJSON), int64(len(validOpenAPIJSON)), ToolUploadOptions{Publish: true})
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}

	first, err := InstallUserTool(1001, detail.ID)
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if first.UserID != 1001 || first.ToolID != detail.ID || !first.Enabled || first.Tool.Name != "Weather Tool" {
		t.Fatalf("unexpected installed item: %+v", first)
	}

	second, err := InstallUserTool(1001, detail.ID)
	if err != nil {
		t.Fatalf("second install failed: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("expected idempotent install, got first=%d second=%d", first.ID, second.ID)
	}

	list, err := ListUserTools(1001)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(list.Tools) != 1 || list.Tools[0].ToolID != detail.ID || len(list.Tools[0].Actions) != 1 {
		t.Fatalf("unexpected list: %+v", list)
	}

	otherList, err := ListUserTools(1002)
	if err != nil {
		t.Fatalf("other list failed: %v", err)
	}
	if len(otherList.Tools) != 0 {
		t.Fatalf("expected isolated user tools, got %+v", otherList)
	}

	if err := UninstallUserTool(1001, detail.ID); err != nil {
		t.Fatalf("uninstall failed: %v", err)
	}
	empty, err := ListUserTools(1001)
	if err != nil {
		t.Fatalf("list after uninstall failed: %v", err)
	}
	if len(empty.Tools) != 0 {
		t.Fatalf("expected empty list after uninstall, got %+v", empty)
	}
}
