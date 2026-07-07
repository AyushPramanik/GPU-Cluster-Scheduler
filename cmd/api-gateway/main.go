// Command api-gateway serves the REST API for job and cluster management.
package main

import (
	"context"
	"os"

	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/api"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/config"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/logging"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/redisx"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/runtimex"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/store"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/telemetry"
)

func main() {
	cfg := config.Load("api-gateway")
	log := logging.New(cfg.ServiceName, cfg.LogLevel)
	if err := cfg.Validate(); err != nil {
		log.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	ctx, cancel := runtimex.SignalContext()
	defer cancel()

	shutdownTracer, err := telemetry.InitTracer(ctx, telemetry.TracerConfig{
		ServiceName:  cfg.ServiceName,
		Environment:  cfg.Env,
		OTLPEndpoint: cfg.Telemetry.OTLPEndpoint,
		Enabled:      cfg.Telemetry.TracingEnable,
		SampleRatio:  cfg.Telemetry.SampleRatio,
	})
	if err != nil {
		log.Error("init tracer", "error", err)
		os.Exit(1)
	}
	defer func() { _ = shutdownTracer(context.Background()) }()

	st, err := store.New(ctx, cfg.Postgres)
	if err != nil {
		log.Error("connect postgres", "error", err)
		os.Exit(1)
	}
	defer st.Close()
	if err := st.Migrate(ctx); err != nil {
		log.Error("run migrations", "error", err)
		os.Exit(1)
	}

	rdb, err := redisx.New(ctx, cfg.Redis)
	if err != nil {
		log.Warn("redis unavailable, log streaming disabled", "error", err)
		rdb = nil
	}

	metrics := telemetry.NewMetrics()
	srv := api.NewServer(cfg, st, rdb, metrics, log)

	err = runtimex.RunGroup(ctx,
		srv.Run,
		func(c context.Context) error {
			return telemetry.ServeMetrics(c, cfg.HTTP.MetricsAddr, metrics, log)
		},
	)
	if err != nil {
		log.Error("service exited with error", "error", err)
		os.Exit(1)
	}
	log.Info("api gateway stopped cleanly")
}
