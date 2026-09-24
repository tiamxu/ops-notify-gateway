package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/tiamxu/ops-notify-gateway/api"
	"github.com/tiamxu/ops-notify-gateway/middleware"
	"github.com/tiamxu/ops-notify-gateway/types"
)

func Register(router *gin.Engine, handler *api.Handler, auth types.AuthConfig) {
	router.GET("/ping", handler.Ping)
	v1 := router.Group("/api/v1")
	v1.Use(middleware.Auth(auth.Enabled, auth.Token))
	v1.POST("/alerts/alertmanager", handler.Alertmanager)
	v1.POST("/notifications/jenkins", handler.Jenkins)
	v1.POST("/notifications/generic", handler.Generic)
}
