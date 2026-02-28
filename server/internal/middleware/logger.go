package middleware

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/maxence2997/delta-cast/server/internal/logger"
)

// Logger returns a Gin middleware that logs each HTTP request in a structured format
// modelled after Gin's built-in logger but integrated with the project's logger package.
//
// Fields: method path | status | latency | response-bytes | client-ip | "user-agent"
// Level:  ERROR for 5xx, WARN for 4xx, INFO for everything else.
// Gin-level errors (c.Errors) are appended when present.
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Capture full path including query string before handing to handlers.
		path := c.Request.URL.Path
		if raw := c.Request.URL.RawQuery; raw != "" {
			path = path + "?" + raw
		}

		c.Next()

		status := c.Writer.Status()
		latency := time.Since(start)
		clientIP := c.ClientIP()
		bodySize := c.Writer.Size()
		if bodySize < 0 {
			bodySize = 0
		}
		userAgent := c.Request.UserAgent()

		msg := fmt.Sprintf("%s %s | %d | %v | %dB | %s | %q",
			c.Request.Method, path, status, latency, bodySize, clientIP, userAgent)

		// Append any Gin-level errors (e.g. from c.AbortWithError).
		if errs := c.Errors.ByType(gin.ErrorTypePrivate).String(); errs != "" {
			msg += " | " + errs
		}

		if status >= 500 {
			logger.Errorf("%s", msg)
		} else {
			logger.Infof("%s", msg)
		}
	}
}
