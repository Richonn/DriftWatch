package diff

import (
	"testing"

	"github.com/Richonn/driftwatch/internal/cluster"
)

func deployment(name, ns string, spec map[string]interface{}) cluster.Resource {
	return cluster.Resource{Kind: "Deployment", Name: name, Namespace: ns, Spec: spec}
}

func configmap(name, ns string, data map[string]string) cluster.Resource {
	return cluster.Resource{
		Kind:      "ConfigMap",
		Name:      name,
		Namespace: ns,
		Spec:      map[string]interface{}{"data": data},
	}
}

func specWithReplicas(r int) map[string]interface{} {
	return map[string]interface{}{"replicas": float64(r)}
}

func specWithImage(containerName, image string) map[string]interface{} {
	return map[string]interface{}{
		"template": map[string]interface{}{
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{"name": containerName, "image": image},
				},
			},
		},
	}
}

// --- No drift ---

func TestCompare_NoDrift(t *testing.T) {
	live := []cluster.Resource{deployment("api", "default", specWithReplicas(2))}
	gitops := []cluster.Resource{deployment("api", "default", specWithReplicas(2))}

	report := Compare(live, gitops, "ctx", "https://repo")

	if report.DriftCount != 0 {
		t.Errorf("expected 0 drifts, got %d: %v", report.DriftCount, report.Drifts)
	}
	if report.TotalResources != 1 {
		t.Errorf("expected 1 total resource, got %d", report.TotalResources)
	}
}

// --- MISSING_IN_CLUSTER ---

func TestCompare_MissingInCluster(t *testing.T) {
	live := []cluster.Resource{}
	gitops := []cluster.Resource{deployment("api", "default", nil)}

	report := Compare(live, gitops, "ctx", "https://repo")

	if report.DriftCount != 1 {
		t.Fatalf("expected 1 drift, got %d", report.DriftCount)
	}
	if report.Drifts[0].DriftType != DriftMissingInCluster {
		t.Errorf("expected MISSING_IN_CLUSTER, got %s", report.Drifts[0].DriftType)
	}
	if report.Drifts[0].Name != "api" {
		t.Errorf("expected name 'api', got %s", report.Drifts[0].Name)
	}
}

// --- MISSING_IN_GITOPS ---

func TestCompare_MissingInGitOps(t *testing.T) {
	live := []cluster.Resource{deployment("orphan", "default", nil)}
	gitops := []cluster.Resource{}

	report := Compare(live, gitops, "ctx", "https://repo")

	if report.DriftCount != 1 {
		t.Fatalf("expected 1 drift, got %d", report.DriftCount)
	}
	if report.Drifts[0].DriftType != DriftMissingInGitOps {
		t.Errorf("expected MISSING_IN_GITOPS, got %s", report.Drifts[0].DriftType)
	}
}

// --- SPEC_DRIFT: replicas ---

func TestCompare_SpecDrift_Replicas(t *testing.T) {
	live := []cluster.Resource{deployment("api", "default", specWithReplicas(5))}
	gitops := []cluster.Resource{deployment("api", "default", specWithReplicas(2))}

	report := Compare(live, gitops, "ctx", "https://repo")

	if report.DriftCount != 1 {
		t.Fatalf("expected 1 drift, got %d", report.DriftCount)
	}
	item := report.Drifts[0]
	if item.DriftType != DriftSpecDrift {
		t.Errorf("expected SPEC_DRIFT, got %s", item.DriftType)
	}
	if len(item.Details) == 0 {
		t.Error("expected details for replica drift")
	}
	detail := item.Details[0]
	if detail != "spec.replicas: cluster=5 gitops=2" {
		t.Errorf("unexpected detail: %s", detail)
	}
}

// --- SPEC_DRIFT: container image ---

func TestCompare_SpecDrift_Image(t *testing.T) {
	live := []cluster.Resource{deployment("api", "default", specWithImage("api", "myrepo/api:v1.3.1"))}
	gitops := []cluster.Resource{deployment("api", "default", specWithImage("api", "myrepo/api:v1.2.0"))}

	report := Compare(live, gitops, "ctx", "https://repo")

	if report.DriftCount != 1 {
		t.Fatalf("expected 1 drift, got %d", report.DriftCount)
	}
	item := report.Drifts[0]
	if item.DriftType != DriftSpecDrift {
		t.Errorf("expected SPEC_DRIFT, got %s", item.DriftType)
	}
	found := false
	for _, d := range item.Details {
		if d == `container "api" image: cluster=myrepo/api:v1.3.1 gitops=myrepo/api:v1.2.0` {
			found = true
		}
	}
	if !found {
		t.Errorf("image drift detail not found, got: %v", item.Details)
	}
}

// --- SPEC_DRIFT: ConfigMap data ---

func TestCompare_SpecDrift_ConfigMap(t *testing.T) {
	live := []cluster.Resource{configmap("cfg", "default", map[string]string{"key": "value-live"})}
	gitops := []cluster.Resource{configmap("cfg", "default", map[string]string{"key": "value-gitops"})}

	report := Compare(live, gitops, "ctx", "https://repo")

	if report.DriftCount != 1 {
		t.Fatalf("expected 1 drift, got %d", report.DriftCount)
	}
	if report.Drifts[0].DriftType != DriftSpecDrift {
		t.Errorf("expected SPEC_DRIFT, got %s", report.Drifts[0].DriftType)
	}
}

// --- Multiple drifts ---

func TestCompare_MultipleDrifts(t *testing.T) {
	live := []cluster.Resource{
		deployment("api", "default", specWithReplicas(5)),
		deployment("orphan", "default", nil),
	}
	gitops := []cluster.Resource{
		deployment("api", "default", specWithReplicas(2)),
		deployment("missing", "default", nil),
	}

	report := Compare(live, gitops, "ctx", "https://repo")

	if report.DriftCount != 3 {
		t.Errorf("expected 3 drifts (spec + missing-in-gitops + missing-in-cluster), got %d", report.DriftCount)
	}
}

// --- Namespace isolation ---

func TestCompare_SameNameDifferentNamespace(t *testing.T) {
	// "api" in "default" vs "api" in "staging" — should be treated as two different resources
	live := []cluster.Resource{deployment("api", "default", nil)}
	gitops := []cluster.Resource{deployment("api", "staging", nil)}

	report := Compare(live, gitops, "ctx", "https://repo")

	// "api/default" missing in gitops + "api/staging" missing in cluster
	if report.DriftCount != 2 {
		t.Errorf("expected 2 drifts (namespace isolation), got %d", report.DriftCount)
	}
}

// --- SPEC_DRIFT: Service type ---

func TestCompare_SpecDrift_Service(t *testing.T) {
	live := []cluster.Resource{{
		Kind: "Service", Name: "svc", Namespace: "default",
		Spec: map[string]interface{}{"type": "ClusterIP", "clusterIP": "10.0.0.1"},
	}}
	gitops := []cluster.Resource{{
		Kind: "Service", Name: "svc", Namespace: "default",
		Spec: map[string]interface{}{"type": "NodePort", "clusterIP": "10.0.0.1"},
	}}

	report := Compare(live, gitops, "ctx", "https://repo")

	if report.DriftCount != 1 {
		t.Fatalf("expected 1 drift, got %d", report.DriftCount)
	}
	found := false
	for _, d := range report.Drifts[0].Details {
		if d == "spec.type: cluster=ClusterIP gitops=NodePort" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected service type detail, got: %v", report.Drifts[0].Details)
	}
}

// --- SPEC_DRIFT: Ingress ---

func TestCompare_SpecDrift_Ingress(t *testing.T) {
	live := []cluster.Resource{{
		Kind: "Ingress", Name: "ing", Namespace: "default",
		Spec: map[string]interface{}{"ingressClassName": "nginx"},
	}}
	gitops := []cluster.Resource{{
		Kind: "Ingress", Name: "ing", Namespace: "default",
		Spec: map[string]interface{}{"ingressClassName": "traefik"},
	}}

	report := Compare(live, gitops, "ctx", "https://repo")

	if report.DriftCount != 1 {
		t.Fatalf("expected 1 drift, got %d", report.DriftCount)
	}
	found := false
	for _, d := range report.Drifts[0].Details {
		if d == "spec.ingressClassName: cluster=nginx gitops=traefik" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ingressClassName detail, got: %v", report.Drifts[0].Details)
	}
}

// --- SPEC_DRIFT: labels ---

func TestCompare_SpecDrift_Labels(t *testing.T) {
	live := []cluster.Resource{{
		Kind: "Deployment", Name: "api", Namespace: "default",
		Labels: map[string]string{"env": "staging"},
		Spec:   specWithReplicas(1),
	}}
	gitops := []cluster.Resource{{
		Kind: "Deployment", Name: "api", Namespace: "default",
		Labels: map[string]string{"env": "prod"},
		Spec:   specWithReplicas(1),
	}}

	report := Compare(live, gitops, "ctx", "https://repo")

	if report.DriftCount != 1 {
		t.Fatalf("expected 1 drift, got %d", report.DriftCount)
	}
	found := false
	for _, d := range report.Drifts[0].Details {
		if d == `label "env": cluster=staging gitops=prod` {
			found = true
		}
	}
	if !found {
		t.Errorf("expected label detail, got: %v", report.Drifts[0].Details)
	}
}

// --- SPEC_DRIFT: default kind (ServiceAccount) ---

func TestCompare_SpecDrift_DefaultKind_Labels(t *testing.T) {
	live := []cluster.Resource{{
		Kind: "ServiceAccount", Name: "sa", Namespace: "default",
		Labels: map[string]string{"team": "a"},
		Spec:   map[string]interface{}{},
	}}
	gitops := []cluster.Resource{{
		Kind: "ServiceAccount", Name: "sa", Namespace: "default",
		Labels: map[string]string{"team": "b"},
		Spec:   map[string]interface{}{},
	}}

	report := Compare(live, gitops, "ctx", "https://repo")

	if report.DriftCount != 1 {
		t.Fatalf("expected 1 drift, got %d", report.DriftCount)
	}
}

// --- ConfigMap: key in gitops missing in cluster ---

func TestCompare_SpecDrift_ConfigMap_MissingInLive(t *testing.T) {
	live := []cluster.Resource{configmap("cfg", "default", map[string]string{})}
	gitops := []cluster.Resource{configmap("cfg", "default", map[string]string{"newkey": "val"})}

	report := Compare(live, gitops, "ctx", "https://repo")

	if report.DriftCount != 1 {
		t.Fatalf("expected 1 drift, got %d", report.DriftCount)
	}
	found := false
	for _, d := range report.Drifts[0].Details {
		if d == "data.newkey: missing in cluster, defined in GitOps" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected missing-in-cluster detail, got: %v", report.Drifts[0].Details)
	}
}

// --- intField: int and int64 types ---

func TestIntField_IntType(t *testing.T) {
	spec := map[string]interface{}{"replicas": int(3)}
	v := intField(spec, "replicas")
	if v == nil || *v != 3 {
		t.Errorf("expected 3, got %v", v)
	}
}

func TestIntField_Int64Type(t *testing.T) {
	spec := map[string]interface{}{"replicas": int64(7)}
	v := intField(spec, "replicas")
	if v == nil || *v != 7 {
		t.Errorf("expected 7, got %v", v)
	}
}

func TestIntField_MissingKey(t *testing.T) {
	v := intField(map[string]interface{}{}, "replicas")
	if v != nil {
		t.Errorf("expected nil, got %v", v)
	}
}

// --- mapStr: map[interface{}]interface{} ---

func TestMapStr_InterfaceMap(t *testing.T) {
	input := map[interface{}]interface{}{"key": "val"}
	result := mapStr(input)
	if result["key"] != "val" {
		t.Errorf("expected val, got %q", result["key"])
	}
}

// --- diffGenericSpec (CRDs) ---

func TestCompare_SpecDrift_CRD_FieldDiffers(t *testing.T) {
	live := cluster.Resource{Kind: "Widget", Name: "w", Namespace: "default",
		Spec: map[string]interface{}{"color": "blue"}}
	gitops := cluster.Resource{Kind: "Widget", Name: "w", Namespace: "default",
		Spec: map[string]interface{}{"color": "red"}}

	report := Compare([]cluster.Resource{live}, []cluster.Resource{gitops}, "ctx", "repo")

	if report.DriftCount != 1 {
		t.Fatalf("expected 1 drift, got %d", report.DriftCount)
	}
	found := false
	for _, d := range report.Drifts[0].Details {
		if d == "spec.color differs" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'spec.color differs', got %v", report.Drifts[0].Details)
	}
}

func TestCompare_SpecDrift_CRD_FieldOnlyInCluster(t *testing.T) {
	live := cluster.Resource{Kind: "Widget", Name: "w", Namespace: "default",
		Spec: map[string]interface{}{"color": "blue", "extra": "val"}}
	gitops := cluster.Resource{Kind: "Widget", Name: "w", Namespace: "default",
		Spec: map[string]interface{}{"color": "blue"}}

	report := Compare([]cluster.Resource{live}, []cluster.Resource{gitops}, "ctx", "repo")

	if report.DriftCount != 1 {
		t.Fatalf("expected 1 drift, got %d", report.DriftCount)
	}
	found := false
	for _, d := range report.Drifts[0].Details {
		if d == "spec.extra: present in cluster, missing in GitOps" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected cluster-only field detail, got %v", report.Drifts[0].Details)
	}
}

func TestCompare_SpecDrift_CRD_FieldOnlyInGitOps(t *testing.T) {
	live := cluster.Resource{Kind: "Widget", Name: "w", Namespace: "default",
		Spec: map[string]interface{}{"color": "blue"}}
	gitops := cluster.Resource{Kind: "Widget", Name: "w", Namespace: "default",
		Spec: map[string]interface{}{"color": "blue", "size": "large"}}

	report := Compare([]cluster.Resource{live}, []cluster.Resource{gitops}, "ctx", "repo")

	if report.DriftCount != 1 {
		t.Fatalf("expected 1 drift, got %d", report.DriftCount)
	}
	found := false
	for _, d := range report.Drifts[0].Details {
		if d == "spec.size: missing in cluster, present in GitOps" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected gitops-only field detail, got %v", report.Drifts[0].Details)
	}
}

func TestCompare_SpecDrift_CRD_NoDrift(t *testing.T) {
	spec := map[string]interface{}{"color": "blue", "size": "large"}
	live := cluster.Resource{Kind: "Widget", Name: "w", Namespace: "default", Spec: spec}
	gitops := cluster.Resource{Kind: "Widget", Name: "w", Namespace: "default", Spec: spec}

	report := Compare([]cluster.Resource{live}, []cluster.Resource{gitops}, "ctx", "repo")

	if report.DriftCount != 0 {
		t.Errorf("expected 0 drifts for identical CRD spec, got %d", report.DriftCount)
	}
}

// --- diffContainerResources ---

func specWithResources(containerName, cpu, mem string) map[string]interface{} {
	return map[string]interface{}{
		"template": map[string]interface{}{
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":  containerName,
						"image": "img:latest",
						"resources": map[string]interface{}{
							"requests": map[string]interface{}{"cpu": cpu, "memory": mem},
						},
					},
				},
			},
		},
	}
}

func TestCompare_SpecDrift_ContainerResources(t *testing.T) {
	live := cluster.Resource{Kind: "Deployment", Name: "api", Namespace: "default",
		Spec: specWithResources("api", "500m", "256Mi")}
	gitops := cluster.Resource{Kind: "Deployment", Name: "api", Namespace: "default",
		Spec: specWithResources("api", "100m", "128Mi")}

	report := Compare([]cluster.Resource{live}, []cluster.Resource{gitops}, "ctx", "repo")

	if report.DriftCount != 1 {
		t.Fatalf("expected 1 drift, got %d", report.DriftCount)
	}
	found := false
	for _, d := range report.Drifts[0].Details {
		if len(d) > 0 && d[:len("container")] == "container" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected container resources detail, got %v", report.Drifts[0].Details)
	}
}

// --- intField: unknown type ---

func TestIntField_UnknownType(t *testing.T) {
	spec := map[string]interface{}{"replicas": "not-a-number"}
	v := intField(spec, "replicas")
	if v != nil {
		t.Errorf("expected nil for unknown type, got %v", v)
	}
}

// --- Metadata ---

func TestCompare_Metadata(t *testing.T) {
	report := Compare(nil, nil, "my-context", "https://github.com/org/repo")

	if report.ClusterContext != "my-context" {
		t.Errorf("ClusterContext: got %q, want %q", report.ClusterContext, "my-context")
	}
	if report.GitOpsRepo != "https://github.com/org/repo" {
		t.Errorf("GitOpsRepo: got %q, want %q", report.GitOpsRepo, "https://github.com/org/repo")
	}
	if report.ScannedAt.IsZero() {
		t.Error("ScannedAt should not be zero")
	}
}
