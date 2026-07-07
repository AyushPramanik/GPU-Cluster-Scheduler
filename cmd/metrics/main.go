// Command metrics runs the metrics service: it polls cluster state and exposes
// aggregate Prometheus metrics on /metrics.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/config"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/logging"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/metricsvc"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/runtimex"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/store"
)

func main() {
	cfg := config.Load("metrics")
	log := logging.New(cfg.ServiceName, cfg.LogLevel)

	ctx, cancel := runtimex.SignalContext()
	defer cancel()

	st, err := store.New(ctx, cfg.Postgres)
	if err != nil {
		log.Error("connect postgres", "error", err)
		os.Exit(1)
	}
	defer st.Close()

	collector := metricsvc.New(st, 10*time.Second, log)

	err = runtimex.RunGroup(ctx,
		collector.Run,
		func(c context.Context) error {
			return serveMetrics(c, cfg.HTTP.MetricsAddr, collector, log)
		},
	)
	if err != nil {
		log.Error("metrics service exited with error", "error", err)
		os.Exit(1)
	}
	log.Info("metrics service stopped cleanly")
}

func serveMetrics(ctx context.Context, addr string, c *metricsvc.Collector, log *slog.Logger) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(c.Registry(), promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	errCh := make(chan error, 1)
	go func() {
		log.Info("metrics service listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
