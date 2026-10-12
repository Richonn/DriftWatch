package gitops

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func helmChartDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..")
	return filepath.Join(root, "testdata", "manifests", "helm-chart")
}

func TestRenderChart_ProducesDeployment(t *testing.T) {
	resources, err := renderChart(helmChartDir(t), HelmOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if findResource(resources, "Deployment", "helm-app", "helm-ns") == nil {
		t.Fatalf("expected Deployment 'helm-app' in 'helm-ns', got %+v", resources)
	}
}

func TestRenderChart_ValuesOverride(t *testing.T) {
	overrideFile := filepath.Join(t.TempDir(), "override.yaml")
	content := []byte("replicaCount: 5\nnamespace: override-ns\nimage: myrepo/helm-app:v2.0.0\n")
	if err := os.WriteFile(overrideFile, content, 0600); err != nil {
		t.Fatalf("write values file: %v", err)
	}

	resources, err := renderChart(helmChartDir(t), HelmOptions{ValuesFiles: []string{overrideFile}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if findResource(resources, "Deployment", "helm-app", "override-ns") == nil {
		t.Errorf("expected Deployment in 'override-ns' after values override, got: %+v", resources)
	}
}

func TestParseManifests_WithHelmChart(t *testing.T) {
	resources, err := ParseManifests(testdataDir(t), ".", HelmOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if findResource(resources, "Deployment", "helm-app", "helm-ns") == nil {
		t.Errorf("expected Deployment rendered from Helm chart, got: %+v", resources)
	}
}
