package controller

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccountSecurityPolicyPermissions(t *testing.T) {
	db, _ := setupSMSAuthTest(t)
	require.NoError(t, db.Create(&model.User{Id: 8, Username: "ordinary", AffCode: "test8", Role: common.RoleCommonUser}).Error)
	require.NoError(t, db.Create(&model.User{Id: 9, Username: "admin", AffCode: "test9", Role: common.RoleAdminUser}).Error)
	for _, tc := range []struct {
		name, target string
		role         int
		allowed      bool
	}{
		{"admin global forbidden", "0", common.RoleAdminUser, false},
		{"admin peer forbidden", "9", common.RoleAdminUser, false},
		{"admin ordinary allowed", "8", common.RoleAdminUser, true},
		{"root global allowed", "0", common.RoleRootUser, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.PUT("/policy/:id", func(c *gin.Context) { c.Set("role", tc.role); c.Set("id", 1); c.Next() }, PutAccountSecurity)
			payload, err := json.Marshal(model.DefaultAccountSecurityPolicy())
			require.NoError(t, err)
			request := httptest.NewRequest("PUT", "/policy/"+tc.target, bytes.NewReader(payload))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			r.ServeHTTP(response, request)
			var result struct {
				Success bool `json:"success"`
			}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
			require.Equal(t, tc.allowed, result.Success, response.Body.String())
		})
	}
}
