package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tiamxu/kit/log"
)

func RequestLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Infof("request method=%s path=%s status=%d cost=%s", c.Request.Method, c.FullPath(), c.Writer.Status(), time.Since(start))
	}
}
