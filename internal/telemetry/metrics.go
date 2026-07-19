// Package telemetry wires up Prometheus metrics and OpenTelemetry tracing for
// every service behind a small, dependency-injectable surface.
package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus collectors used by the platform. It is created
// once per process and injected into the components that record measurements.
type Metrics struct {
	reg *prometheus.Registry

	// Scheduler metrics.
	JobsTotal       *prometheus.CounterVec
	QueueDepth      *prometheus.GaugeVec
	SchedLatency    prometheus.Histogram
	FailedSchedules prometheus.Counter
	Preemptions     prometheus.Counter
	SchedRuns       prometheus.Counter

	// Node metrics.
	NodeGPUUtil   *prometheus.GaugeVec
	NodeCPUUtil   *prometheus.GaugeVec
	NodeMemUtil   *prometheus.GaugeVec
	NodesReady    prometheus.Gauge
	NodesTotal    prometheus.Gauge
	Fragmentation prometheus.Gauge

	// API metrics.
	HTTPRequests *prometheus.CounterVec
	HTTPLatency  *prometheus.HistogramVec
}

// NewMetrics registers all collectors on a fresh registry and returns the
// Metrics handle plus the registry to serve on /metrics.
func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	factory := promauto.With(reg)

	return &Metrics{
		reg: reg,
		JobsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "scheduler_jobs_total",
			Help: "Total number of jobs processed by the scheduler, by terminal status.",
		}, []string{"status"}),
		QueueDepth: factory.NewGaugeVec(prometheus.GaugeOpts{
			Name: "scheduler_queue_depth",
			Help: "Current number of jobs waiting in the scheduling queue, by priority band.",
		}, []string{"band"}),
		SchedLatency: factory.NewHistogram(prometheus.HistogramOpts{
			Name:    "scheduler_latency_seconds",
			Help:    "Time taken to make a placement decision for a job.",
			Buckets: []float64{0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5},
		}),
		FailedSchedules: factory.NewCounter(prometheus.CounterOpts{
			Name: "failed_schedules_total",
			Help: "Total number of scheduling attempts that could not place a job.",
		}),
		Preemptions: factory.NewCounter(prometheus.CounterOpts{
			Name: "scheduler_preemptions_total",
			Help: "Total number of jobs preempted to make room for higher priority work.",
		}),
		SchedRuns: factory.NewCounter(prometheus.CounterOpts{
			Name: "scheduler_runs_total",
			Help: "Total number of scheduling loop iterations executed.",
		}),
		NodeGPUUtil: factory.NewGaugeVec(prometheus.GaugeOpts{
			Name: "node_gpu_utilization",
			Help: "Per-node GPU utilization in the range [0,1].",
		}, []string{"node"}),
		NodeCPUUtil: factory.NewGaugeVec(prometheus.GaugeOpts{
			Name: "node_cpu_utilization",
			Help: "Per-node CPU utilization in the range [0,1].",
		}, []string{"node"}),
		NodeMemUtil: factory.NewGaugeVec(prometheus.GaugeOpts{
			Name: "node_memory_utilization",
			Help: "Per-node memory utilization in the range [0,1].",
		}, []string{"node"}),
		NodesReady: factory.NewGauge(prometheus.GaugeOpts{
			Name: "cluster_nodes_ready",
			Help: "Number of nodes currently in the ready state.",
		}),
		NodesTotal: factory.NewGauge(prometheus.GaugeOpts{
			Name: "cluster_nodes_total",
			Help: "Total number of registered nodes.",
		}),
		Fragmentation: factory.NewGauge(prometheus.GaugeOpts{
			Name: "cluster_gpu_fragmentation",
			Help: "GPU fragmentation index in [0,1]; higher means more scattered free GPUs.",
		}),
		HTTPRequests: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests handled by the API gateway.",
		}, []string{"method", "route", "status"}),
		HTTPLatency: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency by route.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "route"}),
	}
}

// Registry exposes the underlying Prometheus registry for the metrics handler.
func (m *Metrics) Registry() *prometheus.Registry {
	return m.reg
}
