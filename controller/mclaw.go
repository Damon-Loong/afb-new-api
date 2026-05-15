package controller

import (
	"net/http"
	"path"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

type mclawDownloadLink struct {
	URL      string `json:"url"`
	Filename string `json:"filename,omitempty"`
}

func GetMClawDownload(c *gin.Context) {
	_, link, ok := getMClawDownloadLink(c)
	if !ok {
		return
	}

	downloadURL := strings.TrimSpace(link.URL)
	if !strings.HasPrefix(downloadURL, "http://") && !strings.HasPrefix(downloadURL, "https://") {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "MClaw download link is invalid",
		})
		return
	}

	if link.Filename != "" {
		c.Header("Content-Disposition", `attachment; filename="`+path.Base(link.Filename)+`"`)
	}
	c.Redirect(http.StatusFound, downloadURL)
}

func GetMClawDownloadInfo(c *gin.Context) {
	platform, link, ok := getMClawDownloadLink(c)
	if !ok {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"platform": platform,
			"filename": link.Filename,
		},
	})
}

func getMClawDownloadLink(c *gin.Context) (string, mclawDownloadLink, bool) {
	platform := strings.ToLower(strings.TrimSpace(c.Query("platform")))
	if platform == "" {
		platform = "windows"
	}
	if platform != "windows" && platform != "macos" && platform != "linux" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "unsupported platform",
		})
		return "", mclawDownloadLink{}, false
	}

	common.OptionMapRWMutex.RLock()
	raw := common.OptionMap["MClawDownloadLinks"]
	common.OptionMapRWMutex.RUnlock()

	var links map[string]mclawDownloadLink
	if strings.TrimSpace(raw) == "" || common.UnmarshalJsonStr(raw, &links) != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "MClaw download links are not configured",
		})
		return "", mclawDownloadLink{}, false
	}

	link := links[platform]
	downloadURL := strings.TrimSpace(link.URL)
	if downloadURL == "" {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "MClaw download link is not configured for this platform",
		})
		return "", mclawDownloadLink{}, false
	}

	return platform, link, true
}
