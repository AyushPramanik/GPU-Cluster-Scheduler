-- Core schema for the GPU cluster scheduler.

CREATE TABLE IF NOT EXISTS nodes (
    node_id          TEXT PRIMARY KEY,
    hostname         TEXT NOT NULL,
    status           TEXT NOT NULL DEFAULT 'ready',
    gpu_capacity     INTEGER NOT NULL DEFAULT 0,
    gpu_available    INTEGER NOT NULL DEFAULT 0,
    cpu_capacity     INTEGER NOT NULL DEFAULT 0,
    cpu_available    INTEGER NOT NULL DEFAULT 0,
    memory_capacity  INTEGER NOT NULL DEFAULT 0,
    memory_available INTEGER NOT NULL DEFAULT 0,
    gpu_model        TEXT NOT NULL DEFAULT '',
    cost_per_hour    DOUBLE PRECISION NOT NULL DEFAULT 0,
    spot             BOOLEAN NOT NULL DEFAULT FALSE,
    labels           JSONB NOT NULL DEFAULT '{}'::jsonb,
    last_heartbeat   TIMESTAMPTZ NOT NULL DEFAULT now(),
    registered_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_nodes_status ON nodes (status);

CREATE TABLE IF NOT EXISTS jobs (
    job_id        TEXT PRIMARY KEY,
    name          TEXT NOT NULL DEFAULT '',
    user_id       TEXT NOT NULL,
    team_id       TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT 'queued',
    priority      INTEGER NOT NULL DEFAULT 0,
    gpu_count     INTEGER NOT NULL DEFAULT 0,
    cpu_count     INTEGER NOT NULL DEFAULT 0,
    memory_gb     INTEGER NOT NULL DEFAULT 0,
    image         TEXT NOT NULL DEFAULT '',
    command       TEXT NOT NULL DEFAULT '',
    node_id       TEXT REFERENCES nodes (node_id) ON DELETE SET NULL,
    retry_count   INTEGER NOT NULL DEFAULT 0,
    max_retries   INTEGER NOT NULL DEFAULT 3,
    cost_per_hour DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at    TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs (status);
CREATE INDEX IF NOT EXISTS idx_jobs_user ON jobs (user_id);
CREATE INDEX IF NOT EXISTS idx_jobs_team ON jobs (team_id);
CREATE INDEX IF NOT EXISTS idx_jobs_status_priority ON jobs (status, priority DESC, created_at);

CREATE TABLE IF NOT EXISTS scheduling_events (
    event_id          TEXT PRIMARY KEY,
    job_id            TEXT NOT NULL,
    selected_node     TEXT NOT NULL DEFAULT '',
    scheduling_reason TEXT NOT NULL DEFAULT '',
    algorithm         TEXT NOT NULL DEFAULT '',
    latency_ms        DOUBLE PRECISION NOT NULL DEFAULT 0,
    success           BOOLEAN NOT NULL DEFAULT FALSE,
    timestamp         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_events_job ON scheduling_events (job_id);
CREATE INDEX IF NOT EXISTS idx_events_time ON scheduling_events (timestamp DESC);

CREATE TABLE IF NOT EXISTS team_quotas (
    team_id       TEXT PRIMARY KEY,
    max_gpus      INTEGER NOT NULL DEFAULT 0,
    max_cpus      INTEGER NOT NULL DEFAULT 0,
    max_memory_gb INTEGER NOT NULL DEFAULT 0
);
