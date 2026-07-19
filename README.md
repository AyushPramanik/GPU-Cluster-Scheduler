# GPU Cluster Scheduler

A production-grade, distributed GPU cluster scheduler inspired by Kubernetes, Slurm,
and the internal ML-infrastructure platforms used at large AI companies. It lets ML
engineers submit training and inference workloads, schedules them across a fleet of
heterogeneous GPU nodes using pluggable placement algorithms, tracks execution,
and exposes production-grade observability into cluster health and scheduling
decisions.

---

## Architecture

```
                          ┌────────────────────┐
        REST / JSON       │    Next.js UI       │
   ┌─────────────────────▶│  (dashboard :3000)  │
   │                      └─────────┬──────────┘
   │                                │ REST
┌──┴───────────┐   Postgres   ┌─────▼──────────┐   gRPC   ┌───────────────┐
│ API Gateway  │◀────────────▶│   Scheduler    │◀────────▶│  Node Agent(s) │
│  (Gin :8080) │              │ (leader-elected│          │  register/     │
│ auth,        │    Redis     │  control loop, │          │  heartbeat/    │
│ rate-limit,  │◀────────────▶│  NodeService   │          │  run/stream    │
│ tracing      │  (leader     │  gRPC :50051)  │          │  logs          │
└──────┬───────┘   election)  └───────┬────────┘          └───────┬───────┘
       │                              │                            │
       │        ┌─────────────────────▼────────────────────┐      │
       └───────▶│  PostgreSQL (jobs, nodes, events, quotas) │◀─────┘
                └───────────────────────────────────────────┘
     Metrics service polls state ─▶ Prometheus ─▶ Grafana ; OTel ─▶ Collector ─▶ Jaeger
```

### Services

| Service        | Responsibility                                                                 | Ports |
|----------------|--------------------------------------------------------------------------------|-------|
| **api-gateway**| REST API, auth (JWT), rate limiting, request tracing, cluster views            | 8080 / 9090 |
| **scheduler**  | Leader-elected scheduling loop, placement, preemption, gRPC control plane      | 50051 / 9091 |
| **node-agent** | Runs per node: register, heartbeat, execute workloads, stream logs             | 50061 / 9092 |
| **metrics**    | Polls cluster state and exports aggregate Prometheus metrics                    | 9093 |

## Scheduling

Four pluggable algorithms, switchable via `SCHEDULER_ALGORITHM`:

| Algorithm    | Job ordering                              | Node placement | Use case |
|--------------|-------------------------------------------|----------------|----------|
| `first-fit`  | FIFO                                      | first node that fits | cheapest, simplest |
| `best-fit`   | FIFO                                      | tightest pack (min leftover) | reduce fragmentation |
| `priority`   | effective priority (base + queue aging)   | best-fit       | mixed-priority queues |
| `fair-share` | lowest team dominant-share first          | best-fit       | multi-team clusters |

Advanced features implemented:

- **GPU bin packing** and a **fragmentation index** (`1 − largest_free_block / total_free`)
- **Queue aging** so long-waiting jobs gain priority and never starve
- **Starvation prevention** + **preemption**: high-priority or starving jobs evict the
  fewest / lowest-priority running jobs to make room
- **Node draining / cordoning** to gracefully remove nodes for maintenance
- **Retry policies** with a per-job retry budget
- **Automatic recovery**: nodes that miss heartbeats are marked down and their jobs
  are rescheduled

## Fault tolerance

- **Leader election** over Redis (fenced lease with owner-checked renew/release):
  run multiple scheduler replicas; only the leader schedules, failover is automatic.
- **Heartbeats + TTL**: dead nodes are detected and their work rescheduled.
- **Graceful shutdown** across every service on SIGINT/SIGTERM.
- Simulate failures locally by killing an agent (`docker compose stop node-agent-1`)
  or the leader (`docker compose restart scheduler`) and watch recovery.

## Quick start

```bash
# Bring up the entire stack (Postgres, Redis, all services, Prometheus, Grafana,
# OTel collector, Jaeger, and the dashboard):
make run

# Dashboard   → http://localhost:3000
# REST API    → http://localhost:8080
# Grafana     → http://localhost:3001  (admin/admin)
# Prometheus  → http://localhost:9099
# Jaeger      → http://localhost:16686
```

Submit a job:

```bash
curl -sX POST localhost:8080/api/v1/jobs -H 'content-type: application/json' -d '{
  "name": "resnet-train", "user_id": "ayush", "team_id": "research",
  "priority": 7, "gpu_count": 2, "cpu_count": 8, "memory_gb": 64,
  "image": "pytorch/pytorch:2.3.0", "command": "python train.py"
}' | jq
```

## API

| Method | Path                              | Description |
|--------|-----------------------------------|-------------|
| POST   | `/api/v1/jobs`                    | Submit a job |
| GET    | `/api/v1/jobs`                    | List jobs (`?status=&user_id=&team_id=&limit=`) |
| GET    | `/api/v1/jobs/:id`                | Get a job |
| DELETE | `/api/v1/jobs/:id`                | Cancel a job (releases resources) |
| GET    | `/api/v1/jobs/:id/logs`           | Recent job logs (from Redis cache) |
| GET    | `/api/v1/nodes`                   | Node inventory |
| POST   | `/api/v1/nodes/:id/cordon`        | Mark node unschedulable |
| POST   | `/api/v1/nodes/:id/drain`         | Drain node |
| POST   | `/api/v1/nodes/:id/uncordon`      | Return node to service |
| GET    | `/api/v1/cluster/utilization`     | Aggregate cluster utilization |
| GET    | `/api/v1/scheduling-events`       | Recent scheduling decisions |
| GET    | `/healthz`, `/readyz`             | Liveness / readiness |
| GET    | `/metrics` (:9090)                | Prometheus metrics |

## Observability

Prometheus metrics include `scheduler_jobs_total`, `scheduler_queue_depth`,
`scheduler_latency_seconds`, `node_gpu_utilization`, `node_cpu_utilization`,
`node_memory_utilization`, `failed_schedules_total`, `scheduler_preemptions_total`,
plus cluster aggregates from the metrics service. A provisioned Grafana dashboard
(`deploy/grafana/dashboards/gpu-scheduler.json`) visualises utilization, queue
depth, throughput, scheduling latency quantiles, node health, and fragmentation.
Every service is instrumented with OpenTelemetry; traces flow to the collector and
Jaeger.

## Development

```bash
make build            # compile all binaries to ./bin
make test             # unit tests with race detector + coverage
make test-integration # end-to-end tests (needs Postgres + Redis)
make dev              # datastores + observability only; run services with `go run`
make proto            # regenerate gRPC code
make lint             # golangci-lint
make load-test        # k6 load test against the API
```

Run a single service against local datastores:

```bash
make dev
go run ./cmd/scheduler
go run ./cmd/api-gateway
NODE_ID=gpu-01 NODE_GPU_CAPACITY=8 go run ./cmd/node-agent
```

## Layout

```
cmd/                 service entrypoints (api-gateway, scheduler, node-agent, metrics)
internal/
  api/               Gin REST server, handlers, middleware, rate limiting
  agent/             node agent: register/heartbeat/execute/stream
  config/            env-based configuration
  grpc/              NodeService gRPC server + generated clusterpb
  logging/           structured slog helpers
  metricsvc/         metrics service collector
  models/            domain types (Job, Node, SchedulingEvent, quotas)
  redisx/            Redis client + leader election
  runtimex/          signal handling, run groups
  scheduler/         algorithms, engine, preemption, fragmentation
  store/             PostgreSQL repositories + embedded migrations
  telemetry/         Prometheus metrics + OpenTelemetry tracing
proto/               protobuf definitions
migrations/          SQL schema + seed
deploy/              Dockerfiles, k8s manifests, Prometheus, Grafana, OTel
terraform/           local kind cluster provisioning
web/                 Next.js + TypeScript + Tailwind dashboard
scripts/             k6 load test
```

## Tech stack

Go · Gin · gRPC · PostgreSQL · Redis · Prometheus · Grafana · OpenTelemetry ·
Docker · Kubernetes · Terraform · GitHub Actions · Next.js · TypeScript · Tailwind
