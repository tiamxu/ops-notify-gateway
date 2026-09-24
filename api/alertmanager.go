package api

import (
	"github.com/gin-gonic/gin"
	"github.com/tiamxu/ops-notify-gateway/pkg/e"
	"github.com/tiamxu/ops-notify-gateway/types"
)

func (h *Handler) Alertmanager(c *gin.Context) {
	var req types.AlertmanagerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, e.BadRequest("invalid json body"))
		return
	}
	result, err := h.notify.SendAlertmanager(c.Request.Context(), c.Query("channel"), req)
	if err != nil {
		writeError(c, err)
		return
	}
	writeSuccess(c, result)
}
