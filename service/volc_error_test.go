package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteVolcError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v3/contents/generations/tasks/missing", nil)
	c.Set(common.RequestIdKey, "req-test")

	WriteVolcError(c, http.StatusNotFound, "invalid_request", "任务不存在")

	assert.Equal(t, http.StatusNotFound, recorder.Code)
	assert.Equal(t, "NewAPI_invalid_request", recorder.Header().Get("X-Error-Code"))
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Param   string `json:"param"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, "NotFound", body.Error.Code)
	assert.Contains(t, body.Error.Message, "任务不存在")
	assert.Contains(t, body.Error.Message, "req-test")
	assert.Equal(t, "", body.Error.Param)
	assert.Equal(t, "NotFound", body.Error.Type)
}
