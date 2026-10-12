package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tiamxu/ops-notify-gateway/pkg/e"
	"github.com/tiamxu/ops-notify-gateway/service"
	"github.com/tiamxu/ops-notify-gateway/types"
)

type Handler struct {
	notify *service.NotifyService
}

func NewHandler(notify *service.NotifyService) *Handler {
	return &Handler{notify: notify}
}

func writeSuccess(c *gin.Context, data interface{}) {
	if requestExpired(c) {
		writeError(c, context.DeadlineExceeded)
		return
	}
	c.JSON(http.StatusOK, types.Success(data))
}

func writeError(c *gin.Context, err error) {
	if requestExpired(c) || errors.Is(err, context.DeadlineExceeded) {
		c.JSON(http.StatusGatewayTimeout, types.Error(e.CodeRequestTimeout, "notification request timed out"))
		return
	}
	var biz *e.Error
	if errors.As(err, &biz) {
		status := http.StatusBadRequest
		if biz.Code >= 50000 {
			status = http.StatusInternalServerError
		}
		if biz.Code == e.CodeConfigUnavailable {
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, types.Error(biz.Code, biz.Message))
		return
	}
	c.JSON(http.StatusInternalServerError, types.Error(e.CodeInternal, "internal server error"))
}

func requestExpired(c *gin.Context) bool {
	deadline, ok := c.Request.Context().Deadline()
	return ok && !time.Now().Before(deadline)
}
