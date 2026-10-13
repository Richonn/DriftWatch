package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Richonn/driftwatch/internal/cluster"
	"github.com/Richonn/driftwatch/internal/config"
	"github.com/Richonn/driftwatch/internal/diff"
	"github.com/Richonn/driftwatch/internal/gitops"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

var driftWatchScanGVR = schema.GroupVersionResource{
	Group:    "driftwatch.io",
	Version:  "v1alpha1",
	Resource: "driftwatchscans",
}

const defaultInterval = 5 * time.Minute
const pollInterval = 30 * time.Second

type Controller struct {
	kubeClient kubernetes.Interface
	dynClient  dynamic.Interface
}

func NewController(kubeClient kubernetes.Interface, dynClient dynamic.Interface) *Controller {
	return &Controller{
		kubeClient: kubeClient,
		dynClient:  dynClient,
	}
}

func (c *Controller) Run(ctx context.Context) {
	fmt.Println("driftwatch-operator started")
	c.reconcileAll(ctx)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			fmt.Println("driftwatch-operator stopped")
			return
		case <-ticker.C:
			c.reconcileAll(ctx)
		}
	}
}

func (c *Controller) reconcileAll(ctx context.Context) {
	list, err := c.dynClient.Resource(driftWatchScanGVR).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		fmt.Printf("error listing DriftWatchScans: %v\n", err)
		return
	}
	for _, item := range list.Items {
		c.reconcileOne(ctx, item.GetName(), item.GetNamespace(), item.Object)
	}
}

func (c *Controller) reconcileOne(ctx context.Context, name, ns string, obj map[string]interface{}) {
	specRaw, _ := obj["spec"].(map[string]interface{})
	if specRaw == nil {
		return
	}
	repo, _ := specRaw["repo"].(string)
	if repo == "" {
		return
	}

	branch, _ := specRaw["branch"].(string)
	if branch == "" {
		branch = "main"
	}
	path, _ := specRaw["path"].(string)
	if path == "" {
		path = "."
	}
	scanNamespace, _ := specRaw["namespace"].(string)
	tokenSecret, _ := specRaw["tokenSecret"].(string)

	interval := defaultInterval
	if v, ok := specRaw["interval"].(float64); ok && v > 0 {
		interval = time.Duration(v) * time.Second
	}

	if statusRaw, ok := obj["status"].(map[string]interface{}); ok {
		if lastScan, ok := statusRaw["lastScanAt"].(string); ok && lastScan != "" {
			t, err := time.Parse(time.RFC3339, lastScan)
			if err == nil && time.Since(t) < interval {
				return
			}
		}
	}

	fmt.Printf("scanning DriftWatchScan %s/%s (%s)\n", ns, name, repo)

	token := ""
	if tokenSecret != "" {
		token = c.readToken(ctx, ns, tokenSecret)
	}

	cfg := &config.Config{
		Repo:      repo,
		Branch:    branch,
		Path:      path,
		Namespace: scanNamespace,
		Token:     token,
	}

	driftReport, scanErr := c.scan(cfg)

	var status map[string]interface{}
	if scanErr != nil {
		fmt.Printf("scan error for %s/%s: %v\n", ns, name, scanErr)
		status = map[string]interface{}{
			"lastScanAt": time.Now().UTC().Format(time.RFC3339),
			"driftCount": 0,
			"lastError":  scanErr.Error(),
		}
	} else {
		status = statusFromReport(driftReport)
	}

	if err := c.patchStatus(ctx, name, ns, status); err != nil {
		fmt.Printf("warn: patch status %s/%s: %v\n", ns, name, err)
	}
}

func (c *Controller) scan(cfg *config.Config) (*diff.DriftReport, error) {
	liveResources, err := cluster.FetchResources(c.kubeClient, cfg)
	if err != nil {
		return nil, fmt.Errorf("fetch cluster resources: %w", err)
	}

	cloneResult, err := gitops.CloneRepo(cfg.Repo, cfg.Branch, cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("clone repo: %w", err)
	}
	defer cloneResult.Cleanup()

	gitopsResources, err := gitops.ParseManifests(cloneResult.Dir, cfg.Path, gitops.HelmOptions{})
	if err != nil {
		return nil, fmt.Errorf("parse manifests: %w", err)
	}

	return diff.Compare(liveResources, gitopsResources, "", cfg.Repo), nil
}

func (c *Controller) readToken(ctx context.Context, ns, secretName string) string {
	secret, err := c.kubeClient.CoreV1().Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return ""
	}
	return string(secret.Data["token"])
}

func (c *Controller) patchStatus(ctx context.Context, name, ns string, status map[string]interface{}) error {
	data, err := json.Marshal(map[string]interface{}{"status": status})
	if err != nil {
		return err
	}
	_, err = c.dynClient.Resource(driftWatchScanGVR).Namespace(ns).Patch(
		ctx, name, types.MergePatchType, data, metav1.PatchOptions{}, "status",
	)
	return err
}

func statusFromReport(r *diff.DriftReport) map[string]interface{} {
	drifts := make([]map[string]interface{}, 0, len(r.Drifts))
	for _, d := range r.Drifts {
		drifts = append(drifts, map[string]interface{}{
			"kind":      d.Kind,
			"name":      d.Name,
			"namespace": d.Namespace,
			"driftType": string(d.DriftType),
			"details":   d.Details,
		})
	}
	return map[string]interface{}{
		"lastScanAt": r.ScannedAt.UTC().Format(time.RFC3339),
		"driftCount": r.DriftCount,
		"drifts":     drifts,
		"lastError":  "",
	}
}
