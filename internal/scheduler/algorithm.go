// Package scheduler implements pluggable GPU placement algorithms and the
// scheduling engine that drives them.
package scheduler

import (
	"fmt"
	"strings"

	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/models"
)

// Algorithm is a pluggable scheduling policy. It decides both the order in which
// queued jobs are considered (Order) and which node a given job is placed on
// (Place). Splitting these two concerns lets us mix ordering policies (FIFO,
// priority, fair-share) with placement strategies (first-fit, best-fit).
type Algorithm interface {
	// Name is the stable identifier used in configuration and event logs.
	Name() string
	// Order returns queued jobs in the sequence they should be attempted.
	Order(jobs []*models.Job, ctx OrderContext) []*models.Job
	// Place selects a node for the job. It returns the chosen node, a
	// human-readable reason, and ok=false if no node can satisfy the request.
	Place(job *models.Job, nodes []*models.Node) (node *models.Node, reason string, ok bool)
}

// OrderContext carries cluster-wide signals ordering policies may need, such as
// per-team usage for fair-share.
type OrderContext struct {
	// TeamUsage maps team ID to its current dominant-resource share in [0,1].
	TeamUsage map[string]float64
	// AgingFactor is priority points added per minute of wait time.
	AgingFactor float64
}

// New returns the Algorithm named by the given key. Supported: first-fit,
// best-fit, priority, fair-share.
func New(name string) (Algorithm, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "first-fit", "firstfit", "first_fit":
		return &FirstFit{}, nil
	case "best-fit", "bestfit", "best_fit", "":
		return &BestFit{}, nil
	case "priority", "priority-queue":
		return &Priority{}, nil
	case "fair-share", "fairshare", "fair_share":
		return &FairShare{}, nil
	default:
		return nil, fmt.Errorf("unknown scheduling algorithm %q", name)
	}
}
