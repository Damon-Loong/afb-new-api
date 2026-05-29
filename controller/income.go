package controller

import (
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetIncomeSummary(c *gin.Context) {
	summary, err := service.GetIncomeSummary(c.GetInt("id"), parseNonNegativeInt(c.Query("days")))
	if err != nil {
		toolAPIError(c, err)
		return
	}
	toolAPISuccess(c, summary)
}
