package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Richonn/driftwatch/internal/diff"
	"gopkg.in/yaml.v3"
)

func makeReport(drifts []diff.DriftItem) *diff.DriftReport {
	return &diff.DriftReport{
		ScannedAt:      time.Date(2026, 9, 15, 14, 32, 1, 0, time.UTC),
		ClusterContext: "test-cluster",
		GitOpsRepo:     "https://github.com/org/gitops",
		TotalResources: 5,
		DriftCount:     len(drifts),
		Drifts:         drifts,
	}
}

func TestRender_Table_NoDrift(t *testing.T) {
	var buf bytes.Buffer
	hasDrift := Render(&buf, makeReport(nil), "table")

	if hasDrift {
		t.Error("expected hasDrift=false")
	}
	out := buf.String()
	if !strings.Contains(out, "No drift detected") {
		t.Errorf("expected 'No drift detected' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "0 drifts detected") {
		t.Errorf("expected '0 drifts detected' in output, got:\n%s", out)
	}
}

func TestRender_Table_WithDrift(t *testing.T) {
	drifts := []diff.DriftItem{
		{Kind: "Deployment", Name: "api", Namespace: "default", DriftType: diff.DriftSpecDrift, Details: []string{"spec.replicas: cluster=5 gitops=2"}},
		{Kind: "Service", Name: "redis", Namespace: "default", DriftType: diff.DriftMissingInCluster},
		{Kind: "ConfigMap", Name: "cfg", Namespace: "staging", DriftType: diff.DriftMissingInGitOps},
	}
	var buf bytes.Buffer
	hasDrift := Render(&buf, makeReport(drifts), "table")

	if !hasDrift {
		t.Error("expected hasDrift=true")
	}
	out := buf.String()
	if !strings.Contains(out, "SPEC DRIFT") {
		t.Errorf("expected 'SPEC DRIFT' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "MISSING IN CLUSTER") {
		t.Errorf("expected 'MISSING IN CLUSTER' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "MISSING IN GITOPS") {
		t.Errorf("expected 'MISSING IN GITOPS' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "3 drift(s) detected") {
		t.Errorf("expected '3 drift(s) detected' in output, got:\n%s", out)
	}
}

func TestRender_Table_ShowsDetails(t *testing.T) {
	drifts := []diff.DriftItem{
		{Kind: "Deployment", Name: "api", Namespace: "default", DriftType: diff.DriftSpecDrift, Details: []string{"spec.replicas: cluster=5 gitops=2"}},
	}
	var buf bytes.Buffer
	Render(&buf, makeReport(drifts), "table")

	if !strings.Contains(buf.String(), "spec.replicas: cluster=5 gitops=2") {
		t.Error("expected drift detail to appear in table output")
	}
}

func TestRender_JSON(t *testing.T) {
	drifts := []diff.DriftItem{
		{Kind: "Deployment", Name: "api", Namespace: "default", DriftType: diff.DriftSpecDrift},
	}
	r := makeReport(drifts)
	var buf bytes.Buffer
	hasDrift := Render(&buf, r, "json")

	if !hasDrift {
		t.Error("expected hasDrift=true")
	}
	var decoded diff.DriftReport
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if decoded.DriftCount != 1 {
		t.Errorf("expected DriftCount=1, got %d", decoded.DriftCount)
	}
	if decoded.ClusterContext != "test-cluster" {
		t.Errorf("expected ClusterContext='test-cluster', got %q", decoded.ClusterContext)
	}
}

func TestRender_YAML(t *testing.T) {
	r := makeReport(nil)
	var buf bytes.Buffer
	Render(&buf, r, "yaml")

	var decoded diff.DriftReport
	if err := yaml.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid YAML: %v\n%s", err, buf.String())
	}
	if decoded.GitOpsRepo != "https://github.com/org/gitops" {
		t.Errorf("unexpected GitOpsRepo in YAML: %q", decoded.GitOpsRepo)
	}
}

func TestRender_UnknownFormat_FallsBackToTable(t *testing.T) {
	var buf bytes.Buffer
	Render(&buf, makeReport(nil), "unknown")

	if !strings.Contains(buf.String(), "DriftWatch Scan Report") {
		t.Error("unknown format should fall back to table")
	}
}

func TestExitCode(t *testing.T) {
	cases := []struct {
		hasDrift    bool
		failOnDrift bool
		want        int
	}{
		{false, false, 0},
		{false, true, 0},
		{true, false, 0},
		{true, true, 1},
	}
	for _, c := range cases {
		got := ExitCode(c.hasDrift, c.failOnDrift)
		if got != c.want {
			t.Errorf("ExitCode(%v, %v) = %d, want %d", c.hasDrift, c.failOnDrift, got, c.want)
		}
	}
}

func TestRender_Table_Truncate(t *testing.T) {
	longName := strings.Repeat("a", 50)
	drifts := []diff.DriftItem{
		{Kind: "Deployment", Name: longName, Namespace: "default", DriftType: diff.DriftMissingInGitOps},
	}
	var buf bytes.Buffer
	Render(&buf, makeReport(drifts), "table")

	if strings.Contains(buf.String(), longName) {
		t.Error("long name should be truncated in table output")
	}
}
