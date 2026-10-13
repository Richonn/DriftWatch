package cluster

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var gitRepositoryGVR = schema.GroupVersionResource{
	Group:    "source.toolkit.fluxcd.io",
	Version:  "v1",
	Resource: "gitrepositories",
}

var kustomizationGVR = schema.GroupVersionResource{
	Group:    "kustomize.toolkit.fluxcd.io",
	Version:  "v1",
	Resource: "kustomizations",
}

type FluxSource struct {
	Name      string
	Namespace string
	URL       string
	Branch    string
	Path      string
}

func DiscoverFluxSources(dynClient dynamic.Interface) ([]FluxSource, error) {
	ctx := context.Background()

	repos, err := dynClient.Resource(gitRepositoryGVR).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list GitRepositories: %w", err)
	}

	repoByKey := make(map[string]map[string]interface{})
	for _, item := range repos.Items {
		ns := item.GetNamespace()
		name := item.GetName()
		spec, _ := item.Object["spec"].(map[string]interface{})
		repoByKey[ns+"/"+name] = spec
	}

	kustomizations, err := dynClient.Resource(kustomizationGVR).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list Kustomizations: %w", err)
	}

	var sources []FluxSource
	for _, item := range kustomizations.Items {
		spec, _ := item.Object["spec"].(map[string]interface{})
		if spec == nil {
			continue
		}

		path, _ := spec["path"].(string)

		srcRef, _ := spec["sourceRef"].(map[string]interface{})
		if srcRef == nil {
			continue
		}
		refKind, _ := srcRef["kind"].(string)
		if refKind != "GitRepository" {
			continue
		}
		refName, _ := srcRef["name"].(string)
		refNS, _ := srcRef["namespace"].(string)
		if refNS == "" {
			refNS = item.GetNamespace()
		}

		repoSpec := repoByKey[refNS+"/"+refName]
		if repoSpec == nil {
			continue
		}

		url, _ := repoSpec["url"].(string)
		branch := "main"
		if ref, ok := repoSpec["ref"].(map[string]interface{}); ok {
			if b, ok := ref["branch"].(string); ok && b != "" {
				branch = b
			}
		}

		sources = append(sources, FluxSource{
			Name:      item.GetName(),
			Namespace: item.GetNamespace(),
			URL:       url,
			Branch:    branch,
			Path:      path,
		})
	}

	return sources, nil
}
