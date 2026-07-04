package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/models"
)

// ErrNotFound is returned when a requested row does not exist.
var ErrNotFound = errors.New("not found")

// JobRepository persists and queries jobs.
type JobRepository struct {
	pool *pgxpool.Pool
}

const jobColumns = `job_id, name, user_id, team_id, status, priority, gpu_count, cpu_count,
	memory_gb, image, command, COALESCE(node_id, ''), retry_count, max_retries,
	cost_per_hour, created_at, started_at, completed_at`

// Create inserts a new job.
func (r *JobRepository) Create(ctx context.Context, j *models.Job) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO jobs (job_id, name, user_id, team_id, status, priority, gpu_count,
			cpu_count, memory_gb, image, command, retry_count, max_retries, cost_per_hour, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		j.JobID, j.Name, j.UserID, j.TeamID, j.Status, j.Priority, j.GPUCount,
		j.CPUCount, j.MemoryGB, j.Image, j.Command, j.RetryCount, j.MaxRetries,
		j.CostPerHour, j.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert job: %w", err)
	}
	return nil
}

// Get fetches a job by ID.
func (r *JobRepository) Get(ctx context.Context, id string) (*models.Job, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+jobColumns+` FROM jobs WHERE job_id = $1`, id)
	j, err := scanJob(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return j, err
}

// JobFilter narrows a job listing.
type JobFilter struct {
	Status   models.JobStatus
	UserID   string
	TeamID   string
	Statuses []models.JobStatus
	Limit    int
}

// List returns jobs matching the filter, newest first.
func (r *JobRepository) List(ctx context.Context, f JobFilter) ([]*models.Job, error) {
	var (
		clauses []string
		args    []any
		i       = 1
	)
	if f.Status != "" {
		clauses = append(clauses, fmt.Sprintf("status = $%d", i))
		args = append(args, f.Status)
		i++
	}
	if len(f.Statuses) > 0 {
		placeholders := make([]string, len(f.Statuses))
		for k, s := range f.Statuses {
			placeholders[k] = fmt.Sprintf("$%d", i)
			args = append(args, s)
			i++
		}
		clauses = append(clauses, "status IN ("+strings.Join(placeholders, ",")+")")
	}
	if f.UserID != "" {
		clauses = append(clauses, fmt.Sprintf("user_id = $%d", i))
		args = append(args, f.UserID)
		i++
	}
	if f.TeamID != "" {
		clauses = append(clauses, fmt.Sprintf("team_id = $%d", i))
		args = append(args, f.TeamID)
		i++
	}

	q := `SELECT ` + jobColumns + ` FROM jobs`
	if len(clauses) > 0 {
		q += " WHERE " + strings.Join(clauses, " AND ")
	}
	q += " ORDER BY created_at DESC"
	if f.Limit > 0 {
		q += fmt.Sprintf(" LIMIT $%d", i)
		args = append(args, f.Limit)
	}

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query jobs: %w", err)
	}
	defer rows.Close()
	return collectJobs(rows)
}

// ListByStatuses is a convenience wrapper used by the scheduler to fetch the
// queue or the set of active jobs.
func (r *JobRepository) ListByStatuses(ctx context.Context, statuses ...models.JobStatus) ([]*models.Job, error) {
	return r.List(ctx, JobFilter{Statuses: statuses})
}

// ListScheduledForNode returns jobs placed on a node that the agent has not yet
// picked up (status = scheduled).
func (r *JobRepository) ListScheduledForNode(ctx context.Context, nodeID string) ([]*models.Job, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+jobColumns+`
		FROM jobs WHERE node_id = $1 AND status = 'scheduled' ORDER BY created_at`, nodeID)
	if err != nil {
		return nil, fmt.Errorf("query assignments: %w", err)
	}
	defer rows.Close()
	return collectJobs(rows)
}

// UpdateStatus transitions a job's status and stamps lifecycle timestamps.
func (r *JobRepository) UpdateStatus(ctx context.Context, id string, status models.JobStatus) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE jobs SET
			status = $2,
			started_at = CASE WHEN $2 = 'running' AND started_at IS NULL THEN now() ELSE started_at END,
			completed_at = CASE WHEN $2 IN ('completed','failed','cancelled') THEN now() ELSE completed_at END
		WHERE job_id = $1`, id, status)
	return err
}

// Schedule marks a job as scheduled onto a node.
func (r *JobRepository) Schedule(ctx context.Context, id, nodeID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE jobs SET status = 'scheduled', node_id = $2 WHERE job_id = $1`, id, nodeID)
	return err
}

// Requeue returns a job to the queue, incrementing its retry counter and
// clearing its node assignment.
func (r *JobRepository) Requeue(ctx context.Context, id string, status models.JobStatus) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE jobs SET status = $2, node_id = NULL, retry_count = retry_count + 1,
			started_at = NULL WHERE job_id = $1`, id, status)
	return err
}

// CountByStatus returns the number of jobs in the given status.
func (r *JobRepository) CountByStatus(ctx context.Context, status models.JobStatus) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM jobs WHERE status = $1`, status).Scan(&n)
	return n, err
}

// TeamGPUUsage returns per-team GPU consumption across active jobs, used by the
// fair-share scheduler.
func (r *JobRepository) TeamGPUUsage(ctx context.Context) (map[string]int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT team_id, COALESCE(SUM(gpu_count),0) FROM jobs
		WHERE status IN ('scheduled','running') GROUP BY team_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int)
	for rows.Next() {
		var team string
		var gpus int
		if err := rows.Scan(&team, &gpus); err != nil {
			return nil, err
		}
		out[team] = gpus
	}
	return out, rows.Err()
}

type scannable interface {
	Scan(dest ...any) error
}

func scanJob(row scannable) (*models.Job, error) {
	var j models.Job
	var started, completed *time.Time
	if err := row.Scan(&j.JobID, &j.Name, &j.UserID, &j.TeamID, &j.Status, &j.Priority,
		&j.GPUCount, &j.CPUCount, &j.MemoryGB, &j.Image, &j.Command, &j.NodeID,
		&j.RetryCount, &j.MaxRetries, &j.CostPerHour, &j.CreatedAt, &started, &completed); err != nil {
		return nil, err
	}
	j.StartedAt = started
	j.CompletedAt = completed
	return &j, nil
}

func collectJobs(rows pgx.Rows) ([]*models.Job, error) {
	var out []*models.Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}
