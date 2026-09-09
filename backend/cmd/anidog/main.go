package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/anidog/anidog-go/internal/application"
	"github.com/anidog/anidog-go/internal/config"
)

func main() {
	cfg := config.Load()
	app, err := application.New(cfg)
	if err != nil {
		application.InitLogger(cfg)
		zap.L().Fatal("AniDog 初始化失败", zap.Error(err))
	}

	srv := &http.Server{Addr: ":8088", Handler: app.Handler}
	serverErr := make(chan error, 1)
	go func() {
		zap.L().Info("服务器启动", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-quit:
		zap.L().Info("收到信号，正在关闭...", zap.String("signal", sig.String()))
	case err := <-serverErr:
		zap.L().Error("服务器异常退出", zap.Error(err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		zap.L().Error("服务器关闭失败", zap.Error(err))
	}
	if err := app.Close(ctx); err != nil {
		zap.L().Error("AniDog 运行时关闭失败", zap.Error(err))
	}
}
