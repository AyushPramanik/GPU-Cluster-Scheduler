package scheduler

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/config"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/models"
)

// --- in-memory fakes implementing the engine's store interfaces ---

type fakeJobStore struct {
	jobs map[string]*models.Job
}

func newFakeJobStore(js ...*models.Job) *fakeJobStore {
	m := make(map[string]*models.Job)
	for _, j := range js {
		m[j.JobID] = j
	}
	return &fakeJobStore{jobs: m}
}

func (f *fakeJobStore) ListByStatuses(_ context.Context, statuses ...models.JobStatus) ([]*models.Job, error) {
	set := make(map[models.JobStatus]bool)
	for _, s := range statuses {
		set[s] = true
	}
	var out []*models.Job
	for _, j := range f.jobs {
		if set[j.Status] {
			out = append(out, j)
		}
	}
	return out, nil
}

func (f *fakeJobStore) Schedule(_ context.Context, id, nodeID string) error {
	j := f.jobs[id]
	j.Status = models.JobStatusScheduled
	j.NodeID = nodeID
	return nil
}

func (f *fakeJobStore) Requeue(_ context.Context, id string, status models.JobStatus) error {
	j := f.jobs[id]
	j.Status = status
	j.NodeID = ""
	j.RetryCount++
	return nil
}

func (f *fakeJobStore) UpdateStatus(_ context.Context, id string, status models.JobStatus) error {
	f.jobs[id].Status = status
	return nil
}

func (f *fakeJobStore) TeamGPUUsage(_ context.Context) (map[string]int, error) {
	out := make(map[string]int)
	for _, j := range f.jobs {
		if j.Status.IsActive() {
			out[j.TeamID] += j.GPUCount
		}
	}
	return out, nil
}

func (f *fakeJobStore) CountByStatus(_ context.Context, status models.JobStatus) (int, error) {
	n := 0
	for _, j := range f.jobs {
		if j.Status == status {
			n++
		}
	}
	return n, nil
}

type fakeNodeStore struct {
	nodes map[string]*models.Node
	stale []string
}

func newFakeNodeStore(ns ...*models.Node) *fakeNodeStore {
	m := make(map[string]*models.Node)
	for _, n := range ns {
		m[n.NodeID] = n
	}
	return &fakeNodeStore{nodes: m}
}

func (f *fakeNodeStore) List(_ context.Context) ([]*models.Node, error) {
	var out []*models.Node
	for _, n := range f.nodes {
		out = append(out, n)
	}
	return out, nil
}

func (f *fakeNodeStore) UpdateAvailability(_ context.Context, id string, gpu, cpu, mem int) error {
	n := f.nodes[id]
	n.GPUAvailable, n.CPUAvailable, n.MemoryAvailable = gpu, cpu, mem
	return nil
}

func (f *fakeNodeStore) MarkStaleDown(_ context.Context, _ time.Duration) ([]string, error) {
	for _, id := range f.stale {
		if n, ok := f.nodes[id]; ok {
			n.Status = models.NodeStatusDown
		}
	}
	out := f.stale
	f.stale = nil
	return out, nil
}

type fakeEventStore struct{ events []*models.SchedulingEvent }

func (f *fakeEventStore) Record(_ context.Context, e *models.SchedulingEvent) error {
	f.events = append(f.events, e)
	return nil
}

func testEngine(js *fakeJobStore, ns *fakeNodeStore, es *fakeEventStore, alg Algorithm) *Engine {
	cfg := config.SchedulerConfig{
		AgingFactor:         1,
		StarvationThreshold: 5 * time.Minute,
		EnablePreemption:    true,
		HeartbeatTTL:        30 * time.Second,
		MaxRetries:          3,
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewEngine(js, ns, es, alg, cfg, nil, log)
}

func TestEngine_Tick_SchedulesQueuedJob(t *testing.T) {
	js := newFakeJobStore(job("j1", 2, 8, 64, 5))
	ns := newFakeNodeStore(node("n1", 8, 64, 512))
	es := &fakeEventStore{}
	eng := testEngine(js, ns, es, BestFit{})

	require.NoError(t, eng.Tick(context.Background()))

	assert.Equal(t, models.JobStatusScheduled, js.jobs["j1"].Status)
	assert.Equal(t, "n1", js.jobs["j1"].NodeID)
	assert.Equal(t, 6, ns.nodes["n1"].GPUAvailable, "node GPUs should be reserved")
	require.Len(t, es.events, 1)
	assert.True(t, es.events[0].Success)
}

func TestEngine_Tick_LeavesUnschedulableJobQueued(t *testing.T) {
	js := newFakeJobStore(job("big", 16, 8, 64, 5))
	ns := newFakeNodeStore(node("n1", 8, 64, 512))
	es := &fakeEventStore{}
	eng := testEngine(js, ns, es, BestFit{})

	require.NoError(t, eng.Tick(context.Background()))
	assert.Equal(t, models.JobStatusQueued, js.jobs["big"].Status)
	require.Len(t, es.events, 1)
	assert.False(t, es.events[0].Success)
}

func TestEngine_Tick_PreemptsForHighPriority(t *testing.T) {
	n := node("n1", 4, 32, 256)
	n.GPUAvailable = 0 // full
	victim := job("victim", 4, 8, 64, 1)
	victim.Status = models.JobStatusRunning
	victim.NodeID = "n1"
	pending := job("urgent", 4, 8, 64, 9)

	js := newFakeJobStore(victim, pending)
	ns := newFakeNodeStore(n)
	es := &fakeEventStore{}
	eng := testEngine(js, ns, es, Priority{})

	require.NoError(t, eng.Tick(context.Background()))
	assert.Equal(t, models.JobStatusScheduled, js.jobs["urgent"].Status, "urgent job should be placed after preemption")
	assert.Contains(t, []models.JobStatus{models.JobStatusPreempted, models.JobStatusQueued}, js.jobs["victim"].Status)
}

func TestEngine_ReconcileDeadNodes_RequeuesJobs(t *testing.T) {
	n := node("dead", 8, 64, 512)
	running := job("r1", 2, 8, 64, 5)
	running.Status = models.JobStatusRunning
	running.NodeID = "dead"
	running.MaxRetries = 3

	js := newFakeJobStore(running)
	ns := newFakeNodeStore(n)
	ns.stale = []string{"dead"}
	es := &fakeEventStore{}
	eng := testEngine(js, ns, es, BestFit{})

	eng.reconcileDeadNodes(context.Background())
	assert.Equal(t, models.NodeStatusDown, ns.nodes["dead"].Status)
	assert.Equal(t, models.JobStatusQueued, js.jobs["r1"].Status, "jobs on dead node should be requeued")
	assert.Equal(t, 1, js.jobs["r1"].RetryCount)
}

func TestEngine_ReconcileDeadNodes_FailsExhaustedJobs(t *testing.T) {
	n := node("dead", 8, 64, 512)
	running := job("r1", 2, 8, 64, 5)
	running.Status = models.JobStatusRunning
	running.NodeID = "dead"
	running.RetryCount = 3
	running.MaxRetries = 3

	js := newFakeJobStore(running)
	ns := newFakeNodeStore(n)
	ns.stale = []string{"dead"}
	eng := testEngine(js, ns, &fakeEventStore{}, BestFit{})

	eng.reconcileDeadNodes(context.Background())
	assert.Equal(t, models.JobStatusFailed, js.jobs["r1"].Status, "job past retry budget should fail")
}
