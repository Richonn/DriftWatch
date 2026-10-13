package operator

import (
	"testing"
	"time"

	"github.com/Richonn/driftwatch/internal/diff"
)

func TestStatusFromReport_WithDrifts(t *testing.T) {
	r := &diff.DriftReport{
		ScannedAt:  time.Date(2026, 10, 13, 14, 32, 1, 0, time.UTC),
		GitOpsRepo: "https://github.com/org/gitops",
		DriftCount: 1,
		Drifts: []diff.DriftItem{
			{
				Kind:      "Deployment",
				Name:      "api",
				Namespace: "default",
				DriftType: diff.DriftSpecDrift,
				Details:   []string{"container \"api\" image differs"},
			},
		},
	}

	status := statusFromReport(r)

	if status["driftCount"] != 1 {
		t.Errorf("expected driftCount=1, got %v", status["driftCount"])
	}
	if status["lastScanAt"] != "2026-10-13T14:32:01Z" {
		t.Errorf("unexpected lastScanAt: %v", status["lastScanAt"])
	}
	if status["lastError"] != "" {
		t.Errorf("expected empty lastError, got %v", status["lastError"])
	}

	drifts, ok := status["drifts"].([]map[string]interface{})
	if !ok || len(drifts) != 1 {
		t.Fatalf("expected 1 drift in status, got %v", status["drifts"])
	}
	if drifts[0]["kind"] != "Deployment" {
		t.Errorf("expected kind=Deployment, got %v", drifts[0]["kind"])
	}
	if drifts[0]["driftType"] != string(diff.DriftSpecDrift) {
		t.Errorf("unexpected driftType: %v", drifts[0]["driftType"])
	}
}

func TestStatusFromReport_NoDrifts(t *testing.T) {
	r := &diff.DriftReport{
		ScannedAt:  time.Now(),
		DriftCount: 0,
	}

	status := statusFromReport(r)

	if status["driftCount"] != 0 {
		t.Errorf("expected driftCount=0, got %v", status["driftCount"])
	}

	drifts, ok := status["drifts"].([]map[string]interface{})
	if !ok {
		t.Fatal("expected drifts to be []map[string]interface{}")
	}
	if len(drifts) != 0 {
		t.Errorf("expected 0 drifts, got %d", len(drifts))
	}
}

func TestStatusFromReport_MultipleDriftTypes(t *testing.T) {
	r := &diff.DriftReport{
		ScannedAt:  time.Now(),
		DriftCount: 2,
		Drifts: []diff.DriftItem{
			{Kind: "Service", Name: "redis", Namespace: "prod", DriftType: diff.DriftMissingInCluster},
			{Kind: "Deployment", Name: "legacy", Namespace: "prod", DriftType: diff.DriftMissingInGitOps},
		},
	}

	status := statusFromReport(r)

	drifts, _ := status["drifts"].([]map[string]interface{})
	if len(drifts) != 2 {
		t.Fatalf("expected 2 drifts, got %d", len(drifts))
	}
	if drifts[0]["driftType"] != string(diff.DriftMissingInCluster) {
		t.Errorf("unexpected driftType[0]: %v", drifts[0]["driftType"])
	}
	if drifts[1]["driftType"] != string(diff.DriftMissingInGitOps) {
		t.Errorf("unexpected driftType[1]: %v", drifts[1]["driftType"])
	}
}
