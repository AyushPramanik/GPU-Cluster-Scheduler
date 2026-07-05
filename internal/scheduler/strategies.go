package scheduler

import (
	"sort"
	"time"

	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/models"
)

// FirstFit considers jobs FIFO and places each on the first node that fits. It
// is the cheapest strategy but tends to fragment the cluster over time.
type FirstFit struct{}

func (FirstFit) Name() string { return "first-fit" }

func (FirstFit) Order(jobs []*models.Job, _ OrderContext) []*models.Job {
	return byCreatedAt(jobs)
}

func (FirstFit) Place(job *models.Job, nodes []*models.Node) (*models.Node, string, bool) {
	return firstFitNode(job, nodes)
}

// BestFit considers jobs FIFO but packs each onto the node that leaves the
// least free capacity, minimising GPU fragmentation across the cluster.
type BestFit struct{}

func (BestFit) Name() string { return "best-fit" }

func (BestFit) Order(jobs []*models.Job, _ OrderContext) []*models.Job {
	return byCreatedAt(jobs)
}

func (BestFit) Place(job *models.Job, nodes []*models.Node) (*models.Node, string, bool) {
	return bestFitNode(job, nodes)
}

// Priority orders jobs by effective priority (base priority plus queue aging so
// long-waiting jobs bubble up and never starve) and packs them best-fit.
type Priority struct{}

func (Priority) Name() string { return "priority" }

func (Priority) Order(jobs []*models.Job, ctx OrderContext) []*models.Job {
	out := append([]*models.Job(nil), jobs...)
	now := time.Now()
	for _, j := range out {
		j.EffectivePriority = EffectivePriority(j, now, ctx.AgingFactor)
	}
	sort.SliceStable(out, func(i, k int) bool {
		if out[i].EffectivePriority != out[k].EffectivePriority {
			return out[i].EffectivePriority > out[k].EffectivePriority
		}
		// Older jobs win ties.
		return out[i].CreatedAt.Before(out[k].CreatedAt)
	})
	return out
}

func (Priority) Place(job *models.Job, nodes []*models.Node) (*models.Node, string, bool) {
	return bestFitNode(job, nodes)
}

// FairShare orders jobs to equalise resource consumption across teams: jobs
// belonging to teams with the lowest current dominant-resource share are
// considered first. Within a team, higher priority (and older) jobs win. This
// prevents a single heavy team from monopolising the cluster.
type FairShare struct{}

func (FairShare) Name() string { return "fair-share" }

func (FairShare) Order(jobs []*models.Job, ctx OrderContext) []*models.Job {
	out := append([]*models.Job(nil), jobs...)
	now := time.Now()
	share := func(teamID string) float64 {
		if ctx.TeamUsage == nil {
			return 0
		}
		return ctx.TeamUsage[teamID]
	}
	sort.SliceStable(out, func(i, k int) bool {
		si, sk := share(out[i].TeamID), share(out[k].TeamID)
		if si != sk {
			return si < sk // team using less of its fair share goes first
		}
		pi := EffectivePriority(out[i], now, ctx.AgingFactor)
		pk := EffectivePriority(out[k], now, ctx.AgingFactor)
		if pi != pk {
			return pi > pk
		}
		return out[i].CreatedAt.Before(out[k].CreatedAt)
	})
	return out
}

func (FairShare) Place(job *models.Job, nodes []*models.Node) (*models.Node, string, bool) {
	return bestFitNode(job, nodes)
}

// EffectivePriority computes a job's priority accounting for queue aging: each
// minute spent waiting adds agingFactor points, guaranteeing that even the
// lowest priority job eventually outranks fresh arrivals and avoids starvation.
func EffectivePriority(j *models.Job, now time.Time, agingFactor float64) int {
	waitMinutes := now.Sub(j.CreatedAt).Minutes()
	if waitMinutes < 0 {
		waitMinutes = 0
	}
	return j.Priority + int(waitMinutes*agingFactor)
}
