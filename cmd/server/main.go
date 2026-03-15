package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	_ "github.com/finish06/drug-gate/docs"
	"github.com/finish06/drug-gate/internal/apikey"
	"github.com/finish06/drug-gate/internal/client"
	"github.com/finish06/drug-gate/internal/handler"
	"github.com/finish06/drug-gate/internal/middleware"
	"github.com/finish06/drug-gate/internal/ratelimit"
	"github.com/finish06/drug-gate/internal/service"
	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger"
)

// @title       drug-gate API
// @version     0.1.0
// @description Drug information gateway — NDC lookup, therapeutic classes, and more.

// @host     localhost:8081
// @BasePath /

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

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis:6379"
	}

	adminSecret := os.Getenv("ADMIN_SECRET")
	if adminSecret == "" {
		slog.Warn("ADMIN_SECRET not set — admin endpoints will reject all requests")
	}

	// Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr: redisURL,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		slog.Warn("redis not reachable at startup", "addr", redisURL, "err", err)
	}

	// Dependencies
	store := apikey.NewRedisStore(rdb)
	limiter := ratelimit.NewRedisLimiter(rdb)
	drugClient := client.NewHTTPDrugClient(cashDrugsURL)
	drugHandler := handler.NewDrugHandler(drugClient)
	drugClassHandler := handler.NewDrugClassHandler(drugClient)
	dataSvc := service.NewDrugDataService(drugClient, rdb)
	drugNamesHandler := handler.NewDrugNamesHandler(dataSvc)
	drugClassesHandler := handler.NewDrugClassesHandler(dataSvc)
	drugsByClassHandler := handler.NewDrugsByClassHandler(dataSvc)
	adminHandler := handler.NewAdminHandler(store)

	r := chi.NewRouter()
	r.Use(middleware.RequestLogger)

	// Public routes (no auth)
	r.Get("/health", handler.HealthCheck)
	r.Get("/swagger/*", httpSwagger.WrapHandler)
	r.Get("/openapi.json", handler.OpenAPIJSON)

	// Protected API routes
	r.Route("/v1", func(r chi.Router) {
		r.Use(middleware.APIKeyAuth(store))
		r.Use(middleware.PerKeyCORS)
		r.Use(middleware.RateLimit(limiter))
		r.Get("/drugs/ndc/{ndc}", drugHandler.HandleNDCLookup)
		r.Get("/drugs/class", drugClassHandler.HandleDrugClassLookup)
		r.Get("/drugs/names", drugNamesHandler.HandleDrugNames)
		r.Get("/drugs/classes", drugClassesHandler.HandleDrugClasses)
		r.Get("/drugs/classes/drugs", drugsByClassHandler.HandleDrugsByClass)
	})

	// Admin routes
	r.Route("/admin", func(r chi.Router) {
		r.Use(middleware.AdminAuth(adminSecret))
		r.Post("/keys", adminHandler.CreateKey)
		r.Get("/keys", adminHandler.ListKeys)
		r.Get("/keys/{key}", adminHandler.GetKey)
		r.Delete("/keys/{key}", adminHandler.DeactivateKey)
		r.Post("/keys/{key}/rotate", adminHandler.RotateKey)
	})

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
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "err", err)
	}
}
