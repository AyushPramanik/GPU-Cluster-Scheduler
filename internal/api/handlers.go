package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	grpcsrv "github.com/ayushpramanik/gpu-cluster-scheduler/internal/grpc"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/models"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/scheduler"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/store"
)

// handleSubmitJob validates and enqueues a new job.
func (s *Server) handleSubmitJob(c *gin.Context) {
	var req models.SubmitJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}
	if err := validateSubmit(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	maxRetries := req.MaxRetries
	if maxRetries <= 0 {
		maxRetries = s.cfg.Scheduler.MaxRetries
	}
	job := &models.Job{
		JobID:      uuid.NewString(),
		Name:       req.Name,
		UserID:     req.UserID,
		TeamID:     req.TeamID,
		Status:     models.JobStatusQueued,
		Priority:   clamp(req.Priority, 0, 10),
		GPUCount:   req.GPUCount,
		CPUCount:   req.CPUCount,
		MemoryGB:   req.MemoryGB,
		Image:      req.Image,
		Command:    req.Command,
		MaxRetries: maxRetries,
		CreatedAt:  time.Now().UTC(),
	}
	if err := s.store.Jobs.Create(c.Request.Context(), job); err != nil {
		s.log.Error("create job failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create job"})
		return
	}
	if s.metrics != nil {
		s.metrics.JobsTotal.WithLabelValues("submitted").Inc()
	}
	c.JSON(http.StatusCreated, job)
}

func validateSubmit(r *models.SubmitJobRequest) error {
	if r.UserID == "" {
		return errors.New("user_id is required")
	}
	if r.GPUCount < 0 || r.CPUCount < 0 || r.MemoryGB < 0 {
		return errors.New("resource requests must be non-negative")
	}
	if r.GPUCount == 0 && r.CPUCount == 0 {
		return errors.New("job must request at least one GPU or CPU")
	}
	if r.GPUCount > 64 {
		return errors.New("gpu_count exceeds maximum of 64")
	}
	return nil
}

// handleListJobs returns jobs, optionally filtered by status/user.
func (s *Server) handleListJobs(c *gin.Context) {
	filter := store.JobFilter{
		Status: models.JobStatus(c.Query("status")),
		UserID: c.Query("user_id"),
		TeamID: c.Query("team_id"),
		Limit:  atoiDefault(c.Query("limit"), 200),
	}
	jobs, err := s.store.Jobs.List(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list jobs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"jobs": nonNilJobs(jobs)})
}

// handleGetJob returns a single job by ID.
func (s *Server) handleGetJob(c *gin.Context) {
	job, err := s.store.Jobs.Get(c.Request.Context(), c.Param("id"))
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch job"})
		return
	}
	c.JSON(http.StatusOK, job)
}

// handleCancelJob cancels a job, releasing its resources if it was placed.
func (s *Server) handleCancelJob(c *gin.Context) {
	ctx := c.Request.Context()
	job, err := s.store.Jobs.Get(ctx, c.Param("id"))
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch job"})
		return
	}
	if job.Status.IsTerminal() {
		c.JSON(http.StatusConflict, gin.H{"error": "job already in terminal state", "status": job.Status})
		return
	}
	if job.Status.IsActive() && job.NodeID != "" {
		if node, err := s.store.Nodes.Get(ctx, job.NodeID); err == nil {
			node.Release(job.ResourceRequest())
			_ = s.store.Nodes.UpdateAvailability(ctx, node.NodeID, node.GPUAvailable, node.CPUAvailable, node.MemoryAvailable)
		}
	}
	if err := s.store.Jobs.UpdateStatus(ctx, job.JobID, models.JobStatusCancelled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to cancel job"})
		return
	}
	if s.metrics != nil {
		s.metrics.JobsTotal.WithLabelValues("cancelled").Inc()
	}
	c.JSON(http.StatusOK, gin.H{"status": "cancelled"})
}

// handleJobLogs returns cached recent log lines for a job.
func (s *Server) handleJobLogs(c *gin.Context) {
	limit := int64(atoiDefault(c.Query("limit"), 200))
	lines, err := grpcsrv.FetchLogs(c.Request.Context(), s.redis, c.Param("id"), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch logs"})
		return
	}
	if lines == nil {
		lines = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"job_id": c.Param("id"), "lines": lines})
}

// handleListNodes returns the node inventory.
func (s *Server) handleListNodes(c *gin.Context) {
	nodes, err := s.store.Nodes.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list nodes"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"nodes": nonNilNodes(nodes)})
}

func (s *Server) handleCordonNode(c *gin.Context)   { s.setNodeStatus(c, models.NodeStatusCordoned) }
func (s *Server) handleDrainNode(c *gin.Context)    { s.setNodeStatus(c, models.NodeStatusDraining) }
func (s *Server) handleUncordonNode(c *gin.Context) { s.setNodeStatus(c, models.NodeStatusReady) }

func (s *Server) setNodeStatus(c *gin.Context, status models.NodeStatus) {
	if _, err := s.store.Nodes.Get(c.Request.Context(), c.Param("id")); errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}
	if err := s.store.Nodes.SetStatus(c.Request.Context(), c.Param("id"), status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update node"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"node_id": c.Param("id"), "status": status})
}

// handleUtilization returns an aggregate cluster snapshot.
func (s *Server) handleUtilization(c *gin.Context) {
	ctx := c.Request.Context()
	nodes, err := s.store.Nodes.List(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load nodes"})
		return
	}
	running, _ := s.store.Jobs.CountByStatus(ctx, models.JobStatusRunning)
	scheduled, _ := s.store.Jobs.CountByStatus(ctx, models.JobStatusScheduled)
	queued, _ := s.store.Jobs.CountByStatus(ctx, models.JobStatusQueued)
	snap := scheduler.ClusterSnapshot(nodes, running+scheduled, queued)
	c.JSON(http.StatusOK, snap)
}

// handleEvents returns recent scheduling decisions.
func (s *Server) handleEvents(c *gin.Context) {
	events, err := s.store.Events.List(c.Request.Context(), atoiDefault(c.Query("limit"), 100))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list events"})
		return
	}
	if events == nil {
		events = []*models.SchedulingEvent{}
	}
	c.JSON(http.StatusOK, gin.H{"events": events})
}

// --- helpers ---

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	if n, err := strconv.Atoi(s); err == nil && n > 0 {
		return n
	}
	return def
}

func nonNilJobs(j []*models.Job) []*models.Job {
	if j == nil {
		return []*models.Job{}
	}
	return j
}

func nonNilNodes(n []*models.Node) []*models.Node {
	if n == nil {
		return []*models.Node{}
	}
	return n
}
