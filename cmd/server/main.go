package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	_ "github.com/finish06/drug-gate/docs"
	"github.com/finish06/drug-gate/internal/apikey"
	"github.com/finish06/drug-gate/internal/client"
	"github.com/finish06/drug-gate/internal/handler"
	"github.com/finish06/drug-gate/internal/metrics"
	"github.com/finish06/drug-gate/internal/middleware"
	"github.com/finish06/drug-gate/internal/ratelimit"
	"github.com/finish06/drug-gate/internal/service"
	"github.com/finish06/drug-gate/internal/spl"
	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger"
)

// @title       drug-gate API
// @version     0.9.0
// @description Open-source drug information gateway. Provides NDC lookup, therapeutic class search, drug interactions, RxNorm fuzzy search, and structured product label data — all through a clean REST API with Redis caching, per-key rate limiting, and circuit breaker protection.
// @description
// @description ## Getting Started
// @description 1. **Create an API key:** `POST /admin/keys` with `Authorization: Bearer {ADMIN_SECRET}` and body `{"app_name": "my-app", "origins": ["*"], "rate_limit": 1000}`
// @description 2. **Search for a drug:** `GET /v1/drugs/autocomplete?q=lipit` with header `X-API-Key: {your_key}` — returns matching drug names
// @description 3. **Look up details:** `GET /v1/drugs/class?name=atorvastatin` — returns therapeutic classes and brand names
// @description 4. **Get interactions:** `GET /v1/drugs/info?name=warfarin` — returns FDA label interaction warnings
// @description 5. **Check multi-drug interactions:** `POST /v1/drugs/interactions` with `{"drugs": [{"name": "warfarin"}, {"name": "aspirin"}]}`

// @contact.name  drug-gate on GitHub
// @contact.url   https://github.com/finish06/drug-gate
// @license.name  MIT
// @license.url   https://github.com/finish06/drug-gate/blob/main/LICENSE

// @BasePath /

// @tag.name system
// @tag.description Health, version, metrics, and documentation endpoints. No authentication required.
// @tag.name drugs
// @tag.description Drug lookup by NDC, name, and class — core data from FDA/DailyMed. Requires API key.
// @tag.name rxnorm
// @tag.description RxNorm drug search, profiles, NDCs, generics, and related concepts. Requires API key.
// @tag.name spl
// @tag.description Structured Product Labels — drug interaction and safety data from FDA labels. Requires API key.
// @tag.name admin
// @tag.description API key management and cache administration. Requires admin bearer token.

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key
// @description Publishable API key for frontend applications. Create one via `POST /admin/keys`. Include in all `/v1/*` requests.

// @securityDefinitions.apikey AdminAuth
// @in header
// @name Authorization
// @description Admin bearer token set via `ADMIN_SECRET` environment variable. Format: `Bearer {secret}`. Required for all `/admin/*` endpoints.

func main() {
	startTime := time.Now().UTC()

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

	// Cache TTL configuration
	if cacheTTLStr := os.Getenv("CACHE_TTL"); cacheTTLStr != "" {
		if d, err := time.ParseDuration(cacheTTLStr); err == nil {
			service.SetCacheTTL(d)
			spl.SetIndexerCacheTTL(d)
			slog.Info("cache TTL configured", "ttl", d)
		} else {
			slog.Warn("invalid CACHE_TTL, using default 60m", "value", cacheTTLStr, "err", err)
		}
	}

	// Redis client (tuned pool for production load)
	rdb := redis.NewClient(&redis.Options{
		Addr:         redisURL,
		PoolSize:     128,
		MinIdleConns: 16,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		slog.Warn("redis not reachable at startup", "addr", redisURL, "err", err)
	}

	// Prometheus metrics
	m := metrics.NewMetrics(prometheus.DefaultRegisterer)

	// Start background Redis health collector
	redisCollector := metrics.NewRedisCollector(rdb, m, 30*time.Second)
	redisCollector.Start()

	// Start background system metrics collector (Linux only)
	sysMetricsInterval := os.Getenv("SYSTEM_METRICS_INTERVAL")
	sysInterval := 15 * time.Second
	if sysMetricsInterval != "" {
		if d, err := time.ParseDuration(sysMetricsInterval); err == nil {
			sysInterval = d
		} else {
			slog.Warn("invalid SYSTEM_METRICS_INTERVAL, using 15s", "value", sysMetricsInterval, "err", err)
		}
	}
	var sysCollector *metrics.SystemCollector
	if runtime.GOOS == "linux" {
		sysSource := metrics.NewProcfsSource()
		sysCollector = metrics.NewSystemCollector(sysSource, m, sysInterval, "/")
		sysCollector.Start()
		slog.Info("system metrics collector started", "interval", sysInterval)
	} else {
		slog.Info("system metrics collector skipped (not linux)", "os", runtime.GOOS)
	}

	// Dependencies
	store := apikey.NewRedisStore(rdb)
	limiter := ratelimit.NewRedisLimiter(rdb)
	// Shared transport and circuit breaker for all upstream clients (same cash-drugs backend)
	upstreamTransport := client.NewSharedTransport()
	upstreamBreaker := client.NewCircuitBreaker(10, 30*time.Second)
	clientOpts := []client.ClientOption{
		client.WithBreaker(upstreamBreaker),
		client.WithTransport(upstreamTransport),
	}

	drugClient := client.NewHTTPDrugClient(cashDrugsURL, clientOpts...)
	drugHandler := handler.NewDrugHandler(drugClient)
	drugClassHandler := handler.NewDrugClassHandler(drugClient)
	dataSvc := service.NewDrugDataService(drugClient, rdb, m)
	drugNamesHandler := handler.NewDrugNamesHandler(dataSvc)
	drugClassesHandler := handler.NewDrugClassesHandler(dataSvc)
	drugsByClassHandler := handler.NewDrugsByClassHandler(dataSvc)
	autocompleteHandler := handler.NewAutocompleteHandler(dataSvc)
	rxnormClient := client.NewHTTPRxNormClient(cashDrugsURL, clientOpts...)
	rxnormSvc := service.NewRxNormService(rxnormClient, rdb, m)
	rxnormHandler := handler.NewRxNormHandler(rxnormSvc)
	adminHandler := handler.NewAdminHandler(store)
	cacheHandler := handler.NewCacheHandler(rdb)
	splClient := client.NewHTTPSPLClient(cashDrugsURL, clientOpts...)
	splSvc := service.NewSPLService(splClient, drugClient, rdb, m)
	splHandler := handler.NewSPLHandler(splSvc)

	// Start background SPL interaction indexer
	splIndexer := spl.NewIndexer(splClient, rdb, 24*time.Hour, 200)
	splIndexer.Start()

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RequestLogger)
	r.Use(middleware.MetricsMiddleware(m))

	// Landing page redirect (config-driven)
	if landingURL := os.Getenv("LANDING_URL"); landingURL != "" {
		parsed, err := url.Parse(landingURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			slog.Warn("LANDING_URL ignored (must be an http or https URL)", "value", landingURL)
		} else {
			slog.Info("landing page redirect enabled", "url", landingURL)
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, landingURL, http.StatusFound)
			})
		}
	}

	// Public routes (no auth)
	healthHandler := handler.NewHealthHandler(rdb, cashDrugsURL, startTime, upstreamBreaker)
	r.Get("/health", healthHandler.Handle)
	r.Get("/version", handler.VersionInfo)
	r.Handle("/metrics", promhttp.Handler())
	r.Get("/swagger/*", httpSwagger.WrapHandler)
	r.Get("/openapi.json", handler.OpenAPIJSON)

	// Protected API routes
	r.Route("/v1", func(r chi.Router) {
		r.Use(middleware.APIKeyAuth(store, m))
		r.Use(middleware.PerKeyCORS)
		r.Use(middleware.RateLimit(limiter, m))
		r.Get("/drugs/ndc/{ndc}", drugHandler.HandleNDCLookup)
		r.Get("/drugs/class", drugClassHandler.HandleDrugClassLookup)
		r.Get("/drugs/names", drugNamesHandler.HandleDrugNames)
		r.Get("/drugs/classes", drugClassesHandler.HandleDrugClasses)
		r.Get("/drugs/classes/drugs", drugsByClassHandler.HandleDrugsByClass)
		r.Get("/drugs/autocomplete", autocompleteHandler.HandleAutocomplete)
		r.Get("/drugs/spls", splHandler.HandleSearchSPLs)
		r.Get("/drugs/spls/{setid}", splHandler.HandleSPLDetail)
		r.Get("/drugs/info", splHandler.HandleDrugInfo)
		r.Post("/drugs/interactions", splHandler.HandleCheckInteractions)
		r.Get("/drugs/rxnorm/search", rxnormHandler.HandleSearch)
		r.Get("/drugs/rxnorm/profile", rxnormHandler.HandleProfile)
		r.Get("/drugs/rxnorm/{rxcui}/ndcs", rxnormHandler.HandleNDCs)
		r.Get("/drugs/rxnorm/{rxcui}/generics", rxnormHandler.HandleGenerics)
		r.Get("/drugs/rxnorm/{rxcui}/related", rxnormHandler.HandleRelated)
	})

	// Admin routes
	r.Route("/admin", func(r chi.Router) {
		r.Use(middleware.AdminAuth(adminSecret))
		r.Post("/keys", adminHandler.CreateKey)
		r.Get("/keys", adminHandler.ListKeys)
		r.Get("/keys/{key}", adminHandler.GetKey)
		r.Delete("/keys/{key}", adminHandler.DeactivateKey)
		r.Post("/keys/{key}/rotate", adminHandler.RotateKey)
		r.Delete("/cache", cacheHandler.ClearCache)
	})

	srv := &http.Server{
		Addr:              listenAddr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
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
	splIndexer.Stop()
	redisCollector.Stop()
	if sysCollector != nil {
		sysCollector.Stop()
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "err", err)
	}
}
