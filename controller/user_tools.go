package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func InstallTool(c *gin.Context) {
	item, err := service.InstallUserTool(c.GetInt("id"), c.Param("tool_id"))
	if err != nil {
		toolAPIError(c, err)
		return
	}
	toolAPISuccess(c, item)
}

func UninstallTool(c *gin.Context) {
	if err := service.UninstallUserTool(c.GetInt("id"), c.Param("tool_id")); err != nil {
		toolAPIError(c, err)
		return
	}
	toolAPISuccess(c, gin.H{"deleted": true})
}

func GetUserTools(c *gin.Context) {
	list, err := service.ListUserTools(c.GetInt("id"))
	if err != nil {
		toolAPIError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    list,
	})
}

func RunUserToolAction(c *gin.Context) {
	var req service.ToolActionRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		toolAPIError(c, service.NewToolAppError("invalid_request", "工具调用参数无效"))
		return
	}
	result, err := service.RunUserToolAction(c.GetInt("id"), c.GetInt("role") >= common.RoleAdminUser, c.Param("tool_id"), c.Param("action_id"), req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"code":    "tool_run_failed",
			"message": err.Error(),
			"data":    result,
		})
		return
	}
	toolAPISuccess(c, result)
}
