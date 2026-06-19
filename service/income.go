package service

import (
	"sort"
	"strconv"
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

type IncomeSource struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Quota       int    `json:"quota"`
	Count       int64  `json:"count"`
	LatestAt    int64  `json:"latest_at"`
}

type IncomeSourcesResult struct {
	Sources []IncomeSource `json:"sources"`
	Total   int            `json:"total"`
}

type IncomeSourceFlow struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	UserID     int    `json:"user_id"`
	UserName   string `json:"user_name"`
	Quota      int    `json:"quota"`
	OccurredAt int64  `json:"occurred_at"`
	Note       string `json:"note"`
}

type IncomeSourceFlowsResult struct {
	Flows []IncomeSourceFlow `json:"flows"`
	Total int                `json:"total"`
}

type incomeQuotaRow struct {
	Timestamp int64
	Quota     int
}

type incomeSourceRow struct {
	ID       string
	Name     string
	Quota    int
	Count    int64
	LatestAt int64
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
		Select("user_skills.updated_at AS timestamp, user_skills.paid_quota AS quota").
		Joins("JOIN skills ON skills.id = user_skills.skill_id").
		Where("skills.user_id = ? AND user_skills.paid_quota > 0 AND user_skills.updated_at >= ?", userID, startUnix).
		Scan(&rows).Error
	return rows, err
}

func GetIncomeSources(userID int) (IncomeSourcesResult, error) {
	if userID <= 0 {
		return IncomeSourcesResult{}, NewToolAppError("invalid_request", "用户未登录")
	}

	sources := make([]IncomeSource, 0)
	skillSources, err := listSkillIncomeSources(userID, 0)
	if err != nil {
		return IncomeSourcesResult{}, err
	}
	sources = append(sources, skillSources...)
	mcpSources, err := listMCPIncomeSources(userID, 0)
	if err != nil {
		return IncomeSourcesResult{}, err
	}
	sources = append(sources, mcpSources...)

	sort.SliceStable(sources, func(i, j int) bool {
		if sources[i].Quota == sources[j].Quota {
			return sources[i].LatestAt > sources[j].LatestAt
		}
		return sources[i].Quota > sources[j].Quota
	})
	return IncomeSourcesResult{Sources: sources, Total: len(sources)}, nil
}

func GetIncomeSourceFlows(userID int, kind string, sourceID string, limit int) (IncomeSourceFlowsResult, error) {
	if userID <= 0 {
		return IncomeSourceFlowsResult{}, NewToolAppError("invalid_request", "用户未登录")
	}
	limit = normalizeIncomeFlowLimit(limit)
	switch kind {
	case "skill":
		return listSkillIncomeFlows(userID, sourceID, limit)
	case "mcp":
		return listMCPIncomeFlows(userID, sourceID, limit)
	default:
		return IncomeSourceFlowsResult{}, NewToolAppError("invalid_request", "奖励来源类型无效")
	}
}

func listSkillIncomeSources(userID int, startUnix int64) ([]IncomeSource, error) {
	var rows []struct {
		SkillID     int
		Title       string
		Description string
		Quota       int
		Count       int64
		LatestAt    int64
	}
	err := model.DB.Table("user_skills").
		Select("skills.id AS skill_id, skills.title, skills.description, SUM(user_skills.paid_quota) AS quota, COUNT(*) AS count, MAX(user_skills.updated_at) AS latest_at").
		Joins("JOIN skills ON skills.id = user_skills.skill_id").
		Where("skills.user_id = ? AND user_skills.paid_quota > 0 AND user_skills.updated_at >= ?", userID, startUnix).
		Group("skills.id, skills.title, skills.description").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	sources := make([]IncomeSource, 0, len(rows))
	for _, row := range rows {
		sources = append(sources, IncomeSource{
			ID:          strconv.Itoa(row.SkillID),
			Kind:        "skill",
			Name:        row.Title,
			Description: row.Description,
			Quota:       row.Quota,
			Count:       row.Count,
			LatestAt:    row.LatestAt,
		})
	}
	return sources, nil
}

func listSkillIncomeFlows(userID int, sourceID string, limit int) (IncomeSourceFlowsResult, error) {
	skillID, err := strconv.Atoi(sourceID)
	if err != nil || skillID <= 0 {
		return IncomeSourceFlowsResult{}, NewToolAppError("invalid_request", "Skill ID 无效")
	}
	var skill model.Skill
	if err := model.DB.Select("id, user_id, title").Where("id = ? AND user_id = ?", skillID, userID).First(&skill).Error; err != nil {
		return IncomeSourceFlowsResult{}, NewToolAppError("not_found", "奖励来源不存在")
	}
	var rows []struct {
		ID         int64
		UserID     int
		UserName   string
		Phone      *string
		Quota      int
		OccurredAt int64
	}
	err = model.DB.Table("user_skills").
		Select("user_skills.id, user_skills.user_id, users.username AS user_name, users.phone, user_skills.paid_quota AS quota, user_skills.updated_at AS occurred_at").
		Joins("LEFT JOIN users ON users.id = user_skills.user_id").
		Where("user_skills.skill_id = ? AND user_skills.paid_quota > 0", skillID).
		Order("user_skills.updated_at DESC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return IncomeSourceFlowsResult{}, err
	}
	flows := make([]IncomeSourceFlow, 0, len(rows))
	for _, row := range rows {
		flows = append(flows, IncomeSourceFlow{
			ID:         strconv.FormatInt(row.ID, 10),
			Kind:       "skill",
			UserID:     row.UserID,
			UserName:   normalizeIncomeUserName(row.UserID, row.UserName, row.Phone),
			Quota:      row.Quota,
			OccurredAt: row.OccurredAt,
			Note:       "调用 Skill",
		})
	}
	return IncomeSourceFlowsResult{Flows: flows, Total: len(flows)}, nil
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

func listMCPIncomeSources(userID int, startUnix int64) ([]IncomeSource, error) {
	tools, err := listCreatedToolSummaries(userID)
	if err != nil {
		return nil, err
	}
	if len(tools) == 0 {
		return nil, nil
	}
	toolIDs := make([]string, 0, len(tools))
	toolByID := make(map[string]ToolSummary, len(tools))
	for _, tool := range tools {
		toolIDs = append(toolIDs, tool.ID)
		toolByID[tool.ID] = tool
	}
	if err := ensureToolDB(); err != nil {
		return nil, err
	}
	var rows []incomeSourceRow
	err = model.ToolDB.Model(&model.ToolRun{}).
		Select("tool_id AS id, SUM(reward_quota) AS quota, COUNT(*) AS count, MAX(finished_at) AS latest_at").
		Where("tool_id IN ? AND billing_status = ? AND reward_quota > 0 AND finished_at >= ?", toolIDs, "charged", startUnix).
		Group("tool_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	sources := make([]IncomeSource, 0, len(rows))
	for _, row := range rows {
		tool := toolByID[row.ID]
		name := tool.Name
		if name == "" {
			name = row.ID
		}
		sources = append(sources, IncomeSource{
			ID:          row.ID,
			Kind:        "mcp",
			Name:        name,
			Description: tool.Description,
			Quota:       row.Quota,
			Count:       row.Count,
			LatestAt:    row.LatestAt,
		})
	}
	return sources, nil
}

func listMCPIncomeFlows(userID int, sourceID string, limit int) (IncomeSourceFlowsResult, error) {
	tool, err := getCreatedToolSummary(userID, sourceID)
	if err != nil {
		return IncomeSourceFlowsResult{}, err
	}
	if tool.ID == "" {
		return IncomeSourceFlowsResult{}, NewToolAppError("not_found", "奖励来源不存在")
	}
	if err := ensureToolDB(); err != nil {
		return IncomeSourceFlowsResult{}, err
	}
	var rows []model.ToolRun
	if err := model.ToolDB.Where("tool_id = ? AND billing_status = ? AND reward_quota > 0", tool.ID, "charged").
		Order("finished_at DESC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return IncomeSourceFlowsResult{}, err
	}
	userNames := getIncomeUserDisplayNames(rows)
	flows := make([]IncomeSourceFlow, 0, len(rows))
	for _, row := range rows {
		note := "调用 MCP"
		if row.FunctionName != "" {
			note = "调用 " + row.FunctionName
		}
		flows = append(flows, IncomeSourceFlow{
			ID:         strconv.FormatInt(row.ID, 10),
			Kind:       "mcp",
			UserID:     row.UserID,
			UserName:   userNames[row.UserID],
			Quota:      row.RewardQuota,
			OccurredAt: row.FinishedAt,
			Note:       note,
		})
	}
	return IncomeSourceFlowsResult{Flows: flows, Total: len(flows)}, nil
}

func listCreatedToolIDs(userID int) ([]string, error) {
	tools, err := listCreatedToolSummaries(userID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(tools))
	for _, tool := range tools {
		ids = append(ids, tool.ID)
	}
	return ids, nil
}

func listCreatedToolSummaries(userID int) ([]ToolSummary, error) {
	if model.DB == nil {
		return nil, NewToolAppError("tool_database_unavailable", "工具数据库不可用")
	}
	var rows []model.Tool
	if err := model.DB.Where("created_by = ?", userID).
		Order("updated_at desc, created_at desc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	tools := make([]ToolSummary, 0, len(rows))
	for _, row := range rows {
		tools = append(tools, toolSummaryFromModel(row))
	}
	return tools, nil
}

func getCreatedToolSummary(userID int, toolID string) (ToolSummary, error) {
	tools, err := listCreatedToolSummaries(userID)
	if err != nil {
		return ToolSummary{}, err
	}
	for _, tool := range tools {
		if tool.ID == toolID {
			return tool, nil
		}
	}
	return ToolSummary{}, nil
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

func normalizeIncomeFlowLimit(value int) int {
	if value <= 0 {
		return 50
	}
	if value > 100 {
		return 100
	}
	return value
}

func getIncomeUserDisplayNames(rows []model.ToolRun) map[int]string {
	ids := make([]int, 0)
	seen := map[int]bool{}
	for _, row := range rows {
		if row.UserID > 0 && !seen[row.UserID] {
			seen[row.UserID] = true
			ids = append(ids, row.UserID)
		}
	}
	names := make(map[int]string, len(ids))
	if len(ids) == 0 || model.DB == nil {
		return names
	}
	var users []model.User
	if err := model.DB.Select("id, username, phone").Where("id IN ?", ids).Find(&users).Error; err != nil {
		return names
	}
	for _, user := range users {
		names[user.Id] = normalizeIncomeUserName(user.Id, user.Username, user.Phone)
	}
	for _, id := range ids {
		if names[id] == "" {
			names[id] = normalizeIncomeUserName(id, "", nil)
		}
	}
	return names
}

func normalizeIncomeUserName(userID int, username string, phone *string) string {
	if phone != nil && *phone != "" {
		return maskIncomePhone(*phone)
	}
	if username != "" {
		return username
	}
	if userID > 0 {
		return "用户 " + strconv.Itoa(userID)
	}
	return "未知用户"
}

func maskIncomePhone(phone string) string {
	if len(phone) >= 13 && phone[:3] == "+86" {
		return phone[:5] + "****" + phone[len(phone)-4:]
	}
	if len(phone) <= 7 {
		return phone
	}
	return phone[:3] + "****" + phone[len(phone)-4:]
}
