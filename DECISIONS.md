# DriftWatch — Architecture Decisions

This document captures the key technical choices made for DriftWatch and the reasoning behind each one. The goal is to make the trade-offs explicit so that future contributors (or a future self) can understand the intent before changing direction.

---

## 1. CLI tool — not a GitHub Action, not a Kubernetes operator

**Decision:** DriftWatch is a standalone CLI binary.

**Alternatives considered:**
- **GitHub Action** — would tie the tool to GitHub CI and make it unusable outside of a pipeline. A CLI can be invoked from any CI system (GitHub Actions, GitLab CI, Jenkins, local terminal) by simply running the binary.
- **Kubernetes operator** — an operator runs continuously inside the cluster and is the right model for *reactive* automation. DriftWatch is intentionally *on-demand*: you run it when you want a snapshot, or you schedule it yourself. An operator adds significant deployment complexity (CRDs, RBAC, controller-runtime) for a v1 that doesn't need continuous reconciliation.

**Why CLI:**
- Zero cluster-side installation — no RBAC to grant, no CRD to install, no operator to manage.
- Works from a developer laptop, a CI runner, or a cron job with the exact same binary.
- Easy to script: pipe the JSON output, use `--fail-on-drift` to gate a pipeline, grep the table output.
- Faster iteration: shipping a new binary is simpler than upgrading a running operator.

The operator model is in the v2 roadmap for teams that want continuous drift alerting built into the cluster.

---

## 2. `client-go` — not `kubectl` wrapped in `exec.Command`

**Decision:** Use the official `k8s.io/client-go` library to talk to the Kubernetes API.

**Alternatives considered:**
- **Wrapping `kubectl`** — parse the output of `kubectl get -o json`. Simple to start, but brittle: depends on `kubectl` being in PATH, on the output format staying stable, and makes structured access to API objects painful. Error handling is also harder (exit codes vs. structured API errors).
- **`dynamic` client only** — `client-go` also has a dynamic client that works with `unstructured.Unstructured` objects, which avoids importing typed API packages. Useful for CRDs, but makes field access verbose and error-prone for known resource types.

**Why `client-go`:**
- Typed structs for all built-in resources — `appsv1.Deployment`, `corev1.Service`, etc. — mean compile-time safety and straightforward field access.
- First-class support for kubeconfig, in-cluster config, and multi-context out of the box.
- The same library `kubectl` itself uses — no risk of format drift.
- Enables future support for `--watch` mode via informers without changing the client layer.

---

## 3. `go-git` — not shelling out to `git`

**Decision:** Use `github.com/go-git/go-git` for all Git operations (clone, pull, walk).

**Alternatives considered:**
- **`exec.Command("git", ...)`** — requires `git` to be installed on the machine running DriftWatch. This is fine on a developer laptop but not guaranteed in a minimal CI container or in a future Docker image. It also makes testing harder (no mocking, real filesystem side effects).
- **GitHub/GitLab REST API** — would let us list and read files without cloning. But it couples the tool to specific Git hosting providers and requires pagination for large repos. We want to support any HTTPS or SSH remote.

**Why `go-git`:**
- Pure Go — zero system dependency. The binary is fully self-contained.
- Works with any Git remote over HTTPS or SSH, regardless of hosting provider.
- Supports token auth and SSH key auth natively.
- Cloning into `os.MkdirTemp` + `defer os.RemoveAll` gives clean, testable isolation with no leftover state.
- The library is mature and widely used in the Go ecosystem (e.g. Flux itself uses it).

---

## 4. Cobra — not `flag` or `urfave/cli`

**Decision:** Use `github.com/spf13/cobra` as the CLI framework.

**Alternatives considered:**
- **`flag` (stdlib)** — sufficient for a single command but has no built-in concept of subcommands, help formatting, or shell completion generation. We'd reimplement all of that.
- **`urfave/cli`** — a solid alternative, but Cobra has broader adoption in the Kubernetes ecosystem (kubectl, helm, k9s, and most Go CLIs use it), which means more familiar patterns for contributors coming from that world.

**Why Cobra:**
- First-class subcommand support (`scan`, `version`, `completion`) with automatic `--help` generation.
- Built-in shell completion for bash, zsh, fish, and PowerShell via `cobra.Command.GenBashCompletion` — no extra code needed.
- `MarkFlagRequired` and flag validation hooks make input validation declarative.
- The de facto standard in the Go/K8s ecosystem — contributors know it.

---

## 5. GoReleaser — not manual `goreleaser` scripts or GitHub Actions matrix

**Decision:** Use GoReleaser to build and publish releases.

**Alternatives considered:**
- **Manual GitHub Actions matrix** — a `strategy.matrix` over `GOOS`/`GOARCH` can cross-compile the binary, but assembling archives, generating checksums, writing changelogs, and publishing to GitHub Releases requires significant boilerplate YAML.
- **`Makefile` with `go build` loops** — works locally but doesn't integrate with GitHub Releases or Homebrew without additional scripts.

**Why GoReleaser:**
- Single `.goreleaser.yml` declares all targets, archive formats, checksums, and the changelog strategy.
- Native GitHub Releases integration — one `goreleaser release` command does everything.
- Homebrew tap generation out of the box, enabling `brew install` without manual formula maintenance.
- Reproducible: the same config runs identically in CI and locally.
