package main

import (
	"fmt"
	"os"

	"github.com/Richonn/driftwatch/internal/config"
	"github.com/spf13/cobra"
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
	scanCmd.Flags().StringVar(&cfg.Branch, "branch", "main", "branch")
	scanCmd.Flags().StringVar(&cfg.Context, "context", "", "context")
	scanCmd.Flags().BoolVar(&cfg.FailOnDrift, "fail-on-drift", false, "Fail on drift")
	scanCmd.Flags().StringVar(&cfg.KubeConfig, "kubeconfig", config.DefaultKubeConfig(), "Kube config")
	scanCmd.Flags().StringVar(&cfg.Namespace, "namespace", "", "namespace")
	scanCmd.Flags().StringVar(&cfg.Output, "output", "table", "Repo Git's URL")
	scanCmd.Flags().StringVar(&cfg.Path, "path", ".", "Repo Git's URL")
	scanCmd.Flags().StringVar(&cfg.Repo, "repo", "", "Repo Git's URL")
	scanCmd.Flags().StringVar(&cfg.Token, "token", "", "Repo Git's URL")

	scanCmd.MarkFlagRequired("repo")
	scanCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := cfg.Validate(); err != nil {
			return err
		}
		return nil
	}

	versionCmd := new(cobra.Command)
	versionCmd.Use = "version"
	versionCmd.Run = func(cmd *cobra.Command, args []string) {
		println("drifwatch v0.1.0")
	}

	rootCmd.AddCommand(scanCmd, versionCmd)

	return rootCmd.Execute()
}
