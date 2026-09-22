package main

import (
	"fmt"
	"os"

	"github.com/Richonn/driftwatch/internal/cluster"
	"github.com/Richonn/driftwatch/internal/config"
	"github.com/Richonn/driftwatch/internal/diff"
	"github.com/Richonn/driftwatch/internal/gitops"
	"github.com/Richonn/driftwatch/internal/report"
	"github.com/spf13/cobra"
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

	_ = scanCmd.MarkFlagRequired("repo")
	scanCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := cfg.Validate(); err != nil {
			return err
		}

		client, err := cluster.NewClient(&cfg)
		if err != nil {
			return fmt.Errorf("connect to cluster: %w", err)
		}

		fmt.Fprintln(os.Stderr, "→ Fetching live resources from cluster…")
		liveResources, err := cluster.FetchResources(client, &cfg)
		if err != nil {
			return fmt.Errorf("fetch cluster resources: %w", err)
		}

		fmt.Fprintf(os.Stderr, "→ Cloning %s (branch: %s)…\n", cfg.Repo, cfg.Branch)
		cloneResult, err := gitops.CloneRepo(cfg.Repo, cfg.Branch, cfg.Token)
		if err != nil {
			return fmt.Errorf("clone repo: %w", err)
		}
		defer cloneResult.Cleanup()

		fmt.Fprintln(os.Stderr, "→ Parsing GitOps manifests…")
		gitopsResources, err := gitops.ParseManifests(cloneResult.Dir, cfg.Path)
		if err != nil {
			return fmt.Errorf("parse manifests: %w", err)
		}

		driftReport := diff.Compare(liveResources, gitopsResources, cfg.Context, cfg.Repo)

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
