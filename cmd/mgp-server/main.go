package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/techmigos/mgp/internal/auth"
	"github.com/techmigos/mgp/internal/config"
	"github.com/techmigos/mgp/internal/documents"
	"github.com/techmigos/mgp/internal/gatepass"
	httpserver "github.com/techmigos/mgp/internal/http"
	"github.com/techmigos/mgp/internal/platform/database"
	"github.com/techmigos/mgp/internal/platform/sessions"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	db, err := database.Open(context.Background(), cfg.DatabaseURL)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := database.Migrate(context.Background(), db); err != nil {
		logger.Error("database migration failed", "error", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(cfg.DocumentDir, 0o750); err != nil {
		logger.Error("document directory unavailable", "error", err)
		os.Exit(1)
	}

	server := &httpserver.Server{
		DB:        db,
		Config:    cfg,
		Users:     auth.UserStore{DB: db},
		Passes:    gatepass.Store{DB: db},
		Documents: documents.Service{DB: db, Root: cfg.DocumentDir},
		Sessions:  sessions.Store{DB: db, SecureCookie: cfg.SecureCookie, Key: cfg.SessionKey},
		Logger:    logger,
	}
	httpServer := &http.Server{Addr: cfg.Addr, Handler: server.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		logger.Info("MGP server started", "addr", cfg.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server stopped", "error", err)
			stop()
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}
