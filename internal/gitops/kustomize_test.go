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

func TestRenderKustomize_StrategicMergePatch_Replicas(t *testing.T) {
	resources, err := renderKustomize(kustomizeDir(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dep := findResource(resources, "Deployment", "kust-app", "kustomize-ns")
	if dep == nil {
		t.Fatal("expected Deployment")
	}
	replicas, _ := dep.Spec["replicas"].(int)
	if replicas != 5 {
		t.Errorf("expected replicas=5 after strategic merge patch, got %v", dep.Spec["replicas"])
	}
}

func TestRenderKustomize_Json6902Patch_ServiceType(t *testing.T) {
	resources, err := renderKustomize(kustomizeDir(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	svc := findResource(resources, "Service", "kust-app", "kustomize-ns")
	if svc == nil {
		t.Fatal("expected Service")
	}
	if svc.Spec["type"] != "NodePort" {
		t.Errorf("expected type=NodePort after json6902 patch, got %v", svc.Spec["type"])
	}
}

func TestRenderKustomize_InlinePatch(t *testing.T) {
	dir := t.TempDir()
	dep := filepath.Join(dir, "deploy.yaml")
	if err := os.WriteFile(dep, []byte("apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: api\nspec:\n  replicas: 1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	kfile := filepath.Join(dir, "kustomization.yaml")
	content := `resources:
  - deploy.yaml
patches:
  - target:
      kind: Deployment
      name: api
    patch: |
      - op: replace
        path: /replicas
        value: 10
`
	if err := os.WriteFile(kfile, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	resources, err := renderKustomize(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) == 0 {
		t.Fatal("expected resources")
	}
	if resources[0].Spec["replicas"] != 10 {
		t.Errorf("expected replicas=10 after inline patch, got %v", resources[0].Spec["replicas"])
	}
}

func TestRenderKustomize_Json6902Remove(t *testing.T) {
	dir := t.TempDir()
	dep := filepath.Join(dir, "deploy.yaml")
	if err := os.WriteFile(dep, []byte("apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: api\nspec:\n  replicas: 2\n  extra: value\n"), 0600); err != nil {
		t.Fatal(err)
	}
	patch := filepath.Join(dir, "rm.yaml")
	if err := os.WriteFile(patch, []byte("- op: remove\n  path: /extra\n"), 0600); err != nil {
		t.Fatal(err)
	}
	kfile := filepath.Join(dir, "kustomization.yaml")
	content := "resources:\n  - deploy.yaml\npatchesJson6902:\n  - target:\n      kind: Deployment\n      name: api\n    path: rm.yaml\n"
	if err := os.WriteFile(kfile, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	resources, err := renderKustomize(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resources[0].Spec["extra"]; ok {
		t.Error("expected 'extra' key to be removed by json6902 patch")
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
