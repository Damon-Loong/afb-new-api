package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func TestCreateUserPhone(t *testing.T) {
	db, router := setupSMSAuthTest(t)
	router.POST("/api/user/", func(c *gin.Context) {
		c.Set("role", common.RoleRootUser)
		CreateUser(c)
	})

	for _, tc := range []struct {
		name, phone, expected string
		success               bool
	}{
		{"normalized", "13800000001", "+8613800000001", true},
		{"duplicate", "+86 138-0000-0001", "", false},
		{"invalid", "12345", "", false},
		{"empty", " ", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := callSMSAuthEndpoint(t, router, "/api/user/", map[string]any{
				"username": "phone_" + tc.name,
				"password": "test-password-123",
				"phone":    tc.phone,
			})
			if response.Success != tc.success {
				t.Fatalf("success = %v, message = %s", response.Success, response.Message)
			}
			var user model.User
			result := db.Where("username = ?", "phone_"+tc.name).Find(&user)
			if result.Error != nil {
				t.Fatal(result.Error)
			}
			if !tc.success {
				if result.RowsAffected != 0 {
					t.Fatal("invalid or duplicate phone created a user")
				}
				return
			}
			if result.RowsAffected != 1 {
				t.Fatal("user was not saved")
			}
			if tc.expected == "" {
				if user.Phone != nil {
					t.Fatal("blank phone should be NULL")
				}
			} else if user.Phone == nil || *user.Phone != tc.expected {
				t.Fatalf("phone not normalized and saved: %v", user.Phone)
			}
		})
	}
}
