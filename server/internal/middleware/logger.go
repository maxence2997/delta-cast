package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger returns a Gin middleware that logs each request with method, path, status, and duration.
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		log.Printf("%s %s %d %v",
			c.Request.Method,
			path,
			c.Writer.Status(),
			time.Since(start),
		)
	}
}
