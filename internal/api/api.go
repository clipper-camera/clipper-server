package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/clipper-camera/clipper-server/internal/config"
	"github.com/clipper-camera/clipper-server/internal/helpers"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Handler struct {
	cfg    *config.Config
	logger *log.Logger
}

// HealthCheck handles the health check endpoint
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "ok"}`))
}

// RequestLogger is a middleware that logs the start and end of each request
func RequestLogger(logger *log.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			logger.Printf("=> %s %s %s", r.Method, r.URL.Path, r.RemoteAddr)

			// Create a custom response writer to capture the status code
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			duration := time.Since(start)
			logger.Printf("<= %s %s %s %d %v", r.Method, r.URL.Path, r.RemoteAddr, ww.Status(), duration)
		})
	}
}

func NewServer(ctx context.Context, cfg *config.Config, logger *log.Logger) *http.Server {
	r := chi.NewRouter()

	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.Port),
		Handler: r,
	}

	h := &Handler{
		cfg:    cfg,
		logger: logger,
	}

	// Start the cleanup service
	cleanupService := helpers.NewCleanupService(cfg, logger)
	cleanupService.Start()

	// Use the enhanced request logger
	r.Use(RequestLogger(logger))
	r.Use(middleware.Logger)

	r.Route("/_api", func(r chi.Router) {
		r.Route("/v1", func(r chi.Router) {
			r.Get("/health", h.HealthCheck)
			r.Get("/contacts/{user_password}", h.GetContacts)
			r.Post("/upload", h.UploadMedia)
			r.Get("/mailbox/{user_password}", h.GetMailbox)
			r.Get("/download/{user_password}/{filename}", h.DownloadFile)
		})
	})

	return server
}
