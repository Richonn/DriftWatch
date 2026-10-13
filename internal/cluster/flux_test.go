package cluster

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

var fluxScheme = runtime.NewScheme()

var fluxListKinds = map[schema.GroupVersionResource]string{
	gitRepositoryGVR:  "GitRepositoryList",
	kustomizationGVR:  "KustomizationList",
}

func newFakeDynClient(objs ...runtime.Object) *fake.FakeDynamicClient {
	return fake.NewSimpleDynamicClientWithCustomListKinds(fluxScheme, fluxListKinds, objs...)
}

func fakeGitRepo(ns, name, url, branch string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "source.toolkit.fluxcd.io/v1",
		"kind":       "GitRepository",
		"metadata":   map[string]interface{}{"name": name, "namespace": ns},
		"spec": map[string]interface{}{
			"url": url,
			"ref": map[string]interface{}{"branch": branch},
		},
	}}
}

func fakeKustomization(ns, name, repoName, repoNS, path string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
		"kind":       "Kustomization",
		"metadata":   map[string]interface{}{"name": name, "namespace": ns},
		"spec": map[string]interface{}{
			"path": path,
			"sourceRef": map[string]interface{}{
				"kind":      "GitRepository",
				"name":      repoName,
				"namespace": repoNS,
			},
		},
	}}
}

func TestDiscoverFluxSources_Basic(t *testing.T) {
	repo := fakeGitRepo("flux-system", "infra", "https://github.com/org/gitops", "main")
	kust := fakeKustomization("flux-system", "apps", "infra", "flux-system", "./clusters/prod")

	dynClient := newFakeDynClient(
		repo.DeepCopyObject(),
		kust.DeepCopyObject(),
	)

	sources, err := DiscoverFluxSources(dynClient)
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
	if s.Path != "./clusters/prod" {
		t.Errorf("expected path, got %q", s.Path)
	}
}

func TestDiscoverFluxSources_NoKustomizations(t *testing.T) {
	repo := fakeGitRepo("flux-system", "infra", "https://github.com/org/gitops", "main")
	dynClient := newFakeDynClient( repo.DeepCopyObject())

	sources, err := DiscoverFluxSources(dynClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sources) != 0 {
		t.Errorf("expected 0 sources, got %d", len(sources))
	}
}

func TestDiscoverFluxSources_MissingRepo(t *testing.T) {
	kust := fakeKustomization("flux-system", "apps", "nonexistent", "flux-system", "./clusters/prod")
	dynClient := newFakeDynClient( kust.DeepCopyObject())

	sources, err := DiscoverFluxSources(dynClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sources) != 0 {
		t.Errorf("expected 0 sources (repo not found), got %d", len(sources))
	}
}

func TestDiscoverFluxSources_DefaultBranch(t *testing.T) {
	repo := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "source.toolkit.fluxcd.io/v1",
		"kind":       "GitRepository",
		"metadata":   map[string]interface{}{"name": "infra", "namespace": "flux-system"},
		"spec":       map[string]interface{}{"url": "https://github.com/org/gitops"},
	}}
	kust := fakeKustomization("flux-system", "apps", "infra", "flux-system", "./k8s")
	dynClient := newFakeDynClient( repo.DeepCopyObject(), kust.DeepCopyObject())

	sources, err := DiscoverFluxSources(dynClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}
	if sources[0].Branch != "main" {
		t.Errorf("expected default branch 'main', got %q", sources[0].Branch)
	}
}
