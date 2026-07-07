// Package api implements the REST API gateway: routing, middleware
// (auth, rate limiting, tracing, metrics), and job/node/cluster handlers.
package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/config"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/store"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/telemetry"
)

// Server is the HTTP API gateway.
type Server struct {
	cfg     *config.Config
	store   *store.Store
	redis   *redis.Client
	metrics *telemetry.Metrics
	log     *slog.Logger
	limiter *rateLimiter
	engine  *gin.Engine
}

// NewServer wires the router with all middleware and routes.
func NewServer(cfg *config.Config, s *store.Store, rdb *redis.Client, m *telemetry.Metrics, log *slog.Logger) *Server {
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	srv := &Server{
		cfg:     cfg,
		store:   s,
		redis:   rdb,
		metrics: m,
		log:     log,
		limiter: newRateLimiter(cfg.HTTP.RateLimitRPS, cfg.HTTP.RateLimitBurst),
	}
	srv.engine = srv.buildRouter()
	return srv
}

// Handler exposes the underlying http.Handler (useful for tests).
func (s *Server) Handler() http.Handler { return s.engine }

func (s *Server) buildRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(s.cors())
	r.Use(s.tracing())
	r.Use(s.requestLogger())
	r.Use(s.metricsMiddleware())
	r.Use(s.rateLimit())

	r.GET("/healthz", s.handleHealth)
	r.GET("/readyz", s.handleReady)

	v1 := r.Group("/api/v1")
	v1.Use(s.auth())
	{
		v1.POST("/jobs", s.handleSubmitJob)
		v1.GET("/jobs", s.handleListJobs)
		v1.GET("/jobs/:id", s.handleGetJob)
		v1.DELETE("/jobs/:id", s.handleCancelJob)
		v1.GET("/jobs/:id/logs", s.handleJobLogs)

		v1.GET("/nodes", s.handleListNodes)
		v1.POST("/nodes/:id/cordon", s.handleCordonNode)
		v1.POST("/nodes/:id/drain", s.handleDrainNode)
		v1.POST("/nodes/:id/uncordon", s.handleUncordonNode)

		v1.GET("/cluster/utilization", s.handleUtilization)
		v1.GET("/scheduling-events", s.handleEvents)
	}
	return r
}

// Run starts the HTTP server and blocks until ctx is cancelled, then shuts down
// gracefully within the configured timeout.
func (s *Server) Run(ctx context.Context) error {
	httpSrv := &http.Server{
		Addr:         s.cfg.HTTP.Addr,
		Handler:      s.engine,
		ReadTimeout:  s.cfg.HTTP.ReadTimeout,
		WriteTimeout: s.cfg.HTTP.WriteTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		s.log.Info("api gateway listening", "addr", s.cfg.HTTP.Addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.HTTP.ShutdownTimeout)
		defer cancel()
		s.log.Info("api gateway shutting down")
		return httpSrv.Shutdown(shutdownCtx)
	}
}

func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "time": time.Now().UTC()})
}

func (s *Server) handleReady(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	if err := s.store.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
