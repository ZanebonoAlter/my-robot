package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"syntopica-backend/internal/platform/config"
)

func CORS(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		allowedOrigin := ""
		for _, allowed := range cfg.CORS.Origins {
			if allowed == "*" || allowed == origin {
				allowedOrigin = allowed
				break
			}
		}

		if allowedOrigin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		}

		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		// Drive allowed methods from config so PATCH/DELETE/etc. are editable per
		// deployment without code changes. Default (config.go) already includes PATCH.
		methods := cfg.CORS.Methods
		if len(methods) == 0 {
			methods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
		}
		c.Writer.Header().Set("Access-Control-Allow-Methods", strings.Join(methods, ", "))
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
