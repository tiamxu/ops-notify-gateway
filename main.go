package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tiamxu/kit/log"
	"github.com/tiamxu/ops-notify-gateway/api"
	appconfig "github.com/tiamxu/ops-notify-gateway/config"
	"github.com/tiamxu/ops-notify-gateway/middleware"
	"github.com/tiamxu/ops-notify-gateway/pkg/httpclient"
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var store service.ChannelStore = service.NewStaticChannelStore(cfg.Channels)
	var syncDone chan struct{}
	if cfg.ChannelSource.Mode == "platform" {
		platformStore, err := service.NewPlatformChannelStore(cfg.ChannelSource.Platform, nil)
		if err != nil {
			log.Fatalf("initialize platform channel source failed")
			return
		}
		platformStore.Initialize(ctx)
		store = platformStore
		syncDone = make(chan struct{})
		go func() { defer close(syncDone); platformStore.Run(ctx) }()
	}
	notify := service.NewNotifyServiceWithStore(
		store,
		service.NewConfigSenderFactory(client),
		tmpl.NewFileRenderer("templates"),
	)
	handler := api.NewHandler(notify)
	router := gin.New()
	router.Use(gin.Recovery())
	routes.Register(router, handler, cfg.Auth)

	writeTimeout := time.Duration(cfg.Server.WriteTimeoutSeconds) * time.Second
	server := &http.Server{
		Addr:         cfg.Server.Address,
		Handler:      middleware.RequestDeadline(router, writeTimeout*9/10, time.Duration(cfg.Server.ReadTimeoutSeconds)*time.Second),
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeoutSeconds) * time.Second,
		WriteTimeout: writeTimeout,
	}
	log.Infof("ops-notify-gateway listen: %s", cfg.Server.Address)
	serveDone := make(chan error, 1)
	listenFailed := false
	go func() { serveDone <- server.ListenAndServe() }()
	select {
	case err := <-serveDone:
		if !errors.Is(err, http.ErrServerClosed) {
			listenFailed = true
		}
		stop()
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		_ = server.Close()
	}
	if syncDone != nil {
		<-syncDone
	}
	if listenFailed {
		log.Fatalf("HTTP server failed to listen")
	}
}
