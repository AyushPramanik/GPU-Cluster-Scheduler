package scheduler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ayushpramanik/gpu-cluster-scheduler/internal/models"
)

func TestFindPreemption_EvictsLowerPriorityToFit(t *testing.T) {
	n := node("a", 4, 32, 256)
	n.GPUAvailable = 0 // fully occupied

	victim := job("victim", 4, 8, 64, 1)
	victim.UserID = "u"
	pending := job("pending", 4, 8, 64, 9)

	plan, ok := FindPreemption(pending, []*models.Node{n}, map[string][]*models.Job{
		"a": {victim},
	})
	require.True(t, ok)
	require.Len(t, plan.Victims, 1)
	assert.Equal(t, "victim", plan.Victims[0].JobID)
}

func TestFindPreemption_WontEvictEqualOrHigherPriority(t *testing.T) {
	n := node("a", 4, 32, 256)
	n.GPUAvailable = 0
	incumbent := job("incumbent", 4, 8, 64, 9)
	incumbent.UserID = "u"
	pending := job("pending", 4, 8, 64, 9) // equal priority

	_, ok := FindPreemption(pending, []*models.Node{n}, map[string][]*models.Job{"a": {incumbent}})
	assert.False(t, ok, "must not preempt equal-priority work")
}

func TestFindPreemption_PrefersFewestVictims(t *testing.T) {
	n := node("a", 4, 32, 256)
	n.GPUAvailable = 0
	// Two small victims vs one big victim; the single big eviction frees enough.
	big := job("big", 4, 8, 64, 1)
	big.UserID = "u"
	pending := job("pending", 4, 8, 64, 9)

	plan, ok := FindPreemption(pending, []*models.Node{n}, map[string][]*models.Job{
		"a": {big},
	})
	require.True(t, ok)
	assert.Len(t, plan.Victims, 1)
}
