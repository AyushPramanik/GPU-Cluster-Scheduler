package models

import "time"

// SchedulingEvent records a single scheduling decision for auditability and
// observability into why a job landed (or failed to land) on a node.
type SchedulingEvent struct {
	EventID          string    `json:"event_id"`
	JobID            string    `json:"job_id"`
	SelectedNode     string    `json:"selected_node"`
	SchedulingReason string    `json:"scheduling_reason"`
	Algorithm        string    `json:"algorithm"`
	LatencyMS        float64   `json:"latency_ms"`
	Success          bool      `json:"success"`
	Timestamp        time.Time `json:"timestamp"`
}

// ClusterUtilization is an aggregate snapshot of cluster resource usage.
type ClusterUtilization struct {
	TotalGPUs         int     `json:"total_gpus"`
	UsedGPUs          int     `json:"used_gpus"`
	TotalCPUs         int     `json:"total_cpus"`
	UsedCPUs          int     `json:"used_cpus"`
	TotalMemoryGB     int     `json:"total_memory_gb"`
	UsedMemoryGB      int     `json:"used_memory_gb"`
	GPUUtilization    float64 `json:"gpu_utilization"`
	CPUUtilization    float64 `json:"cpu_utilization"`
	MemoryUtilization float64 `json:"memory_utilization"`
	NodesReady        int     `json:"nodes_ready"`
	NodesTotal        int     `json:"nodes_total"`
	JobsRunning       int     `json:"jobs_running"`
	JobsQueued        int     `json:"jobs_queued"`
	Fragmentation     float64 `json:"fragmentation"`
}

// TeamQuota caps the resources a team may consume concurrently.
type TeamQuota struct {
	TeamID       string `json:"team_id"`
	MaxGPUs      int    `json:"max_gpus"`
	MaxCPUs      int    `json:"max_cpus"`
	MaxMemoryGB  int    `json:"max_memory_gb"`
	UsedGPUs     int    `json:"used_gpus"`
	UsedCPUs     int    `json:"used_cpus"`
	UsedMemoryGB int    `json:"used_memory_gb"`
}

// HasCapacityFor reports whether the team can admit the request without
// exceeding its quota.
func (q *TeamQuota) HasCapacityFor(r ResourceRequest) bool {
	return q.UsedGPUs+r.GPU <= q.MaxGPUs &&
		q.UsedCPUs+r.CPU <= q.MaxCPUs &&
		q.UsedMemoryGB+r.MemoryGB <= q.MaxMemoryGB
}
