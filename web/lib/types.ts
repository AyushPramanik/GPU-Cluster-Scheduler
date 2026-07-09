// Shared domain types mirroring the Go REST API contract.

export type JobStatus =
  | "queued"
  | "scheduled"
  | "running"
  | "completed"
  | "failed"
  | "cancelled"
  | "preempted";

export type NodeStatus = "ready" | "draining" | "cordoned" | "down";

export interface Job {
  job_id: string;
  name: string;
  user_id: string;
  status: JobStatus;
  priority: number;
  gpu_count: number;
  cpu_count: number;
  memory_gb: number;
  image: string;
  command: string;
  node_id: string;
  created_at: string;
  started_at: string | null;
  completed_at: string | null;
  retry_count: number;
}

export interface ClusterNode {
  node_id: string;
  hostname: string;
  status: NodeStatus;
  gpu_capacity: number;
  gpu_available: number;
  cpu_capacity: number;
  cpu_available: number;
  memory_capacity: number;
  memory_available: number;
  last_heartbeat: string;
  labels: Record<string, string> | null;
}

export interface ClusterUtilization {
  total_gpus: number;
  used_gpus: number;
  total_cpus: number;
  used_cpus: number;
  total_memory_gb: number;
  used_memory_gb: number;
  gpu_utilization: number;
  cpu_utilization: number;
  memory_utilization: number;
  nodes_ready: number;
  nodes_total: number;
  jobs_running: number;
  jobs_queued: number;
  fragmentation: number;
}

export interface SchedulingEvent {
  event_id: string;
  job_id: string;
  selected_node: string;
  scheduling_reason: string;
  algorithm: string;
  latency_ms: number;
  timestamp: string;
}

// Request payload for creating a job.
export interface CreateJobRequest {
  name: string;
  user_id: string;
  priority: number;
  gpu_count: number;
  cpu_count: number;
  memory_gb: number;
  image: string;
  command: string;
}

// API envelope response shapes.
export interface JobsResponse {
  jobs: Job[];
}

export interface NodesResponse {
  nodes: ClusterNode[];
}

export interface SchedulingEventsResponse {
  events: SchedulingEvent[];
}

export interface CancelJobResponse {
  status: "cancelled";
}

export interface JobsQuery {
  status?: JobStatus | "";
  user_id?: string;
  limit?: number;
}
