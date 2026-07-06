// Package grpc implements the control-plane gRPC server (NodeService) that node
// agents call to register, heartbeat, fetch assignments, and report status.
package grpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	pb "github.com/ayushpramanik/gpu-cluster-scheduler/internal/grpc/clusterpb"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/models"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/store"
	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/telemetry"
)

// NodeServer implements the NodeService gRPC service on the scheduler.
type NodeServer struct {
	pb.UnimplementedNodeServiceServer
	store   *store.Store
	redis   *redis.Client
	metrics *telemetry.Metrics
	log     *slog.Logger
}

// NewNodeServer builds the gRPC server handler.
func NewNodeServer(s *store.Store, rdb *redis.Client, m *telemetry.Metrics, log *slog.Logger) *NodeServer {
	return &NodeServer{store: s, redis: rdb, metrics: m, log: log}
}

// Register registers or re-registers a node with its capacity.
func (s *NodeServer) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	cap := req.GetCapacity()
	node := &models.Node{
		NodeID:          req.GetNodeId(),
		Hostname:        req.GetHostname(),
		Status:          models.NodeStatusReady,
		GPUCapacity:     int(cap.GetGpu()),
		GPUAvailable:    int(cap.GetGpu()),
		CPUCapacity:     int(cap.GetCpu()),
		CPUAvailable:    int(cap.GetCpu()),
		MemoryCapacity:  int(cap.GetMemoryGb()),
		MemoryAvailable: int(cap.GetMemoryGb()),
		GPUModel:        req.GetGpuModel(),
		CostPerHour:     req.GetCostPerHour(),
		Spot:            req.GetSpot(),
		Labels:          req.GetLabels(),
	}
	if err := s.store.Nodes.Upsert(ctx, node); err != nil {
		return nil, fmt.Errorf("register node: %w", err)
	}
	s.log.Info("node registered", "node", node.NodeID, "gpus", node.GPUCapacity)
	return &pb.RegisterResponse{Accepted: true, Message: "registered"}, nil
}

// Heartbeat refreshes node liveness and returns any desired status change.
func (s *NodeServer) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	if err := s.store.Nodes.Heartbeat(ctx, req.GetNodeId()); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return &pb.HeartbeatResponse{DesiredStatus: "unregistered"}, nil
		}
		return nil, err
	}
	desired := ""
	if n, err := s.store.Nodes.Get(ctx, req.GetNodeId()); err == nil {
		desired = string(n.Status)
	}
	return &pb.HeartbeatResponse{DesiredStatus: desired}, nil
}

// GetAssignments returns jobs placed on the node awaiting execution.
func (s *NodeServer) GetAssignments(ctx context.Context, req *pb.AssignmentsRequest) (*pb.AssignmentsResponse, error) {
	jobs, err := s.store.Jobs.ListScheduledForNode(ctx, req.GetNodeId())
	if err != nil {
		return nil, err
	}
	resp := &pb.AssignmentsResponse{}
	for _, j := range jobs {
		resp.Jobs = append(resp.Jobs, &pb.Job{
			JobId:    j.JobID,
			Name:     j.Name,
			Image:    j.Image,
			Command:  j.Command,
			Request:  &pb.Resources{Gpu: int32(j.GPUCount), Cpu: int32(j.CPUCount), MemoryGb: int32(j.MemoryGB)},
			Priority: int32(j.Priority),
		})
	}
	return resp, nil
}

// UpdateJobStatus applies a lifecycle transition reported by the agent. On
// terminal states the job's resources are released back to its node.
func (s *NodeServer) UpdateJobStatus(ctx context.Context, req *pb.JobStatusUpdate) (*pb.Ack, error) {
	status := models.JobStatus(req.GetStatus())
	job, err := s.store.Jobs.Get(ctx, req.GetJobId())
	if err != nil {
		return nil, err
	}

	switch status {
	case models.JobStatusRunning:
		if err := s.store.Jobs.UpdateStatus(ctx, job.JobID, models.JobStatusRunning); err != nil {
			return nil, err
		}
	case models.JobStatusCompleted, models.JobStatusFailed:
		if err := s.releaseAndFinalize(ctx, job, status); err != nil {
			return nil, err
		}
		if s.metrics != nil {
			s.metrics.JobsTotal.WithLabelValues(string(status)).Inc()
		}
	default:
		return nil, fmt.Errorf("unsupported status transition %q", status)
	}
	s.log.Info("job status updated", "job", job.JobID, "status", status, "node", req.GetNodeId())
	return &pb.Ack{Ok: true}, nil
}

// releaseAndFinalize returns a finished job's resources to its node and records
// the terminal status.
func (s *NodeServer) releaseAndFinalize(ctx context.Context, job *models.Job, status models.JobStatus) error {
	if job.NodeID != "" {
		if node, err := s.store.Nodes.Get(ctx, job.NodeID); err == nil {
			node.Release(job.ResourceRequest())
			if err := s.store.Nodes.UpdateAvailability(ctx, node.NodeID,
				node.GPUAvailable, node.CPUAvailable, node.MemoryAvailable); err != nil {
				s.log.Error("release resources failed", "node", node.NodeID, "error", err)
			}
		}
	}
	return s.store.Jobs.UpdateStatus(ctx, job.JobID, status)
}

// StreamLogs ingests a stream of log lines from an agent and caches the most
// recent lines per job in Redis for the API to serve.
func (s *NodeServer) StreamLogs(stream pb.NodeService_StreamLogsServer) error {
	ctx := stream.Context()
	count := 0
	for {
		line, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return stream.SendAndClose(&pb.Ack{Ok: true})
		}
		if err != nil {
			return err
		}
		count++
		if s.redis != nil {
			key := logKey(line.GetJobId())
			pipe := s.redis.Pipeline()
			pipe.RPush(ctx, key, formatLog(line))
			pipe.LTrim(ctx, key, -500, -1) // keep the last 500 lines
			pipe.Expire(ctx, key, 24*time.Hour)
			if _, err := pipe.Exec(ctx); err != nil {
				s.log.Warn("cache log line failed", "error", err)
			}
		}
	}
}

func logKey(jobID string) string { return "logs:" + jobID }

func formatLog(l *pb.LogLine) string {
	ts := time.UnixMilli(l.GetTimestampUnixMs()).UTC().Format(time.RFC3339)
	return fmt.Sprintf("%s %s", ts, l.GetLine())
}

// FetchLogs returns the cached recent log lines for a job.
func FetchLogs(ctx context.Context, rdb *redis.Client, jobID string, limit int64) ([]string, error) {
	if rdb == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 200
	}
	return rdb.LRange(ctx, logKey(jobID), -limit, -1).Result()
}
