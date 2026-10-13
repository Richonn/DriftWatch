package gitops

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Richonn/driftwatch/internal/cluster"
)

func testdataDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
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

var noHelm = HelmOptions{}

func TestParseManifests_Deployment(t *testing.T) {
	resources, err := ParseManifests(testdataDir(t), ".", noHelm)
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
	resources, err := ParseManifests(testdataDir(t), ".", noHelm)
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
	resources, err := ParseManifests(testdataDir(t), ".", noHelm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := findResource(resources, "ConfigMap", "feature-flags", "default")
	if found == nil {
		t.Fatal("expected ConfigMap 'feature-flags'")
	}
}

func TestParseManifests_SecretKeysOnly(t *testing.T) {
	resources, err := ParseManifests(testdataDir(t), ".", noHelm)
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
	resources, err := ParseManifests(testdataDir(t), ".", noHelm)
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
	resources, err := ParseManifests(testdataDir(t), ".", noHelm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, r := range resources {
		if r.Name == "my-chart" {
			t.Errorf("Helm Chart.yaml should be skipped, got resource: %+v", r)
		}
	}
}

func TestParseManifests_NonexistentDir_ReturnsError(t *testing.T) {
	_, err := ParseManifests("/nonexistent/path", ".", noHelm)
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

func TestParseManifests_NonEmptyResult(t *testing.T) {
	resources, err := ParseManifests(testdataDir(t), ".", noHelm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) == 0 {
		t.Error("expected at least some resources to be parsed")
	}
}

const crdYAML = `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
  namespace: my-ns
spec:
  destination:
    namespace: production
`

func TestParseYAMLDocs_CRDIncluded(t *testing.T) {
	resources, err := parseYAMLDocs(strings.NewReader(crdYAML), true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 CRD resource, got %d", len(resources))
	}
	if resources[0].Kind != "Application" {
		t.Errorf("expected Kind=Application, got %q", resources[0].Kind)
	}
}

func TestParseYAMLDocs_CRDExcludedByDefault(t *testing.T) {
	resources, err := parseYAMLDocs(strings.NewReader(crdYAML), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 0 {
		t.Fatalf("expected 0 resources (CRDs excluded), got %d", len(resources))
	}
}

func TestParseYAMLDocs_BuiltinUnsupportedKindAlwaysSkipped(t *testing.T) {
	// CronJob is a built-in K8s kind (batch/v1) not in supportedKinds.
	// It must be skipped even when includeCRDs=true.
	cronjobYAML := `
apiVersion: batch/v1
kind: CronJob
metadata:
  name: cleanup
  namespace: default
spec:
  schedule: "0 * * * *"
`
	for _, includeCRDs := range []bool{false, true} {
		resources, err := parseYAMLDocs(strings.NewReader(cronjobYAML), includeCRDs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resources) != 0 {
			t.Errorf("includeCRDs=%v: CronJob (batch/v1) should always be skipped, got %d resources", includeCRDs, len(resources))
		}
	}
}
