package cluster

import (
	"github.com/Richonn/driftwatch/internal/config"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func restConfig(cfg *config.Config) (*rest.Config, error) {
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: cfg.KubeConfig},
		&clientcmd.ConfigOverrides{CurrentContext: cfg.Context},
	)
	rc, err := loader.ClientConfig()
	if err != nil {
		return rest.InClusterConfig()
	}
	return rc, nil
}

func NewClient(cfg *config.Config) (*kubernetes.Clientset, error) {
	rc, err := restConfig(cfg)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(rc)
}

func NewDynamicClient(cfg *config.Config) (dynamic.Interface, error) {
	rc, err := restConfig(cfg)
	if err != nil {
		return nil, err
	}
	return dynamic.NewForConfig(rc)
}
