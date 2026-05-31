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

func GetIncomeSources(c *gin.Context) {
	sources, err := service.GetIncomeSources(c.GetInt("id"))
	if err != nil {
		toolAPIError(c, err)
		return
	}
	toolAPISuccess(c, sources)
}

func GetIncomeSourceFlows(c *gin.Context) {
	flows, err := service.GetIncomeSourceFlows(
		c.GetInt("id"),
		c.Query("kind"),
		c.Query("id"),
		parseNonNegativeInt(c.Query("limit")),
	)
	if err != nil {
		toolAPIError(c, err)
		return
	}
	toolAPISuccess(c, flows)
}
