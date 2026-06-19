package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

type SkillCreateOptions struct {
	UserID          int
	Title           string
	Description     string
	PackageURL      string
	SkillMarkdown   string
	ContentHash     string
	TokenMultiplier int
	PromotionMode   string
	Visibility      string
	Publish         bool
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
	Skill    model.Skill `json:"skill"`
	Acquired bool        `json:"acquired"`
	Charged  int         `json:"charged"`
}

type SkillCallBillingResult struct {
	SkillID         int  `json:"skill_id"`
	AuthorID        int  `json:"author_id"`
	Charged         int  `json:"charged"`
	SkillMDTokens   int  `json:"skill_md_tokens"`
	TokenMultiplier int  `json:"token_multiplier"`
	SelfCall        bool `json:"self_call"`
}

const (
	skillBillingIdempotencyTTL = 5 * time.Minute
)

type skillBillingIdempotencyMemoryEntry struct {
	Result    SkillCallBillingResult
	ExpiresAt time.Time
}

var skillBillingIdempotencyMemory = struct {
	sync.Mutex
	results   map[string]skillBillingIdempotencyMemoryEntry
	lastPrune time.Time
}{
	results: map[string]skillBillingIdempotencyMemoryEntry{},
}

type AcquiredSkillDetail struct {
	ID              int    `json:"id"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	PackageURL      string `json:"package_url"`
	UpdatedAt       int64  `json:"updated_at"`
	ContentHash     string `json:"content_hash"`
	SkillMDTokens   int    `json:"skill_md_tokens"`
	TokenMultiplier int    `json:"token_multiplier"`
}

type AcquiredSkillListResult struct {
	Skills []AcquiredSkillDetail `json:"skills"`
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
	skillMarkdown := strings.TrimSpace(opts.SkillMarkdown)
	if skillMarkdown == "" {
		return model.Skill{}, NewToolAppError("invalid_request", "SKILL.md 不能为空")
	}
	status := model.SkillStatusPublished
	if !opts.Publish {
		status = model.SkillStatusDraft
	}
	skill := model.Skill{
		UserID:          opts.UserID,
		Title:           title,
		Description:     description,
		PackageURL:      packageURL,
		SkillMarkdown:   skillMarkdown,
		ContentHash:     normalizeSkillContentHash(opts.ContentHash, skillMarkdown),
		SkillMDTokens:   CountTextToken(skillMarkdown, "gpt-4o"),
		TokenMultiplier: normalizeSkillTokenMultiplier(opts.TokenMultiplier),
		PromotionMode:   normalizeSkillPromotionMode(opts.PromotionMode),
		Visibility:      normalizeSkillVisibility(opts.Visibility),
		Status:          status,
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

func ListAcquiredSkillDetails(userID int) (AcquiredSkillListResult, error) {
	if userID <= 0 {
		return AcquiredSkillListResult{}, NewToolAppError("invalid_request", "用户未登录")
	}
	var items []AcquiredSkillDetail
	if err := model.DB.Table("skills").
		Select(strings.Join([]string{
			"skills.id",
			"skills.title",
			"skills.description",
			"skills.package_url",
			"skills.updated_at",
			"skills.content_hash",
			"skills.skill_md_tokens",
			"skills.token_multiplier",
		}, ", ")).
		Joins("JOIN user_skills ON user_skills.skill_id = skills.id").
		Where("user_skills.user_id = ? AND skills.status = ?", userID, model.SkillStatusPublished).
		Order("skills.updated_at desc").
		Scan(&items).Error; err != nil {
		return AcquiredSkillListResult{}, err
	}
	for i := range items {
		items[i].TokenMultiplier = normalizeSkillTokenMultiplier(items[i].TokenMultiplier)
	}
	return AcquiredSkillListResult{Skills: items}, nil
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
		item.PackageURL = ""
		if !item.Acquired {
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
			skill.PackageURL = ""
			result = SkillAcquireResult{Skill: skill, Acquired: true, Charged: 0}
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		row := model.UserSkill{
			UserID:     userID,
			SkillID:    skillID,
			PaidQuota:  0,
			PackageURL: skill.PackageURL,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		skill.PackageURL = ""
		result = SkillAcquireResult{Skill: skill, Acquired: true, Charged: 0}
		return nil
	})
	if err != nil {
		return SkillAcquireResult{}, err
	}
	return result, nil
}

func BillSkillCallWithIdempotency(userID int, skillID int, idempotencyKey string) (SkillCallBillingResult, error) {
	normalizedKey, err := normalizeSkillBillingIdempotencyKey(idempotencyKey)
	if err != nil {
		return SkillCallBillingResult{}, err
	}
	if normalizedKey == "" {
		return BillSkillCall(userID, skillID)
	}
	resultKey := skillBillingIdempotencyCacheKey(userID, skillID, normalizedKey)

	if cached, ok, err := getSkillBillingIdempotencyResult(resultKey); err != nil {
		return SkillCallBillingResult{}, err
	} else if ok {
		return cached, nil
	}
	result, err := BillSkillCall(userID, skillID)
	if err != nil {
		return SkillCallBillingResult{}, err
	}
	if err := setSkillBillingIdempotencyResult(resultKey, result); err != nil {
		common.SysError(fmt.Sprintf("failed to cache skill billing idempotency result: %s", err.Error()))
	}
	return result, nil
}

func BillSkillCall(userID int, skillID int) (SkillCallBillingResult, error) {
	if userID <= 0 {
		return SkillCallBillingResult{}, NewToolAppError("invalid_request", "用户未登录")
	}
	if skillID <= 0 {
		return SkillCallBillingResult{}, NewToolAppError("invalid_request", "Skill ID 无效")
	}

	var result SkillCallBillingResult
	skillTitle := ""
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var skill struct {
			ID              int
			UserID          int
			Title           string
			Status          string
			SkillMDTokens   int
			TokenMultiplier int
		}
		if err := tx.Model(&model.Skill{}).
			Select("id, user_id, title, status, skill_md_tokens, token_multiplier").
			Where("id = ?", skillID).
			First(&skill).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return NewToolAppError("skill_not_found", "Skill 不存在")
			}
			return err
		}
		skillTitle = strings.TrimSpace(skill.Title)
		if skill.Status != model.SkillStatusPublished {
			return NewToolAppError("skill_not_available", "Skill 暂不可调用")
		}

		multiplier := normalizeSkillTokenMultiplier(skill.TokenMultiplier)
		tokens := skill.SkillMDTokens
		if tokens <= 0 {
			var skillMarkdown string
			if err := tx.Model(&model.Skill{}).
				Select("skill_markdown").
				Where("id = ?", skill.ID).
				Scan(&skillMarkdown).Error; err != nil {
				return err
			}
			skillMarkdown = strings.TrimSpace(skillMarkdown)
			if skillMarkdown != "" {
				tokens = CountTextToken(skillMarkdown, "gpt-4o")
			}
			if err := tx.Model(&model.Skill{}).Where("id = ?", skill.ID).Update("skill_md_tokens", tokens).Error; err != nil {
				return err
			}
		}
		if tokens < 0 {
			tokens = 0
		}

		result = SkillCallBillingResult{
			SkillID:         skill.ID,
			AuthorID:        skill.UserID,
			SkillMDTokens:   tokens,
			TokenMultiplier: multiplier,
			SelfCall:        skill.UserID == userID,
		}

		if skill.UserID == userID {
			return nil
		}

		maxInt := int(^uint(0) >> 1)
		if tokens > 0 && multiplier > maxInt/tokens {
			return NewToolAppError("invalid_request", "Skill 调用计费金额超出有效范围")
		}
		charge := tokens * multiplier
		result.Charged = charge
		if charge <= 0 {
			checkAcquired := tx.Model(&model.UserSkill{}).
				Where("user_id = ? AND skill_id = ?", userID, skill.ID).
				Update("updated_at", common.GetTimestamp())
			if checkAcquired.Error != nil {
				return checkAcquired.Error
			}
			if checkAcquired.RowsAffected == 0 {
				return NewToolAppError("user_skill_not_found", "请先获取该 Skill")
			}
			return nil
		}

		updateUserSkill := tx.Model(&model.UserSkill{}).
			Where("user_id = ? AND skill_id = ?", userID, skill.ID).
			Updates(map[string]interface{}{
				"paid_quota": gorm.Expr("paid_quota + ?", charge),
				"updated_at": common.GetTimestamp(),
			})
		if updateUserSkill.Error != nil {
			return updateUserSkill.Error
		}
		if updateUserSkill.RowsAffected == 0 {
			return NewToolAppError("user_skill_not_found", "请先获取该 Skill")
		}

		consume := tx.Model(&model.User{}).
			Where("id = ? AND quota >= ?", userID, charge).
			Updates(map[string]interface{}{
				"quota":         gorm.Expr("quota - ?", charge),
				"used_quota":    gorm.Expr("used_quota + ?", charge),
				"request_count": gorm.Expr("request_count + ?", 1),
			})
		if consume.Error != nil {
			return consume.Error
		}
		if consume.RowsAffected == 0 {
			return NewToolAppError("insufficient_quota", "Token 余额不足")
		}

		reward := tx.Model(&model.User{}).
			Where("id = ?", skill.UserID).
			Updates(map[string]interface{}{
				"aff_quota":   gorm.Expr("aff_quota + ?", charge),
				"aff_history": gorm.Expr("aff_history + ?", charge),
			})
		if reward.Error != nil {
			return reward.Error
		}
		if reward.RowsAffected == 0 {
			return NewToolAppError("skill_author_not_found", "Skill 作者不存在")
		}
		return nil
	})
	if err != nil {
		return SkillCallBillingResult{}, err
	}

	if result.Charged > 0 {
		if skillTitle == "" {
			skillTitle = fmt.Sprintf("%d", skillID)
		}
		model.RecordTaskBillingLog(model.RecordTaskBillingLogParams{
			UserId:    userID,
			LogType:   model.LogTypeMarketConsume,
			Content:   fmt.Sprintf("调用 Skill「%s」扣除 %d Token", skillTitle, result.Charged),
			ModelName: fmt.Sprintf("skill:%d", skillID),
			Quota:     result.Charged,
			Other: map[string]interface{}{
				"billing_type":     "skill_call",
				"skill_id":         skillID,
				"author_id":        result.AuthorID,
				"skill_md_tokens":  result.SkillMDTokens,
				"token_multiplier": result.TokenMultiplier,
			},
		})
		model.RecordTaskBillingLog(model.RecordTaskBillingLogParams{
			UserId:    result.AuthorID,
			LogType:   model.LogTypeMarketReward,
			Content:   fmt.Sprintf("Skill「%s」被调用，获得奖励 %d Token", skillTitle, result.Charged),
			ModelName: fmt.Sprintf("skill:%d", skillID),
			Quota:     result.Charged,
			Other: map[string]interface{}{
				"billing_type":     "skill_reward",
				"skill_id":         skillID,
				"caller_id":        userID,
				"skill_md_tokens":  result.SkillMDTokens,
				"token_multiplier": result.TokenMultiplier,
			},
		})
	}
	return result, nil
}

func normalizeSkillBillingIdempotencyKey(value string) (string, error) {
	key := strings.TrimSpace(value)
	if key == "" {
		return "", nil
	}
	if len(key) > 200 {
		return "", NewToolAppError("invalid_request", "Idempotency-Key 不能超过 200 个字符")
	}
	return key, nil
}

func skillBillingIdempotencyCacheKey(userID int, skillID int, idempotencyKey string) string {
	sum := sha256.Sum256([]byte(idempotencyKey))
	base := fmt.Sprintf("skill_billing_idem:%d:%d:%s", userID, skillID, hex.EncodeToString(sum[:]))
	return base + ":result"
}

func useRedisSkillBillingIdempotency() bool {
	return common.RedisEnabled && common.RDB != nil
}

func getSkillBillingIdempotencyResult(key string) (SkillCallBillingResult, bool, error) {
	if useRedisSkillBillingIdempotency() {
		raw, err := common.RDB.Get(context.Background(), key).Result()
		if errors.Is(err, redis.Nil) {
			return SkillCallBillingResult{}, false, nil
		}
		if err != nil {
			return SkillCallBillingResult{}, false, err
		}
		var result SkillCallBillingResult
		if err := json.Unmarshal([]byte(raw), &result); err != nil {
			return SkillCallBillingResult{}, false, err
		}
		return result, true, nil
	}

	now := time.Now()
	skillBillingIdempotencyMemory.Lock()
	defer skillBillingIdempotencyMemory.Unlock()
	pruneSkillBillingIdempotencyMemoryLocked(now)
	entry, ok := skillBillingIdempotencyMemory.results[key]
	if !ok {
		return SkillCallBillingResult{}, false, nil
	}
	if !entry.ExpiresAt.After(now) {
		delete(skillBillingIdempotencyMemory.results, key)
		return SkillCallBillingResult{}, false, nil
	}
	return entry.Result, true, nil
}

func setSkillBillingIdempotencyResult(key string, result SkillCallBillingResult) error {
	if useRedisSkillBillingIdempotency() {
		raw, err := json.Marshal(result)
		if err != nil {
			return err
		}
		return common.RDB.Set(context.Background(), key, raw, skillBillingIdempotencyTTL).Err()
	}

	skillBillingIdempotencyMemory.Lock()
	defer skillBillingIdempotencyMemory.Unlock()
	pruneSkillBillingIdempotencyMemoryLocked(time.Now())
	skillBillingIdempotencyMemory.results[key] = skillBillingIdempotencyMemoryEntry{
		Result:    result,
		ExpiresAt: time.Now().Add(skillBillingIdempotencyTTL),
	}
	return nil
}

func pruneSkillBillingIdempotencyMemoryLocked(now time.Time) {
	if now.Sub(skillBillingIdempotencyMemory.lastPrune) < time.Minute {
		return
	}
	skillBillingIdempotencyMemory.lastPrune = now
	for key, entry := range skillBillingIdempotencyMemory.results {
		if !entry.ExpiresAt.After(now) {
			delete(skillBillingIdempotencyMemory.results, key)
		}
	}
}

func normalizeSkillTokenMultiplier(value int) int {
	if value <= 0 {
		return 1
	}
	return value
}

func normalizeSkillContentHash(value string, fallbackContent string) string {
	hash := strings.TrimSpace(value)
	if hash != "" {
		if strings.HasPrefix(strings.ToLower(hash), "sha256:") {
			return hash
		}
		return "sha256:" + hash
	}
	sum := sha256.Sum256([]byte(fallbackContent))
	return "sha256:" + hex.EncodeToString(sum[:])
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
