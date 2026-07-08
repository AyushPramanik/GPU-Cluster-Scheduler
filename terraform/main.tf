###############################################################################
# Local kind cluster for the GPU Cluster Scheduler.
#
# Flow:
#   1. kind_cluster        - provisions a local Kubernetes cluster in Docker.
#   2. kubernetes provider - configured from the kind cluster's kubeconfig.
#   3. namespace/secret/configmap - core resources created natively so the rest
#      of the manifests (deploy/k8s) can be applied on top with kustomize.
#   4. null_resource       - applies deploy/k8s via `kubectl apply -k`.
#
# For a cloud target, swap the kind_cluster block for a managed-cluster module
# (EKS/GKE/AKS) and point the kubernetes provider at that cluster's endpoint;
# everything below stays the same. See README.md.
###############################################################################

resource "kind_cluster" "this" {
  name           = var.cluster_name
  node_image     = "kindest/node:${var.kubernetes_version}"
  wait_for_ready = true

  kind_config {
    kind        = "Cluster"
    api_version = "kind.x-k8s.io/v1alpha4"

    # Control-plane node with ingress-ready port mappings for ingress-nginx.
    node {
      role = "control-plane"

      kubeadm_config_patches = [
        "kind: InitConfiguration\nnodeRegistration:\n  kubeletExtraArgs:\n    node-labels: \"ingress-ready=true\"\n"
      ]

      extra_port_mappings {
        container_port = 80
        host_port      = 80
        protocol       = "TCP"
      }
      extra_port_mappings {
        container_port = 443
        host_port      = 443
        protocol       = "TCP"
      }
    }

    # Worker nodes - each will run a node-agent pod via the DaemonSet.
    dynamic "node" {
      for_each = range(var.node_agent_count)
      content {
        role = "worker"
      }
    }
  }
}

provider "kubernetes" {
  host                   = kind_cluster.this.endpoint
  client_certificate     = kind_cluster.this.client_certificate
  client_key             = kind_cluster.this.client_key
  cluster_ca_certificate = kind_cluster.this.cluster_ca_certificate
}

resource "kubernetes_namespace" "this" {
  metadata {
    name = var.namespace
    labels = {
      "app.kubernetes.io/part-of" = "gpu-cluster-scheduler"
    }
  }
}

resource "kubernetes_secret" "db" {
  metadata {
    name      = "gpu-scheduler-db"
    namespace = kubernetes_namespace.this.metadata[0].name
  }

  type = "Opaque"

  data = {
    POSTGRES_DB       = var.postgres_db
    POSTGRES_USER     = var.postgres_user
    POSTGRES_PASSWORD = var.postgres_password
    DATABASE_URL      = "postgres://${var.postgres_user}:${var.postgres_password}@postgres:5432/${var.postgres_db}?sslmode=disable"
    REDIS_URL         = "redis://redis:6379/0"
  }
}

resource "kubernetes_config_map" "config" {
  metadata {
    name      = "gpu-scheduler-config"
    namespace = kubernetes_namespace.this.metadata[0].name
  }

  data = {
    DB_HOST                     = "postgres"
    DB_PORT                     = "5432"
    DB_NAME                     = var.postgres_db
    DB_SSLMODE                  = "disable"
    REDIS_ADDR                  = "redis:6379"
    SCHEDULER_ADDR              = "scheduler:50051"
    NODE_AGENT_ADDRS            = "node-agent:50061"
    LOG_LEVEL                   = "info"
    OTEL_EXPORTER_OTLP_ENDPOINT = "http://otel-collector:4317"
    OTEL_EXPORTER_OTLP_PROTOCOL = "grpc"
  }
}

# Apply the remaining application manifests with kustomize. The namespace,
# secret and configmap already exist (created above); kustomize is applied with
# server-side apply so the overlap is reconciled instead of erroring.
resource "null_resource" "apply_manifests" {
  count = var.apply_manifests ? 1 : 0

  depends_on = [
    kubernetes_namespace.this,
    kubernetes_secret.db,
    kubernetes_config_map.config,
  ]

  triggers = {
    cluster        = kind_cluster.this.name
    manifests_hash = sha1(join(",", fileset(path.module, "${var.manifests_path}/*.yaml")))
  }

  provisioner "local-exec" {
    command = "kubectl --context kind-${kind_cluster.this.name} apply --server-side --force-conflicts -k ${var.manifests_path}"
  }
}
