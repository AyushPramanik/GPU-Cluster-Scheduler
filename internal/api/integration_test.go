//go:build integration

// Package api integration tests exercise the full REST surface against a real
// PostgreSQL (and optional Redis) instance. Run with:
//
//	make run                 # bring up postgres + redis
//	make test-integration
//
// They connect using POSTGRES_DSN/DATABASE_URL and REDIS_ADDR, and skip cleanly
// if the database is unreachable.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/config"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/models"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/redisx"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/scheduler"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/store"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/telemetry"
)

func setupIntegration(t *testing.T) (*httptest.Server, *store.Store, *scheduler.Engine) {
	t.Helper()
	cfg := config.Load("api-gateway-test")
	ctx := context.Background()

	st, err := store.New(ctx, cfg.Postgres)
	if err != nil {
		t.Skipf("postgres unavailable, skipping integration test: %v", err)
	}
	require.NoError(t, st.Migrate(ctx))

	rdb, err := redisx.New(ctx, cfg.Redis)
	if err != nil {
		rdb = nil // redis is optional for these tests
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	metrics := telemetry.NewMetrics()
	srv := NewServer(cfg, st, rdb, metrics, log)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(func() {
		ts.Close()
		st.Close()
	})

	alg, _ := scheduler.New("best-fit")
	engine := scheduler.NewEngine(st.Jobs, st.Nodes, st.Events, alg, cfg.Scheduler, metrics, log)
	return ts, st, engine
}

func TestIntegration_JobLifecycle(t *testing.T) {
	ts, st, engine := setupIntegration(t)
	ctx := context.Background()

	// Register a node with capacity so the job can be placed.
	nodeID := "it-node-" + uuid.NewString()[:8]
	require.NoError(t, st.Nodes.Upsert(ctx, &models.Node{
		NodeID: nodeID, Hostname: nodeID, Status: models.NodeStatusReady,
		GPUCapacity: 8, GPUAvailable: 8, CPUCapacity: 64, CPUAvailable: 64,
		MemoryCapacity: 512, MemoryAvailable: 512,
	}))

	// Submit a job.
	body, _ := json.Marshal(models.SubmitJobRequest{
		Name: "integration-train", UserID: "it-user", TeamID: "research",
		Priority: 5, GPUCount: 2, CPUCount: 8, MemoryGB: 64,
		Image: "pytorch/pytorch:2.3.0", Command: "python train.py",
	})
	resp, err := http.Post(ts.URL+"/api/v1/jobs", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created models.Job
	decode(t, resp, &created)
	require.NotEmpty(t, created.JobID)
	require.Equal(t, models.JobStatusQueued, created.Status)

	// Run a scheduling tick; the job should be placed on our node.
	require.NoError(t, engine.Tick(ctx))

	got := getJob(t, ts.URL, created.JobID)
	require.Equal(t, models.JobStatusScheduled, got.Status)
	require.Equal(t, nodeID, got.NodeID)

	// Utilization should reflect the reserved GPUs.
	util := getUtilization(t, ts.URL)
	require.GreaterOrEqual(t, util.UsedGPUs, 2)

	// Cancel the job; resources should be released.
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/jobs/"+created.JobID, nil)
	delResp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, delResp.StatusCode)
	_ = delResp.Body.Close()

	final := getJob(t, ts.URL, created.JobID)
	require.Equal(t, models.JobStatusCancelled, final.Status)

	node, err := st.Nodes.Get(ctx, nodeID)
	require.NoError(t, err)
	require.Equal(t, 8, node.GPUAvailable, "cancelled job should release its GPUs")
}

func TestIntegration_SubmitValidation(t *testing.T) {
	ts, _, _ := setupIntegration(t)
	// Missing user_id must be rejected.
	body, _ := json.Marshal(models.SubmitJobRequest{GPUCount: 1})
	resp, err := http.Post(ts.URL+"/api/v1/jobs", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestIntegration_NodeCordon(t *testing.T) {
	ts, st, _ := setupIntegration(t)
	ctx := context.Background()
	nodeID := "it-node-" + uuid.NewString()[:8]
	require.NoError(t, st.Nodes.Upsert(ctx, &models.Node{
		NodeID: nodeID, Hostname: nodeID, Status: models.NodeStatusReady,
		GPUCapacity: 4, GPUAvailable: 4, CPUCapacity: 32, CPUAvailable: 32,
		MemoryCapacity: 256, MemoryAvailable: 256,
	}))

	resp, err := http.Post(ts.URL+"/api/v1/nodes/"+nodeID+"/cordon", "application/json", nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	node, err := st.Nodes.Get(ctx, nodeID)
	require.NoError(t, err)
	require.Equal(t, models.NodeStatusCordoned, node.Status)
}

// --- helpers ---

func decode(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(v))
}

func getJob(t *testing.T, base, id string) models.Job {
	t.Helper()
	resp, err := http.Get(base + "/api/v1/jobs/" + id)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var j models.Job
	decode(t, resp, &j)
	return j
}

func getUtilization(t *testing.T, base string) models.ClusterUtilization {
	t.Helper()
	resp, err := http.Get(base + "/api/v1/cluster/utilization")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var u models.ClusterUtilization
	decode(t, resp, &u)
	return u
}
