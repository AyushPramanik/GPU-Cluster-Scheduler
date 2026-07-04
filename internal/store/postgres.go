// Package store provides PostgreSQL-backed repositories for jobs, nodes, and
// scheduling events behind small interfaces for dependency injection and
// testability.
package store

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/config"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Store bundles the repositories over a shared pgx connection pool.
type Store struct {
	pool   *pgxpool.Pool
	Jobs   *JobRepository
	Nodes  *NodeRepository
	Events *EventRepository
}

// New connects to PostgreSQL using the provided config and returns a Store.
func New(ctx context.Context, cfg config.PostgresConfig) (*Store, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse postgres dsn: %w", err)
	}
	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLife

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &Store{
		pool:   pool,
		Jobs:   &JobRepository{pool: pool},
		Nodes:  &NodeRepository{pool: pool},
		Events: &EventRepository{pool: pool},
	}, nil
}

// Pool exposes the underlying connection pool (used for health checks).
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// Close releases all pooled connections.
func (s *Store) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

// Ping verifies database connectivity.
func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// Migrate applies the embedded SQL migrations in lexical order. Migrations use
// IF NOT EXISTS / ON CONFLICT guards so they are safe to run repeatedly.
func (s *Store) Migrate(ctx context.Context) error {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		sqlBytes, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if _, err := s.pool.Exec(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
	}
	return nil
}
