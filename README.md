# DriftWatch

> Detect drift between your live Kubernetes cluster and your GitOps repository — works with ArgoCD and Flux.

[![CI](https://github.com/Richonn/DriftWatch/actions/workflows/ci.yml/badge.svg)](https://github.com/Richonn/DriftWatch/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.26+-00ADD8.svg)](go.mod)

---

## What is DriftWatch?

DriftWatch is a CLI tool written in Go that compares the **live state of your Kubernetes cluster** against the **desired state declared in your GitOps repository**. It helps you catch configuration drift before it causes incidents.

It detects three types of drift:

- **Missing in cluster** — a resource is defined in GitOps but absent from the cluster
- **Missing in GitOps** — a resource is running in the cluster but not tracked in Git
- **Spec drift** — a resource exists on both sides but with differing configuration (image, replicas, resources, etc.)

---

## Installation

### Binary (Linux / macOS)

```bash
curl -sSfL https://github.com/Richonn/DriftWatch/releases/latest/download/driftwatch_linux_amd64.tar.gz | tar xz
sudo mv driftwatch /usr/local/bin/
```

> Windows and ARM binaries are available on the [releases page](https://github.com/Richonn/DriftWatch/releases).

### Build from source

```bash
git clone https://github.com/Richonn/DriftWatch.git
cd DriftWatch
go build -o driftwatch ./cmd/driftwatch
```

Go 1.26 or later is required.

---

## Quick start

```bash
# Scan all namespaces against a public GitOps repo
driftwatch scan --repo https://github.com/your-org/your-gitops-repo

# Scan a specific namespace, on a specific cluster context
driftwatch scan \
  --repo https://github.com/your-org/your-gitops-repo \
  --namespace production \
  --context my-cluster

# Scan a monorepo — only look at the k8s/ subfolder
driftwatch scan \
  --repo https://github.com/your-org/monorepo \
  --path k8s/

# Private repo with a token
driftwatch scan \
  --repo https://github.com/your-org/private-gitops \
  --token $GITHUB_TOKEN

# Output as JSON (useful for scripting)
driftwatch scan --repo https://github.com/your-org/gitops --output json
```

### Example output

```
DriftWatch Scan Report
Scanned at : 2026-09-15 14:32:01
Cluster    : my-cluster
GitOps repo: https://github.com/your-org/gitops

KIND             NAME                                 NAMESPACE            STATUS
────────────────────────────────────────────────────────────────────────────────────────────────────────
Deployment       api-server                           production           ⚠ SPEC DRIFT (container "api" image: cluster=myrepo/api:v1.3.1 gitops=myrepo/api:v1.2.0)
Service          redis                                production           ✗ MISSING IN CLUSTER
Deployment       legacy-worker                        production           ⚠ MISSING IN GITOPS
────────────────────────────────────────────────────────────────────────────────────────────────────────
✗ 12 resources scanned, 3 drift(s) detected
```

---

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--repo` | *(required)* | GitOps repository URL (HTTPS or SSH) |
| `--branch` | `main` | Branch to compare against |
| `--path` | `.` | Subdirectory within the repo (useful for monorepos) |
| `--namespace` | all | Restrict the scan to a specific namespace |
| `--kubeconfig` | `~/.kube/config` | Path to the kubeconfig file |
| `--context` | current context | Kubeconfig context to use |
| `--token` | | Auth token for private repositories |
| `--output` | `table` | Output format: `table`, `json`, or `yaml` |
| `--fail-on-drift` | `false` | Exit with code 1 if any drift is detected |

---

## Usage in CI

DriftWatch is designed to fit into your CI/CD pipelines. Use `--fail-on-drift` to block a pipeline when drift is detected.

```yaml
# .github/workflows/drift-check.yml
name: Drift Check

on:
  schedule:
    - cron: "0 * * * *"   # every hour
  workflow_dispatch:

jobs:
  scan:
    runs-on: ubuntu-latest
    steps:
      - name: Download DriftWatch
        run: |
          curl -sSfL https://github.com/Richonn/DriftWatch/releases/latest/download/driftwatch_linux_amd64.tar.gz | tar xz
          sudo mv driftwatch /usr/local/bin/

      - name: Run drift scan
        run: |
          driftwatch scan \
            --repo https://github.com/${{ github.repository_owner }}/gitops \
            --token ${{ secrets.GITHUB_TOKEN }} \
            --output json \
            --fail-on-drift
```

---

## Supported resources

DriftWatch compares the following Kubernetes resource types:

| Kind | Compared fields |
|------|----------------|
| `Deployment` | `replicas`, container images, resource requests/limits, labels |
| `StatefulSet` | `replicas`, container images, resource requests/limits, labels |
| `DaemonSet` | Container images, resource requests/limits, labels |
| `Service` | `spec.type`, `spec.clusterIP` |
| `ConfigMap` | All keys in `data` |
| `Secret` | Key names only — values are **never** read or logged |
| `Ingress` | `spec.ingressClassName` |
| `ServiceAccount` | Presence only |
| `NetworkPolicy` | Presence + spec |

System namespaces (`kube-system`, `kube-public`, `kube-node-lease`) are automatically excluded.

---

## Limitations (v1)

- **Helm charts are not rendered.** DriftWatch detects `Chart.yaml` and warns, but does not template the chart for comparison. Helm support is planned for v2.
- **Kustomize overlays are not applied.** Raw manifests only.
- **CRDs are not supported.** Only built-in Kubernetes resource types are compared.

---

## Roadmap

| Feature | Status |
|---------|--------|
| Core drift detection (Deployments, Services, ConfigMaps…) | ✅ v1 |
| JSON / YAML output | ✅ v1 |
| `--fail-on-drift` CI mode | ✅ v1 |
| Shell autocompletion (bash/zsh/fish/powershell) | ✅ v1 |
| Multi-platform releases via GoReleaser | ✅ v1 |
| CI (lint, tests, cross-build) | ✅ v1 |
| Helm chart rendering | v2 |
| Kustomize overlay support | v2 |
| CRD support | v2 |
| `--watch` continuous mode | v2 |
| Slack / webhook notifications | v2 |
| Flux `GitRepository` support | v2 |
| Docker image | v2 |
| Kubernetes operator | v2 |

---

## License

MIT — see [LICENSE](LICENSE).
