package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNode_FitsRespectsAllDimensions(t *testing.T) {
	n := &Node{Status: NodeStatusReady, GPUAvailable: 4, CPUAvailable: 16, MemoryAvailable: 128}
	assert.True(t, n.Fits(ResourceRequest{GPU: 4, CPU: 16, MemoryGB: 128}))
	assert.False(t, n.Fits(ResourceRequest{GPU: 5}), "GPU over capacity")
	assert.False(t, n.Fits(ResourceRequest{CPU: 17}), "CPU over capacity")
	assert.False(t, n.Fits(ResourceRequest{MemoryGB: 129}), "memory over capacity")
}

func TestNode_UnschedulableStatusNeverFits(t *testing.T) {
	for _, s := range []NodeStatus{NodeStatusCordoned, NodeStatusDraining, NodeStatusDown} {
		n := &Node{Status: s, GPUAvailable: 8, CPUAvailable: 64, MemoryAvailable: 512}
		assert.Falsef(t, n.Fits(ResourceRequest{GPU: 1}), "status %s should not be schedulable", s)
	}
}

func TestNode_ReserveAndRelease(t *testing.T) {
	n := &Node{GPUCapacity: 8, GPUAvailable: 8, CPUCapacity: 64, CPUAvailable: 64, MemoryCapacity: 512, MemoryAvailable: 512}
	req := ResourceRequest{GPU: 2, CPU: 8, MemoryGB: 64}
	n.Reserve(req)
	assert.Equal(t, 6, n.GPUAvailable)
	n.Release(req)
	assert.Equal(t, 8, n.GPUAvailable)
}

func TestNode_ReleaseNeverExceedsCapacity(t *testing.T) {
	n := &Node{GPUCapacity: 8, GPUAvailable: 8, CPUCapacity: 64, CPUAvailable: 64, MemoryCapacity: 512, MemoryAvailable: 512}
	n.Release(ResourceRequest{GPU: 4, CPU: 4, MemoryGB: 4})
	assert.Equal(t, 8, n.GPUAvailable, "release must clamp to capacity")
}

func TestNode_UtilizationRatios(t *testing.T) {
	n := &Node{GPUCapacity: 8, GPUAvailable: 2}
	assert.InDelta(t, 0.75, n.GPUUtilization(), 1e-9)
	empty := &Node{}
	assert.Equal(t, 0.0, empty.GPUUtilization(), "no capacity should not divide by zero")
}

func TestJobStatus_TerminalAndActive(t *testing.T) {
	assert.True(t, JobStatusCompleted.IsTerminal())
	assert.False(t, JobStatusRunning.IsTerminal())
	assert.True(t, JobStatusRunning.IsActive())
	assert.False(t, JobStatusQueued.IsActive())
}
