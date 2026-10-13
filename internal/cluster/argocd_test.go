package cluster

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

var argoListKinds = map[schema.GroupVersionResource]string{
	argoCDApplicationGVR: "ApplicationList",
}

func newFakeArgoDynClient(objs ...runtime.Object) *fake.FakeDynamicClient {
	return fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), argoListKinds, objs...)
}

func fakeApplication(ns, name, repoURL, targetRevision, path string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata":   map[string]interface{}{"name": name, "namespace": ns},
		"spec": map[string]interface{}{
			"source": map[string]interface{}{
				"repoURL":        repoURL,
				"targetRevision": targetRevision,
				"path":           path,
			},
		},
	}}
}

func TestDiscoverArgoCDSources_Basic(t *testing.T) {
	app := fakeApplication("argocd", "my-app", "https://github.com/org/gitops", "main", "k8s/prod")
	dynClient := newFakeArgoDynClient(app.DeepCopyObject())

	sources, err := DiscoverArgoCDSources(dynClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}
	s := sources[0]
	if s.URL != "https://github.com/org/gitops" {
		t.Errorf("expected URL, got %q", s.URL)
	}
	if s.Branch != "main" {
		t.Errorf("expected branch main, got %q", s.Branch)
	}
	if s.Path != "k8s/prod" {
		t.Errorf("expected path k8s/prod, got %q", s.Path)
	}
	if s.Name != "my-app" {
		t.Errorf("expected name my-app, got %q", s.Name)
	}
}

func TestDiscoverArgoCDSources_NoApps(t *testing.T) {
	dynClient := newFakeArgoDynClient()

	sources, err := DiscoverArgoCDSources(dynClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sources) != 0 {
		t.Errorf("expected 0 sources, got %d", len(sources))
	}
}

func TestDiscoverArgoCDSources_HEADRevision_DefaultsBranchToMain(t *testing.T) {
	app := fakeApplication("argocd", "app", "https://github.com/org/gitops", "HEAD", ".")
	dynClient := newFakeArgoDynClient(app.DeepCopyObject())

	sources, err := DiscoverArgoCDSources(dynClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sources[0].Branch != "main" {
		t.Errorf("expected HEAD to default to 'main', got %q", sources[0].Branch)
	}
}

func TestDiscoverArgoCDSources_CustomBranch(t *testing.T) {
	app := fakeApplication("argocd", "app", "https://github.com/org/gitops", "release/v2", ".")
	dynClient := newFakeArgoDynClient(app.DeepCopyObject())

	sources, err := DiscoverArgoCDSources(dynClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sources[0].Branch != "release/v2" {
		t.Errorf("expected branch release/v2, got %q", sources[0].Branch)
	}
}

func TestDiscoverArgoCDSources_MultipleApps(t *testing.T) {
	app1 := fakeApplication("argocd", "app1", "https://github.com/org/gitops", "main", "k8s/prod")
	app2 := fakeApplication("argocd", "app2", "https://github.com/org/gitops", "main", "k8s/staging")
	dynClient := newFakeArgoDynClient(app1.DeepCopyObject(), app2.DeepCopyObject())

	sources, err := DiscoverArgoCDSources(dynClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(sources))
	}
}

func TestDiscoverArgoCDSources_MissingRepoURL_Skipped(t *testing.T) {
	app := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata":   map[string]interface{}{"name": "app", "namespace": "argocd"},
		"spec": map[string]interface{}{
			"source": map[string]interface{}{
				"path": "k8s/",
			},
		},
	}}
	dynClient := newFakeArgoDynClient(app.DeepCopyObject())

	sources, err := DiscoverArgoCDSources(dynClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sources) != 0 {
		t.Errorf("expected 0 sources (no repoURL), got %d", len(sources))
	}
}
