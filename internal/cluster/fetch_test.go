package cluster

import (
	"testing"

	"github.com/Richonn/driftwatch/internal/config"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func cfg(ns string) *config.Config {
	return &config.Config{Namespace: ns}
}

func int32p(i int32) *int32 { return &i }

func TestFetchResources_Deployment(t *testing.T) {
	client := fake.NewSimpleClientset(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "api",
				Namespace: "default",
				Labels:    map[string]string{"app": "api"},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: int32p(2),
			},
		},
	)

	resources, err := FetchResources(client, cfg(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := findResource(resources, "Deployment", "api", "default")
	if found == nil {
		t.Fatal("expected Deployment 'api' in 'default', not found")
	}
	if found.Labels["app"] != "api" {
		t.Errorf("label app: got %q, want %q", found.Labels["app"], "api")
	}
}

func TestFetchResources_SystemNamespacesFiltered(t *testing.T) {
	client := fake.NewSimpleClientset(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "coredns", Namespace: "kube-system"},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "production"},
		},
	)

	resources, err := FetchResources(client, cfg(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if findResource(resources, "Deployment", "coredns", "kube-system") != nil {
		t.Error("kube-system Deployment should be filtered out")
	}
	if findResource(resources, "Deployment", "app", "production") == nil {
		t.Error("production Deployment should be present")
	}
}

func TestFetchResources_NamespaceFilter(t *testing.T) {
	client := fake.NewSimpleClientset(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "svc-a", Namespace: "staging"},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "svc-b", Namespace: "production"},
		},
	)

	resources, err := FetchResources(client, cfg("staging"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if findResource(resources, "Deployment", "svc-b", "production") != nil {
		t.Error("production Deployment should not appear when filtering on staging")
	}
	if findResource(resources, "Deployment", "svc-a", "staging") == nil {
		t.Error("staging Deployment should be present")
	}
}

func TestFetchResources_SecretNoValues(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "db-creds", Namespace: "default"},
			Type:       corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"username": []byte("admin"),
				"password": []byte("s3cr3t"),
			},
		},
	)

	resources, err := FetchResources(client, cfg(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := findResource(resources, "Secret", "db-creds", "default")
	if found == nil {
		t.Fatal("expected Secret 'db-creds', not found")
	}

	if _, ok := found.Spec["data"]; ok {
		t.Error("Secret spec must not contain raw data values")
	}

	keys, ok := found.Spec["keys"]
	if !ok {
		t.Fatal("Secret spec must contain 'keys'")
	}
	keySlice, ok := keys.([]string)
	if !ok {
		t.Fatalf("keys should be []string, got %T", keys)
	}
	if len(keySlice) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keySlice))
	}
}

func TestFetchResources_AllKinds(t *testing.T) {
	replicas := int32(1)
	client := fake.NewSimpleClientset(
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "deploy", Namespace: "default"}, Spec: appsv1.DeploymentSpec{Replicas: &replicas}},
		&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "sts", Namespace: "default"}},
		&appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "ds", Namespace: "default"}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "svc", Namespace: "default"}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: "default"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "sec", Namespace: "default"}},
		&networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "ing", Namespace: "default"}},
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "sa", Namespace: "default"}},
		&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "np", Namespace: "default"}},
	)

	resources, err := FetchResources(client, cfg(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []struct{ kind, name, ns string }{
		{"Deployment", "deploy", "default"},
		{"StatefulSet", "sts", "default"},
		{"DaemonSet", "ds", "default"},
		{"Service", "svc", "default"},
		{"ConfigMap", "cm", "default"},
		{"Secret", "sec", "default"},
		{"Ingress", "ing", "default"},
		{"ServiceAccount", "sa", "default"},
		{"NetworkPolicy", "np", "default"},
	}

	for _, e := range expected {
		if findResource(resources, e.kind, e.name, e.ns) == nil {
			t.Errorf("expected %s/%s in %s, not found", e.kind, e.name, e.ns)
		}
	}
}

func findResource(resources []Resource, kind, name, ns string) *Resource {
	for i := range resources {
		r := &resources[i]
		if r.Kind == kind && r.Name == name && r.Namespace == ns {
			return r
		}
	}
	return nil
}
