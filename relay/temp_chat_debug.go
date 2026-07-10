package relay

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/gin-gonic/gin"
)

const tempChatDebugPrefix = "[TEMP CHAT DEBUG]"

func logTempChatRequestBody(c *gin.Context) {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		logger.LogInfo(c, fmt.Sprintf("%s downstream request body read failed: %v", tempChatDebugPrefix, err))
		return
	}
	body, err := storage.Bytes()
	if err != nil {
		logger.LogInfo(c, fmt.Sprintf("%s downstream request body bytes failed: %v", tempChatDebugPrefix, err))
		return
	}
	logger.LogInfo(c, fmt.Sprintf("%s downstream request body: %s", tempChatDebugPrefix, string(body)))
}

func logTempChatUpstreamBody(c *gin.Context, body []byte) {
	logger.LogInfo(c, fmt.Sprintf("%s upstream request body: %s", tempChatDebugPrefix, string(body)))
}

func logTempChatUpstreamBodyFromStorage(c *gin.Context, storage common.BodyStorage) {
	if storage == nil {
		return
	}
	body, err := storage.Bytes()
	if err != nil {
		logger.LogInfo(c, fmt.Sprintf("%s upstream request body bytes failed: %v", tempChatDebugPrefix, err))
		return
	}
	if _, err := storage.Seek(0, 0); err != nil {
		logger.LogInfo(c, fmt.Sprintf("%s upstream request body seek failed: %v", tempChatDebugPrefix, err))
	}
	logTempChatUpstreamBody(c, body)
}
