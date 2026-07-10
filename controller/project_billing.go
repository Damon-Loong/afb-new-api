package controller

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const projectBillingSignatureTTL = 5 * time.Minute

type projectBillingChargeRequest struct {
	ProjectKey     string                 `json:"project_key"`
	TokenAmount    int                    `json:"token_amount"`
	IdempotencyKey string                 `json:"idempotency_key"`
	Description    string                 `json:"description"`
	Metadata       map[string]interface{} `json:"metadata"`
}

func CreateProjectBillingCharge(c *gin.Context) {
	var req projectBillingChargeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request body"})
		return
	}

	projectKey := strings.TrimSpace(req.ProjectKey)
	if projectKey == "" {
		projectKey = strings.TrimSpace(c.GetHeader("X-Project-Key"))
	}
	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	}
	if projectKey == "" || idempotencyKey == "" || req.TokenAmount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "project_key, token_amount and idempotency_key are required"})
		return
	}

	project, err := model.GetSSOProjectByKey(projectKey)
	if err != nil || project == nil || !project.Enabled {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "project is not found or disabled"})
		return
	}
	if strings.TrimSpace(project.BillingSecret) == "" {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "project billing secret is not configured"})
		return
	}
	if !verifyProjectBillingSignature(c, project.BillingSecret, projectKey, idempotencyKey, req.TokenAmount) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "invalid project billing signature"})
		return
	}

	record, replayed, err := model.ChargeProjectQuota(model.ProjectBillingChargeParams{
		ProjectKey:     projectKey,
		UserID:         c.GetInt("id"),
		UserGroup:      c.GetString("group"),
		IdempotencyKey: idempotencyKey,
		TokenAmount:    req.TokenAmount,
		Description:    req.Description,
		Metadata:       req.Metadata,
	})
	if errors.Is(err, model.ErrProjectBillingInsufficientQuota) {
		c.JSON(http.StatusPaymentRequired, gin.H{"success": false, "message": "insufficient remaining tokens"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"id":              record.ID,
			"project_key":     record.ProjectKey,
			"idempotency_key": record.IdempotencyKey,
			"token_amount":    record.TokenAmount,
			"remaining_quota": record.RemainingQuota,
			"replayed":        replayed,
		},
	})
}

func verifyProjectBillingSignature(c *gin.Context, secret string, projectKey string, idempotencyKey string, tokenAmount int) bool {
	timestampText := strings.TrimSpace(c.GetHeader("X-Project-Timestamp"))
	signature := strings.TrimSpace(c.GetHeader("X-Project-Signature"))
	if timestampText == "" || signature == "" {
		return false
	}
	timestamp, err := strconv.ParseInt(timestampText, 10, 64)
	if err != nil {
		return false
	}
	now := time.Now()
	requestTime := time.Unix(timestamp, 0)
	if requestTime.Before(now.Add(-projectBillingSignatureTTL)) || requestTime.After(now.Add(projectBillingSignatureTTL)) {
		return false
	}

	payload := fmt.Sprintf("%s\n%s\n%s\n%d", projectKey, timestampText, idempotencyKey, tokenAmount)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(strings.ToLower(signature)), []byte(expected))
}
