package cluster

import (
	"github.com/Richonn/driftwatch/internal/config"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func NewClient(cfg *config.Config) (*kubernetes.Clientset, error) {
	file := cfg.KubeConfig
	kubeContext := cfg.Context

	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: file},
		&clientcmd.ConfigOverrides{CurrentContext: kubeContext},
	)

	restConfig, err := loader.ClientConfig()
	if err != nil {
		restConfig, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	}

	return kubernetes.NewForConfig(restConfig)
}
