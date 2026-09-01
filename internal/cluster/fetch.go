package cluster

import (
	"context"

	"github.com/Richonn/driftwatch/internal/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Resource struct {
	Kind      string
	Name      string
	Namespace string
	Labels    map[string]string
	Spec      map[string]interface{}
	LiveOnly  bool
}

func FetchResource(client *kubernetes.Clientset, cfg *config.Config) ([]Resource, error) {
	ctx := context.Background()
	resources := []Resource{}
	namespace := cfg.Namespace

	liste, err := client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, item := range liste.Items {
		if item.Namespace == "kube-system" || item.Namespace == "kube-public" || item.Namespace == "kube-node-lease" {
			continue
		}
	}

	return resources, nil
}
