package cluster

import (
	"context"
	"encoding/json"

	"github.com/Richonn/driftwatch/internal/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var systemNamespaces = map[string]bool{
	"kube-system":     true,
	"kube-public":     true,
	"kube-node-lease": true,
}

type Resource struct {
	Kind      string
	Name      string
	Namespace string
	Labels    map[string]string
	Spec      map[string]interface{}
	LiveOnly  bool
}

func specToMap(obj interface{}) map[string]interface{} {
	b, err := json.Marshal(obj)
	if err != nil {
		return nil
	}
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)
	return m
}

func FetchResources(client kubernetes.Interface, cfg *config.Config) ([]Resource, error) {
	ctx := context.Background()
	ns := cfg.Namespace
	list := metav1.ListOptions{}
	var resources []Resource

	deployments, err := client.AppsV1().Deployments(ns).List(ctx, list)
	if err != nil {
		return nil, err
	}
	for _, item := range deployments.Items {
		if systemNamespaces[item.Namespace] {
			continue
		}
		resources = append(resources, Resource{
			Kind:      "Deployment",
			Name:      item.Name,
			Namespace: item.Namespace,
			Labels:    item.Labels,
			Spec:      specToMap(item.Spec),
		})
	}

	statefulsets, err := client.AppsV1().StatefulSets(ns).List(ctx, list)
	if err != nil {
		return nil, err
	}
	for _, item := range statefulsets.Items {
		if systemNamespaces[item.Namespace] {
			continue
		}
		resources = append(resources, Resource{
			Kind:      "StatefulSet",
			Name:      item.Name,
			Namespace: item.Namespace,
			Labels:    item.Labels,
			Spec:      specToMap(item.Spec),
		})
	}

	daemonsets, err := client.AppsV1().DaemonSets(ns).List(ctx, list)
	if err != nil {
		return nil, err
	}
	for _, item := range daemonsets.Items {
		if systemNamespaces[item.Namespace] {
			continue
		}
		resources = append(resources, Resource{
			Kind:      "DaemonSet",
			Name:      item.Name,
			Namespace: item.Namespace,
			Labels:    item.Labels,
			Spec:      specToMap(item.Spec),
		})
	}

	services, err := client.CoreV1().Services(ns).List(ctx, list)
	if err != nil {
		return nil, err
	}
	for _, item := range services.Items {
		if systemNamespaces[item.Namespace] {
			continue
		}
		resources = append(resources, Resource{
			Kind:      "Service",
			Name:      item.Name,
			Namespace: item.Namespace,
			Labels:    item.Labels,
			Spec:      specToMap(item.Spec),
		})
	}

	configmaps, err := client.CoreV1().ConfigMaps(ns).List(ctx, list)
	if err != nil {
		return nil, err
	}
	for _, item := range configmaps.Items {
		if systemNamespaces[item.Namespace] {
			continue
		}
		resources = append(resources, Resource{
			Kind:      "ConfigMap",
			Name:      item.Name,
			Namespace: item.Namespace,
			Labels:    item.Labels,
			Spec:      map[string]interface{}{"data": item.Data},
		})
	}

	secrets, err := client.CoreV1().Secrets(ns).List(ctx, list)
	if err != nil {
		return nil, err
	}
	for _, item := range secrets.Items {
		if systemNamespaces[item.Namespace] {
			continue
		}
		keys := make([]string, 0, len(item.Data))
		for k := range item.Data {
			keys = append(keys, k)
		}
		resources = append(resources, Resource{
			Kind:      "Secret",
			Name:      item.Name,
			Namespace: item.Namespace,
			Labels:    item.Labels,
			Spec:      map[string]interface{}{"type": string(item.Type), "keys": keys},
		})
	}

	ingresses, err := client.NetworkingV1().Ingresses(ns).List(ctx, list)
	if err != nil {
		return nil, err
	}
	for _, item := range ingresses.Items {
		if systemNamespaces[item.Namespace] {
			continue
		}
		resources = append(resources, Resource{
			Kind:      "Ingress",
			Name:      item.Name,
			Namespace: item.Namespace,
			Labels:    item.Labels,
			Spec:      specToMap(item.Spec),
		})
	}

	serviceaccounts, err := client.CoreV1().ServiceAccounts(ns).List(ctx, list)
	if err != nil {
		return nil, err
	}
	for _, item := range serviceaccounts.Items {
		if systemNamespaces[item.Namespace] {
			continue
		}
		resources = append(resources, Resource{
			Kind:      "ServiceAccount",
			Name:      item.Name,
			Namespace: item.Namespace,
			Labels:    item.Labels,
			Spec:      map[string]interface{}{},
		})
	}

	netpols, err := client.NetworkingV1().NetworkPolicies(ns).List(ctx, list)
	if err != nil {
		return nil, err
	}
	for _, item := range netpols.Items {
		if systemNamespaces[item.Namespace] {
			continue
		}
		resources = append(resources, Resource{
			Kind:      "NetworkPolicy",
			Name:      item.Name,
			Namespace: item.Namespace,
			Labels:    item.Labels,
			Spec:      specToMap(item.Spec),
		})
	}

	return resources, nil
}
