// Package agent implements the node agent: a per-node process that registers
// with the scheduler, heartbeats liveness, pulls assigned jobs, executes them
// (simulated GPU workloads here), streams logs, and reports completion.
package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/ayushpramanik/gpu-cluster-scheduler/internal/grpc/clusterpb"
)

// Config describes the node this agent represents.
type Config struct {
	NodeID            string
	Hostname          string
	GPUCapacity       int
	CPUCapacity       int
	MemoryCapacity    int
	GPUModel          string
	CostPerHour       float64
	Spot              bool
	SchedulerAddr     string
	HeartbeatInterval time.Duration
	PollInterval      time.Duration
	// MeanJobSeconds controls the simulated runtime of workloads.
	MeanJobSeconds int
}

// Agent runs the node lifecycle against the scheduler's NodeService.
type Agent struct {
	cfg    Config
	client pb.NodeServiceClient
	conn   *grpc.ClientConn
	log    *slog.Logger

	mu      sync.Mutex
	running map[string]bool // jobs currently executing, to avoid double-starts
	rng     *rng
}

// New dials the scheduler and returns an Agent.
func New(cfg Config, log *slog.Logger) (*Agent, error) {
	if cfg.HeartbeatInterval == 0 {
		cfg.HeartbeatInterval = 10 * time.Second
	}
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 3 * time.Second
	}
	if cfg.MeanJobSeconds == 0 {
		cfg.MeanJobSeconds = 20
	}
	conn, err := grpc.NewClient(cfg.SchedulerAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial scheduler: %w", err)
	}
	return &Agent{
		cfg:     cfg,
		client:  pb.NewNodeServiceClient(conn),
		conn:    conn,
		log:     log,
		running: make(map[string]bool),
		rng:     newRNG(hashString(cfg.NodeID)),
	}, nil
}

// Close releases the gRPC connection.
func (a *Agent) Close() error { return a.conn.Close() }

// Run registers the node then drives the heartbeat and assignment loops until
// ctx is cancelled.
func (a *Agent) Run(ctx context.Context) error {
	if err := a.register(ctx); err != nil {
		return err
	}
	a.log.Info("node agent started", "node", a.cfg.NodeID, "scheduler", a.cfg.SchedulerAddr)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); a.heartbeatLoop(ctx) }()
	go func() { defer wg.Done(); a.pollLoop(ctx) }()
	wg.Wait()
	return nil
}

func (a *Agent) register(ctx context.Context) error {
	req := &pb.RegisterRequest{
		NodeId:      a.cfg.NodeID,
		Hostname:    a.cfg.Hostname,
		GpuModel:    a.cfg.GPUModel,
		CostPerHour: a.cfg.CostPerHour,
		Spot:        a.cfg.Spot,
		Capacity: &pb.Resources{
			Gpu:      int32(a.cfg.GPUCapacity),
			Cpu:      int32(a.cfg.CPUCapacity),
			MemoryGb: int32(a.cfg.MemoryCapacity),
		},
	}
	// Retry registration until the control plane is reachable.
	backoff := time.Second
	for {
		_, err := a.client.Register(ctx, req)
		if err == nil {
			return nil
		}
		a.log.Warn("register failed, retrying", "error", err, "backoff", backoff)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 15*time.Second {
			backoff *= 2
		}
	}
}

func (a *Agent) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(a.cfg.HeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			_, err := a.client.Heartbeat(hbCtx, &pb.HeartbeatRequest{
				NodeId:         a.cfg.NodeID,
				GpuUtilization: a.currentGPUUtil(),
			})
			cancel()
			if err != nil {
				a.log.Warn("heartbeat failed", "error", err)
			}
		}
	}
}

func (a *Agent) pollLoop(ctx context.Context) {
	ticker := time.NewTicker(a.cfg.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pollCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			resp, err := a.client.GetAssignments(pollCtx, &pb.AssignmentsRequest{NodeId: a.cfg.NodeID})
			cancel()
			if err != nil {
				a.log.Warn("poll assignments failed", "error", err)
				continue
			}
			for _, job := range resp.GetJobs() {
				if a.claim(job.GetJobId()) {
					go a.execute(ctx, job)
				}
			}
		}
	}
}

// claim marks a job as running locally, returning false if already claimed.
func (a *Agent) claim(jobID string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.running[jobID] {
		return false
	}
	a.running[jobID] = true
	return true
}

func (a *Agent) done(jobID string) {
	a.mu.Lock()
	delete(a.running, jobID)
	a.mu.Unlock()
}

func (a *Agent) currentGPUUtil() float64 {
	a.mu.Lock()
	n := len(a.running)
	a.mu.Unlock()
	if a.cfg.GPUCapacity == 0 {
		return 0
	}
	// Rough live signal: proportion of GPUs busy, capped at 1.
	util := float64(n) / float64(a.cfg.GPUCapacity)
	if util > 1 {
		util = 1
	}
	return util
}
