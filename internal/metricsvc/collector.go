// Package metricsvc implements the metrics service: a control-plane exporter
// that periodically reads cluster state from PostgreSQL and publishes aggregate
// Prometheus metrics (utilization, fragmentation, job runtime statistics) that
// complement the per-service metrics scraped elsewhere.
package metricsvc

import (
	"context"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/models"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/scheduler"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/store"
)

// Collector periodically samples cluster state and exposes it as metrics.
type Collector struct {
	store    *store.Store
	log      *slog.Logger
	interval time.Duration
	reg      *prometheus.Registry

	gpuUtil       prometheus.Gauge
	cpuUtil       prometheus.Gauge
	memUtil       prometheus.Gauge
	fragmentation prometheus.Gauge
	nodesReady    prometheus.Gauge
	nodesTotal    prometheus.Gauge
	jobsByStatus  *prometheus.GaugeVec
	avgRuntime    prometheus.Gauge
}

// New builds a Collector with its own Prometheus registry.
func New(s *store.Store, interval time.Duration, log *slog.Logger) *Collector {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	reg := prometheus.NewRegistry()
	factory := prometheus.WrapRegistererWith(prometheus.Labels{}, reg)

	c := &Collector{
		store:         s,
		log:           log,
		interval:      interval,
		reg:           reg,
		gpuUtil:       newGauge(factory, "cluster_gpu_utilization_ratio", "Cluster-wide GPU utilization in [0,1]."),
		cpuUtil:       newGauge(factory, "cluster_cpu_utilization_ratio", "Cluster-wide CPU utilization in [0,1]."),
		memUtil:       newGauge(factory, "cluster_memory_utilization_ratio", "Cluster-wide memory utilization in [0,1]."),
		fragmentation: newGauge(factory, "cluster_fragmentation_index", "GPU fragmentation index in [0,1]."),
		nodesReady:    newGauge(factory, "cluster_nodes_ready_count", "Number of ready nodes."),
		nodesTotal:    newGauge(factory, "cluster_nodes_total_count", "Total number of nodes."),
		avgRuntime:    newGauge(factory, "job_runtime_seconds_avg", "Average runtime of recently completed jobs."),
		jobsByStatus: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "cluster_jobs_by_status",
			Help: "Number of jobs in each lifecycle status.",
		}, []string{"status"}),
	}
	reg.MustRegister(c.jobsByStatus)
	return c
}

func newGauge(r prometheus.Registerer, name, help string) prometheus.Gauge {
	g := prometheus.NewGauge(prometheus.GaugeOpts{Name: name, Help: help})
	r.MustRegister(g)
	return g
}

// Registry exposes the collector's registry for the /metrics handler.
func (c *Collector) Registry() *prometheus.Registry { return c.reg }

// Run samples state on a ticker until ctx is cancelled.
func (c *Collector) Run(ctx context.Context) error {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	c.log.Info("metrics collector started", "interval", c.interval)
	c.sample(ctx) // initial sample so /metrics is populated immediately
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			c.sample(ctx)
		}
	}
}

func (c *Collector) sample(ctx context.Context) {
	nodes, err := c.store.Nodes.List(ctx)
	if err != nil {
		c.log.Warn("sample nodes failed", "error", err)
		return
	}
	running, _ := c.store.Jobs.CountByStatus(ctx, models.JobStatusRunning)
	scheduled, _ := c.store.Jobs.CountByStatus(ctx, models.JobStatusScheduled)
	queued, _ := c.store.Jobs.CountByStatus(ctx, models.JobStatusQueued)
	snap := scheduler.ClusterSnapshot(nodes, running+scheduled, queued)

	c.gpuUtil.Set(snap.GPUUtilization)
	c.cpuUtil.Set(snap.CPUUtilization)
	c.memUtil.Set(snap.MemoryUtilization)
	c.fragmentation.Set(snap.Fragmentation)
	c.nodesReady.Set(float64(snap.NodesReady))
	c.nodesTotal.Set(float64(snap.NodesTotal))

	for _, status := range []models.JobStatus{
		models.JobStatusQueued, models.JobStatusScheduled, models.JobStatusRunning,
		models.JobStatusCompleted, models.JobStatusFailed, models.JobStatusCancelled,
		models.JobStatusPreempted,
	} {
		n, _ := c.store.Jobs.CountByStatus(ctx, status)
		c.jobsByStatus.WithLabelValues(string(status)).Set(float64(n))
	}

	c.updateRuntime(ctx)
}

// updateRuntime computes the average runtime of recently completed jobs.
func (c *Collector) updateRuntime(ctx context.Context) {
	completed, err := c.store.Jobs.List(ctx, store.JobFilter{Status: models.JobStatusCompleted, Limit: 100})
	if err != nil || len(completed) == 0 {
		return
	}
	var total time.Duration
	var n int
	for _, j := range completed {
		if j.StartedAt != nil && j.CompletedAt != nil {
			total += j.CompletedAt.Sub(*j.StartedAt)
			n++
		}
	}
	if n > 0 {
		c.avgRuntime.Set(total.Seconds() / float64(n))
	}
}
