output "cluster_name" {
  description = "Name of the provisioned kind cluster."
  value       = kind_cluster.this.name
}

output "kubeconfig_path" {
  description = "Path to the generated kubeconfig for the cluster."
  value       = kind_cluster.this.kubeconfig_path
}

output "kube_context" {
  description = "kubectl context for the cluster."
  value       = "kind-${kind_cluster.this.name}"
}

output "cluster_endpoint" {
  description = "Kubernetes API server endpoint."
  value       = kind_cluster.this.endpoint
}

output "namespace" {
  description = "Namespace the scheduler is deployed into."
  value       = kubernetes_namespace.this.metadata[0].name
}

output "next_steps" {
  description = "How to reach the deployed services."
  value       = <<-EOT
    Cluster ready. Try:
      kubectl --context kind-${kind_cluster.this.name} -n ${var.namespace} get pods

    Add to /etc/hosts for ingress:
      127.0.0.1 api.gpu-scheduler.local app.gpu-scheduler.local

    Install ingress-nginx (kind flavour):
      kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
  EOT
}
