package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/models"
)

// NodeRepository persists and queries cluster nodes.
type NodeRepository struct {
	pool *pgxpool.Pool
}

const nodeColumns = `node_id, hostname, status, gpu_capacity, gpu_available, cpu_capacity,
	cpu_available, memory_capacity, memory_available, gpu_model, cost_per_hour, spot,
	labels, last_heartbeat, registered_at`

// Upsert registers a node or updates its capacity/metadata on re-registration.
func (r *NodeRepository) Upsert(ctx context.Context, n *models.Node) error {
	labels, err := json.Marshal(n.Labels)
	if err != nil {
		return fmt.Errorf("marshal labels: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO nodes (node_id, hostname, status, gpu_capacity, gpu_available,
			cpu_capacity, cpu_available, memory_capacity, memory_available, gpu_model,
			cost_per_hour, spot, labels, last_heartbeat, registered_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,now(),now())
		ON CONFLICT (node_id) DO UPDATE SET
			hostname = EXCLUDED.hostname,
			gpu_capacity = EXCLUDED.gpu_capacity,
			cpu_capacity = EXCLUDED.cpu_capacity,
			memory_capacity = EXCLUDED.memory_capacity,
			gpu_model = EXCLUDED.gpu_model,
			cost_per_hour = EXCLUDED.cost_per_hour,
			spot = EXCLUDED.spot,
			labels = EXCLUDED.labels,
			last_heartbeat = now()`,
		n.NodeID, n.Hostname, n.Status, n.GPUCapacity, n.GPUAvailable, n.CPUCapacity,
		n.CPUAvailable, n.MemoryCapacity, n.MemoryAvailable, n.GPUModel, n.CostPerHour,
		n.Spot, labels)
	return err
}

// Get fetches a node by ID.
func (r *NodeRepository) Get(ctx context.Context, id string) (*models.Node, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+nodeColumns+` FROM nodes WHERE node_id = $1`, id)
	n, err := scanNode(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return n, err
}

// List returns all nodes ordered by hostname.
func (r *NodeRepository) List(ctx context.Context) ([]*models.Node, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+nodeColumns+` FROM nodes ORDER BY hostname`)
	if err != nil {
		return nil, fmt.Errorf("query nodes: %w", err)
	}
	defer rows.Close()
	var out []*models.Node
	for rows.Next() {
		n, err := scanNode(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// UpdateAvailability sets a node's currently available resources; called after
// placement/release so the persisted view matches reservations.
func (r *NodeRepository) UpdateAvailability(ctx context.Context, id string, gpu, cpu, mem int) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE nodes SET gpu_available = $2, cpu_available = $3, memory_available = $4
		WHERE node_id = $1`, id, gpu, cpu, mem)
	return err
}

// Heartbeat records a fresh heartbeat. Available-resource bookkeeping is owned
// by the scheduler (reserve on placement, release on completion), so heartbeats
// only refresh liveness and revive a node that had been marked down.
func (r *NodeRepository) Heartbeat(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE nodes SET last_heartbeat = now(),
			status = CASE WHEN status = 'down' THEN 'ready' ELSE status END
		WHERE node_id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetStatus transitions a node's operational status (cordon/drain/ready/down).
func (r *NodeRepository) SetStatus(ctx context.Context, id string, status models.NodeStatus) error {
	_, err := r.pool.Exec(ctx, `UPDATE nodes SET status = $2 WHERE node_id = $1`, id, status)
	return err
}

// MarkStaleDown flips nodes that have not sent a heartbeat within ttl to the
// down state and returns the IDs affected so their jobs can be rescheduled.
func (r *NodeRepository) MarkStaleDown(ctx context.Context, ttl time.Duration) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		UPDATE nodes SET status = 'down'
		WHERE status IN ('ready','draining','cordoned')
		  AND last_heartbeat < now() - make_interval(secs => $1)
		RETURNING node_id`, ttl.Seconds())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func scanNode(row scannable) (*models.Node, error) {
	var n models.Node
	var labels []byte
	if err := row.Scan(&n.NodeID, &n.Hostname, &n.Status, &n.GPUCapacity, &n.GPUAvailable,
		&n.CPUCapacity, &n.CPUAvailable, &n.MemoryCapacity, &n.MemoryAvailable, &n.GPUModel,
		&n.CostPerHour, &n.Spot, &labels, &n.LastHeartbeat, &n.RegisteredAt); err != nil {
		return nil, err
	}
	if len(labels) > 0 {
		_ = json.Unmarshal(labels, &n.Labels)
	}
	return &n, nil
}
