package model

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

// BillExportFilter intentionally excludes pagination: a bill covers all matching
// consumption/refund records, not just the visible log page.
type BillExportFilter struct {
	StartTimestamp int64    `json:"start_timestamp"`
	EndTimestamp   int64    `json:"end_timestamp"`
	Username       string   `json:"username"`
	TokenName      string   `json:"token_name"`
	ModelName      string   `json:"model_name"`
	UserIDs        []int    `json:"user_ids"`
	ModelNames     []string `json:"model_names"`
	Channel        int      `json:"channel"`
	Group          string   `json:"group"`
	RequestID      string   `json:"request_id"`
	LogType        int      `json:"log_type"`
}

const MaxBillExportRows = 100000

var ErrBillExportTooLarge = errors.New("账单超过 100000 条，请缩小时间或筛选范围后重试；未导出任何截断数据")

func IsBillLogType(logType int) bool {
	return IsConsumeLogType(logType) || logType == LogTypeRefund
}

// GetBillExportLogs uses one bounded query, so details and summaries are always
// derived from the same set. userID is enforced by the server for self exports.
func GetBillExportLogs(ctx context.Context, filter BillExportFilter, userID int, admin bool) ([]Log, error) {
	if !admin && userID <= 0 {
		return nil, errors.New("无权导出账单")
	}
	if filter.LogType != 0 && !IsBillLogType(filter.LogType) {
		return nil, errors.New("消费账单仅支持消费、市场消耗、项目消耗和额度退回")
	}
	query := LOG_DB.WithContext(ctx).Model(&Log{})
	if filter.LogType == 0 {
		query = query.Where("type IN ?", []int{LogTypeConsume, LogTypeRefund, LogTypeMarketConsume, LogTypeProjectConsume})
	} else {
		query = query.Where("type = ?", filter.LogType)
	}
	if !admin {
		query = query.Where("user_id = ?", userID)
	} else {
		if len(filter.UserIDs) > 0 {
			query = query.Where("user_id IN ?", filter.UserIDs)
		} else if filter.Username != "" {
			query = query.Where("username = ?", filter.Username)
		}
		if filter.Channel != 0 {
			query = query.Where("channel_id = ?", filter.Channel)
		}
	}
	query = query.Where("created_at >= ? AND created_at <= ?", filter.StartTimestamp, filter.EndTimestamp)
	if len(filter.ModelNames) > 0 {
		query = query.Where("model_name IN ?", filter.ModelNames)
	} else if filter.ModelName != "" {
		if admin {
			query = query.Where("model_name LIKE ?", filter.ModelName)
		} else {
			pattern, err := sanitizeLikePattern(filter.ModelName)
			if err != nil {
				return nil, err
			}
			query = query.Where("model_name LIKE ? ESCAPE '!'", pattern)
		}
	}
	for column, value := range map[string]string{"token_name": filter.TokenName, "request_id": filter.RequestID} {
		if value != "" {
			query = query.Where(column+" = ?", value)
		}
	}
	if filter.Group != "" {
		query = query.Where(map[string]interface{}{"group": filter.Group})
	}
	var logs []Log
	// Do not load IPs or free-form content into a financial export.
	err := query.Omit("Content", "Ip").Order("id ASC").Limit(MaxBillExportRows + 1).Find(&logs).Error
	if err != nil {
		return nil, err
	}
	if len(logs) > MaxBillExportRows {
		return nil, ErrBillExportTooLarge
	}
	return logs, nil
}

type BillExportUserOption struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
}

type BillExportFilterOptions struct {
	Users  []BillExportUserOption `json:"users"`
	Models []string               `json:"models"`
}

// Options come from billable logs so retired models and deleted users remain
// selectable. Self exports must never expose other users or their model names.
func GetBillExportFilterOptions(ctx context.Context, start, end int64, userID int, admin bool) (BillExportFilterOptions, error) {
	options := BillExportFilterOptions{Users: []BillExportUserOption{}, Models: []string{}}
	if !admin && userID <= 0 {
		return options, errors.New("无权导出账单")
	}
	query := func() *gorm.DB {
		q := LOG_DB.WithContext(ctx).Model(&Log{}).
			Where("type IN ?", []int{LogTypeConsume, LogTypeRefund, LogTypeMarketConsume, LogTypeProjectConsume}).
			Where("created_at >= ? AND created_at <= ?", start, end)
		if !admin {
			q = q.Where("user_id = ?", userID)
		}
		return q
	}
	if admin {
		if err := query().Select("user_id, MAX(username) AS username").Group("user_id").Order("user_id ASC").Scan(&options.Users).Error; err != nil {
			return options, err
		}
	}
	err := query().Where("model_name <> ?", "").Distinct("model_name").Order("model_name ASC").Pluck("model_name", &options.Models).Error
	return options, err
}
