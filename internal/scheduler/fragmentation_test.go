package scheduler

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/models"
)

func TestFragmentation_ConsolidatedIsZero(t *testing.T) {
	// All 4 free GPUs on a single node: not fragmented.
	nodes := []*models.Node{node("a", 4, 8, 64), node("b", 4, 8, 64)}
	nodes[1].GPUAvailable = 0 // fully used
	assert.InDelta(t, 0.0, Fragmentation(nodes), 1e-9)
}

func TestFragmentation_ScatteredIsHigh(t *testing.T) {
	// 1 free GPU on each of 4 nodes: highly fragmented (max block 1 of 4).
	nodes := []*models.Node{node("a", 4, 8, 64), node("b", 4, 8, 64), node("c", 4, 8, 64), node("d", 4, 8, 64)}
	for _, n := range nodes {
		n.GPUAvailable = 1
	}
	assert.InDelta(t, 0.75, Fragmentation(nodes), 1e-9)
}

func TestFragmentation_NoFreeCapacity(t *testing.T) {
	n := node("a", 4, 8, 64)
	n.GPUAvailable = 0
	assert.Equal(t, 0.0, Fragmentation([]*models.Node{n}))
}

func TestClusterSnapshot_AggregatesCorrectly(t *testing.T) {
	nodes := []*models.Node{node("a", 8, 64, 512), node("b", 8, 64, 512)}
	nodes[0].GPUAvailable = 4 // 4 used on a
	snap := ClusterSnapshot(nodes, 3, 5)

	assert.Equal(t, 16, snap.TotalGPUs)
	assert.Equal(t, 4, snap.UsedGPUs)
	assert.InDelta(t, 0.25, snap.GPUUtilization, 1e-9)
	assert.Equal(t, 2, snap.NodesReady)
	assert.Equal(t, 3, snap.JobsRunning)
	assert.Equal(t, 5, snap.JobsQueued)
}
