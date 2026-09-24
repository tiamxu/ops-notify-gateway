package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tiamxu/ops-notify-gateway/pkg/security"
	"github.com/tiamxu/ops-notify-gateway/types"
)

func Auth(enabled bool, token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !enabled {
			c.Next()
			return
		}
		if !security.TokenMatches(c.GetHeader("Authorization"), c.GetHeader("X-Webhook-Token"), token) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, types.Error(http.StatusUnauthorized, "unauthorized"))
			return
		}
		c.Next()
	}
}
