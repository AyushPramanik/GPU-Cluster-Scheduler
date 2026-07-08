variable "cluster_name" {
  description = "Name of the local kind cluster."
  type        = string
  default     = "gpu-cluster-scheduler"
}

variable "namespace" {
  description = "Kubernetes namespace for the GPU cluster scheduler."
  type        = string
  default     = "gpu-scheduler"
}

variable "node_agent_count" {
  description = "Number of kind worker nodes (each runs a node-agent via the DaemonSet)."
  type        = number
  default     = 2
}

variable "kubernetes_version" {
  description = "kindest/node image tag (Kubernetes version) for cluster nodes."
  type        = string
  default     = "v1.30.0"
}

variable "postgres_db" {
  description = "PostgreSQL database name."
  type        = string
  default     = "gpuscheduler"
}

variable "postgres_user" {
  description = "PostgreSQL user."
  type        = string
  default     = "gpuscheduler"
}

variable "postgres_password" {
  description = "PostgreSQL password."
  type        = string
  default     = "gpuscheduler"
  sensitive   = true
}

variable "apply_manifests" {
  description = "Whether to apply deploy/k8s manifests via kubectl after the cluster is up."
  type        = bool
  default     = true
}

variable "manifests_path" {
  description = "Path to the kustomize root containing the app manifests."
  type        = string
  default     = "../deploy/k8s"
}
