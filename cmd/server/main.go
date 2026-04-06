package main

import (
	"context"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sananguliyev/webpage-analyzer/internal/config"
	"github.com/sananguliyev/webpage-analyzer/internal/fetcher"
	"github.com/sananguliyev/webpage-analyzer/internal/handler"
	"github.com/sananguliyev/webpage-analyzer/internal/linkcheck"
	"github.com/sananguliyev/webpage-analyzer/templates"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to parse config", "error", err)
		os.Exit(1)
	}

	tmpl, err := template.ParseFS(templates.FS, "index.html")
	if err != nil {
		logger.Error("failed to parse templates", "error", err)
		os.Exit(1)
	}

	h := &handler.Handler{
		Template: tmpl,
		Fetcher: &fetcher.Fetcher{
			Client: &http.Client{
				Timeout: cfg.FetchTimeout,
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					if len(via) >= 10 {
						return http.ErrUseLastResponse
					}
					return nil
				},
			},
		},
		LinkChecker: &linkcheck.Checker{
			Client: &http.Client{
				Timeout: 10 * time.Second,
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					if len(via) >= 5 {
						return http.ErrUseLastResponse
					}
					return nil
				},
			},
			MaxWorkers: cfg.MaxConcurrentChecks,
			MaxLinks:   100,
		},
		FetchTimeout:     cfg.FetchTimeout,
		LinkCheckTimeout: cfg.LinkCheckTimeout,
		Logger:           logger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", h.Index)
	mux.HandleFunc("POST /analyze", h.Analyze)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		logger.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server forced to shutdown", "error", err)
	}
	logger.Info("server stopped")
}
