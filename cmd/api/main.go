package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	pwhttp "pulsewatch-go/internal/http"
	"pulsewatch-go/internal/seed"

	"pulsewatch-go/internal/config"
	"pulsewatch-go/internal/db"
	"pulsewatch-go/internal/worker"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("bad config", "err", err)
		os.Exit(1)
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("db connect", "err", err)
		os.Exit(1)
	}
	if pool != nil {
		defer pool.Close()
		if err := db.Migrate(ctx, pool, "migrations"); err != nil {
			slog.Error("migrate", "err", err)
			os.Exit(1)
		}
		if cfg.Seed {
			if err := seed.Run(ctx, pool); err != nil {
				slog.Error("seed", "err", err)
			}
		}
	}

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: pwhttp.NewRouter(&pwhttp.Server{DB: pool, Secret: cfg.JWTSecret}),
	}
	wctx, wcancel := context.WithCancel(context.Background())
	defer wcancel()
	if pool != nil {
		go worker.New(pool).Run(wctx)
	}
	go func() {
		slog.Info("listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("serve", "err", err)
			os.Exit(1)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	wcancel()
	sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(sctx)
	slog.Info("shutdown complete")
}
