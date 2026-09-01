package config

import (
	"errors"
	"os"
	"path/filepath"
)

type Config struct {
	KubeConfig  string
	Context     string
	Namespace   string
	Repo        string
	Branch      string
	Path        string
	Output      string
	FailOnDrift bool
	Token       string
}

func DefaultKubeConfig() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".kube", "config")
}

func (c *Config) Validate() error {
	if c.Repo == "" {
		return errors.New("--repo is required")
	}
	validOutputs := map[string]bool{"table": true, "json": true, "yaml": true}
	if !validOutputs[c.Output] {
		return errors.New("--output must be one of: table, json, yaml")
	}
	return nil
}
