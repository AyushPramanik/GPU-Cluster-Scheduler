// Command node-agent runs on each GPU node: it registers with the scheduler,
// heartbeats, executes assigned workloads, and streams logs.
package main

import (
	"context"
	"os"
	"strconv"

	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/agent"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/config"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/logging"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/runtimex"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/telemetry"
)

func main() {
	cfg := config.Load("node-agent")
	log := logging.New(cfg.ServiceName, cfg.LogLevel)

	hostname, _ := os.Hostname()
	agentCfg := agent.Config{
		NodeID:         envDefault("NODE_ID", hostname),
		Hostname:       envDefault("NODE_HOSTNAME", hostname),
		GPUCapacity:    envInt("NODE_GPU_CAPACITY", 8),
		CPUCapacity:    envInt("NODE_CPU_CAPACITY", 128),
		MemoryCapacity: envInt("NODE_MEMORY_CAPACITY", 1024),
		GPUModel:       envDefault("NODE_GPU_MODEL", "A100-80GB"),
		CostPerHour:    envFloat("NODE_COST_PER_HOUR", 32.77),
		Spot:           envBool("NODE_SPOT", false),
		SchedulerAddr:  cfg.GRPC.SchedulerAddr,
		MeanJobSeconds: envInt("NODE_MEAN_JOB_SECONDS", 20),
	}

	ctx, cancel := runtimex.SignalContext()
	defer cancel()

	metrics := telemetry.NewMetrics()

	a, err := agent.New(agentCfg, log)
	if err != nil {
		log.Error("create agent", "error", err)
		os.Exit(1)
	}
	defer func() { _ = a.Close() }()

	err = runtimex.RunGroup(ctx,
		a.Run,
		func(c context.Context) error {
			return telemetry.ServeMetrics(c, cfg.HTTP.MetricsAddr, metrics, log)
		},
	)
	if err != nil {
		log.Error("node agent exited with error", "error", err)
		os.Exit(1)
	}
	log.Info("node agent stopped cleanly")
}

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}
