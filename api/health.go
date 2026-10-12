package api

import (
	"github.com/gin-gonic/gin"
	"github.com/tiamxu/ops-notify-gateway/pkg/e"
	"github.com/tiamxu/ops-notify-gateway/types"
	"net/http"
)

func (h *Handler) Ping(c *gin.Context) {
	writeSuccess(c, gin.H{"message": "pong"})
}

func (h *Handler) Ready(c *gin.Context) {
	status := h.notify.ChannelSourceStatus()
	c.Header("Cache-Control", "no-store")
	if !status.Ready {
		c.JSON(http.StatusServiceUnavailable, types.Response{Code: e.CodeConfigUnavailable, Message: "channel configuration unavailable", Data: status})
		return
	}
	writeSuccess(c, status)
}
