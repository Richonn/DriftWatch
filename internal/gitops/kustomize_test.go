package gitops

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func kustomizeDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..")
	return filepath.Join(root, "testdata", "manifests", "kustomize-base")
}

func TestRenderKustomize_ProducesDeployment(t *testing.T) {
	resources, err := renderKustomize(kustomizeDir(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if findResource(resources, "Deployment", "kust-app", "kustomize-ns") == nil {
		t.Fatalf("expected Deployment 'kust-app' in 'kustomize-ns', got %+v", resources)
	}
}

func TestRenderKustomize_ProducesService(t *testing.T) {
	resources, err := renderKustomize(kustomizeDir(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if findResource(resources, "Service", "kust-app", "kustomize-ns") == nil {
		t.Fatalf("expected Service 'kust-app' in 'kustomize-ns', got %+v", resources)
	}
}

func TestRenderKustomize_ImageOverride(t *testing.T) {
	resources, err := renderKustomize(kustomizeDir(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dep := findResource(resources, "Deployment", "kust-app", "kustomize-ns")
	if dep == nil {
		t.Fatal("expected Deployment")
	}

	tmpl, _ := dep.Spec["template"].(map[string]interface{})
	podSpec, _ := tmpl["spec"].(map[string]interface{})
	ctrs, _ := podSpec["containers"].([]interface{})
	if len(ctrs) == 0 {
		t.Fatal("expected containers in deployment")
	}
	cm, _ := ctrs[0].(map[string]interface{})
	if cm["image"] != "myrepo/app:v2.0.0" {
		t.Errorf("expected image myrepo/app:v2.0.0, got %v", cm["image"])
	}
}

func TestRenderKustomize_InvalidDir_ReturnsError(t *testing.T) {
	_, err := renderKustomize("/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for invalid dir")
	}
}

func TestRenderKustomize_MissingResource_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	kfile := filepath.Join(dir, "kustomization.yaml")
	if err := os.WriteFile(kfile, []byte("resources:\n  - missing.yaml\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := renderKustomize(dir)
	if err == nil {
		t.Fatal("expected error for missing resource file")
	}
}

func TestIsKustomize_WithYaml(t *testing.T) {
	if !isKustomize(kustomizeDir(t)) {
		t.Error("expected isKustomize to return true for kustomize-base dir")
	}
}

func TestIsKustomize_WithoutFile(t *testing.T) {
	if isKustomize(t.TempDir()) {
		t.Error("expected isKustomize to return false for empty dir")
	}
}

func TestParseManifests_WithKustomize(t *testing.T) {
	resources, err := ParseManifests(testdataDir(t), ".", noHelm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if findResource(resources, "Deployment", "kust-app", "kustomize-ns") == nil {
		t.Errorf("expected Deployment rendered from kustomize, got: %+v", resources)
	}
}

func TestRenderKustomize_NameSuffix(t *testing.T) {
	dir := t.TempDir()
	dep := filepath.Join(dir, "deploy.yaml")
	if err := os.WriteFile(dep, []byte("apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: api\n"), 0600); err != nil {
		t.Fatal(err)
	}
	kfile := filepath.Join(dir, "kustomization.yaml")
	content := "resources:\n  - deploy.yaml\nnameSuffix: -prod\nnamespace: prod\n"
	if err := os.WriteFile(kfile, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	resources, err := renderKustomize(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if findResource(resources, "Deployment", "api-prod", "prod") == nil {
		t.Errorf("expected Deployment 'api-prod' in 'prod', got %+v", resources)
	}
}
