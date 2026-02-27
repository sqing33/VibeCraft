package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"vibe-tree/backend/internal/api"
	"vibe-tree/backend/internal/config"
	"vibe-tree/backend/internal/execution"
	"vibe-tree/backend/internal/expert"
	"vibe-tree/backend/internal/logx"
	"vibe-tree/backend/internal/paths"
	"vibe-tree/backend/internal/runner"
	"vibe-tree/backend/internal/scheduler"
	"vibe-tree/backend/internal/server"
	"vibe-tree/backend/internal/store"
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

	stateDBPath, err := paths.StateDBPath()
	if err != nil {
		logx.Error("daemon", "open-state-db", "解析 state.db 路径失败", "err", err)
		os.Exit(1)
	}
	if err := paths.EnsureDir(filepath.Dir(stateDBPath)); err != nil {
		logx.Error("daemon", "open-state-db", "创建数据目录失败", "err", err, "path", stateDBPath)
		os.Exit(1)
	}
	stateStore, err := store.Open(context.Background(), stateDBPath)
	if err != nil {
		logx.Error("daemon", "open-state-db", "打开 state.db 失败", "err", err, "path", stateDBPath)
		os.Exit(1)
	}
	defer stateStore.Close()
	if err := stateStore.Migrate(context.Background()); err != nil {
		logx.Error("daemon", "migrate-state-db", "迁移 state.db 失败", "err", err, "path", stateDBPath)
		os.Exit(1)
	}
	logx.Info("daemon", "open-state-db", "初始化 state.db 成功", "path", stateDBPath)

	if fixed, err := stateStore.RecoverAfterRestart(context.Background()); err != nil {
		logx.Warn("daemon", "recover", "启动恢复失败（将由后续状态机兜底）", "err", err)
	} else if fixed > 0 {
		logx.Warn("daemon", "recover", "检测到未收敛的 running execution，已标记为 failed", "count", fixed)
	}

	hub := ws.NewHub()
	execRunner := runner.PTYRunner{DefaultGrace: grace}
	execMgr := execution.NewManager(execRunner, grace, hub)
	experts := expert.NewRegistry(cfg)

	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()

	sched := scheduler.New(scheduler.Options{
		Store:          stateStore,
		Executions:     execMgr,
		Hub:            hub,
		Experts:        experts,
		MaxConcurrency: cfg.Execution.MaxConcurrency,
	})
	logx.Info("workflow-scheduler", "start", "启动调度器", "max_concurrency", cfg.Execution.MaxConcurrency)
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				tickCtx, cancel := context.WithTimeout(runCtx, 2*time.Second)
				if err := sched.Tick(tickCtx); err != nil {
					logx.Warn("workflow-scheduler", "tick", "调度 tick 失败", "err", err)
				}
				cancel()
			}
		}
	}()

	engine := server.New(
		server.Options{DevCORS: server.DevCORSFromEnv()},
		api.Deps{Executions: execMgr, Hub: hub, Store: stateStore, Experts: experts},
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
		runCancel()
		return
	}

	runCancel()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logx.Warn("daemon", "shutdown", "HTTP 服务优雅退出失败", "err", err)
	}
}
