package models

import "time"

// NodeStatus represents the operational state of a cluster node.
type NodeStatus string

const (
	// NodeStatusReady means the node is healthy and accepting new workloads.
	NodeStatusReady NodeStatus = "ready"
	// NodeStatusDraining means existing jobs run to completion but no new jobs
	// are placed; used ahead of maintenance.
	NodeStatusDraining NodeStatus = "draining"
	// NodeStatusCordoned means the node is marked unschedulable but running jobs
	// are left untouched.
	NodeStatusCordoned NodeStatus = "cordoned"
	// NodeStatusDown means the node has missed heartbeats and is considered lost.
	NodeStatusDown NodeStatus = "down"
)

// Schedulable reports whether new jobs may be placed on the node.
func (s NodeStatus) Schedulable() bool {
	return s == NodeStatusReady
}

// Node is a GPU machine in the cluster.
type Node struct {
	NodeID          string            `json:"node_id"`
	Hostname        string            `json:"hostname"`
	Status          NodeStatus        `json:"status"`
	GPUCapacity     int               `json:"gpu_capacity"`
	GPUAvailable    int               `json:"gpu_available"`
	CPUCapacity     int               `json:"cpu_capacity"`
	CPUAvailable    int               `json:"cpu_available"`
	MemoryCapacity  int               `json:"memory_capacity"`
	MemoryAvailable int               `json:"memory_available"`
	GPUModel        string            `json:"gpu_model,omitempty"`
	CostPerHour     float64           `json:"cost_per_hour,omitempty"`
	Spot            bool              `json:"spot"`
	Labels          map[string]string `json:"labels,omitempty"`
	LastHeartbeat   time.Time         `json:"last_heartbeat"`
	RegisteredAt    time.Time         `json:"registered_at"`
}

// Fits reports whether the node currently has room for the given request.
func (n *Node) Fits(r ResourceRequest) bool {
	return n.Status.Schedulable() &&
		n.GPUAvailable >= r.GPU &&
		n.CPUAvailable >= r.CPU &&
		n.MemoryAvailable >= r.MemoryGB
}

// GPUUtilization returns the fraction of GPUs in use, in [0,1].
func (n *Node) GPUUtilization() float64 {
	if n.GPUCapacity == 0 {
		return 0
	}
	return float64(n.GPUCapacity-n.GPUAvailable) / float64(n.GPUCapacity)
}

// CPUUtilization returns the fraction of CPUs in use, in [0,1].
func (n *Node) CPUUtilization() float64 {
	if n.CPUCapacity == 0 {
		return 0
	}
	return float64(n.CPUCapacity-n.CPUAvailable) / float64(n.CPUCapacity)
}

// MemoryUtilization returns the fraction of memory in use, in [0,1].
func (n *Node) MemoryUtilization() float64 {
	if n.MemoryCapacity == 0 {
		return 0
	}
	return float64(n.MemoryCapacity-n.MemoryAvailable) / float64(n.MemoryCapacity)
}

// Reserve deducts the request from the node's available resources.
func (n *Node) Reserve(r ResourceRequest) {
	n.GPUAvailable -= r.GPU
	n.CPUAvailable -= r.CPU
	n.MemoryAvailable -= r.MemoryGB
}

// Release returns the request's resources to the node.
func (n *Node) Release(r ResourceRequest) {
	n.GPUAvailable += r.GPU
	if n.GPUAvailable > n.GPUCapacity {
		n.GPUAvailable = n.GPUCapacity
	}
	n.CPUAvailable += r.CPU
	if n.CPUAvailable > n.CPUCapacity {
		n.CPUAvailable = n.CPUCapacity
	}
	n.MemoryAvailable += r.MemoryGB
	if n.MemoryAvailable > n.MemoryCapacity {
		n.MemoryAvailable = n.MemoryCapacity
	}
}
