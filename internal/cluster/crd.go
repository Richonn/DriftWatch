package cluster

import (
	"context"
	"strings"

	"github.com/Richonn/driftwatch/internal/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type discoverer interface {
	ServerPreferredResources() ([]*metav1.APIResourceList, error)
}

var builtinGroups = map[string]bool{
	"":                              true,
	"apps":                          true,
	"extensions":                    true,
	"batch":                         true,
	"autoscaling":                   true,
	"networking.k8s.io":             true,
	"policy":                        true,
	"rbac.authorization.k8s.io":     true,
	"storage.k8s.io":                true,
	"admissionregistration.k8s.io":  true,
	"apiextensions.k8s.io":          true,
	"apiregistration.k8s.io":        true,
	"coordination.k8s.io":           true,
	"events.k8s.io":                 true,
	"discovery.k8s.io":              true,
	"node.k8s.io":                   true,
	"scheduling.k8s.io":             true,
	"flowcontrol.apiserver.k8s.io":  true,
	"authentication.k8s.io":         true,
	"authorization.k8s.io":          true,
	"certificates.k8s.io":           true,
	"metrics.k8s.io":                true,
}

func IsBuiltinAPIVersion(apiVersion string) bool {
	parts := strings.SplitN(apiVersion, "/", 2)
	var group string
	if len(parts) == 2 {
		group = parts[0]
	}
	return builtinGroups[group]
}

func FetchCRDResources(disco discoverer, dynClient dynamic.Interface, cfg *config.Config) ([]Resource, error) {
	ctx := context.Background()

	resourceLists, err := disco.ServerPreferredResources()
	if err != nil && resourceLists == nil {
		return nil, err
	}

	var resources []Resource

	for _, rl := range resourceLists {
		gv, parseErr := schema.ParseGroupVersion(rl.GroupVersion)
		if parseErr != nil {
			continue
		}
		if builtinGroups[gv.Group] {
			continue
		}

		for _, apiResource := range rl.APIResources {
			if strings.Contains(apiResource.Name, "/") {
				continue
			}
			if !containsVerb(apiResource.Verbs, "list") {
				continue
			}

			gvr := schema.GroupVersionResource{
				Group:    gv.Group,
				Version:  gv.Version,
				Resource: apiResource.Name,
			}

			list, listErr := dynClient.Resource(gvr).Namespace(cfg.Namespace).List(ctx, metav1.ListOptions{})
			if listErr != nil {
				continue
			}

			for _, item := range list.Items {
				itemNS := item.GetNamespace()
				if isExcluded(itemNS, cfg) {
					continue
				}

				spec, _ := item.Object["spec"].(map[string]interface{})
				if spec == nil {
					spec = map[string]interface{}{}
				}

				resources = append(resources, Resource{
					Kind:      item.GetKind(),
					Name:      item.GetName(),
					Namespace: itemNS,
					Labels:    item.GetLabels(),
					Spec:      spec,
				})
			}
		}
	}

	return resources, nil
}

func containsVerb(verbs metav1.Verbs, verb string) bool {
	for _, v := range verbs {
		if v == verb {
			return true
		}
	}
	return false
}
