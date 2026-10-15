package controller

import (
	"context"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func getBillExportFilterOptions(c *gin.Context, admin bool) {
	start, startErr := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	end, endErr := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	options := service.BillExportOptions{BillExportFilter: model.BillExportFilter{StartTimestamp: start, EndTimestamp: end}, Format: "xlsx"}
	if startErr != nil || endErr != nil {
		common.ApiErrorMsg(c, "请选择有效的账单起止时间")
		return
	}
	if err := options.Validate(admin); err != nil {
		common.ApiError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	choices, err := model.GetBillExportFilterOptions(ctx, start, end, c.GetInt("id"), admin)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	common.ApiSuccess(c, choices)
}

func GetBillExportFilterOptions(c *gin.Context)     { getBillExportFilterOptions(c, true) }
func GetSelfBillExportFilterOptions(c *gin.Context) { getBillExportFilterOptions(c, false) }

func createBillExport(c *gin.Context, admin bool) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024)
	var options service.BillExportOptions
	if err := c.ShouldBindJSON(&options); err != nil {
		common.ApiErrorMsg(c, "账单导出参数无效")
		return
	}
	job, err := service.StartBillExport(c.GetInt("id"), admin, options)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	common.ApiSuccess(c, job)
}

func getBillExport(c *gin.Context, admin bool) {
	c.Header("Cache-Control", "no-store")
	job, ok := service.GetBillExportJob(c.Param("id"), c.GetInt("id"), admin)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "账单任务已过期或无权访问"})
		return
	}
	common.ApiSuccess(c, job)
}

func downloadBillExport(c *gin.Context, admin bool) {
	c.Header("Cache-Control", "no-store")
	file, job, err := service.OpenBillExport(c.Param("id"), c.GetInt("id"), admin)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "账单尚未完成、已过期或无权访问"})
		return
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		common.ApiErrorMsg(c, "无法读取账单文件")
		return
	}
	contentType := "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	if strings.HasSuffix(job.Filename, ".zip") {
		contentType = "application/zip"
	}
	c.Header("X-Content-Type-Options", "nosniff")
	// RFC 5987 encoding is handled by Go's standard MIME implementation.
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": job.Filename}))
	c.DataFromReader(http.StatusOK, stat.Size(), contentType, file, nil)
}

func CreateBillExport(c *gin.Context)       { createBillExport(c, true) }
func CreateSelfBillExport(c *gin.Context)   { createBillExport(c, false) }
func GetBillExport(c *gin.Context)          { getBillExport(c, true) }
func GetSelfBillExport(c *gin.Context)      { getBillExport(c, false) }
func DownloadBillExport(c *gin.Context)     { downloadBillExport(c, true) }
func DownloadSelfBillExport(c *gin.Context) { downloadBillExport(c, false) }
