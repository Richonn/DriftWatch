package gitops

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Richonn/driftwatch/internal/cluster"
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

// --- filterByTarget ---

func TestFilterByTarget_KindAndName(t *testing.T) {
	resources := []cluster.Resource{
		{Kind: "Deployment", Name: "api", Namespace: "default"},
		{Kind: "Service", Name: "api", Namespace: "default"},
		{Kind: "Deployment", Name: "worker", Namespace: "default"},
	}
	got := filterByTarget(resources, patchTarget{Kind: "Deployment", Name: "api"})
	if len(got) != 1 || got[0].Kind != "Deployment" || got[0].Name != "api" {
		t.Errorf("unexpected filterByTarget result: %+v", got)
	}
}

func TestFilterByTarget_NoMatch(t *testing.T) {
	resources := []cluster.Resource{
		{Kind: "Deployment", Name: "api", Namespace: "default"},
	}
	got := filterByTarget(resources, patchTarget{Kind: "Service"})
	if len(got) != 0 {
		t.Errorf("expected 0 matches, got %d", len(got))
	}
}

func TestFilterByTarget_NamespaceMatch(t *testing.T) {
	resources := []cluster.Resource{
		{Kind: "Deployment", Name: "api", Namespace: "prod"},
		{Kind: "Deployment", Name: "api", Namespace: "staging"},
	}
	got := filterByTarget(resources, patchTarget{Kind: "Deployment", Name: "api", Namespace: "prod"})
	if len(got) != 1 || got[0].Namespace != "prod" {
		t.Errorf("expected 1 match in prod, got %+v", got)
	}
}

// --- jsonSet nested path ---

func TestJsonSet_NestedPath(t *testing.T) {
	m := map[string]interface{}{
		"spec": map[string]interface{}{"replicas": 1},
	}
	if err := jsonSet(m, []string{"spec", "replicas"}, 5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	spec, _ := m["spec"].(map[string]interface{})
	if spec["replicas"] != 5 {
		t.Errorf("expected replicas=5, got %v", spec["replicas"])
	}
}

func TestJsonSet_CreatesIntermediateMap(t *testing.T) {
	m := map[string]interface{}{}
	if err := jsonSet(m, []string{"a", "b"}, "val"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a, _ := m["a"].(map[string]interface{})
	if a["b"] != "val" {
		t.Errorf("expected val, got %v", a["b"])
	}
}

func TestJsonSet_EmptyPath(t *testing.T) {
	m := map[string]interface{}{"x": 1}
	if err := jsonSet(m, []string{}, "val"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m["x"] != 1 {
		t.Error("map should be unchanged with empty path")
	}
}

// --- jsonRemove nested path ---

func TestJsonRemove_NestedPath(t *testing.T) {
	m := map[string]interface{}{
		"spec": map[string]interface{}{"extra": "val", "keep": "ok"},
	}
	jsonRemove(m, []string{"spec", "extra"})
	spec, _ := m["spec"].(map[string]interface{})
	if _, ok := spec["extra"]; ok {
		t.Error("expected 'extra' to be removed")
	}
	if spec["keep"] != "ok" {
		t.Error("'keep' should remain")
	}
}

func TestJsonRemove_NonExistentPath(t *testing.T) {
	m := map[string]interface{}{"spec": map[string]interface{}{"a": 1}}
	jsonRemove(m, []string{"spec", "nonexistent"})
	spec, _ := m["spec"].(map[string]interface{})
	if spec["a"] != 1 {
		t.Error("existing key should remain")
	}
}

func TestJsonRemove_EmptyPath(t *testing.T) {
	m := map[string]interface{}{"x": 1}
	jsonRemove(m, []string{})
	if m["x"] != 1 {
		t.Error("map should be unchanged with empty path")
	}
}

// --- applyImageOverrides ---

func TestApplyImageOverrides_NoTag(t *testing.T) {
	r := &cluster.Resource{
		Kind: "Deployment",
		Spec: map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{"name": "app", "image": "myrepo/app"},
					},
				},
			},
		},
	}
	idx := buildImageIndex([]imageOverride{{Name: "myrepo/app", NewTag: "v3.0.0"}})
	applyImageOverrides(r, idx)

	ctrs, _ := r.Spec["template"].(map[string]interface{})["spec"].(map[string]interface{})["containers"].([]interface{})
	cm, _ := ctrs[0].(map[string]interface{})
	if cm["image"] != "myrepo/app:v3.0.0" {
		t.Errorf("expected myrepo/app:v3.0.0, got %v", cm["image"])
	}
}

func TestApplyImageOverrides_NewName(t *testing.T) {
	r := &cluster.Resource{
		Kind: "Deployment",
		Spec: map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{"name": "app", "image": "old/app:v1"},
					},
				},
			},
		},
	}
	idx := buildImageIndex([]imageOverride{{Name: "old/app", NewName: "new/app", NewTag: "v2"}})
	applyImageOverrides(r, idx)

	ctrs, _ := r.Spec["template"].(map[string]interface{})["spec"].(map[string]interface{})["containers"].([]interface{})
	cm, _ := ctrs[0].(map[string]interface{})
	if cm["image"] != "new/app:v2" {
		t.Errorf("expected new/app:v2, got %v", cm["image"])
	}
}

func TestApplyImageOverrides_NoMatch(t *testing.T) {
	r := &cluster.Resource{
		Kind: "Deployment",
		Spec: map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{"name": "app", "image": "other/app:v1"},
					},
				},
			},
		},
	}
	idx := buildImageIndex([]imageOverride{{Name: "myrepo/app", NewTag: "v2"}})
	applyImageOverrides(r, idx)

	ctrs, _ := r.Spec["template"].(map[string]interface{})["spec"].(map[string]interface{})["containers"].([]interface{})
	cm, _ := ctrs[0].(map[string]interface{})
	if cm["image"] != "other/app:v1" {
		t.Errorf("expected image unchanged, got %v", cm["image"])
	}
}

// --- matchesTarget ---

func TestMatchesTarget_EmptyTarget(t *testing.T) {
	r := &cluster.Resource{Kind: "Deployment", Name: "api", Namespace: "default"}
	if !matchesTarget(r, patchTarget{}) {
		t.Error("empty target should match everything")
	}
}

func TestMatchesTarget_KindMismatch(t *testing.T) {
	r := &cluster.Resource{Kind: "Deployment", Name: "api", Namespace: "default"}
	if matchesTarget(r, patchTarget{Kind: "Service"}) {
		t.Error("kind mismatch should not match")
	}
}

// --- deepMerge ---

func TestDeepMerge_OverridesScalar(t *testing.T) {
	base := map[string]interface{}{"replicas": 1, "other": "keep"}
	patch := map[string]interface{}{"replicas": 5}
	result := deepMerge(base, patch)
	if result["replicas"] != 5 {
		t.Errorf("expected replicas=5, got %v", result["replicas"])
	}
	if result["other"] != "keep" {
		t.Errorf("expected other=keep, got %v", result["other"])
	}
}

func TestDeepMerge_RecursiveMap(t *testing.T) {
	base := map[string]interface{}{
		"spec": map[string]interface{}{"replicas": 1, "image": "old"},
	}
	patch := map[string]interface{}{
		"spec": map[string]interface{}{"replicas": 3},
	}
	result := deepMerge(base, patch)
	spec, _ := result["spec"].(map[string]interface{})
	if spec["replicas"] != 3 {
		t.Errorf("expected replicas=3, got %v", spec["replicas"])
	}
	if spec["image"] != "old" {
		t.Errorf("expected image=old preserved, got %v", spec["image"])
	}
}

// --- splitJSONPointer ---

func TestSplitJSONPointer_Empty(t *testing.T) {
	if parts := splitJSONPointer(""); len(parts) != 0 {
		t.Errorf("expected empty parts for empty path, got %v", parts)
	}
}

func TestSplitJSONPointer_Root(t *testing.T) {
	if parts := splitJSONPointer("/"); len(parts) != 0 {
		t.Errorf("expected empty parts for root path, got %v", parts)
	}
}

func TestSplitJSONPointer_EscapedChars(t *testing.T) {
	parts := splitJSONPointer("/a~1b/c~0d")
	if len(parts) != 2 || parts[0] != "a/b" || parts[1] != "c~d" {
		t.Errorf("unexpected parts: %v", parts)
	}
}

// --- NameSuffix ---

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
