package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Richonn/driftwatch/internal/diff"
)

func WriteFile(dir string, r *diff.DriftReport) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create report dir: %w", err)
	}

	prev, _ := loadPreviousReport(dir, r.ClusterContext)
	if prev != nil {
		applyResolved(r, prev)
	}

	ts := r.ScannedAt.UTC().Format("2006-01-02T15-04-05Z")
	ctx := sanitize(r.ClusterContext)
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
	if err := enc.Encode(r); err != nil {
		return "", fmt.Errorf("write report: %w", err)
	}
	return path, nil
}

type driftKey struct {
	Kind      string
	Name      string
	Namespace string
	DriftType diff.DriftType
}

func keyOf(item diff.DriftItem) driftKey {
	return driftKey{item.Kind, item.Name, item.Namespace, item.DriftType}
}

func applyResolved(current, prev *diff.DriftReport) {
	currentKeys := make(map[driftKey]bool, len(current.Drifts))
	for _, d := range current.Drifts {
		currentKeys[keyOf(d)] = true
	}

	for _, pd := range prev.Drifts {
		if pd.Status == diff.DriftStatusResolved {
			continue
		}
		if !currentKeys[keyOf(pd)] {
			pd.Status = diff.DriftStatusResolved
			current.Drifts = append(current.Drifts, pd)
		}
	}
}

func loadPreviousReport(dir, clusterContext string) (*diff.DriftReport, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	ctx := sanitize(clusterContext)
	var candidates []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		var prefix string
		if ctx != "" {
			prefix = "drift-" + ctx + "-"
		} else {
			prefix = "drift-"
		}
		if strings.HasPrefix(e.Name(), prefix) {
			candidates = append(candidates, e.Name())
		}
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	sort.Strings(candidates)
	latest := candidates[len(candidates)-1]

	data, err := os.ReadFile(filepath.Join(dir, latest))
	if err != nil {
		return nil, err
	}
	var r diff.DriftReport
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func sanitize(s string) string {
	r := strings.NewReplacer("/", "-", ":", "-", " ", "-")
	return r.Replace(s)
}
