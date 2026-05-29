package service

import (
	"time"

	"github.com/QuantumNous/new-api/model"
)

type IncomePoint struct {
	Date       string `json:"date"`
	Label      string `json:"label"`
	SkillQuota int    `json:"skill_quota"`
	MCPQuota   int    `json:"mcp_quota"`
	TotalQuota int    `json:"total_quota"`
}

type IncomeSummary struct {
	RangeDays    int           `json:"range_days"`
	TotalQuota   int           `json:"total_quota"`
	SkillQuota   int           `json:"skill_quota"`
	MCPQuota     int           `json:"mcp_quota"`
	PendingQuota int           `json:"pending_quota"`
	HistoryQuota int           `json:"history_quota"`
	TodayQuota   int           `json:"today_quota"`
	Points       []IncomePoint `json:"points"`
}

type incomeQuotaRow struct {
	Timestamp int64
	Quota     int
}

func GetIncomeSummary(userID int, days int) (IncomeSummary, error) {
	if userID <= 0 {
		return IncomeSummary{}, NewToolAppError("invalid_request", "用户未登录")
	}
	days = normalizeIncomeDays(days)
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -(days - 1))
	startUnix := start.Unix()

	points, dateIndex := buildIncomePoints(start, days)
	summary := IncomeSummary{
		RangeDays: days,
		Points:    points,
	}

	var user model.User
	if err := model.DB.Select("id, aff_quota, aff_history").Where("id = ?", userID).First(&user).Error; err != nil {
		return IncomeSummary{}, err
	}
	summary.PendingQuota = user.AffQuota
	summary.HistoryQuota = user.AffHistoryQuota

	skillRows, err := listSkillIncomeRows(userID, startUnix)
	if err != nil {
		return IncomeSummary{}, err
	}
	applyIncomeRows(summary.Points, dateIndex, skillRows, func(point *IncomePoint, quota int) {
		point.SkillQuota += quota
		summary.SkillQuota += quota
	})

	mcpRows, err := listMCPIncomeRows(userID, startUnix)
	if err != nil {
		return IncomeSummary{}, err
	}
	applyIncomeRows(summary.Points, dateIndex, mcpRows, func(point *IncomePoint, quota int) {
		point.MCPQuota += quota
		summary.MCPQuota += quota
	})

	for i := range summary.Points {
		summary.Points[i].TotalQuota = summary.Points[i].SkillQuota + summary.Points[i].MCPQuota
		summary.TotalQuota += summary.Points[i].TotalQuota
	}
	if len(summary.Points) > 0 {
		summary.TodayQuota = summary.Points[len(summary.Points)-1].TotalQuota
	}
	return summary, nil
}

func listSkillIncomeRows(userID int, startUnix int64) ([]incomeQuotaRow, error) {
	var rows []incomeQuotaRow
	err := model.DB.Table("user_skills").
		Select("user_skills.acquired_at AS timestamp, user_skills.paid_quota AS quota").
		Joins("JOIN skills ON skills.id = user_skills.skill_id").
		Where("skills.user_id = ? AND user_skills.paid_quota > 0 AND user_skills.acquired_at >= ?", userID, startUnix).
		Scan(&rows).Error
	return rows, err
}

func listMCPIncomeRows(userID int, startUnix int64) ([]incomeQuotaRow, error) {
	toolIDs, err := listCreatedToolIDs(userID)
	if err != nil {
		return nil, err
	}
	if len(toolIDs) == 0 {
		return nil, nil
	}
	if err := ensureToolDB(); err != nil {
		return nil, err
	}
	var rows []incomeQuotaRow
	err = model.ToolDB.Model(&model.ToolRun{}).
		Select("finished_at AS timestamp, reward_quota AS quota").
		Where("tool_id IN ? AND billing_status = ? AND reward_quota > 0 AND finished_at >= ?", toolIDs, "charged", startUnix).
		Scan(&rows).Error
	return rows, err
}

func listCreatedToolIDs(userID int) ([]string, error) {
	index, err := readToolIndex()
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0)
	for i, tool := range index.Tools {
		hydrated := hydrateToolSummary(tool)
		if hydrated != tool {
			index.Tools[i] = hydrated
		}
		if hydrated.CreatedBy == userID && hydrated.ID != "" {
			ids = append(ids, hydrated.ID)
		}
	}
	return ids, nil
}

func buildIncomePoints(start time.Time, days int) ([]IncomePoint, map[string]int) {
	points := make([]IncomePoint, 0, days)
	dateIndex := make(map[string]int, days)
	for i := 0; i < days; i++ {
		day := start.AddDate(0, 0, i)
		date := day.Format("2006-01-02")
		points = append(points, IncomePoint{
			Date:  date,
			Label: formatIncomeDayLabel(day),
		})
		dateIndex[date] = i
	}
	return points, dateIndex
}

func applyIncomeRows(points []IncomePoint, dateIndex map[string]int, rows []incomeQuotaRow, apply func(point *IncomePoint, quota int)) {
	for _, row := range rows {
		if row.Quota <= 0 || row.Timestamp <= 0 {
			continue
		}
		date := time.Unix(row.Timestamp, 0).Format("2006-01-02")
		index, ok := dateIndex[date]
		if !ok {
			continue
		}
		apply(&points[index], row.Quota)
	}
}

func formatIncomeDayLabel(day time.Time) string {
	return day.Format("1月2日")
}

func normalizeIncomeDays(days int) int {
	switch days {
	case 30, 60:
		return days
	default:
		return 7
	}
}
