# DriftWatch v1.0.0

First stable release. DriftWatch is a CLI tool written in Go that detects drift between the live state of a Kubernetes cluster and the desired state declared in a GitOps repository.

---

## Installation

**Homebrew (macOS / Linux)**
```bash
brew tap Richonn/driftwatch
brew install driftwatch
```

**Binary**
```bash
curl -sSfL https://github.com/Richonn/DriftWatch/releases/latest/download/driftwatch_linux_amd64.tar.gz | tar xz
sudo mv driftwatch /usr/local/bin/
```

**Docker**
```bash
docker run --rm \
  -v ~/.kube/config:/home/nonroot/.kube/config:ro \
  ghcr.io/richonn/driftwatch:latest \
  scan --repo https://github.com/your-org/gitops
```

---

## What's included

### Core drift detection
Compares live cluster state against GitOps manifests by `(Kind, Name, Namespace)` triplet and reports three drift types:
- `MISSING_IN_CLUSTER` — manifest defined in GitOps but resource not found in cluster
- `MISSING_IN_GITOPS` — resource running in cluster but not tracked in Git
- `SPEC_DRIFT` — resource exists on both sides with differing configuration

Supported resource types: `Deployment`, `StatefulSet`, `DaemonSet`, `Service`, `ConfigMap`, `Secret` (key names only), `Ingress`, `ServiceAccount`, `NetworkPolicy`.

### GitOps rendering
- **Raw YAML** — recursive walk, multi-document support
- **Helm** — full chart rendering via the Helm Go SDK (`--helm-values` for value overrides)
- **Kustomize** — native rendering (no SDK) with support for `namespace`, `namePrefix`, `nameSuffix`, `images`, `patchesStrategicMerge`, `patchesJson6902`, and inline `patches`

### Auto-discovery
- `--flux` — reads `GitRepository` + `Kustomization` CRs directly from the cluster
- `--argocd` — reads `Application` CRs directly from the cluster

### CRD support
- `--crd` — fetches and compares Custom Resource Definition instances via the discovery API + dynamic client
- Generic top-level field diff for unknown resource shapes

### Output & CI
- `--output table|json|yaml`
- `--fail-on-drift` — exits with code 1 when drift is detected, designed for CI pipelines
- `--watch` + `--interval` — continuous scan mode with Ctrl+C support
- `--webhook` — Slack-compatible HTTP webhook notifications on drift

### Multi-cluster
- `--cluster` (repeatable) — scans multiple kubeconfig contexts against the same GitOps repo; the repo is cloned and parsed only once

### Namespace control
- `--namespace` — restrict scan to one namespace
- `--exclude-namespace` (repeatable) — exclude additional namespaces on top of the built-in system list

### Per-scan report files
- `--report-dir` — writes one JSON file per scan session (`drift-<context>-<timestamp>.json`)
- Drifts that disappear between scans are automatically marked `resolved` by comparing against the previous report file

### Kubernetes Operator
A separate `driftwatch-operator` binary reconciles `DriftWatchScan` custom resources and writes results to their `status`:

```bash
kubectl apply -f https://github.com/Richonn/DriftWatch/releases/latest/download/crd.yaml
kubectl apply -f https://github.com/Richonn/DriftWatch/releases/latest/download/rbac.yaml
kubectl apply -f https://github.com/Richonn/DriftWatch/releases/latest/download/deployment.yaml
```

```yaml
apiVersion: driftwatch.io/v1alpha1
kind: DriftWatchScan
metadata:
  name: production
  namespace: driftwatch-system
spec:
  repo: https://github.com/your-org/gitops
  branch: main
  interval: 300
```

```bash
kubectl get driftwatchscans
# NAME         REPO                                DRIFTS   LAST SCAN              AGE
# production   https://github.com/your-org/gitops  3        2026-10-13T14:32:01Z   2d
```

### Distribution
- Multi-platform binaries via GoReleaser: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`
- Docker images on GHCR: `ghcr.io/richonn/driftwatch` and `ghcr.io/richonn/driftwatch-operator` (base images pinned by SHA256 digest)
- Homebrew tap: `Richonn/homebrew-driftwatch`
- SHA256 checksums for all archives

### CI & quality
- GitHub Actions: lint, test, cross-platform build — all actions pinned by SHA
- Test coverage: `internal/diff` 94.8%, `internal/gitops` 88.2%, overall 90.5%
- Real-cluster bug fixes validated with kind: `clusterIP` false drift, system namespace leakage

---

## Limitations

- Helm subchart dependencies are not fetched automatically — run `helm dependency update` before scanning
- Kustomize remote bases are not supported — only local resources are resolved
- CRD spec comparison is generic (top-level field diff), not schema-aware

---

## Checksums

See `checksums.txt` attached to this release.
