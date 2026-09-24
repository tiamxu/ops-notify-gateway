package api

import (
	"github.com/gin-gonic/gin"
	"github.com/tiamxu/ops-notify-gateway/pkg/e"
	"github.com/tiamxu/ops-notify-gateway/types"
)

func (h *Handler) Jenkins(c *gin.Context) {
	var req types.JenkinsNotifyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, e.BadRequest("invalid json body"))
		return
	}
	result, err := h.notify.SendJenkinsBuild(c.Request.Context(), req)
	if err != nil {
		writeError(c, err)
		return
	}
	writeSuccess(c, result)
}
