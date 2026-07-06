package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/config"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/models"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/telemetry"
)

// JobStore is the subset of job persistence the engine depends on. Depending on
// an interface (rather than the concrete repository) keeps the engine unit
// testable with in-memory fakes.
type JobStore interface {
	ListByStatuses(ctx context.Context, statuses ...models.JobStatus) ([]*models.Job, error)
	Schedule(ctx context.Context, id, nodeID string) error
	Requeue(ctx context.Context, id string, status models.JobStatus) error
	UpdateStatus(ctx context.Context, id string, status models.JobStatus) error
	TeamGPUUsage(ctx context.Context) (map[string]int, error)
	CountByStatus(ctx context.Context, status models.JobStatus) (int, error)
}

// NodeStore is the subset of node persistence the engine depends on.
type NodeStore interface {
	List(ctx context.Context) ([]*models.Node, error)
	UpdateAvailability(ctx context.Context, id string, gpu, cpu, mem int) error
	MarkStaleDown(ctx context.Context, ttl time.Duration) ([]string, error)
}

// EventStore records scheduling decisions.
type EventStore interface {
	Record(ctx context.Context, e *models.SchedulingEvent) error
}

// Engine is the scheduling control loop. It is safe to run only on the elected
// leader. Each Tick drains the queue once using the configured Algorithm,
// applying aging, preemption, and starvation prevention.
type Engine struct {
	jobs    JobStore
	nodes   NodeStore
	events  EventStore
	alg     Algorithm
	cfg     config.SchedulerConfig
	metrics *telemetry.Metrics
	log     *slog.Logger
}

// NewEngine constructs an engine with its dependencies injected.
func NewEngine(jobs JobStore, nodes NodeStore, events EventStore, alg Algorithm,
	cfg config.SchedulerConfig, m *telemetry.Metrics, log *slog.Logger) *Engine {
	return &Engine{jobs: jobs, nodes: nodes, events: events, alg: alg, cfg: cfg, metrics: m, log: log}
}

// Algorithm returns the active placement algorithm.
func (e *Engine) Algorithm() Algorithm { return e.alg }

// Run drives the scheduling loop until ctx is cancelled. It is invoked by the
// leader; when leadership is lost the ctx is cancelled and the loop exits.
func (e *Engine) Run(ctx context.Context) {
	ticker := time.NewTicker(e.cfg.Interval)
	defer ticker.Stop()
	e.log.Info("scheduling loop started", "algorithm", e.alg.Name(), "interval", e.cfg.Interval)
	for {
		select {
		case <-ctx.Done():
			e.log.Info("scheduling loop stopped")
			return
		case <-ticker.C:
			if err := e.Tick(ctx); err != nil {
				e.log.Error("scheduling tick failed", "error", err)
			}
		}
	}
}

// Tick performs one full scheduling pass: reconcile dead nodes, then place as
// many queued jobs as possible.
func (e *Engine) Tick(ctx context.Context) error {
	if e.metrics != nil {
		e.metrics.SchedRuns.Inc()
	}
	e.reconcileDeadNodes(ctx)

	nodes, err := e.nodes.List(ctx)
	if err != nil {
		return err
	}
	queued, err := e.jobs.ListByStatuses(ctx, models.JobStatusQueued)
	if err != nil {
		return err
	}
	active, err := e.jobs.ListByStatuses(ctx, models.JobStatusScheduled, models.JobStatusRunning)
	if err != nil {
		return err
	}

	e.publishGauges(ctx, nodes, queued, active)
	if len(queued) == 0 {
		return nil
	}

	orderCtx := OrderContext{
		AgingFactor: e.cfg.AgingFactor,
		TeamUsage:   e.teamShares(nodes, active),
	}
	ordered := e.alg.Order(queued, orderCtx)

	// running maps node ID -> jobs occupying it, for preemption planning.
	running := make(map[string][]*models.Job)
	for _, j := range active {
		if j.NodeID != "" {
			running[j.NodeID] = append(running[j.NodeID], j)
		}
	}

	now := time.Now()
	for _, job := range ordered {
		e.placeOne(ctx, job, nodes, running, now)
	}
	return nil
}

// placeOne attempts to schedule a single job, falling back to preemption when
// enabled and warranted.
func (e *Engine) placeOne(ctx context.Context, job *models.Job, nodes []*models.Node,
	running map[string][]*models.Job, now time.Time) {
	start := time.Now()
	node, reason, ok := e.alg.Place(job, nodes)
	if ok {
		e.commitPlacement(ctx, job, node, reason, running, start)
		return
	}

	// Could not place directly. Consider preemption for high priority or
	// starving jobs to guarantee forward progress.
	if e.cfg.EnablePreemption && e.shouldPreempt(job, now) {
		if plan, found := FindPreemption(job, nodes, running); found {
			e.applyPreemption(ctx, plan)
			node, reason, ok = e.alg.Place(job, nodes)
			if ok {
				reason = "preemption: " + reason
				e.commitPlacement(ctx, job, node, reason, running, start)
				return
			}
		}
	}

	// Give up on this job for now; it stays queued and will be retried next tick.
	e.recordEvent(ctx, job, "", "no capacity: "+reason, false, start)
	if e.metrics != nil {
		e.metrics.FailedSchedules.Inc()
	}
}

// commitPlacement reserves resources in-memory and persists the placement.
func (e *Engine) commitPlacement(ctx context.Context, job *models.Job, node *models.Node,
	reason string, running map[string][]*models.Job, start time.Time) {
	req := job.ResourceRequest()
	node.Reserve(req)
	running[node.NodeID] = append(running[node.NodeID], job)

	if err := e.jobs.Schedule(ctx, job.JobID, node.NodeID); err != nil {
		e.log.Error("persist schedule failed", "job", job.JobID, "error", err)
		node.Release(req) // roll back in-memory reservation
		return
	}
	if err := e.nodes.UpdateAvailability(ctx, node.NodeID, node.GPUAvailable, node.CPUAvailable, node.MemoryAvailable); err != nil {
		e.log.Error("persist node availability failed", "node", node.NodeID, "error", err)
	}
	e.recordEvent(ctx, job, node.NodeID, reason, true, start)
	if e.metrics != nil {
		e.metrics.JobsTotal.WithLabelValues("scheduled").Inc()
	}
	e.log.Info("scheduled job", "job", job.JobID, "node", node.NodeID, "reason", reason)
}

// applyPreemption evicts the plan's victims, returning their resources.
func (e *Engine) applyPreemption(ctx context.Context, plan PreemptionPlan) {
	for _, v := range plan.Victims {
		plan.Node.Release(v.ResourceRequest())
		if err := e.jobs.Requeue(ctx, v.JobID, models.JobStatusPreempted); err != nil {
			e.log.Error("preempt requeue failed", "job", v.JobID, "error", err)
			continue
		}
		// Immediately return it to the queue so it competes again next tick.
		_ = e.jobs.UpdateStatus(ctx, v.JobID, models.JobStatusQueued)
		if e.metrics != nil {
			e.metrics.Preemptions.Inc()
			e.metrics.JobsTotal.WithLabelValues("preempted").Inc()
		}
		e.log.Info("preempted job", "job", v.JobID, "node", plan.Node.NodeID)
	}
	_ = e.nodes.UpdateAvailability(ctx, plan.Node.NodeID, plan.Node.GPUAvailable,
		plan.Node.CPUAvailable, plan.Node.MemoryAvailable)
}

// shouldPreempt reports whether a job earns the right to preempt: either it is
// high priority, or it has waited past the starvation threshold.
func (e *Engine) shouldPreempt(job *models.Job, now time.Time) bool {
	if job.Age(now) >= e.cfg.StarvationThreshold {
		return true
	}
	return job.Priority >= 8 // high-priority band always eligible
}

// reconcileDeadNodes marks nodes that stopped heartbeating as down and requeues
// the jobs that were running on them for automatic rescheduling.
func (e *Engine) reconcileDeadNodes(ctx context.Context) {
	dead, err := e.nodes.MarkStaleDown(ctx, e.cfg.HeartbeatTTL)
	if err != nil {
		e.log.Error("reconcile dead nodes failed", "error", err)
		return
	}
	if len(dead) == 0 {
		return
	}
	deadSet := make(map[string]bool, len(dead))
	for _, id := range dead {
		deadSet[id] = true
	}
	active, err := e.jobs.ListByStatuses(ctx, models.JobStatusScheduled, models.JobStatusRunning)
	if err != nil {
		return
	}
	for _, job := range active {
		if !deadSet[job.NodeID] {
			continue
		}
		if job.RetryCount >= job.MaxRetries {
			_ = e.jobs.UpdateStatus(ctx, job.JobID, models.JobStatusFailed)
			if e.metrics != nil {
				e.metrics.JobsTotal.WithLabelValues("failed").Inc()
			}
			e.log.Warn("job exhausted retries after node loss", "job", job.JobID, "node", job.NodeID)
			continue
		}
		if err := e.jobs.Requeue(ctx, job.JobID, models.JobStatusQueued); err == nil {
			e.log.Warn("rescheduling job from dead node", "job", job.JobID, "node", job.NodeID)
		}
	}
}

// teamShares computes each team's GPU consumption as a fraction of total cluster
// GPU capacity, for fair-share ordering.
func (e *Engine) teamShares(nodes []*models.Node, _ []*models.Job) map[string]float64 {
	total := 0
	for _, n := range nodes {
		total += n.GPUCapacity
	}
	usage, err := e.jobs.TeamGPUUsage(context.Background())
	if err != nil || total == 0 {
		return nil
	}
	shares := make(map[string]float64, len(usage))
	for team, gpus := range usage {
		shares[team] = float64(gpus) / float64(total)
	}
	return shares
}

// publishGauges updates Prometheus gauges describing current cluster state.
func (e *Engine) publishGauges(_ context.Context, nodes []*models.Node, queued, active []*models.Job) {
	if e.metrics == nil {
		return
	}
	// Queue depth broken down into low/normal/high priority bands.
	var low, normal, high int
	for _, j := range queued {
		switch {
		case j.Priority >= 8:
			high++
		case j.Priority >= 4:
			normal++
		default:
			low++
		}
	}
	e.metrics.QueueDepth.WithLabelValues("low").Set(float64(low))
	e.metrics.QueueDepth.WithLabelValues("normal").Set(float64(normal))
	e.metrics.QueueDepth.WithLabelValues("high").Set(float64(high))

	ready := 0
	for _, n := range nodes {
		if n.Status == models.NodeStatusReady {
			ready++
		}
		e.metrics.NodeGPUUtil.WithLabelValues(n.Hostname).Set(n.GPUUtilization())
		e.metrics.NodeCPUUtil.WithLabelValues(n.Hostname).Set(n.CPUUtilization())
		e.metrics.NodeMemUtil.WithLabelValues(n.Hostname).Set(n.MemoryUtilization())
	}
	e.metrics.NodesReady.Set(float64(ready))
	e.metrics.NodesTotal.Set(float64(len(nodes)))
	e.metrics.Fragmentation.Set(Fragmentation(nodes))
}

// recordEvent persists a scheduling decision and observes latency.
func (e *Engine) recordEvent(ctx context.Context, job *models.Job, nodeID, reason string, success bool, start time.Time) {
	latency := time.Since(start)
	if e.metrics != nil {
		e.metrics.SchedLatency.Observe(latency.Seconds())
	}
	ev := &models.SchedulingEvent{
		EventID:          uuid.NewString(),
		JobID:            job.JobID,
		SelectedNode:     nodeID,
		SchedulingReason: reason,
		Algorithm:        e.alg.Name(),
		LatencyMS:        float64(latency.Microseconds()) / 1000.0,
		Success:          success,
		Timestamp:        time.Now(),
	}
	if err := e.events.Record(ctx, ev); err != nil {
		e.log.Error("record scheduling event failed", "error", err)
	}
}
