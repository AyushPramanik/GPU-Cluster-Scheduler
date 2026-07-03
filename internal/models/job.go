package models

import "time"

// JobStatus represents the lifecycle state of a job.
type JobStatus string

const (
	JobStatusQueued    JobStatus = "queued"
	JobStatusScheduled JobStatus = "scheduled"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
	JobStatusPreempted JobStatus = "preempted"
)

// IsTerminal reports whether the status is a final state that will not change.
func (s JobStatus) IsTerminal() bool {
	switch s {
	case JobStatusCompleted, JobStatusFailed, JobStatusCancelled:
		return true
	default:
		return false
	}
}

// IsActive reports whether the job currently occupies cluster resources.
func (s JobStatus) IsActive() bool {
	return s == JobStatusScheduled || s == JobStatusRunning
}

// Job is a unit of GPU work submitted by a user.
type Job struct {
	JobID       string     `json:"job_id"`
	Name        string     `json:"name"`
	UserID      string     `json:"user_id"`
	TeamID      string     `json:"team_id,omitempty"`
	Status      JobStatus  `json:"status"`
	Priority    int        `json:"priority"`
	GPUCount    int        `json:"gpu_count"`
	CPUCount    int        `json:"cpu_count"`
	MemoryGB    int        `json:"memory_gb"`
	Image       string     `json:"image"`
	Command     string     `json:"command"`
	NodeID      string     `json:"node_id"`
	RetryCount  int        `json:"retry_count"`
	MaxRetries  int        `json:"max_retries"`
	CostPerHour float64    `json:"cost_per_hour,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	// EffectivePriority is derived at scheduling time from Priority plus queue
	// aging; it is not persisted directly.
	EffectivePriority int `json:"effective_priority,omitempty"`
}

// ResourceRequest is the resource footprint a job needs to be placed.
func (j *Job) ResourceRequest() ResourceRequest {
	return ResourceRequest{
		GPU:      j.GPUCount,
		CPU:      j.CPUCount,
		MemoryGB: j.MemoryGB,
	}
}

// Age returns how long the job has been waiting since submission.
func (j *Job) Age(now time.Time) time.Duration {
	return now.Sub(j.CreatedAt)
}

// ResourceRequest describes the resources required to place a job.
type ResourceRequest struct {
	GPU      int `json:"gpu"`
	CPU      int `json:"cpu"`
	MemoryGB int `json:"memory_gb"`
}

// SubmitJobRequest is the API payload for creating a new job.
type SubmitJobRequest struct {
	Name       string `json:"name"`
	UserID     string `json:"user_id"`
	TeamID     string `json:"team_id"`
	Priority   int    `json:"priority"`
	GPUCount   int    `json:"gpu_count"`
	CPUCount   int    `json:"cpu_count"`
	MemoryGB   int    `json:"memory_gb"`
	Image      string `json:"image"`
	Command    string `json:"command"`
	MaxRetries int    `json:"max_retries"`
}
