package scheduler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/models"
)

func node(id string, gpu, cpu, mem int) *models.Node {
	return &models.Node{
		NodeID: id, Hostname: id, Status: models.NodeStatusReady,
		GPUCapacity: gpu, GPUAvailable: gpu,
		CPUCapacity: cpu, CPUAvailable: cpu,
		MemoryCapacity: mem, MemoryAvailable: mem,
	}
}

func job(id string, gpu, cpu, mem, priority int) *models.Job {
	return &models.Job{
		JobID: id, UserID: "u", Status: models.JobStatusQueued,
		GPUCount: gpu, CPUCount: cpu, MemoryGB: mem, Priority: priority,
		CreatedAt: time.Now(),
	}
}

func TestNew_KnownAndUnknownAlgorithms(t *testing.T) {
	for _, name := range []string{"first-fit", "best-fit", "priority", "fair-share", ""} {
		alg, err := New(name)
		require.NoError(t, err, "algorithm %q should be known", name)
		require.NotNil(t, alg)
	}
	_, err := New("round-robin")
	require.Error(t, err)
}

func TestFirstFit_PicksFirstFittingNode(t *testing.T) {
	nodes := []*models.Node{node("a", 2, 8, 64), node("b", 8, 64, 512)}
	got, reason, ok := FirstFit{}.Place(job("j", 4, 8, 64, 0), nodes)
	require.True(t, ok)
	assert.Equal(t, "b", got.NodeID, "first node lacks GPUs, should pick b")
	assert.Contains(t, reason, "first-fit")
}

func TestBestFit_PacksTightlyToReduceFragmentation(t *testing.T) {
	// A job needing 2 GPUs should land on the node that leaves the least
	// headroom (the 2-GPU node), not the large 8-GPU node.
	nodes := []*models.Node{node("big", 8, 64, 512), node("tight", 2, 16, 128)}
	got, _, ok := BestFit{}.Place(job("j", 2, 8, 64, 0), nodes)
	require.True(t, ok)
	assert.Equal(t, "tight", got.NodeID)
}

func TestPlace_NoCapacityReturnsFalse(t *testing.T) {
	nodes := []*models.Node{node("a", 1, 4, 32)}
	_, _, ok := BestFit{}.Place(job("j", 8, 8, 64, 0), nodes)
	assert.False(t, ok)
}

func TestPlace_SkipsUnschedulableNodes(t *testing.T) {
	n := node("cordoned", 8, 64, 512)
	n.Status = models.NodeStatusCordoned
	_, _, ok := FirstFit{}.Place(job("j", 1, 1, 8, 0), []*models.Node{n})
	assert.False(t, ok, "cordoned nodes must not receive new jobs")
}

func TestPriorityOrder_HighestFirstWithAging(t *testing.T) {
	now := time.Now()
	low := job("low", 1, 1, 8, 1)
	high := job("high", 1, 1, 8, 9)
	// An old low-priority job (aged to ~6) should beat a fresh low job but not
	// the fresh priority-9 job.
	oldLow := job("oldLow", 1, 1, 8, 1)
	oldLow.CreatedAt = now.Add(-5 * time.Minute)

	ordered := Priority{}.Order([]*models.Job{low, high, oldLow}, OrderContext{AgingFactor: 1})
	require.Len(t, ordered, 3)
	assert.Equal(t, "high", ordered[0].JobID, "priority 9 should lead")
	assert.Equal(t, "oldLow", ordered[1].JobID, "aged low job should beat fresh low job")
}

func TestFairShare_FavorsUnderusedTeams(t *testing.T) {
	heavy := job("heavy", 1, 1, 8, 5)
	heavy.TeamID = "research"
	light := job("light", 1, 1, 8, 5)
	light.TeamID = "platform"

	ctx := OrderContext{TeamUsage: map[string]float64{"research": 0.8, "platform": 0.1}}
	ordered := FairShare{}.Order([]*models.Job{heavy, light}, ctx)
	assert.Equal(t, "light", ordered[0].JobID, "team using less of its share goes first")
}

func TestEffectivePriority_AgingIsMonotonic(t *testing.T) {
	now := time.Now()
	j := job("j", 1, 1, 8, 2)
	j.CreatedAt = now.Add(-10 * time.Minute)
	assert.Equal(t, 12, EffectivePriority(j, now, 1.0))
	assert.Equal(t, 2, EffectivePriority(job("fresh", 1, 1, 8, 2), now, 1.0))
}
