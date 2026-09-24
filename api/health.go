package api

import "github.com/gin-gonic/gin"

func (h *Handler) Ping(c *gin.Context) {
	writeSuccess(c, gin.H{"message": "pong"})
}
