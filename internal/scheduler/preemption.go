package scheduler

import (
	"sort"

	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/models"
)

// PreemptionPlan describes which running jobs to evict from a node so that a
// pending higher-priority job can be placed there.
type PreemptionPlan struct {
	Node    *models.Node
	Victims []*models.Job
}

// FindPreemption looks for a node where evicting one or more strictly
// lower-priority running jobs would free enough capacity for the pending job. It
// prefers the plan that evicts the fewest jobs, and among those the lowest total
// victim priority, minimising disruption. running maps node ID -> jobs running
// on it. It returns ok=false if no viable plan exists.
func FindPreemption(job *models.Job, nodes []*models.Node, running map[string][]*models.Job) (PreemptionPlan, bool) {
	req := job.ResourceRequest()
	var best *PreemptionPlan

	for _, n := range nodes {
		if !n.Status.Schedulable() {
			continue
		}
		// Candidate victims: strictly lower priority than the pending job.
		var victims []*models.Job
		for _, r := range running[n.NodeID] {
			if r.Priority < job.Priority && r.UserID != "" {
				victims = append(victims, r)
			}
		}
		// Evict the cheapest (lowest priority, then newest) victims first.
		sort.SliceStable(victims, func(i, k int) bool {
			if victims[i].Priority != victims[k].Priority {
				return victims[i].Priority < victims[k].Priority
			}
			return victims[i].CreatedAt.After(victims[k].CreatedAt)
		})

		freeGPU := n.GPUAvailable
		freeCPU := n.CPUAvailable
		freeMem := n.MemoryAvailable
		var chosen []*models.Job
		for _, v := range victims {
			if freeGPU >= req.GPU && freeCPU >= req.CPU && freeMem >= req.MemoryGB {
				break
			}
			chosen = append(chosen, v)
			freeGPU += v.GPUCount
			freeCPU += v.CPUCount
			freeMem += v.MemoryGB
		}

		if freeGPU < req.GPU || freeCPU < req.CPU || freeMem < req.MemoryGB {
			continue // even evicting everything eligible is not enough
		}

		plan := PreemptionPlan{Node: n, Victims: chosen}
		if best == nil || betterPlan(plan, *best) {
			p := plan
			best = &p
		}
	}

	if best == nil {
		return PreemptionPlan{}, false
	}
	return *best, true
}

// betterPlan reports whether plan a disrupts less than plan b.
func betterPlan(a, b PreemptionPlan) bool {
	if len(a.Victims) != len(b.Victims) {
		return len(a.Victims) < len(b.Victims)
	}
	return victimPriority(a) < victimPriority(b)
}

func victimPriority(p PreemptionPlan) int {
	sum := 0
	for _, v := range p.Victims {
		sum += v.Priority
	}
	return sum
}
