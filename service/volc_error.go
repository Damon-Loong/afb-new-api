package service

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

type volcErrorBody struct {
	Error volcError `json:"error"`
}

type volcError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Param   string `json:"param"`
	Type    string `json:"type"`
}

func IsVolcContentsRequest(c *gin.Context) bool {
	return c != nil && c.Request != nil && strings.HasPrefix(c.Request.URL.Path, "/api/v3/contents/generations/tasks")
}

func WriteVolcError(c *gin.Context, statusCode int, code, message string) {
	internalCode := code
	if code == "" {
		internalCode = volcDefaultErrorCode(statusCode)
	}
	requestID := c.GetString(common.RequestIdKey)
	if requestID != "" {
		message = common.MessageWithRequestId(message, requestID)
	}
	c.Header("X-Error-Code", "NewAPI_"+internalCode)
	c.JSON(statusCode, volcErrorBody{Error: volcError{
		Code:    volcDefaultErrorCode(statusCode),
		Message: message,
		Param:   "",
		Type:    volcErrorType(statusCode),
	}})
}

func volcDefaultErrorCode(statusCode int) string {
	switch statusCode {
	case http.StatusBadRequest:
		return "InvalidRequestError"
	case http.StatusUnauthorized:
		return "AuthenticationError"
	case http.StatusForbidden:
		return "PermissionDenied"
	case http.StatusNotFound:
		return "NotFound"
	case http.StatusTooManyRequests:
		return "RateLimitExceeded"
	default:
		return "InternalServiceError"
	}
}

func volcErrorType(statusCode int) string {
	switch statusCode {
	case http.StatusBadRequest:
		return "BadRequest"
	case http.StatusUnauthorized:
		return "Unauthorized"
	case http.StatusForbidden:
		return "Forbidden"
	case http.StatusNotFound:
		return "NotFound"
	case http.StatusTooManyRequests:
		return "TooManyRequests"
	default:
		if statusCode >= 500 {
			return "InternalServerError"
		}
	}
	return fmt.Sprintf("HTTP%d", statusCode)
}
