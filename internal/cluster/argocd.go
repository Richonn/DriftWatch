package cluster

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var argoCDApplicationGVR = schema.GroupVersionResource{
	Group:    "argoproj.io",
	Version:  "v1alpha1",
	Resource: "applications",
}

type ArgoCDSource struct {
	Name      string
	Namespace string
	URL       string
	Branch    string
	Path      string
}

func DiscoverArgoCDSources(dynClient dynamic.Interface) ([]ArgoCDSource, error) {
	ctx := context.Background()

	apps, err := dynClient.Resource(argoCDApplicationGVR).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list ArgoCD Applications: %w", err)
	}

	var sources []ArgoCDSource
	for _, item := range apps.Items {
		spec, _ := item.Object["spec"].(map[string]interface{})
		if spec == nil {
			continue
		}

		src, _ := spec["source"].(map[string]interface{})
		if src == nil {
			continue
		}

		url, _ := src["repoURL"].(string)
		if url == "" {
			continue
		}
		path, _ := src["path"].(string)

		branch := "main"
		if targetRevision, ok := src["targetRevision"].(string); ok && targetRevision != "" && targetRevision != "HEAD" {
			branch = targetRevision
		}

		sources = append(sources, ArgoCDSource{
			Name:      item.GetName(),
			Namespace: item.GetNamespace(),
			URL:       url,
			Branch:    branch,
			Path:      path,
		})
	}

	return sources, nil
}
