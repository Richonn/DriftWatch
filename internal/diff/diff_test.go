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
