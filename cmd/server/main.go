package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/finish06/drug-gate/internal/client"
	"github.com/finish06/drug-gate/internal/handler"
	"github.com/finish06/drug-gate/internal/middleware"
	"github.com/go-chi/chi/v5"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cashDrugsURL := os.Getenv("CASHDRUGS_URL")
	if cashDrugsURL == "" {
		cashDrugsURL = "http://localhost:8083"
	}

	listenAddr := os.Getenv("LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = ":8081"
	}

	drugClient := client.NewHTTPDrugClient(cashDrugsURL)
	drugHandler := handler.NewDrugHandler(drugClient)

	r := chi.NewRouter()
	r.Use(middleware.RequestLogger)

	r.Get("/health", handler.HealthCheck)
	r.Get("/v1/drugs/ndc/{ndc}", drugHandler.HandleNDCLookup)

	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("starting drug-gate", "addr", listenAddr, "upstream", cashDrugsURL)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("shutdown error", "err", err)
	}
}
