package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Richonn/driftwatch/internal/diff"
)

func makeDrift(kind, name, ns string, dt diff.DriftType) diff.DriftItem {
	return diff.DriftItem{
		Kind:      kind,
		Name:      name,
		Namespace: ns,
		DriftType: dt,
		Status:    diff.DriftStatusOpen,
	}
}

func TestWriteFile_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	r := &diff.DriftReport{
		ScannedAt:      time.Date(2026, 10, 13, 14, 32, 1, 0, time.UTC),
		ClusterContext: "prod",
		GitOpsRepo:     "https://github.com/org/gitops",
		DriftCount:     1,
		Drifts:         []diff.DriftItem{makeDrift("Deployment", "api", "default", diff.DriftSpecDrift)},
	}

	path, err := WriteFile(dir, r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if !strings.Contains(filepath.Base(path), "drift-prod-") {
		t.Errorf("unexpected filename: %s", filepath.Base(path))
	}
}

func TestWriteFile_ValidJSON(t *testing.T) {
	dir := t.TempDir()
	r := &diff.DriftReport{
		ScannedAt:  time.Now(),
		DriftCount: 0,
	}

	path, err := WriteFile(dir, r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
	var decoded diff.DriftReport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("file is not valid JSON: %v", err)
	}
}

func TestWriteFile_ResolvedOnNextScan(t *testing.T) {
	dir := t.TempDir()

	r1 := &diff.DriftReport{
		ScannedAt:      time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC),
		ClusterContext: "prod",
		DriftCount:     2,
		Drifts: []diff.DriftItem{
			makeDrift("Deployment", "api", "default", diff.DriftSpecDrift),
			makeDrift("Service", "redis", "default", diff.DriftMissingInCluster),
		},
	}
	if _, err := WriteFile(dir, r1); err != nil {
		t.Fatalf("write r1: %v", err)
	}

	r2 := &diff.DriftReport{
		ScannedAt:      time.Date(2026, 10, 13, 11, 0, 0, 0, time.UTC),
		ClusterContext: "prod",
		DriftCount:     1,
		Drifts: []diff.DriftItem{
			makeDrift("Deployment", "api", "default", diff.DriftSpecDrift),
		},
	}
	path2, err := WriteFile(dir, r2)
	if err != nil {
		t.Fatalf("write r2: %v", err)
	}

	data, _ := os.ReadFile(path2)
	var decoded diff.DriftReport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	var resolved []diff.DriftItem
	for _, d := range decoded.Drifts {
		if d.Status == diff.DriftStatusResolved {
			resolved = append(resolved, d)
		}
	}
	if len(resolved) != 1 {
		t.Fatalf("expected 1 resolved drift, got %d", len(resolved))
	}
	if resolved[0].Name != "redis" {
		t.Errorf("expected redis to be resolved, got %q", resolved[0].Name)
	}
}

func TestWriteFile_AlreadyResolvedNotDuplicated(t *testing.T) {
	dir := t.TempDir()

	r1 := &diff.DriftReport{
		ScannedAt:      time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC),
		ClusterContext: "prod",
		DriftCount:     1,
		Drifts:         []diff.DriftItem{makeDrift("Service", "redis", "default", diff.DriftMissingInCluster)},
	}
	if _, err := WriteFile(dir, r1); err != nil {
		t.Fatalf("write r1: %v", err)
	}

	r2 := &diff.DriftReport{
		ScannedAt:      time.Date(2026, 10, 13, 11, 0, 0, 0, time.UTC),
		ClusterContext: "prod",
		DriftCount:     0,
		Drifts:         nil,
	}
	if _, err := WriteFile(dir, r2); err != nil {
		t.Fatalf("write r2: %v", err)
	}

	r3 := &diff.DriftReport{
		ScannedAt:      time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC),
		ClusterContext: "prod",
		DriftCount:     0,
		Drifts:         nil,
	}
	path3, err := WriteFile(dir, r3)
	if err != nil {
		t.Fatalf("write r3: %v", err)
	}

	data, _ := os.ReadFile(path3)
	var decoded diff.DriftReport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	count := 0
	for _, d := range decoded.Drifts {
		if d.Name == "redis" {
			count++
		}
	}
	if count > 1 {
		t.Errorf("redis should appear at most once, got %d", count)
	}
}

func TestWriteFile_NoPreviousFile(t *testing.T) {
	dir := t.TempDir()
	r := &diff.DriftReport{
		ScannedAt:      time.Now(),
		ClusterContext: "prod",
		DriftCount:     1,
		Drifts:         []diff.DriftItem{makeDrift("Deployment", "api", "default", diff.DriftSpecDrift)},
	}

	_, err := WriteFile(dir, r)
	if err != nil {
		t.Fatalf("unexpected error on first scan: %v", err)
	}
	if r.Drifts[0].Status != diff.DriftStatusOpen {
		t.Errorf("first scan drift should be open, got %q", r.Drifts[0].Status)
	}
}

func TestWriteFile_FilenameNoContext(t *testing.T) {
	dir := t.TempDir()
	r := &diff.DriftReport{
		ScannedAt:      time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC),
		ClusterContext: "",
		DriftCount:     0,
	}

	path, err := WriteFile(dir, r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	base := filepath.Base(path)
	if !strings.HasPrefix(base, "drift-2026") {
		t.Errorf("unexpected filename without context: %s", base)
	}
}
