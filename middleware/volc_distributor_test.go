package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsSingleKeyVolcChannel(t *testing.T) {
	assert.True(t, isSingleKeyVolcChannel(&model.Channel{Type: constant.ChannelTypeDoubaoVideo, Key: "key-a"}))
	assert.True(t, isSingleKeyVolcChannel(&model.Channel{Type: constant.ChannelTypeVolcEngine, Key: "key-a"}))
	assert.False(t, isSingleKeyVolcChannel(&model.Channel{Type: constant.ChannelTypeDoubaoVideo, Key: "key-a\nkey-b"}))
	assert.False(t, isSingleKeyVolcChannel(&model.Channel{Type: constant.ChannelTypeDoubaoVideo, Key: "key-a", ChannelInfo: model.ChannelInfo{IsMultiKey: true}}))
	assert.False(t, isSingleKeyVolcChannel(&model.Channel{Type: constant.ChannelTypeOpenAI, Key: "key-a"}))
}

func TestSetupContextForSelectedChannelUsesTaskKeyFingerprint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	channel := &model.Channel{
		Id:   12,
		Type: constant.ChannelTypeDoubaoVideo,
		Key:  "key-a\nkey-b",
		ChannelInfo: model.ChannelInfo{
			IsMultiKey: true,
		},
	}
	common.SetContextKey(c, constant.ContextKeyTaskChannelKeyFingerprint, model.ChannelKeyFingerprint("key-b"))

	err := SetupContextForSelectedChannel(c, channel, "doubao-seedance-2-0-260128")
	require.Nil(t, err)
	assert.Equal(t, "key-b", common.GetContextKeyString(c, constant.ContextKeyChannelKey))
	assert.Equal(t, 1, common.GetContextKeyInt(c, constant.ContextKeyChannelMultiKeyIndex))
}
