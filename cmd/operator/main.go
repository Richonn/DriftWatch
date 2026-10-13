package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Richonn/driftwatch/internal/cluster"
	"github.com/Richonn/driftwatch/internal/config"
	"github.com/Richonn/driftwatch/internal/operator"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "driftwatch-operator: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	fmt.Printf("driftwatch-operator %s (commit: %s, built: %s)\n", version, commit, date)

	cfg := &config.Config{}

	kubeClient, err := cluster.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("connect to cluster: %w", err)
	}

	dynClient, err := cluster.NewDynamicClient(cfg)
	if err != nil {
		return fmt.Errorf("dynamic client: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	ctrl := operator.NewController(kubeClient, dynClient)
	ctrl.Run(ctx)
	return nil
}
