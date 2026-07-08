# Terraform - GPU Cluster Scheduler infrastructure

Provisions a **local Kubernetes cluster** with [kind](https://kind.sigs.k8s.io/)
(via the `tehcyx/kind` provider), creates the `gpu-scheduler` namespace, the DB
secret and app config, and applies the manifests in `../deploy/k8s` with
kustomize.

## Prerequisites

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.5
- [Docker](https://docs.docker.com/get-docker/) (kind runs nodes as containers)
- [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- App images built locally and loaded into kind (see below)

## Usage

```bash
cd terraform

terraform init
terraform plan
terraform apply
```

Outputs include the kubeconfig path, kube context, and next steps.

### Load locally built images into kind

The manifests use `imagePullPolicy: IfNotPresent` with local tags, so build and
load the images before (or right after) `apply`:

```bash
# from repo root - build all four service images + frontend
make build-images   # or: docker build ... (see Makefile)

for svc in api-gateway scheduler node-agent metrics; do
  kind load docker-image gpu-cluster-scheduler/$svc:latest --name gpu-cluster-scheduler
done
kind load docker-image gpu-cluster-scheduler/frontend:latest --name gpu-cluster-scheduler
```

### Ingress

```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
echo "127.0.0.1 api.gpu-scheduler.local app.gpu-scheduler.local" | sudo tee -a /etc/hosts
```

Then browse http://app.gpu-scheduler.local and the API at
http://api.gpu-scheduler.local.

## Variables

| Variable             | Default                  | Description                             |
|----------------------|--------------------------|-----------------------------------------|
| `cluster_name`       | `gpu-cluster-scheduler`  | kind cluster name                       |
| `namespace`          | `gpu-scheduler`          | Kubernetes namespace                    |
| `node_agent_count`   | `2`                      | Worker nodes (one node-agent each)      |
| `kubernetes_version` | `v1.30.0`                | kindest/node image tag                  |
| `apply_manifests`    | `true`                   | Run `kubectl apply -k ../deploy/k8s`    |
| `manifests_path`     | `../deploy/k8s`          | kustomize root                          |

## Teardown

```bash
terraform destroy
```

## Cloud targets (EKS / GKE / AKS)

This config targets a local kind cluster for fast iteration. For a managed
cluster:

1. Replace the `kind_cluster` resource with the relevant cluster module, e.g.
   [`terraform-aws-modules/eks/aws`](https://registry.terraform.io/modules/terraform-aws-modules/eks/aws),
   `terraform-google-modules/kubernetes-engine/google`, or the AzureRM AKS
   resource.
2. Point the `kubernetes` provider at that cluster's endpoint / CA / token
   (most modules expose these as outputs, often paired with an auth data source
   such as `aws_eks_cluster_auth`).
3. Drop the ingress `extra_port_mappings` (managed clusters use a cloud load
   balancer via the ingress controller) and use GPU node pools for the
   node-agent DaemonSet.

The namespace, secret, configmap and manifest-apply steps are cloud-agnostic and
carry over unchanged.
