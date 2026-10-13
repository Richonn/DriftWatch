package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Richonn/driftwatch/internal/diff"
)

func WriteFile(dir string, report *diff.DriftReport) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create report dir: %w", err)
	}

	ts := report.ScannedAt.UTC().Format("2006-01-02T15-04-05Z")
	ctx := sanitize(report.ClusterContext)
	var name string
	if ctx != "" {
		name = fmt.Sprintf("drift-%s-%s.json", ctx, ts)
	} else {
		name = fmt.Sprintf("drift-%s.json", ts)
	}

	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create report file: %w", err)
	}
	defer func() { _ = f.Close() }()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		return "", fmt.Errorf("write report: %w", err)
	}
	return path, nil
}

func sanitize(s string) string {
	r := strings.NewReplacer("/", "-", ":", "-", " ", "-")
	return r.Replace(s)
}
