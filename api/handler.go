package api

import (
	"errors"
	"net/http"

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
	c.JSON(http.StatusOK, types.Success(data))
}

func writeError(c *gin.Context, err error) {
	var biz *e.Error
	if errors.As(err, &biz) {
		status := http.StatusBadRequest
		if biz.Code >= 50000 {
			status = http.StatusInternalServerError
		}
		c.JSON(status, types.Error(biz.Code, biz.Message))
		return
	}
	c.JSON(http.StatusInternalServerError, types.Error(e.CodeInternal, "internal server error"))
}
