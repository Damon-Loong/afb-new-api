package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

type SkillCreateOptions struct {
	UserID        int
	Title         string
	Description   string
	PackageURL    string
	SkillMarkdown string
	DownloadPrice int
	PromotionMode string
	Visibility    string
	Publish       bool
}

type SkillListItem struct {
	model.Skill
	Acquired       bool   `json:"acquired"`
	CreatedByName  string `json:"created_by_name,omitempty"`
	CurrentEarning int    `json:"current_earning"`
}

type SkillListOptions struct {
	UserID       int
	Keyword      string
	AcquiredOnly bool
	Limit        int
	Offset       int
}

type SkillListResult struct {
	Skills  []SkillListItem `json:"skills"`
	Total   int64           `json:"total"`
	Limit   int             `json:"limit"`
	Offset  int             `json:"offset"`
	HasMore bool            `json:"has_more"`
}

type SkillAcquireResult struct {
	Skill      model.Skill `json:"skill"`
	PackageURL string      `json:"package_url"`
	Acquired   bool        `json:"acquired"`
	Charged    int         `json:"charged"`
}

func CreateSkill(opts SkillCreateOptions) (model.Skill, error) {
	if opts.UserID <= 0 {
		return model.Skill{}, NewToolAppError("invalid_request", "用户未登录")
	}
	title := strings.TrimSpace(opts.Title)
	if title == "" {
		return model.Skill{}, NewToolAppError("invalid_request", "Skill 标题不能为空")
	}
	titleTaken, err := skillTitleExists(title)
	if err != nil {
		return model.Skill{}, err
	}
	if titleTaken {
		return model.Skill{}, NewToolAppError("skill_name_conflict", "Skill 标题已存在")
	}
	description := strings.TrimSpace(opts.Description)
	if description == "" {
		return model.Skill{}, NewToolAppError("invalid_request", "Skill 描述不能为空")
	}
	packageURL := strings.TrimSpace(opts.PackageURL)
	if packageURL == "" {
		return model.Skill{}, NewToolAppError("invalid_request", "Skill 压缩包链接不能为空")
	}
	downloadPrice := opts.DownloadPrice
	if downloadPrice < 0 {
		return model.Skill{}, NewToolAppError("invalid_request", "获取价格必须是非负整数")
	}
	status := model.SkillStatusPublished
	if !opts.Publish {
		status = model.SkillStatusDraft
	}
	skill := model.Skill{
		UserID:        opts.UserID,
		Title:         title,
		Description:   description,
		PackageURL:    packageURL,
		SkillMarkdown: opts.SkillMarkdown,
		DownloadPrice: downloadPrice,
		PromotionMode: normalizeSkillPromotionMode(opts.PromotionMode),
		Visibility:    normalizeSkillVisibility(opts.Visibility),
		Status:        status,
	}
	if err := model.DB.Create(&skill).Error; err != nil {
		return model.Skill{}, err
	}
	return skill, nil
}

func skillTitleExists(title string) (bool, error) {
	if model.DB == nil {
		return false, errors.New("database unavailable")
	}
	normalizedTitle := strings.ToLower(strings.TrimSpace(title))
	if normalizedTitle == "" {
		return false, nil
	}
	var count int64
	if err := model.DB.Model(&model.Skill{}).
		Where("LOWER(title) = ?", normalizedTitle).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func ListUserSkills(userID int) ([]SkillListItem, error) {
	if userID <= 0 {
		return nil, NewToolAppError("invalid_request", "用户未登录")
	}
	var skills []model.Skill
	if err := model.DB.Where("user_id = ?", userID).Order("updated_at desc").Find(&skills).Error; err != nil {
		return nil, err
	}
	earnings, err := getSkillEarnings(skills)
	if err != nil {
		return nil, err
	}
	items := make([]SkillListItem, 0, len(skills))
	for _, skill := range skills {
		items = append(items, SkillListItem{
			Skill:          skill,
			CurrentEarning: earnings[skill.ID],
		})
	}
	return items, nil
}

func ListPublicSkills(userID int) ([]SkillListItem, error) {
	result, err := ListPublicSkillsWithOptions(SkillListOptions{UserID: userID})
	if err != nil {
		return nil, err
	}
	return result.Skills, nil
}

func ListPublicSkillsWithOptions(opts SkillListOptions) (SkillListResult, error) {
	limit := normalizeListLimit(opts.Limit)
	offset := normalizeListOffset(opts.Offset)
	keyword := strings.ToLower(strings.TrimSpace(opts.Keyword))
	query := model.DB.Model(&model.Skill{}).
		Where("visibility = ? AND status = ?", "public", model.SkillStatusPublished)
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("LOWER(title) LIKE ? OR LOWER(description) LIKE ?", like, like)
	}
	if opts.AcquiredOnly {
		if opts.UserID <= 0 {
			return SkillListResult{Skills: []SkillListItem{}, Limit: limit, Offset: offset}, nil
		}
		query = query.Where("id IN (?)", model.DB.Model(&model.UserSkill{}).Select("skill_id").Where("user_id = ?", opts.UserID))
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return SkillListResult{}, err
	}
	var skills []model.Skill
	if err := query.
		Order("updated_at desc").
		Limit(limit).
		Offset(offset).
		Find(&skills).Error; err != nil {
		return SkillListResult{}, err
	}
	acquired := map[int]bool{}
	if opts.UserID > 0 && len(skills) > 0 {
		ids := make([]int, 0, len(skills))
		for _, skill := range skills {
			ids = append(ids, skill.ID)
		}
		var rows []model.UserSkill
		if err := model.DB.Select("skill_id").Where("user_id = ? AND skill_id IN ?", opts.UserID, ids).Find(&rows).Error; err != nil {
			return SkillListResult{}, err
		}
		for _, row := range rows {
			acquired[row.SkillID] = true
		}
	}
	items := make([]SkillListItem, 0, len(skills))
	creatorNames := enrichSkillCreatorNames(skills)
	for _, skill := range skills {
		item := SkillListItem{Skill: skill, Acquired: acquired[skill.ID], CreatedByName: creatorNames[skill.UserID]}
		if !item.Acquired {
			item.PackageURL = ""
			item.SkillMarkdown = ""
		}
		items = append(items, item)
	}
	return SkillListResult{
		Skills:  items,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
		HasMore: int64(offset+len(items)) < total,
	}, nil
}

func enrichSkillCreatorNames(skills []model.Skill) map[int]string {
	names := map[int]string{}
	if len(skills) == 0 || model.DB == nil {
		return names
	}
	ids := make([]int, 0)
	seen := map[int]bool{}
	for _, skill := range skills {
		if skill.UserID > 0 && !seen[skill.UserID] {
			seen[skill.UserID] = true
			ids = append(ids, skill.UserID)
		}
	}
	if len(ids) == 0 {
		return names
	}
	var users []model.User
	if err := model.DB.Select("id, username, phone").Where("id IN ?", ids).Find(&users).Error; err != nil {
		return names
	}
	for _, user := range users {
		names[user.Id] = normalizeIncomeUserName(user.Id, strings.TrimSpace(user.Username), user.Phone)
	}
	return names
}

func getSkillEarnings(skills []model.Skill) (map[int]int, error) {
	earnings := map[int]int{}
	if len(skills) == 0 {
		return earnings, nil
	}
	ids := make([]int, 0, len(skills))
	for _, skill := range skills {
		ids = append(ids, skill.ID)
	}
	var rows []struct {
		SkillID int
		Quota   int
	}
	if err := model.DB.Table("user_skills").
		Select("skill_id, COALESCE(SUM(paid_quota), 0) AS quota").
		Where("skill_id IN ? AND paid_quota > 0", ids).
		Group("skill_id").
		Scan(&rows).Error; err != nil {
		return earnings, err
	}
	for _, row := range rows {
		earnings[row.SkillID] = row.Quota
	}
	return earnings, nil
}

func AcquireSkill(userID int, skillID int) (SkillAcquireResult, error) {
	if userID <= 0 {
		return SkillAcquireResult{}, NewToolAppError("invalid_request", "用户未登录")
	}
	if skillID <= 0 {
		return SkillAcquireResult{}, NewToolAppError("invalid_request", "Skill ID 无效")
	}

	var result SkillAcquireResult
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var skill model.Skill
		if err := tx.Where("id = ?", skillID).First(&skill).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return NewToolAppError("skill_not_found", "Skill 不存在")
			}
			return err
		}
		if skill.Visibility != "public" || skill.Status != model.SkillStatusPublished {
			return NewToolAppError("skill_not_available", "Skill 暂不可获取")
		}

		var existing model.UserSkill
		if err := tx.Where("user_id = ? AND skill_id = ?", userID, skillID).First(&existing).Error; err == nil {
			result = SkillAcquireResult{Skill: skill, PackageURL: skill.PackageURL, Acquired: true, Charged: 0}
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		price := skill.DownloadPrice
		if skill.UserID == userID {
			price = 0
		}
		if price < 0 {
			price = 0
		}
		if price > 0 {
			update := tx.Model(&model.User{}).
				Where("id = ? AND quota >= ?", userID, price).
				Update("quota", gorm.Expr("quota - ?", price))
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected == 0 {
				return NewToolAppError("insufficient_quota", "Token 余额不足")
			}
		}

		row := model.UserSkill{
			UserID:     userID,
			SkillID:    skillID,
			PaidQuota:  price,
			PackageURL: skill.PackageURL,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		if price > 0 {
			reward := tx.Model(&model.User{}).
				Where("id = ?", skill.UserID).
				Updates(map[string]interface{}{
					"aff_quota":   gorm.Expr("aff_quota + ?", price),
					"aff_history": gorm.Expr("aff_history + ?", price),
				})
			if reward.Error != nil {
				return reward.Error
			}
			if reward.RowsAffected == 0 {
				return NewToolAppError("skill_author_not_found", "Skill 作者不存在")
			}
			model.RecordLog(userID, model.LogTypeMarketConsume, fmt.Sprintf("获取 Skill「%s」扣除 %d Token", skill.Title, price))
			model.RecordLog(skill.UserID, model.LogTypeMarketReward, fmt.Sprintf("Skill「%s」被获取，获得奖励 %d Token", skill.Title, price))
		}
		result = SkillAcquireResult{Skill: skill, PackageURL: skill.PackageURL, Acquired: true, Charged: price}
		return nil
	})
	if err != nil {
		return SkillAcquireResult{}, err
	}
	if result.Charged > 0 {
		_, _ = model.GetUserQuota(userID, true)
	}
	return result, nil
}

func normalizeSkillPromotionMode(value string) string {
	switch strings.TrimSpace(value) {
	case "manual":
		return "manual"
	case "none":
		return "none"
	default:
		return "platform_auto"
	}
}

func normalizeSkillVisibility(value string) string {
	switch strings.TrimSpace(value) {
	case "private":
		return "private"
	default:
		return "public"
	}
}
