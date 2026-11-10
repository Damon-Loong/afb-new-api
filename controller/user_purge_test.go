package controller

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestPurgeCancelledUser(t *testing.T) {
	db, _ := setupSMSAuthTest(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	cancelled := gorm.DeletedAt{Time: time.Now(), Valid: true}
	for _, u := range []model.User{
		{Id: 81, Username: "cancelled", Role: common.RoleCommonUser, AffCode: "purge81", DeletedAt: cancelled},
		{Id: 82, Username: "active", Role: common.RoleCommonUser, AffCode: "purge82"},
		{Id: 83, Username: "admin", Role: common.RoleAdminUser, AffCode: "purge83", DeletedAt: cancelled},
		{Id: 84, Username: "root", Role: common.RoleRootUser, AffCode: "purge84", DeletedAt: cancelled},
	} {
		require.NoError(t, db.Create(&u).Error)
	}
	require.NoError(t, db.Create(&model.Log{UserId: 81, Username: "cancelled", Quota: 100}).Error)
	for _, tc := range []struct {
		id   string
		role int
		ok   bool
	}{
		{"81", common.RoleCommonUser, false},
		{"82", common.RoleRootUser, false},
		{"83", common.RoleAdminUser, false},
		{"84", common.RoleRootUser, false},
		{"-1", common.RoleRootUser, false},
		{"999", common.RoleRootUser, false},
		{"81", common.RoleAdminUser, true},
		{"81", common.RoleAdminUser, false},
		{"83", common.RoleRootUser, true},
	} {
		r := gin.New()
		r.DELETE("/users/:id", func(c *gin.Context) { c.Set("role", tc.role); c.Set("id", 1) }, PurgeCancelledUser)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("DELETE", "/users/"+tc.id, nil))
		var body struct{ Success bool }
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		require.Equal(t, tc.ok, body.Success, w.Body.String())
	}
	var count int64
	require.NoError(t, db.Unscoped().Model(&model.User{}).Where("id IN ?", []int{81, 83}).Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, db.Model(&model.Log{}).Where("user_id = ?", 81).Count(&count).Error)
	require.EqualValues(t, 1, count)
}
