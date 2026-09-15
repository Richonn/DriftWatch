package gitops

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Richonn/driftwatch/internal/cluster"
)

// testdataDir returns the absolute path to the testdata/manifests directory.
func testdataDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// parse_test.go is at internal/gitops/ — go up two levels to reach repo root
	root := filepath.Join(filepath.Dir(file), "..", "..")
	return filepath.Join(root, "testdata", "manifests")
}

func findResource(resources []cluster.Resource, kind, name, ns string) *cluster.Resource {
	for i := range resources {
		r := &resources[i]
		if r.Kind == kind && r.Name == name && r.Namespace == ns {
			return r
		}
	}
	return nil
}

func TestParseManifests_Deployment(t *testing.T) {
	resources, err := ParseManifests(testdataDir(t), ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := findResource(resources, "Deployment", "api-server", "default")
	if found == nil {
		t.Fatal("expected Deployment 'api-server' in 'default'")
	}
	if found.Labels["app"] != "api-server" {
		t.Errorf("label app: got %q, want %q", found.Labels["app"], "api-server")
	}
}

func TestParseManifests_MultiDoc(t *testing.T) {
	resources, err := ParseManifests(testdataDir(t), ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if findResource(resources, "Deployment", "api-server", "default") == nil {
		t.Error("expected Deployment 'api-server'")
	}
	if findResource(resources, "Deployment", "worker", "staging") == nil {
		t.Error("expected Deployment 'worker'")
	}
}

func TestParseManifests_ConfigMap(t *testing.T) {
	resources, err := ParseManifests(testdataDir(t), ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := findResource(resources, "ConfigMap", "feature-flags", "default")
	if found == nil {
		t.Fatal("expected ConfigMap 'feature-flags'")
	}
}

func TestParseManifests_SecretKeysOnly(t *testing.T) {
	resources, err := ParseManifests(testdataDir(t), ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := findResource(resources, "Secret", "db-creds", "default")
	if found == nil {
		t.Fatal("expected Secret 'db-creds'")
	}
	if _, hasData := found.Spec["data"]; hasData {
		t.Error("Secret spec must not expose raw data values")
	}
}

func TestParseManifests_UnsupportedKindIgnored(t *testing.T) {
	resources, err := ParseManifests(testdataDir(t), ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, r := range resources {
		if r.Kind == "CronJob" {
			t.Error("CronJob should be ignored (unsupported kind)")
		}
	}
}

func TestParseManifests_HelmChartSkipped(t *testing.T) {
	resources, err := ParseManifests(testdataDir(t), ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, r := range resources {
		if r.Name == "my-chart" {
			t.Errorf("Helm Chart.yaml should be skipped, got resource: %+v", r)
		}
	}
}

func TestParseManifests_NonEmptyResult(t *testing.T) {
	resources, err := ParseManifests(testdataDir(t), ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) == 0 {
		t.Error("expected at least some resources to be parsed")
	}
}
