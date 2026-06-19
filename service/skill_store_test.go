package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSkillStoreTestDB(t *testing.T) {
	t.Helper()

	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldRedisEnabled := common.RedisEnabled

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Skill{}, &model.UserSkill{}, &model.Log{}))

	model.DB = db
	model.LOG_DB = db
	common.RedisEnabled = false
	skillBillingIdempotencyMemory.Lock()
	skillBillingIdempotencyMemory.results = map[string]skillBillingIdempotencyMemoryEntry{}
	skillBillingIdempotencyMemory.lastPrune = time.Time{}
	skillBillingIdempotencyMemory.Unlock()

	t.Cleanup(func() {
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.RedisEnabled = oldRedisEnabled
	})
}

func TestAcquireSkillGrantsAccessWithoutCharging(t *testing.T) {
	setupSkillStoreTestDB(t)

	author := model.User{Id: 10, Username: "skill_author", AffCode: "auth"}
	buyer := model.User{Id: 20, Username: "skill_buyer", Quota: 1000, AffCode: "buyr"}
	require.NoError(t, model.DB.Create(&author).Error)
	require.NoError(t, model.DB.Create(&buyer).Error)

	skill := model.Skill{
		UserID:          author.Id,
		Title:           "paid-skill",
		Description:     "A paid skill",
		PackageURL:      "https://example.test/paid.skill",
		SkillMarkdown:   "Use this skill for pricing analysis.",
		SkillMDTokens:   8,
		TokenMultiplier: 5,
		Visibility:      "public",
		Status:          model.SkillStatusPublished,
	}
	require.NoError(t, model.DB.Create(&skill).Error)

	result, err := AcquireSkill(buyer.Id, skill.ID)
	require.NoError(t, err)
	require.Equal(t, 0, result.Charged)
	require.True(t, result.Acquired)
	require.Empty(t, result.Skill.PackageURL)
	require.Equal(t, skill.SkillMarkdown, result.Skill.SkillMarkdown)

	var reloadedBuyer model.User
	require.NoError(t, model.DB.First(&reloadedBuyer, buyer.Id).Error)
	require.Equal(t, 1000, reloadedBuyer.Quota)

	var reloadedAuthor model.User
	require.NoError(t, model.DB.First(&reloadedAuthor, author.Id).Error)
	require.Equal(t, 0, reloadedAuthor.AffQuota)
	require.Equal(t, 0, reloadedAuthor.AffHistoryQuota)

	var userSkill model.UserSkill
	require.NoError(t, model.DB.Where("user_id = ? AND skill_id = ?", buyer.Id, skill.ID).First(&userSkill).Error)
	require.Equal(t, 0, userSkill.PaidQuota)
	require.Equal(t, skill.PackageURL, userSkill.PackageURL)
}

func TestBillSkillCallChargesCallerAndRewardsAuthor(t *testing.T) {
	setupSkillStoreTestDB(t)

	author := model.User{Id: 10, Username: "skill_author", AffCode: "auth"}
	caller := model.User{Id: 20, Username: "skill_caller", Quota: 1000, AffCode: "call"}
	require.NoError(t, model.DB.Create(&author).Error)
	require.NoError(t, model.DB.Create(&caller).Error)

	skill := model.Skill{
		UserID:          author.Id,
		Title:           "callable-skill",
		Description:     "A callable skill",
		PackageURL:      "https://example.test/callable.skill",
		SkillMarkdown:   "Use this skill for pricing analysis.",
		SkillMDTokens:   8,
		TokenMultiplier: 5,
		Visibility:      "public",
		Status:          model.SkillStatusPublished,
	}
	require.NoError(t, model.DB.Create(&skill).Error)
	require.NoError(t, model.DB.Create(&model.UserSkill{UserID: caller.Id, SkillID: skill.ID, PackageURL: skill.PackageURL}).Error)

	result, err := BillSkillCall(caller.Id, skill.ID)
	require.NoError(t, err)
	require.Equal(t, 40, result.Charged)
	require.Equal(t, 8, result.SkillMDTokens)
	require.Equal(t, 5, result.TokenMultiplier)

	var reloadedCaller model.User
	require.NoError(t, model.DB.First(&reloadedCaller, caller.Id).Error)
	require.Equal(t, 960, reloadedCaller.Quota)
	require.Equal(t, 40, reloadedCaller.UsedQuota)
	require.Equal(t, 1, reloadedCaller.RequestCount)

	var reloadedAuthor model.User
	require.NoError(t, model.DB.First(&reloadedAuthor, author.Id).Error)
	require.Equal(t, 40, reloadedAuthor.AffQuota)
	require.Equal(t, 40, reloadedAuthor.AffHistoryQuota)

	var userSkill model.UserSkill
	require.NoError(t, model.DB.Where("user_id = ? AND skill_id = ?", caller.Id, skill.ID).First(&userSkill).Error)
	require.Equal(t, 40, userSkill.PaidQuota)
}

func TestBillSkillCallWithIdempotencyReturnsCachedResult(t *testing.T) {
	setupSkillStoreTestDB(t)

	author := model.User{Id: 10, Username: "skill_author", AffCode: "auth"}
	caller := model.User{Id: 20, Username: "skill_caller", Quota: 1000, AffCode: "call"}
	require.NoError(t, model.DB.Create(&author).Error)
	require.NoError(t, model.DB.Create(&caller).Error)

	skill := model.Skill{
		UserID:          author.Id,
		Title:           "idempotent-skill",
		Description:     "An idempotent skill",
		PackageURL:      "https://example.test/idempotent.skill",
		SkillMarkdown:   "Use this skill for pricing analysis.",
		SkillMDTokens:   8,
		TokenMultiplier: 5,
		Visibility:      "public",
		Status:          model.SkillStatusPublished,
	}
	require.NoError(t, model.DB.Create(&skill).Error)
	require.NoError(t, model.DB.Create(&model.UserSkill{UserID: caller.Id, SkillID: skill.ID, PackageURL: skill.PackageURL}).Error)

	first, err := BillSkillCallWithIdempotency(caller.Id, skill.ID, "request-123")
	require.NoError(t, err)
	second, err := BillSkillCallWithIdempotency(caller.Id, skill.ID, "request-123")
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Equal(t, 40, second.Charged)

	var reloadedCaller model.User
	require.NoError(t, model.DB.First(&reloadedCaller, caller.Id).Error)
	require.Equal(t, 960, reloadedCaller.Quota)
	require.Equal(t, 40, reloadedCaller.UsedQuota)
	require.Equal(t, 1, reloadedCaller.RequestCount)

	var reloadedAuthor model.User
	require.NoError(t, model.DB.First(&reloadedAuthor, author.Id).Error)
	require.Equal(t, 40, reloadedAuthor.AffQuota)
	require.Equal(t, 40, reloadedAuthor.AffHistoryQuota)

	var userSkill model.UserSkill
	require.NoError(t, model.DB.Where("user_id = ? AND skill_id = ?", caller.Id, skill.ID).First(&userSkill).Error)
	require.Equal(t, 40, userSkill.PaidQuota)
}

func TestBillSkillCallWithEmptyIdempotencyKeyChargesEveryTime(t *testing.T) {
	setupSkillStoreTestDB(t)

	author := model.User{Id: 10, Username: "skill_author", AffCode: "auth"}
	caller := model.User{Id: 20, Username: "skill_caller", Quota: 1000, AffCode: "call"}
	require.NoError(t, model.DB.Create(&author).Error)
	require.NoError(t, model.DB.Create(&caller).Error)

	skill := model.Skill{
		UserID:          author.Id,
		Title:           "non-idempotent-skill",
		Description:     "A non-idempotent skill",
		PackageURL:      "https://example.test/non-idempotent.skill",
		SkillMarkdown:   "Use this skill for pricing analysis.",
		SkillMDTokens:   8,
		TokenMultiplier: 5,
		Visibility:      "public",
		Status:          model.SkillStatusPublished,
	}
	require.NoError(t, model.DB.Create(&skill).Error)
	require.NoError(t, model.DB.Create(&model.UserSkill{UserID: caller.Id, SkillID: skill.ID, PackageURL: skill.PackageURL}).Error)

	first, err := BillSkillCallWithIdempotency(caller.Id, skill.ID, "")
	require.NoError(t, err)
	second, err := BillSkillCallWithIdempotency(caller.Id, skill.ID, "")
	require.NoError(t, err)
	require.Equal(t, 40, first.Charged)
	require.Equal(t, 40, second.Charged)

	var reloadedCaller model.User
	require.NoError(t, model.DB.First(&reloadedCaller, caller.Id).Error)
	require.Equal(t, 920, reloadedCaller.Quota)
	require.Equal(t, 80, reloadedCaller.UsedQuota)
	require.Equal(t, 2, reloadedCaller.RequestCount)

	var reloadedAuthor model.User
	require.NoError(t, model.DB.First(&reloadedAuthor, author.Id).Error)
	require.Equal(t, 80, reloadedAuthor.AffQuota)
	require.Equal(t, 80, reloadedAuthor.AffHistoryQuota)

	var userSkill model.UserSkill
	require.NoError(t, model.DB.Where("user_id = ? AND skill_id = ?", caller.Id, skill.ID).First(&userSkill).Error)
	require.Equal(t, 80, userSkill.PaidQuota)
}

func TestBillSkillCallRejectsInsufficientQuota(t *testing.T) {
	setupSkillStoreTestDB(t)

	author := model.User{Id: 10, Username: "skill_author", AffCode: "auth"}
	caller := model.User{Id: 20, Username: "skill_caller", Quota: 39, AffCode: "call"}
	require.NoError(t, model.DB.Create(&author).Error)
	require.NoError(t, model.DB.Create(&caller).Error)

	skill := model.Skill{
		UserID:          author.Id,
		Title:           "expensive-skill",
		Description:     "An expensive skill",
		PackageURL:      "https://example.test/expensive.skill",
		SkillMarkdown:   "Use this skill for pricing analysis.",
		SkillMDTokens:   8,
		TokenMultiplier: 5,
		Visibility:      "public",
		Status:          model.SkillStatusPublished,
	}
	require.NoError(t, model.DB.Create(&skill).Error)
	require.NoError(t, model.DB.Create(&model.UserSkill{UserID: caller.Id, SkillID: skill.ID, PackageURL: skill.PackageURL}).Error)

	_, err := BillSkillCall(caller.Id, skill.ID)
	require.Error(t, err)
	var appErr *ToolAppError
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, "insufficient_quota", appErr.Code)

	var reloadedCaller model.User
	require.NoError(t, model.DB.First(&reloadedCaller, caller.Id).Error)
	require.Equal(t, 39, reloadedCaller.Quota)

	var userSkill model.UserSkill
	require.NoError(t, model.DB.Where("user_id = ? AND skill_id = ?", caller.Id, skill.ID).First(&userSkill).Error)
	require.Equal(t, 0, userSkill.PaidQuota)
}

func TestCreateSkillRejectsDuplicateTitle(t *testing.T) {
	setupSkillStoreTestDB(t)

	author := model.User{Id: 10, Username: "skill_author", AffCode: "auth"}
	require.NoError(t, model.DB.Create(&author).Error)

	created, err := CreateSkill(SkillCreateOptions{
		UserID:          author.Id,
		Title:           "Demo Skill",
		Description:     "A demo skill",
		PackageURL:      "https://example.test/demo.skill",
		ContentHash:     "sha256:demo",
		TokenMultiplier: 2,
		SkillMarkdown:   "Use this skill for demo analysis.",
		Visibility:      "public",
		Publish:         true,
	})
	require.NoError(t, err)
	require.Equal(t, 2, created.TokenMultiplier)
	require.Equal(t, "sha256:demo", created.ContentHash)
	require.Greater(t, created.SkillMDTokens, 0)

	_, err = CreateSkill(SkillCreateOptions{
		UserID:          author.Id,
		Title:           "  demo skill  ",
		Description:     "Another demo skill",
		PackageURL:      "https://example.test/demo-2.skill",
		TokenMultiplier: 1,
		SkillMarkdown:   "Use this skill for another demo.",
		Visibility:      "public",
		Publish:         true,
	})
	require.Error(t, err)
	var appErr *ToolAppError
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, "skill_name_conflict", appErr.Code)
}

func TestListAcquiredSkillDetailsReturnsPackageURL(t *testing.T) {
	setupSkillStoreTestDB(t)

	user := model.User{Id: 20, Username: "skill_user", AffCode: "user"}
	require.NoError(t, model.DB.Create(&user).Error)
	acquiredSkill := model.Skill{
		UserID:          10,
		Title:           "acquired-skill",
		Description:     "An acquired skill",
		PackageURL:      "https://example.test/acquired.skill",
		ContentHash:     "sha256:acquired",
		SkillMarkdown:   "Use this acquired skill.",
		SkillMDTokens:   5,
		TokenMultiplier: 2,
		Visibility:      "public",
		Status:          model.SkillStatusPublished,
	}
	otherSkill := model.Skill{
		UserID:          10,
		Title:           "other-skill",
		Description:     "Another skill",
		PackageURL:      "https://example.test/other.skill",
		ContentHash:     "sha256:other",
		SkillMarkdown:   "Use this other skill.",
		SkillMDTokens:   4,
		TokenMultiplier: 3,
		Visibility:      "public",
		Status:          model.SkillStatusPublished,
	}
	draftSkill := model.Skill{
		UserID:          10,
		Title:           "draft-skill",
		Description:     "A draft skill",
		PackageURL:      "https://example.test/draft.skill",
		ContentHash:     "sha256:draft",
		SkillMarkdown:   "Use this draft skill.",
		SkillMDTokens:   3,
		TokenMultiplier: 4,
		Visibility:      "public",
		Status:          model.SkillStatusDraft,
	}
	require.NoError(t, model.DB.Create(&acquiredSkill).Error)
	require.NoError(t, model.DB.Create(&otherSkill).Error)
	require.NoError(t, model.DB.Create(&draftSkill).Error)
	require.NoError(t, model.DB.Create(&model.UserSkill{UserID: user.Id, SkillID: acquiredSkill.ID, PackageURL: acquiredSkill.PackageURL}).Error)
	require.NoError(t, model.DB.Create(&model.UserSkill{UserID: user.Id, SkillID: draftSkill.ID, PackageURL: draftSkill.PackageURL}).Error)

	result, err := ListAcquiredSkillDetails(user.Id)
	require.NoError(t, err)
	require.Len(t, result.Skills, 1)
	require.Equal(t, acquiredSkill.ID, result.Skills[0].ID)
	require.Equal(t, acquiredSkill.PackageURL, result.Skills[0].PackageURL)
	require.Equal(t, "sha256:acquired", result.Skills[0].ContentHash)
	require.Equal(t, 5, result.Skills[0].SkillMDTokens)
	require.Equal(t, 2, result.Skills[0].TokenMultiplier)
}
