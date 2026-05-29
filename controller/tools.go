package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func toolAPIError(c *gin.Context, err error) {
	var appErr *service.ToolAppError
	if errors.As(err, &appErr) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"code":    appErr.Code,
			"message": appErr.Message,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"code":    "invalid_request",
		"message": err.Error(),
	})
}

func toolAPISuccess(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}

func parseNonNegativeInt(value string) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0
	}
	return parsed
}

func GetTools(c *gin.Context) {
	createdBy := 0
	if c.Query("scope") == "mine" {
		createdBy = c.GetInt("id")
		if createdBy <= 0 {
			createdBy = -1
		}
	}
	index, err := service.ListToolsWithOptions(service.ToolListOptions{
		Keyword:      c.Query("keyword"),
		Category:     c.Query("category"),
		UserID:       c.GetInt("id"),
		CreatedBy:    createdBy,
		AcquiredOnly: c.Query("acquired") == "true",
		Limit:        parseNonNegativeInt(c.Query("limit")),
		Offset:       parseNonNegativeInt(c.Query("offset")),
	})
	if err != nil {
		toolAPIError(c, err)
		return
	}
	toolAPISuccess(c, index)
}

func GetTool(c *gin.Context) {
	detail, err := service.GetToolDetail(c.Param("tool_id"))
	if err != nil {
		toolAPIError(c, err)
		return
	}
	detail.CanEdit = service.CanEditTool(detail, c.GetInt("id"), c.GetInt("role") >= common.RoleAdminUser)
	toolAPISuccess(c, detail)
}

func ParseToolOpenAPI(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		toolAPIError(c, service.NewToolAppError("invalid_request", "请选择要上传的 OpenAPI 文件"))
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		toolAPIError(c, service.NewToolAppError("invalid_request", "读取上传文件失败"))
		return
	}
	defer file.Close()
	result, err := service.ParseOpenAPIUpload(fileHeader.Filename, file, fileHeader.Size)
	if err != nil {
		toolAPIError(c, err)
		return
	}
	toolAPISuccess(c, result)
}

func UploadToolOpenAPI(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		toolAPIError(c, service.NewToolAppError("invalid_request", "请选择要上传的 OpenAPI 文件"))
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		toolAPIError(c, service.NewToolAppError("invalid_request", "读取上传文件失败"))
		return
	}
	defer file.Close()
	opts := service.ToolUploadOptions{
		Category:       c.PostForm("category"),
		Visibility:     c.PostForm("visibility"),
		Publish:        c.DefaultPostForm("publish", "true") != "false",
		AuthType:       c.PostForm("auth_type"),
		APIKeyLocation: c.PostForm("api_key_location"),
		APIKeyName:     c.PostForm("api_key_name"),
		APIKeyValue:    c.PostForm("api_key_value"),
		CreatedBy:      c.GetInt("id"),
		CallPrice:      parseNonNegativeInt(c.PostForm("call_price")),
	}
	detail, err := service.UploadTool(fileHeader.Filename, file, fileHeader.Size, opts)
	if err != nil {
		toolAPIError(c, err)
		return
	}
	toolAPISuccess(c, gin.H{
		"id":              detail.ID,
		"name":            detail.Name,
		"call_price":      detail.CallPrice,
		"source_format":   detail.SourceFormat,
		"openapi_version": detail.OpenAPIVersion,
		"server_url":      detail.ServerURL,
		"action_count":    detail.ActionCount,
		"warnings":        detail.Warnings,
	})
}

func UploadSkillPackage(c *gin.Context) {
	var req struct {
		Name          string `json:"name"`
		Description   string `json:"description"`
		PackageURL    string `json:"package_url"`
		SkillMarkdown string `json:"skill_md"`
		DownloadPrice int    `json:"download_price"`
		PromotionMode string `json:"promotion_mode"`
		Visibility    string `json:"visibility"`
		Publish       *bool  `json:"publish"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		toolAPIError(c, service.NewToolAppError("invalid_request", "Skill 发布参数无效"))
		return
	}
	publish := true
	if req.Publish != nil {
		publish = *req.Publish
	}
	skill, err := service.CreateSkill(service.SkillCreateOptions{
		UserID:        c.GetInt("id"),
		Title:         req.Name,
		Description:   req.Description,
		PackageURL:    req.PackageURL,
		SkillMarkdown: req.SkillMarkdown,
		DownloadPrice: req.DownloadPrice,
		PromotionMode: req.PromotionMode,
		Visibility:    req.Visibility,
		Publish:       publish,
	})
	if err != nil {
		toolAPIError(c, err)
		return
	}
	toolAPISuccess(c, skill)
}

func GetMySkills(c *gin.Context) {
	skills, err := service.ListUserSkills(c.GetInt("id"))
	if err != nil {
		toolAPIError(c, err)
		return
	}
	toolAPISuccess(c, gin.H{"skills": skills})
}

func GetPublicSkills(c *gin.Context) {
	result, err := service.ListPublicSkillsWithOptions(service.SkillListOptions{
		UserID:       c.GetInt("id"),
		Keyword:      c.Query("keyword"),
		AcquiredOnly: c.Query("acquired") == "true",
		Limit:        parseNonNegativeInt(c.Query("limit")),
		Offset:       parseNonNegativeInt(c.Query("offset")),
	})
	if err != nil {
		toolAPIError(c, err)
		return
	}
	toolAPISuccess(c, result)
}

func AcquireSkill(c *gin.Context) {
	skillID, err := strconv.Atoi(c.Param("skill_id"))
	if err != nil {
		toolAPIError(c, service.NewToolAppError("invalid_request", "Skill ID 无效"))
		return
	}
	result, err := service.AcquireSkill(c.GetInt("id"), skillID)
	if err != nil {
		toolAPIError(c, err)
		return
	}
	toolAPISuccess(c, result)
}

func CreateManualTool(c *gin.Context) {
	var req struct {
		Name           string               `json:"name"`
		Description    string               `json:"description"`
		ServerURL      string               `json:"server_url"`
		Category       string               `json:"category"`
		Visibility     string               `json:"visibility"`
		Publish        *bool                `json:"publish"`
		AuthType       string               `json:"auth_type"`
		APIKeyLocation string               `json:"api_key_location"`
		APIKeyName     string               `json:"api_key_name"`
		APIKeyValue    string               `json:"api_key_value"`
		CommonHeaders  []service.ToolHeader `json:"common_headers"`
		CallPrice      int                  `json:"call_price"`
		Action         struct {
			DisplayName  string         `json:"display_name"`
			Description  string         `json:"description"`
			OperationID  string         `json:"operation_id"`
			Method       string         `json:"method"`
			Path         string         `json:"path"`
			InputSchema  map[string]any `json:"input_schema"`
			OutputSchema any            `json:"output_schema"`
			Enabled      *bool          `json:"enabled"`
			RiskLevel    string         `json:"risk_level"`
		} `json:"action"`
		Actions []struct {
			DisplayName  string         `json:"display_name"`
			Description  string         `json:"description"`
			OperationID  string         `json:"operation_id"`
			Method       string         `json:"method"`
			Path         string         `json:"path"`
			InputSchema  map[string]any `json:"input_schema"`
			OutputSchema any            `json:"output_schema"`
			Enabled      *bool          `json:"enabled"`
			RiskLevel    string         `json:"risk_level"`
		} `json:"actions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		toolAPIError(c, service.NewToolAppError("invalid_request", "手动创建工具参数无效"))
		return
	}
	publish := true
	if req.Publish != nil {
		publish = *req.Publish
	}
	enabled := true
	if req.Action.Enabled != nil {
		enabled = *req.Action.Enabled
	}
	actions := make([]service.ToolManualActionOptions, 0, len(req.Actions))
	for _, action := range req.Actions {
		actionEnabled := true
		if action.Enabled != nil {
			actionEnabled = *action.Enabled
		}
		actions = append(actions, service.ToolManualActionOptions{
			DisplayName:  action.DisplayName,
			Description:  action.Description,
			OperationID:  action.OperationID,
			Method:       action.Method,
			Path:         action.Path,
			InputSchema:  action.InputSchema,
			OutputSchema: action.OutputSchema,
			Enabled:      actionEnabled,
			RiskLevel:    action.RiskLevel,
		})
	}
	detail, err := service.CreateManualTool(service.ToolManualCreateOptions{
		Name:           req.Name,
		Description:    req.Description,
		ServerURL:      req.ServerURL,
		Category:       req.Category,
		Visibility:     req.Visibility,
		Publish:        publish,
		AuthType:       req.AuthType,
		APIKeyLocation: req.APIKeyLocation,
		APIKeyName:     req.APIKeyName,
		APIKeyValue:    req.APIKeyValue,
		CreatedBy:      c.GetInt("id"),
		CommonHeaders:  req.CommonHeaders,
		CallPrice:      req.CallPrice,
		Action: service.ToolManualActionOptions{
			DisplayName:  req.Action.DisplayName,
			Description:  req.Action.Description,
			OperationID:  req.Action.OperationID,
			Method:       req.Action.Method,
			Path:         req.Action.Path,
			InputSchema:  req.Action.InputSchema,
			OutputSchema: req.Action.OutputSchema,
			Enabled:      enabled,
			RiskLevel:    req.Action.RiskLevel,
		},
		Actions: actions,
	})
	if err != nil {
		toolAPIError(c, err)
		return
	}
	toolAPISuccess(c, gin.H{
		"id":              detail.ID,
		"name":            detail.Name,
		"call_price":      detail.CallPrice,
		"source_format":   detail.SourceFormat,
		"openapi_version": detail.OpenAPIVersion,
		"server_url":      detail.ServerURL,
		"action_count":    detail.ActionCount,
		"warnings":        detail.Warnings,
	})
}

func ImportTool(c *gin.Context) {
	var req struct {
		Name          string               `json:"name"`
		Description   string               `json:"description"`
		SourceURL     string               `json:"source_url"`
		ServerURL     string               `json:"server_url"`
		Category      string               `json:"category"`
		Visibility    string               `json:"visibility"`
		Publish       *bool                `json:"publish"`
		CommonHeaders []service.ToolHeader `json:"common_headers"`
		CallPrice     int                  `json:"call_price"`
		Actions       []struct {
			DisplayName  string         `json:"display_name"`
			Description  string         `json:"description"`
			OperationID  string         `json:"operation_id"`
			Method       string         `json:"method"`
			Path         string         `json:"path"`
			InputSchema  map[string]any `json:"input_schema"`
			OutputSchema any            `json:"output_schema"`
			Enabled      *bool          `json:"enabled"`
			RiskLevel    string         `json:"risk_level"`
		} `json:"actions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		toolAPIError(c, service.NewToolAppError("invalid_request", "工具导入参数无效"))
		return
	}
	publish := true
	if req.Publish != nil {
		publish = *req.Publish
	}
	actions := make([]service.ToolManualActionOptions, 0, len(req.Actions))
	for _, action := range req.Actions {
		actionEnabled := true
		if action.Enabled != nil {
			actionEnabled = *action.Enabled
		}
		actions = append(actions, service.ToolManualActionOptions{
			DisplayName:  action.DisplayName,
			Description:  action.Description,
			OperationID:  action.OperationID,
			Method:       action.Method,
			Path:         action.Path,
			InputSchema:  action.InputSchema,
			OutputSchema: action.OutputSchema,
			Enabled:      actionEnabled,
			RiskLevel:    action.RiskLevel,
		})
	}
	detail, err := service.CreateManualTool(service.ToolManualCreateOptions{
		Name:          req.Name,
		Description:   req.Description,
		ServerURL:     req.ServerURL,
		Category:      req.Category,
		Visibility:    req.Visibility,
		Publish:       publish,
		AuthType:      "none",
		CreatedBy:     c.GetInt("id"),
		CommonHeaders: req.CommonHeaders,
		SourceURL:     req.SourceURL,
		CallPrice:     req.CallPrice,
		Actions:       actions,
	})
	if err != nil {
		toolAPIError(c, err)
		return
	}
	toolAPISuccess(c, gin.H{
		"id":              detail.ID,
		"name":            detail.Name,
		"call_price":      detail.CallPrice,
		"source_format":   detail.SourceFormat,
		"openapi_version": detail.OpenAPIVersion,
		"server_url":      detail.ServerURL,
		"source_url":      detail.SourceURL,
		"action_count":    detail.ActionCount,
		"warnings":        detail.Warnings,
	})
}

func UpdateToolConfig(c *gin.Context) {
	var req struct {
		Name           string               `json:"name"`
		Description    string               `json:"description"`
		ServerURL      string               `json:"server_url"`
		Category       string               `json:"category"`
		Visibility     string               `json:"visibility"`
		AuthType       string               `json:"auth_type"`
		APIKeyLocation string               `json:"api_key_location"`
		APIKeyName     string               `json:"api_key_name"`
		APIKeyValue    string               `json:"api_key_value"`
		CommonHeaders  []service.ToolHeader `json:"common_headers"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		toolAPIError(c, service.NewToolAppError("invalid_request", "工具配置参数无效"))
		return
	}
	detail, err := service.UpdateToolConfig(c.Param("tool_id"), service.ToolUpdateConfigOptions{
		UserID:         c.GetInt("id"),
		IsAdmin:        c.GetInt("role") >= common.RoleAdminUser,
		Name:           req.Name,
		Description:    req.Description,
		ServerURL:      req.ServerURL,
		Category:       req.Category,
		Visibility:     req.Visibility,
		AuthType:       req.AuthType,
		APIKeyLocation: req.APIKeyLocation,
		APIKeyName:     req.APIKeyName,
		APIKeyValue:    req.APIKeyValue,
		CommonHeaders:  req.CommonHeaders,
	})
	if err != nil {
		toolAPIError(c, err)
		return
	}
	detail.CanEdit = true
	toolAPISuccess(c, detail)
}

func UpdateToolActionConfig(c *gin.Context) {
	var req struct {
		DisplayName  string         `json:"display_name"`
		Description  string         `json:"description"`
		OperationID  string         `json:"operation_id"`
		Method       string         `json:"method"`
		Path         string         `json:"path"`
		InputSchema  map[string]any `json:"input_schema"`
		OutputSchema any            `json:"output_schema"`
		Enabled      bool           `json:"enabled"`
		RiskLevel    string         `json:"risk_level"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		toolAPIError(c, service.NewToolAppError("invalid_request", "工具函数配置参数无效"))
		return
	}
	detail, err := service.UpdateToolActionConfig(c.Param("tool_id"), c.Param("action_id"), service.ToolActionUpdateConfigOptions{
		UserID:       c.GetInt("id"),
		IsAdmin:      c.GetInt("role") >= common.RoleAdminUser,
		DisplayName:  req.DisplayName,
		Description:  req.Description,
		OperationID:  req.OperationID,
		Method:       req.Method,
		Path:         req.Path,
		InputSchema:  req.InputSchema,
		OutputSchema: req.OutputSchema,
		Enabled:      req.Enabled,
		RiskLevel:    req.RiskLevel,
	})
	if err != nil {
		toolAPIError(c, err)
		return
	}
	detail.CanEdit = true
	toolAPISuccess(c, detail)
}

func CheckToolName(c *gin.Context) {
	result, err := service.CheckToolName(c.Query("name"))
	if err != nil {
		toolAPIError(c, err)
		return
	}
	toolAPISuccess(c, result)
}

func DownloadTool(c *gin.Context) {
	zipPath, err := service.BuildToolDownload(c.Param("tool_id"))
	if err != nil {
		toolAPIError(c, err)
		return
	}
	c.FileAttachment(zipPath, "tool-"+c.Param("tool_id")+".zip")
}

func DeleteTool(c *gin.Context) {
	if err := service.DeleteTool(c.Param("tool_id")); err != nil {
		toolAPIError(c, err)
		return
	}
	toolAPISuccess(c, gin.H{"deleted": true})
}
