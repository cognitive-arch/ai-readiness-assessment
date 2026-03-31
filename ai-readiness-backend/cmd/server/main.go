// cmd/server/main.go
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/yourorg/ai-readiness-backend/internal/audit"
	"github.com/yourorg/ai-readiness-backend/internal/config"
	"github.com/yourorg/ai-readiness-backend/internal/handlers"
	"github.com/yourorg/ai-readiness-backend/internal/metrics"
	appMiddleware "github.com/yourorg/ai-readiness-backend/internal/middleware"
	"github.com/yourorg/ai-readiness-backend/internal/ratelimit"
	"github.com/yourorg/ai-readiness-backend/internal/repository"
	"github.com/yourorg/ai-readiness-backend/internal/service"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	// ── Load Config
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	// ── Setup Logger
	log := buildLogger(cfg)
	defer log.Sync() //nolint:errcheck

	log.Info("starting server",
		zap.String("version", Version),
		zap.String("build_time", BuildTime),
		zap.String("env", cfg.Env),
	)

	// ── MongoDB Connection
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	mongoClient, err := connectMongo(ctx, cfg.MongoURI)
	if err != nil {
		log.Fatal("mongodb connect", zap.Error(err))
	}
	defer func() {
		if err := mongoClient.Disconnect(context.Background()); err != nil {
			log.Warn("mongodb disconnect", zap.Error(err))
		}
	}()
	log.Info("mongodb connected", zap.String("db", cfg.MongoDB))

	db := mongoClient.Database(cfg.MongoDB)

	// ── Initialize Repository
	repo, err := repository.NewAssessmentRepository(ctx, db)
	if err != nil {
		log.Fatal("init repository", zap.Error(err))
	}

	// ── Business Services
	bankPath := resolveQuestionBankPath()
	svc, err := service.NewAssessmentService(repo, bankPath, log)
	if err != nil {
		log.Fatal("init service", zap.Error(err))
	}

	pdfExporter := service.NewPDFExporter(cfg.PDFTmpDir)
	auditLog := audit.New(log)

	// ── Request Handlers
	h := handlers.NewAssessmentHandler(svc, pdfExporter, auditLog, log)

	// ── Initialize Router
	r := chi.NewRouter()

	// Global middleware — order matters
	r.Use(appMiddleware.RequestID)
	r.Use(appMiddleware.Logger(log))
	r.Use(appMiddleware.Recoverer(log))
	r.Use(chimiddleware.Timeout(30 * time.Second))
	r.Use(chimiddleware.Compress(5))
	r.Use(metrics.Middleware) // Prometheus — after chi routing so route patterns are available

	// Allow CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID", "Content-Disposition"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Rate limiting
	r.Use(httprate.LimitByIP(cfg.RateLimitRPM, time.Minute))

	// ── API Routes, Health Check and Metrics Setup
	r.Get("/health", healthHandler(mongoClient))
	r.Get("/metrics", metrics.Handler().ServeHTTP) // Prometheus scrape endpoint

	r.Route("/api", func(r chi.Router) {
		r.Get("/questions", h.GetQuestions)

		r.Get("/assessment", h.List)
		r.Post("/assessment", h.Create)

		r.Route("/assessment/{id}", func(r chi.Router) {
			r.Get("/", h.Get)
			r.Delete("/", h.Delete)
			r.Put("/answers", h.SaveAnswers)
			r.With(ratelimit.ComputeLimiter.Middleware).Post("/compute", h.Compute)
			r.Get("/results", h.GetResults)
			r.With(ratelimit.PDFLimiter.Middleware).Get("/export/pdf", h.ExportPDF)
		})
	})

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"error":"route not found"}`))
	})

	// ── Server Setup
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Info("listening", zap.String("addr", srv.Addr))
		serverErr <- srv.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	case sig := <-quit:
		log.Info("shutdown signal received", zap.String("signal", sig.String()))
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", zap.Error(err))
	} else {
		log.Info("server stopped gracefully")
	}
}

// Helpers Section

// connectMongo establishes a connection to MongoDB with the given URI and verifies it.
func connectMongo(ctx context.Context, uri string) (*mongo.Client, error) {
	opts := options.Client().ApplyURI(uri).
		SetConnectTimeout(10 * time.Second).
		SetServerSelectionTimeout(10 * time.Second)

	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("mongo.Connect: %w", err)
	}
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, fmt.Errorf("mongo.Ping: %w", err)
	}
	return client, nil
}

// buildLogger configures a zap.Logger based on the environment and log level settings.
func buildLogger(cfg *config.Config) *zap.Logger {
	level := zapcore.InfoLevel
	switch cfg.LogLevel {
	case "debug":
		level = zapcore.DebugLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	}

	var logCfg zap.Config
	if cfg.IsProduction() {
		logCfg = zap.NewProductionConfig()
	} else {
		logCfg = zap.NewDevelopmentConfig()
	}
	logCfg.Level = zap.NewAtomicLevelAt(level)

	log, err := logCfg.Build()
	if err != nil {
		panic("build logger: " + err.Error())
	}
	return log
}

// resolveQuestionBankPath attempts to find the question bank JSON file in common locations,
// allowing flexibility in where the server is run from.
func resolveQuestionBankPath() string {
	if p := os.Getenv("QUESTION_BANK_PATH"); p != "" {
		return p
	}
	candidates := []string{
		"question-bank-v1.json",
		"../frontend/public/question-bank-v1.json",
		"../../ai-readiness/public/question-bank-v1.json",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "question-bank-v1.json"
}

// healthHandler returns an HTTP handler that checks MongoDB connectivity and responds with a JSON status.
func healthHandler(mc *mongo.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		mongoOK := mc.Ping(ctx, readpref.Primary()) == nil
		status := http.StatusOK
		if !mongoOK {
			status = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprintf(w, `{"status":"ok","mongo":%v,"version":%q,"time":%q}`,
			mongoOK, Version, time.Now().UTC().Format(time.RFC3339))
	}
}
