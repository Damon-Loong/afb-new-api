package service

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm/clause"
)

type UserToolItem struct {
	model.UserTool
	Tool    ToolSummary  `json:"tool"`
	Actions []ToolAction `json:"actions,omitempty"`
}

type UserToolList struct {
	Tools []UserToolItem `json:"tools"`
}

func InstallUserTool(userID int, toolID string) (UserToolItem, error) {
	if userID <= 0 {
		return UserToolItem{}, NewToolAppError("invalid_request", "用户未登录")
	}
	if err := ensureToolDB(); err != nil {
		return UserToolItem{}, err
	}
	detail, err := GetToolDetail(toolID)
	if err != nil {
		return UserToolItem{}, err
	}
	if detail.Status != "" && detail.Status != "published" {
		return UserToolItem{}, NewToolAppError("tool_not_available", "工具暂不可获取")
	}
	userTool := model.UserTool{
		UserID:  userID,
		ToolID:  detail.ID,
		Enabled: true,
	}
	if err := model.ToolDB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "tool_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"enabled", "updated_at"}),
	}).Create(&userTool).Error; err != nil {
		return UserToolItem{}, err
	}
	if err := model.ToolDB.Where("user_id = ? AND tool_id = ?", userID, detail.ID).First(&userTool).Error; err != nil {
		return UserToolItem{}, err
	}
	return makeUserToolItem(userTool, detail), nil
}

func UninstallUserTool(userID int, toolID string) error {
	if userID <= 0 {
		return NewToolAppError("invalid_request", "用户未登录")
	}
	if err := ensureToolDB(); err != nil {
		return err
	}
	toolID = strings.TrimSpace(toolID)
	if toolID == "" {
		return NewToolAppError("invalid_request", "工具 ID 无效")
	}
	result := model.ToolDB.Where("user_id = ? AND tool_id = ?", userID, toolID).Delete(&model.UserTool{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return NewToolAppError("user_tool_not_found", "用户尚未获取该工具")
	}
	return nil
}

func ListUserTools(userID int) (UserToolList, error) {
	if userID <= 0 {
		return UserToolList{}, NewToolAppError("invalid_request", "用户未登录")
	}
	if err := ensureToolDB(); err != nil {
		return UserToolList{}, err
	}
	var rows []model.UserTool
	if err := model.ToolDB.Where("user_id = ?", userID).Order("installed_at desc").Find(&rows).Error; err != nil {
		return UserToolList{}, err
	}
	items := make([]UserToolItem, 0, len(rows))
	for _, row := range rows {
		detail, err := GetToolDetail(row.ToolID)
		if err != nil {
			var appErr *ToolAppError
			if errors.As(err, &appErr) && appErr.Code == "tool_not_found" {
				continue
			}
			return UserToolList{}, err
		}
		items = append(items, makeUserToolItem(row, detail))
	}
	return UserToolList{Tools: items}, nil
}

func GetInstalledToolIDs(userID int) (map[string]bool, error) {
	if userID <= 0 {
		return map[string]bool{}, nil
	}
	if err := ensureToolDB(); err != nil {
		return nil, err
	}
	var rows []model.UserTool
	if err := model.ToolDB.Select("tool_id").Where("user_id = ?", userID).Find(&rows).Error; err != nil {
		return nil, err
	}
	installed := make(map[string]bool, len(rows))
	for _, row := range rows {
		installed[row.ToolID] = true
	}
	return installed, nil
}

func ensureToolDB() error {
	if model.ToolDB != nil {
		return nil
	}
	if err := model.InitToolDB(); err != nil {
		return NewToolAppError("tool_database_unavailable", "工具数据库初始化失败")
	}
	return nil
}

func makeUserToolItem(userTool model.UserTool, detail ToolDetail) UserToolItem {
	return UserToolItem{
		UserTool: userTool,
		Tool:     detail.ToolSummary,
		Actions:  detail.Actions,
	}
}
