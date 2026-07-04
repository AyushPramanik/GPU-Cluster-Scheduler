package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/models"
)

// EventRepository persists and queries scheduling decision events.
type EventRepository struct {
	pool *pgxpool.Pool
}

// Record persists a scheduling event.
func (r *EventRepository) Record(ctx context.Context, e *models.SchedulingEvent) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO scheduling_events (event_id, job_id, selected_node, scheduling_reason,
			algorithm, latency_ms, success, timestamp)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		e.EventID, e.JobID, e.SelectedNode, e.SchedulingReason, e.Algorithm,
		e.LatencyMS, e.Success, e.Timestamp)
	if err != nil {
		return fmt.Errorf("insert scheduling event: %w", err)
	}
	return nil
}

// List returns recent scheduling events, newest first.
func (r *EventRepository) List(ctx context.Context, limit int) ([]*models.SchedulingEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, `
		SELECT event_id, job_id, selected_node, scheduling_reason, algorithm,
			latency_ms, success, timestamp
		FROM scheduling_events ORDER BY timestamp DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("query scheduling events: %w", err)
	}
	defer rows.Close()
	var out []*models.SchedulingEvent
	for rows.Next() {
		var e models.SchedulingEvent
		if err := rows.Scan(&e.EventID, &e.JobID, &e.SelectedNode, &e.SchedulingReason,
			&e.Algorithm, &e.LatencyMS, &e.Success, &e.Timestamp); err != nil {
			return nil, err
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}
