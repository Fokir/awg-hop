package main

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"awghop/internal/api"
	"awghop/internal/config"
	"awghop/internal/db"
	"awghop/internal/netctl"
	"awghop/internal/store"
	"awghop/internal/ui"
)

func main() {
	cfg := config.Load()
	logger := newLogger(cfg)
	slog.SetDefault(logger)

	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		logger.Error("create data dir", "err", err)
		os.Exit(1)
	}

	sqldb, err := db.Open(cfg.DatabasePath)
	if err != nil {
		logger.Error("open db", "err", err)
		os.Exit(1)
	}
	defer sqldb.Close()

	st := store.New(sqldb)
	nc := netctl.New(cfg)

	sub, err := fs.Sub(ui.Dist, "dist")
	if err != nil {
		logger.Error("ui dist", "err", err)
		os.Exit(1)
	}

	if cfg.AutoApply && runtime.GOOS == "linux" {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		if err := nc.Apply(ctx, st); err != nil {
			logger.Warn("startup auto-apply failed (will be retried via /system/apply)", "err", err)
		} else {
			logger.Info("startup auto-apply ok")
		}
		cancel()
	}

	handler := api.NewRouter(st, cfg, sub, nc)
	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	logger.Info("AWG Hop listening", "addr", cfg.ListenAddr)

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server", "err", err)
			os.Exit(1)
		}
	}()

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)
	<-stopCh
	logger.Info("shutdown initiated")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Warn("graceful shutdown error", "err", err)
	}
}

func newLogger(cfg config.Config) *slog.Logger {
	level := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	opts := &slog.HandlerOptions{Level: level}
	if cfg.LogFormat == "json" {
		return slog.New(slog.NewJSONHandler(os.Stderr, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stderr, opts))
}
