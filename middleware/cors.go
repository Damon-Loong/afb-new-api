package middleware

import (
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

var (
	corsOriginOnce sync.Once
	corsOrigins    map[string]struct{}
	corsAllowAll   bool
)

func CORS() gin.HandlerFunc {
	loadCorsOrigins()
	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if origin != "" && isCorsOriginAllowed(origin) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			if requestedHeaders := strings.TrimSpace(c.GetHeader("Access-Control-Request-Headers")); requestedHeaders != "" {
				c.Header("Access-Control-Allow-Headers", requestedHeaders)
			} else {
				c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, New-API-User, x-api-key, anthropic-version, x-goog-api-key")
			}
			c.Header("Access-Control-Max-Age", "86400")
			c.Header("Vary", "Origin, Access-Control-Request-Method, Access-Control-Request-Headers")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func loadCorsOrigins() {
	corsOriginOnce.Do(func() {
		corsOrigins = make(map[string]struct{})
		rawOrigins := strings.TrimSpace(os.Getenv("CORS_ALLOW_ORIGINS"))
		if rawOrigins == "" || rawOrigins == "*" {
			corsAllowAll = true
			return
		}
		for _, item := range strings.Split(rawOrigins, ",") {
			origin := strings.TrimSpace(item)
			if origin == "" {
				continue
			}
			if origin == "*" {
				corsAllowAll = true
				return
			}
			corsOrigins[strings.TrimRight(origin, "/")] = struct{}{}
		}
	})
}

func isCorsOriginAllowed(origin string) bool {
	if corsAllowAll {
		return true
	}
	_, ok := corsOrigins[strings.TrimRight(origin, "/")]
	return ok
}

func PoweredBy() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-New-Api-Version", common.Version)
		c.Next()
	}
}
