package gitops

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	helmchart "helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
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

func TestRenderChart_InvalidDir_ReturnsError(t *testing.T) {
	_, err := renderChart("/nonexistent/path", HelmOptions{})
	if err == nil {
		t.Fatal("expected error for invalid chart dir")
	}
}

func TestRenderChart_InvalidValuesFile_ReturnsError(t *testing.T) {
	_, err := renderChart(helmChartDir(t), HelmOptions{ValuesFiles: []string{"/nonexistent/values.yaml"}})
	if err == nil {
		t.Fatal("expected error for missing values file")
	}
}

func TestWarnMissingDeps_NoDeps_NoPanic(t *testing.T) {
	chrt, err := loader.Load(helmChartDir(t))
	if err != nil {
		t.Fatalf("load chart: %v", err)
	}
	warnMissingDeps(chrt)
}

func TestWarnMissingDeps_WithMissingDep(t *testing.T) {
	chrt, err := loader.Load(helmChartDir(t))
	if err != nil {
		t.Fatalf("load chart: %v", err)
	}
	chrt.Metadata.Dependencies = []*helmchart.Dependency{
		{Name: "redis", Version: "1.0.0", Repository: "https://charts.bitnami.com"},
	}
	warnMissingDeps(chrt)
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
