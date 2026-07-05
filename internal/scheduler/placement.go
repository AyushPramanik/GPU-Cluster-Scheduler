package scheduler

import (
	"fmt"
	"sort"

	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/models"
)

// firstFitNode returns the first node (in registration order) that fits.
func firstFitNode(job *models.Job, nodes []*models.Node) (*models.Node, string, bool) {
	req := job.ResourceRequest()
	for _, n := range nodes {
		if n.Fits(req) {
			return n, fmt.Sprintf("first-fit: node %s has %d free GPUs", n.Hostname, n.GPUAvailable), true
		}
	}
	return nil, "no node satisfies the resource request", false
}

// bestFitNode returns the node that leaves the least GPU headroom after
// placement, packing jobs tightly to reduce fragmentation. Ties are broken by
// least CPU then least memory headroom.
func bestFitNode(job *models.Job, nodes []*models.Node) (*models.Node, string, bool) {
	req := job.ResourceRequest()
	var best *models.Node
	bestScore := scoreVec{gpu: 1 << 30, cpu: 1 << 30, mem: 1 << 30}
	for _, n := range nodes {
		if !n.Fits(req) {
			continue
		}
		s := scoreVec{
			gpu: n.GPUAvailable - req.GPU,
			cpu: n.CPUAvailable - req.CPU,
			mem: n.MemoryAvailable - req.MemoryGB,
		}
		if best == nil || s.less(bestScore) {
			best = n
			bestScore = s
		}
	}
	if best == nil {
		return nil, "no node satisfies the resource request", false
	}
	return best, fmt.Sprintf("best-fit: node %s leaves %d GPUs free (tightest pack)", best.Hostname, bestScore.gpu), true
}

// scoreVec orders candidate nodes by leftover resources for best-fit packing.
type scoreVec struct {
	gpu, cpu, mem int
}

func (a scoreVec) less(b scoreVec) bool {
	if a.gpu != b.gpu {
		return a.gpu < b.gpu
	}
	if a.cpu != b.cpu {
		return a.cpu < b.cpu
	}
	return a.mem < b.mem
}

// byCreatedAt orders jobs FIFO by submission time.
func byCreatedAt(jobs []*models.Job) []*models.Job {
	out := append([]*models.Job(nil), jobs...)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}
