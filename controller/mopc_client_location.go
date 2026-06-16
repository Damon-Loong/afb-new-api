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
	Ret  int `json:"ret"`
	Data struct {
		IP      string `json:"ip"`
		Country string `json:"country"`
		Prov    string `json:"prov"`
		City    string `json:"city"`
		Area    string `json:"area"`
		ISP     string `json:"isp"`
	} `json:"data"`
	Msg string `json:"msg"`
}

type pconlineIPResponse struct {
	IP     string `json:"ip"`
	Pro    string `json:"pro"`
	City   string `json:"city"`
	Region string `json:"region"`
	Addr   string `json:"addr"`
	Err    string `json:"err"`
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
	if location, err := lookupMopcClientLocationWithIP9(ip); err == nil {
		return location, nil
	}
	return lookupMopcClientLocationWithPConline(ip)
}

func lookupMopcClientLocationWithIP9(ip string) (gin.H, error) {
	endpoint := fmt.Sprintf(
		"https://ip9.com.cn/get?ip=%s",
		url.QueryEscape(ip),
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
	if payload.Ret != 200 {
		return nil, fmt.Errorf("ip9 location lookup failed: %s", payload.Msg)
	}

	return gin.H{
		"ip":       firstNonEmpty(payload.Data.IP, ip),
		"country":  payload.Data.Country,
		"province": payload.Data.Prov,
		"city":     payload.Data.City,
		"region":   payload.Data.Area,
		"isp":      payload.Data.ISP,
	}, nil
}

func lookupMopcClientLocationWithPConline(ip string) (gin.H, error) {
	endpoint := fmt.Sprintf(
		"https://whois.pconline.com.cn/ipJson.jsp?ip=%s&json=true",
		url.QueryEscape(ip),
	)
	resp, err := mopcClientLocationHTTP.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("pconline location lookup failed: %s", resp.Status)
	}

	var payload pconlineIPResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if strings.TrimSpace(payload.Err) != "" {
		return nil, fmt.Errorf("pconline location lookup failed: %s", payload.Err)
	}

	return gin.H{
		"ip":       firstNonEmpty(payload.IP, ip),
		"country":  "中国",
		"region":   payload.Region,
		"province": payload.Pro,
		"city":     payload.City,
		"address":  payload.Addr,
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
