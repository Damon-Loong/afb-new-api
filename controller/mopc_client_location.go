package controller

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const mopcClientLocationCacheTTL = 10 * time.Minute

var (
	mopcClientLocationMu    sync.Mutex
	mopcClientLocationCache = map[string]mopcClientLocationCacheEntry{}
	mopcClientLocationHTTP  = &http.Client{Timeout: 2 * time.Second}
)

type mopcClientLocationCacheEntry struct {
	expiresAt time.Time
	data      gin.H
}

type ipAPIResponse struct {
	Status     string `json:"status"`
	Query      string `json:"query"`
	Country    string `json:"country"`
	Region     string `json:"region"`
	RegionName string `json:"regionName"`
	City       string `json:"city"`
	Message    string `json:"message"`
}

func GetMopcClientLocation(c *gin.Context) {
	ip := strings.TrimSpace(c.ClientIP())
	if ip == "" {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{}})
		return
	}

	if cached, ok := getMopcClientLocationCache(ip); ok {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": cached})
		return
	}

	data := gin.H{"ip": ip}
	if parsed := net.ParseIP(ip); parsed == nil || parsed.IsPrivate() || parsed.IsLoopback() || parsed.IsUnspecified() {
		setMopcClientLocationCache(ip, data)
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": data})
		return
	}

	location, err := lookupMopcClientLocation(ip)
	if err == nil {
		data = location
	}
	setMopcClientLocationCache(ip, data)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": data})
}

func lookupMopcClientLocation(ip string) (gin.H, error) {
	endpoint := fmt.Sprintf(
		"http://ip-api.com/json/%s?fields=status,message,country,region,regionName,city,query&lang=zh-CN",
		url.PathEscape(ip),
	)
	resp, err := mopcClientLocationHTTP.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ip location lookup failed: %s", resp.Status)
	}

	var payload ipAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if payload.Status != "success" {
		return nil, fmt.Errorf("ip location lookup failed: %s", payload.Message)
	}

	return gin.H{
		"ip":       firstNonEmpty(payload.Query, ip),
		"country":  payload.Country,
		"region":   payload.Region,
		"province": payload.RegionName,
		"city":     payload.City,
	}, nil
}

func getMopcClientLocationCache(ip string) (gin.H, bool) {
	now := time.Now()
	mopcClientLocationMu.Lock()
	defer mopcClientLocationMu.Unlock()
	entry, ok := mopcClientLocationCache[ip]
	if !ok || now.After(entry.expiresAt) {
		if ok {
			delete(mopcClientLocationCache, ip)
		}
		return nil, false
	}
	return entry.data, true
}

func setMopcClientLocationCache(ip string, data gin.H) {
	mopcClientLocationMu.Lock()
	defer mopcClientLocationMu.Unlock()
	mopcClientLocationCache[ip] = mopcClientLocationCacheEntry{
		expiresAt: time.Now().Add(mopcClientLocationCacheTTL),
		data:      data,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
