package controller

import (
	"errors"
	"net/http"

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

func GetTools(c *gin.Context) {
	index, err := service.ListTools(c.Query("keyword"))
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
	}
	detail, err := service.UploadTool(fileHeader.Filename, file, fileHeader.Size, opts)
	if err != nil {
		toolAPIError(c, err)
		return
	}
	toolAPISuccess(c, gin.H{
		"id":              detail.ID,
		"name":            detail.Name,
		"source_format":   detail.SourceFormat,
		"openapi_version": detail.OpenAPIVersion,
		"server_url":      detail.ServerURL,
		"action_count":    detail.ActionCount,
		"warnings":        detail.Warnings,
	})
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
