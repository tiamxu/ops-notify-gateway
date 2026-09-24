package main

import (
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tiamxu/kit/log"
	"github.com/tiamxu/ops-notify-gateway/api"
	appconfig "github.com/tiamxu/ops-notify-gateway/config"
	"github.com/tiamxu/ops-notify-gateway/pkg/httpclient"
	"github.com/tiamxu/ops-notify-gateway/pkg/sender"
	tmpl "github.com/tiamxu/ops-notify-gateway/pkg/template"
	"github.com/tiamxu/ops-notify-gateway/routes"
	"github.com/tiamxu/ops-notify-gateway/service"
)

func main() {
	_ = log.InitLogger(&log.Config{Level: "info", Type: "stdout", Format: "json"})

	cfgPath := os.Getenv("CONFIG_FILE")
	cfg, err := appconfig.Load(cfgPath)
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	client := httpclient.New(10 * time.Second)
	senders := make(map[string]sender.Sender, len(cfg.Channels))
	for name, ch := range cfg.Channels {
		snd, err := sender.New(sender.Config{Platform: ch.Platform, Webhook: ch.Webhook, Secret: ch.Secret}, client)
		if err != nil {
			log.Fatalf("create sender failed: %v", err)
		}
		senders[name] = snd
	}

	notify := service.NewNotifyService(service.Config{Channels: cfg.Channels}, senders, tmpl.NewFileRenderer("templates"))
	handler := api.NewHandler(notify)
	router := gin.New()
	router.Use(gin.Recovery())
	routes.Register(router, handler, cfg.Auth)

	server := &http.Server{
		Addr:         cfg.Server.Address,
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeoutSeconds) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeoutSeconds) * time.Second,
	}
	log.Infof("ops-notify-gateway listen: %s", cfg.Server.Address)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server stopped: %v", err)
	}
}
