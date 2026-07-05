package scheduler

import "github.com/ayushpramanik/gpu-cluster-scheduler/internal/models"

// Fragmentation returns a GPU fragmentation index in [0,1]. It captures how
// "scattered" free GPU capacity is across the cluster: 0 means all free GPUs
// are consolidated on as few nodes as possible, 1 means free capacity is spread
// thinly everywhere. It is defined as 1 minus the ratio of the largest single
// contiguous free block to the total free capacity — a cluster whose free GPUs
// all live on one node can satisfy a large job, whereas the same number of free
// GPUs spread one-per-node cannot.
func Fragmentation(nodes []*models.Node) float64 {
	totalFree := 0
	maxFree := 0
	for _, n := range nodes {
		if !n.Status.Schedulable() {
			continue
		}
		totalFree += n.GPUAvailable
		if n.GPUAvailable > maxFree {
			maxFree = n.GPUAvailable
		}
	}
	if totalFree == 0 {
		return 0
	}
	return 1 - float64(maxFree)/float64(totalFree)
}

// ClusterSnapshot aggregates node and job state into a ClusterUtilization view.
func ClusterSnapshot(nodes []*models.Node, jobsRunning, jobsQueued int) models.ClusterUtilization {
	var u models.ClusterUtilization
	for _, n := range nodes {
		u.NodesTotal++
		if n.Status == models.NodeStatusReady {
			u.NodesReady++
		}
		u.TotalGPUs += n.GPUCapacity
		u.UsedGPUs += n.GPUCapacity - n.GPUAvailable
		u.TotalCPUs += n.CPUCapacity
		u.UsedCPUs += n.CPUCapacity - n.CPUAvailable
		u.TotalMemoryGB += n.MemoryCapacity
		u.UsedMemoryGB += n.MemoryCapacity - n.MemoryAvailable
	}
	u.GPUUtilization = ratio(u.UsedGPUs, u.TotalGPUs)
	u.CPUUtilization = ratio(u.UsedCPUs, u.TotalCPUs)
	u.MemoryUtilization = ratio(u.UsedMemoryGB, u.TotalMemoryGB)
	u.Fragmentation = Fragmentation(nodes)
	u.JobsRunning = jobsRunning
	u.JobsQueued = jobsQueued
	return u
}

func ratio(used, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(used) / float64(total)
}
