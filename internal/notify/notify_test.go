package notify

import (
	"net/http"
	"net/http/httptest"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/Richonn/driftwatch/internal/diff"
)

func makeReport(drifts ...diff.DriftItem) *diff.DriftReport {
	return &diff.DriftReport{
		ScannedAt:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		ClusterContext: "test-ctx",
		GitOpsRepo:     "https://github.com/org/gitops",
		TotalResources: 5,
		DriftCount:     len(drifts),
		Drifts:         drifts,
	}
}

func TestSend_NoDrift(t *testing.T) {
	var received string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		received = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := Send(srv.URL, makeReport()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(received, "no drift detected") {
		t.Errorf("expected no-drift message, got: %s", received)
	}
}

func TestSend_WithDrift(t *testing.T) {
	var received string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		received = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	r := makeReport(
		diff.DriftItem{Kind: "Deployment", Name: "api", Namespace: "prod", DriftType: diff.DriftSpecDrift, Details: []string{"image drift"}},
		diff.DriftItem{Kind: "Service", Name: "redis", Namespace: "prod", DriftType: diff.DriftMissingInCluster},
	)
	if err := Send(srv.URL, r); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(received, "2 drift(s)") {
		t.Errorf("expected drift count in message, got: %s", received)
	}
	if !strings.Contains(received, "image drift") {
		t.Errorf("expected detail in message, got: %s", received)
	}
}

func TestSend_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := Send(srv.URL, makeReport())
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected 500 in error, got: %v", err)
	}
}

func TestSend_InvalidURL(t *testing.T) {
	err := Send("http://invalid.example.invalid/hook", makeReport())
	if err == nil {
		t.Fatal("expected error for unreachable URL")
	}
}

func TestBuildMessage_NoDrift(t *testing.T) {
	msg := buildMessage(makeReport())
	if !strings.Contains(msg, "no drift detected") {
		t.Errorf("expected no-drift text, got: %s", msg)
	}
}

func TestBuildMessage_MissingInCluster_Icon(t *testing.T) {
	r := makeReport(diff.DriftItem{Kind: "Service", Name: "svc", Namespace: "ns", DriftType: diff.DriftMissingInCluster})
	msg := buildMessage(r)
	if !strings.Contains(msg, "✗") {
		t.Errorf("expected ✗ icon for MISSING_IN_CLUSTER, got: %s", msg)
	}
}
