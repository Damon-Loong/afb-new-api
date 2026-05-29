package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSkillStoreTestDB(t *testing.T) {
	t.Helper()

	oldDB := model.DB
	oldLogDB := model.LOG_DB

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Skill{}, &model.UserSkill{}, &model.Log{}))

	model.DB = db
	model.LOG_DB = db

	t.Cleanup(func() {
		model.DB = oldDB
		model.LOG_DB = oldLogDB
	})
}

func TestAcquireSkillRewardsAuthorPendingQuota(t *testing.T) {
	setupSkillStoreTestDB(t)

	author := model.User{Id: 10, Username: "skill_author", AffCode: "auth"}
	buyer := model.User{Id: 20, Username: "skill_buyer", Quota: 1000, AffCode: "buyr"}
	require.NoError(t, model.DB.Create(&author).Error)
	require.NoError(t, model.DB.Create(&buyer).Error)

	skill := model.Skill{
		UserID:        author.Id,
		Title:         "paid-skill",
		Description:   "A paid skill",
		PackageURL:    "https://example.test/paid.skill",
		DownloadPrice: 300,
		Visibility:    "public",
		Status:        model.SkillStatusPublished,
	}
	require.NoError(t, model.DB.Create(&skill).Error)

	result, err := AcquireSkill(buyer.Id, skill.ID)
	require.NoError(t, err)
	require.Equal(t, 300, result.Charged)
	require.True(t, result.Acquired)

	var reloadedBuyer model.User
	require.NoError(t, model.DB.First(&reloadedBuyer, buyer.Id).Error)
	require.Equal(t, 700, reloadedBuyer.Quota)

	var reloadedAuthor model.User
	require.NoError(t, model.DB.First(&reloadedAuthor, author.Id).Error)
	require.Equal(t, 300, reloadedAuthor.AffQuota)
	require.Equal(t, 300, reloadedAuthor.AffHistoryQuota)

	var userSkill model.UserSkill
	require.NoError(t, model.DB.Where("user_id = ? AND skill_id = ?", buyer.Id, skill.ID).First(&userSkill).Error)
	require.Equal(t, 300, userSkill.PaidQuota)
	require.Equal(t, skill.PackageURL, userSkill.PackageURL)
}
