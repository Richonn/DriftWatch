package cluster

import (
	"testing"

	"github.com/Richonn/driftwatch/internal/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

// mockDiscoverer implements the discoverer interface for tests.
type mockDiscoverer struct {
	resources []*metav1.APIResourceList
}

func (m *mockDiscoverer) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return m.resources, nil
}

var widgetGVR = schema.GroupVersionResource{
	Group:    "example.io",
	Version:  "v1alpha1",
	Resource: "widgets",
}

func newFakeWidgetDynClient(objs ...runtime.Object) *dynamicfake.FakeDynamicClient {
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{widgetGVR: "WidgetList"},
		objs...,
	)
}

func fakeWidget(ns, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "example.io/v1alpha1",
		"kind":       "Widget",
		"metadata":   map[string]interface{}{"name": name, "namespace": ns},
		"spec":       map[string]interface{}{"color": "blue", "size": "large"},
	}}
}

func widgetDiscovery() *mockDiscoverer {
	return &mockDiscoverer{
		resources: []*metav1.APIResourceList{
			{
				GroupVersion: "example.io/v1alpha1",
				APIResources: []metav1.APIResource{
					{
						Name:       "widgets",
						Kind:       "Widget",
						Namespaced: true,
						Verbs:      metav1.Verbs{"list", "get", "watch"},
					},
				},
			},
		},
	}
}

// ── IsBuiltinAPIVersion ──────────────────────────────────────────────────────

func TestIsBuiltinAPIVersion(t *testing.T) {
	cases := []struct {
		apiVersion  string
		wantBuiltin bool
	}{
		{"v1", true},
		{"apps/v1", true},
		{"batch/v1", true},
		{"networking.k8s.io/v1", true},
		{"rbac.authorization.k8s.io/v1", true},
		{"argoproj.io/v1alpha1", false},
		{"cert-manager.io/v1", false},
		{"source.toolkit.fluxcd.io/v1", false},
		{"example.io/v1alpha1", false},
		{"", true}, // empty → core group
	}
	for _, tc := range cases {
		t.Run(tc.apiVersion, func(t *testing.T) {
			got := IsBuiltinAPIVersion(tc.apiVersion)
			if got != tc.wantBuiltin {
				t.Errorf("IsBuiltinAPIVersion(%q) = %v, want %v", tc.apiVersion, got, tc.wantBuiltin)
			}
		})
	}
}

// ── FetchCRDResources ────────────────────────────────────────────────────────

func TestFetchCRDResources_Basic(t *testing.T) {
	widget := fakeWidget("default", "my-widget")
	disco := widgetDiscovery()
	dynClient := newFakeWidgetDynClient(widget.DeepCopyObject())

	resources, err := FetchCRDResources(disco, dynClient, &config.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 CRD resource, got %d", len(resources))
	}
	r := resources[0]
	if r.Kind != "Widget" {
		t.Errorf("expected Kind=Widget, got %q", r.Kind)
	}
	if r.Name != "my-widget" {
		t.Errorf("expected Name=my-widget, got %q", r.Name)
	}
}

func TestFetchCRDResources_SystemNamespaceFiltered(t *testing.T) {
	widget := fakeWidget("kube-system", "sys-widget")
	dynClient := newFakeWidgetDynClient(widget.DeepCopyObject())

	resources, err := FetchCRDResources(widgetDiscovery(), dynClient, &config.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range resources {
		if r.Namespace == "kube-system" {
			t.Errorf("expected kube-system to be filtered, got resource %+v", r)
		}
	}
}

func TestFetchCRDResources_ExcludeNamespace(t *testing.T) {
	widget := fakeWidget("staging", "staging-widget")
	dynClient := newFakeWidgetDynClient(widget.DeepCopyObject())

	resources, err := FetchCRDResources(widgetDiscovery(), dynClient, &config.Config{
		ExcludeNamespaces: []string{"staging"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range resources {
		if r.Namespace == "staging" {
			t.Errorf("expected staging to be excluded, got resource %+v", r)
		}
	}
}

func TestFetchCRDResources_BuiltinGroupSkipped(t *testing.T) {
	disco := &mockDiscoverer{
		resources: []*metav1.APIResourceList{
			{
				GroupVersion: "apps/v1",
				APIResources: []metav1.APIResource{
					{Name: "deployments", Kind: "Deployment", Namespaced: true, Verbs: metav1.Verbs{"list"}},
				},
			},
		},
	}
	dynClient := newFakeWidgetDynClient()

	resources, err := FetchCRDResources(disco, dynClient, &config.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources (built-in group skipped), got %d", len(resources))
	}
}

func TestFetchCRDResources_NoCustomResources(t *testing.T) {
	disco := &mockDiscoverer{resources: nil}
	dynClient := newFakeWidgetDynClient()

	resources, err := FetchCRDResources(disco, dynClient, &config.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
