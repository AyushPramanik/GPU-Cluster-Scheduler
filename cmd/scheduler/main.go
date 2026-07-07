// Command scheduler runs the scheduling control loop (leader-elected) and the
// NodeService gRPC control plane that node agents talk to.
package main

import (
	"context"
	"os"

	grpcsrv "github.com/ayushpramanik/gpu-cluster-scheduler/internal/grpc"

	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/config"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/logging"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/redisx"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/runtimex"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/scheduler"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/store"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/telemetry"
)

func main() {
	cfg := config.Load("scheduler")
	log := logging.New(cfg.ServiceName, cfg.LogLevel)

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
		log.Error("connect redis (required for leader election)", "error", err)
		os.Exit(1)
	}
	defer func() { _ = rdb.Close() }()

	metrics := telemetry.NewMetrics()

	alg, err := scheduler.New(cfg.Scheduler.Algorithm)
	if err != nil {
		log.Error("invalid scheduler algorithm", "error", err)
		os.Exit(1)
	}
	engine := scheduler.NewEngine(st.Jobs, st.Nodes, st.Events, alg, cfg.Scheduler, metrics, log)

	nodeSrv := grpcsrv.NewNodeServer(st, rdb, metrics, log)

	// The gRPC control plane and metrics run on every replica; only the elected
	// leader runs the scheduling loop.
	elector := redisx.NewLeaderElector(rdb, "scheduler:leader", cfg.Scheduler.LeaderTTL, log)

	err = runtimex.RunGroup(ctx,
		func(c context.Context) error {
			return grpcsrv.Serve(c, cfg.GRPC.SchedulerListen, nodeSrv, log)
		},
		func(c context.Context) error {
			return telemetry.ServeMetrics(c, cfg.HTTP.MetricsAddr, metrics, log)
		},
		func(c context.Context) error {
			elector.Run(c, engine.Run)
			return nil
		},
	)
	if err != nil {
		log.Error("scheduler exited with error", "error", err)
		os.Exit(1)
	}
	log.Info("scheduler stopped cleanly")
}
