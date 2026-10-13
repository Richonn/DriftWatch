package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Richonn/driftwatch/internal/cluster"
	"github.com/Richonn/driftwatch/internal/config"
	"github.com/Richonn/driftwatch/internal/diff"
	"github.com/Richonn/driftwatch/internal/gitops"
	"github.com/Richonn/driftwatch/internal/notify"
	"github.com/Richonn/driftwatch/internal/report"
	"github.com/spf13/cobra"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "driftwatch: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Config{}

	rootCmd := new(cobra.Command)
	rootCmd.Use = "driftwatch"
	rootCmd.Short = "Detect drift between your K8s cluster and your GitOps repo"

	scanCmd := new(cobra.Command)
	scanCmd.Use = "scan"
	scanCmd.Short = "Launch drift scan"
	scanCmd.Flags().StringVar(&cfg.Branch, "branch", "main", "Branch to compare against")
	scanCmd.Flags().StringVar(&cfg.Context, "context", "", "Kubeconfig context to use")
	scanCmd.Flags().BoolVar(&cfg.FailOnDrift, "fail-on-drift", false, "Exit with code 1 if drift is detected")
	scanCmd.Flags().StringVar(&cfg.KubeConfig, "kubeconfig", config.DefaultKubeConfig(), "Path to the kubeconfig file")
	scanCmd.Flags().StringVar(&cfg.Namespace, "namespace", "", "Restrict scan to a specific namespace")
	scanCmd.Flags().StringVar(&cfg.Output, "output", "table", "Output format: table, json, yaml")
	scanCmd.Flags().StringVar(&cfg.Path, "path", ".", "Subdirectory within the repo to scan")
	scanCmd.Flags().StringVar(&cfg.Repo, "repo", "", "GitOps repository URL (HTTPS or SSH)")
	scanCmd.Flags().StringVar(&cfg.Token, "token", "", "Auth token for private repositories")
	scanCmd.Flags().StringArrayVar(&cfg.HelmValues, "helm-values", nil, "Additional values file(s) for Helm chart rendering (can be repeated)")
	scanCmd.Flags().StringArrayVar(&cfg.ExcludeNamespaces, "exclude-namespace", nil, "Additional namespaces to exclude from the scan (can be repeated)")
	scanCmd.Flags().BoolVar(&cfg.Flux, "flux", false, "Auto-discover repo and path from Flux GitRepository/Kustomization resources")
	scanCmd.Flags().BoolVar(&cfg.ArgoCD, "argocd", false, "Auto-discover repo and path from ArgoCD Application resources")
	scanCmd.Flags().BoolVar(&cfg.Watch, "watch", false, "Re-run scan at regular intervals (Ctrl+C to stop)")
	scanCmd.Flags().IntVar(&cfg.WatchInterval, "interval", 60, "Seconds between scans in watch mode")
	scanCmd.Flags().StringVar(&cfg.WebhookURL, "webhook", "", "Webhook URL to notify on drift (Slack incoming webhook or generic HTTP endpoint)")
	scanCmd.Flags().StringArrayVar(&cfg.Clusters, "cluster", nil, "Kubeconfig context name to scan (can be repeated for multi-cluster)")

	scanCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if !cfg.Flux && !cfg.ArgoCD && cfg.Repo == "" {
			return fmt.Errorf("--repo is required (or use --flux / --argocd to auto-discover)")
		}
		if err := cfg.Validate(); err != nil {
			return err
		}

		if cfg.Flux {
			return runFluxScan(&cfg)
		}

		if cfg.ArgoCD {
			return runArgoCDScan(&cfg)
		}

		if len(cfg.Clusters) > 0 {
			return runMultiClusterScan(&cfg)
		}

		if cfg.Watch {
			return runWatch(&cfg)
		}

		driftReport, err := runScan(&cfg)
		if err != nil {
			return err
		}
		sendNotification(&cfg, driftReport)
		report.RenderToStdout(driftReport, cfg.Output, cfg.FailOnDrift)
		return nil
	}

	versionCmd := new(cobra.Command)
	versionCmd.Use = "version"
	versionCmd.Short = "Print the version of driftwatch"
	versionCmd.Run = func(cmd *cobra.Command, args []string) {
		fmt.Printf("driftwatch %s (commit: %s, built: %s)\n", version, commit, date)
	}

	completionCmd := new(cobra.Command)
	completionCmd.Use = "completion [bash|zsh|fish|powershell]"
	completionCmd.Short = "Generate shell autocompletion script"
	completionCmd.Long = `Generate an autocompletion script for driftwatch in the specified shell.

Bash:
  driftwatch completion bash > /etc/bash_completion.d/driftwatch

Zsh:
  driftwatch completion zsh > "${fpath[1]}/_driftwatch"

Fish:
  driftwatch completion fish > ~/.config/fish/completions/driftwatch.fish

PowerShell:
  driftwatch completion powershell | Out-String | Invoke-Expression
`
	completionCmd.ValidArgs = []string{"bash", "zsh", "fish", "powershell"}
	completionCmd.Args = cobra.ExactArgs(1)
	completionCmd.RunE = func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		default:
			return fmt.Errorf("unsupported shell: %s", args[0])
		}
	}

	rootCmd.AddCommand(scanCmd, versionCmd, completionCmd)

	return rootCmd.Execute()
}

func runScan(cfg *config.Config) (*diff.DriftReport, error) {
	fmt.Fprintln(os.Stderr, "→ Fetching live resources from cluster…")
	client, err := cluster.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("connect to cluster: %w", err)
	}

	liveResources, err := cluster.FetchResources(client, cfg)
	if err != nil {
		return nil, fmt.Errorf("fetch cluster resources: %w", err)
	}

	fmt.Fprintf(os.Stderr, "→ Cloning %s (branch: %s)…\n", cfg.Repo, cfg.Branch)
	cloneResult, err := gitops.CloneRepo(cfg.Repo, cfg.Branch, cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("clone repo: %w", err)
	}
	defer cloneResult.Cleanup()

	fmt.Fprintln(os.Stderr, "→ Parsing GitOps manifests…")
	gitopsResources, err := gitops.ParseManifests(cloneResult.Dir, cfg.Path, gitops.HelmOptions{ValuesFiles: cfg.HelmValues})
	if err != nil {
		return nil, fmt.Errorf("parse manifests: %w", err)
	}

	return diff.Compare(liveResources, gitopsResources, cfg.Context, cfg.Repo), nil
}

func runArgoCDScan(cfg *config.Config) error {
	dynClient, err := cluster.NewDynamicClient(cfg)
	if err != nil {
		return fmt.Errorf("connect to cluster: %w", err)
	}

	fmt.Fprintln(os.Stderr, "→ Discovering ArgoCD applications…")
	sources, err := cluster.DiscoverArgoCDSources(dynClient)
	if err != nil {
		return fmt.Errorf("discover argocd sources: %w", err)
	}
	if len(sources) == 0 {
		return fmt.Errorf("no ArgoCD Application resources found in cluster")
	}

	var hasDrift bool
	for _, src := range sources {
		fmt.Fprintf(os.Stderr, "\n→ Scanning ArgoCD app: %s/%s (%s @ %s)\n", src.Namespace, src.Name, src.URL, src.Branch)
		scoped := *cfg
		scoped.Repo = src.URL
		scoped.Branch = src.Branch
		scoped.Path = src.Path

		driftReport, err := runScan(&scoped)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warn: scan failed for %s/%s: %v\n", src.Namespace, src.Name, err)
			continue
		}
		sendNotification(cfg, driftReport)
		if report.Render(os.Stdout, driftReport, cfg.Output) {
			hasDrift = true
		}
	}

	os.Exit(report.ExitCode(hasDrift, cfg.FailOnDrift))
	return nil
}

func runFluxScan(cfg *config.Config) error {
	dynClient, err := cluster.NewDynamicClient(cfg)
	if err != nil {
		return fmt.Errorf("connect to cluster: %w", err)
	}

	fmt.Fprintln(os.Stderr, "→ Discovering Flux sources…")
	sources, err := cluster.DiscoverFluxSources(dynClient)
	if err != nil {
		return fmt.Errorf("discover flux sources: %w", err)
	}
	if len(sources) == 0 {
		return fmt.Errorf("no Flux GitRepository/Kustomization resources found in cluster")
	}

	var hasDrift bool
	for _, src := range sources {
		fmt.Fprintf(os.Stderr, "\n→ Scanning Flux source: %s/%s (%s @ %s)\n", src.Namespace, src.Name, src.URL, src.Branch)
		scoped := *cfg
		scoped.Repo = src.URL
		scoped.Branch = src.Branch
		scoped.Path = src.Path

		driftReport, err := runScan(&scoped)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warn: scan failed for %s/%s: %v\n", src.Namespace, src.Name, err)
			continue
		}
		sendNotification(cfg, driftReport)
		if report.Render(os.Stdout, driftReport, cfg.Output) {
			hasDrift = true
		}
	}

	os.Exit(report.ExitCode(hasDrift, cfg.FailOnDrift))
	return nil
}

func sendNotification(cfg *config.Config, driftReport *diff.DriftReport) {
	if cfg.WebhookURL == "" {
		return
	}
	if err := notify.Send(cfg.WebhookURL, driftReport); err != nil {
		fmt.Fprintf(os.Stderr, "warn: webhook notification failed: %v\n", err)
	}
}

func runMultiClusterScan(cfg *config.Config) error {
	fmt.Fprintf(os.Stderr, "→ Cloning %s (branch: %s)…\n", cfg.Repo, cfg.Branch)
	cloneResult, err := gitops.CloneRepo(cfg.Repo, cfg.Branch, cfg.Token)
	if err != nil {
		return fmt.Errorf("clone repo: %w", err)
	}
	defer cloneResult.Cleanup()

	fmt.Fprintln(os.Stderr, "→ Parsing GitOps manifests…")
	gitopsResources, err := gitops.ParseManifests(cloneResult.Dir, cfg.Path, gitops.HelmOptions{ValuesFiles: cfg.HelmValues})
	if err != nil {
		return fmt.Errorf("parse manifests: %w", err)
	}

	var hasDrift bool
	for _, ctx := range cfg.Clusters {
		fmt.Fprintf(os.Stderr, "\n→ Scanning cluster context: %s\n", ctx)
		scoped := *cfg
		scoped.Context = ctx

		client, err := cluster.NewClient(&scoped)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warn: connect to cluster %s: %v\n", ctx, err)
			continue
		}
		liveResources, err := cluster.FetchResources(client, &scoped)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warn: fetch resources from %s: %v\n", ctx, err)
			continue
		}

		driftReport := diff.Compare(liveResources, gitopsResources, ctx, cfg.Repo)
		sendNotification(cfg, driftReport)
		if report.Render(os.Stdout, driftReport, cfg.Output) {
			hasDrift = true
		}
	}

	os.Exit(report.ExitCode(hasDrift, cfg.FailOnDrift))
	return nil
}

func runWatch(cfg *config.Config) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Fprintf(os.Stderr, "Watch mode — scanning every %ds (Ctrl+C to stop)\n", cfg.WatchInterval)

	tick := func() {
		fmt.Print("\033[2J\033[H")
		driftReport, err := runScan(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "scan error: %v\n", err)
			return
		}
		sendNotification(cfg, driftReport)
		report.Render(os.Stdout, driftReport, cfg.Output)
		fmt.Fprintf(os.Stderr, "Next scan in %ds — Ctrl+C to stop\n", cfg.WatchInterval)
	}

	tick()

	ticker := time.NewTicker(time.Duration(cfg.WatchInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tick()
		case <-ctx.Done():
			fmt.Fprintln(os.Stderr, "\nWatch stopped.")
			return nil
		}
	}
}
