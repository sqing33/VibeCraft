package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"vibe-tree/backend/internal/api"
	"vibe-tree/backend/internal/config"
	"vibe-tree/backend/internal/execution"
	"vibe-tree/backend/internal/logx"
	"vibe-tree/backend/internal/runner"
	"vibe-tree/backend/internal/server"
	"vibe-tree/backend/internal/ws"
)

func main() {
	cfg, cfgPath, err := config.Load()
	if err != nil {
		logx.Error("daemon", "load-config", "加载配置失败", "err", err)
		os.Exit(1)
	}
	logx.Info("daemon", "load-config", "加载配置成功", "path", cfgPath)
	logx.Info("daemon", "listen", "启动 HTTP 服务", "addr", cfg.Addr())

	grace := time.Duration(cfg.Execution.KillGraceMs) * time.Millisecond
	hub := ws.NewHub()
	execRunner := runner.PTYRunner{DefaultGrace: grace}
	execMgr := execution.NewManager(execRunner, grace, hub)

	engine := server.New(
		server.Options{DevCORS: server.DevCORSFromEnv()},
		api.Deps{Executions: execMgr, Hub: hub},
	)
	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logx.Info("daemon", "signal", "收到退出信号", "signal", sig.String())
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			logx.Error("daemon", "listen", "HTTP 服务启动失败", "err", err)
			os.Exit(1)
		}
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logx.Warn("daemon", "shutdown", "HTTP 服务优雅退出失败", "err", err)
	}
}
