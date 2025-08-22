package kube

import (
	"fmt"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	topohubclient "github.com/infrastructure-io/topohub/pkg/k8s/client/clientset/versioned/typed/topohub.infrastructure.io/v1beta1"
)

var (
	kubeCli kubernetes.Interface
	crdCli  topohubclient.TopohubV1beta1Interface
)

// InitClients initializes Kubernetes clients
func InitClients() error {
	var (
		config *rest.Config
		err    error
	)
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		return fmt.Errorf("failed to build kube config, err: %w", err)
	}

	kubeCli, err = kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to build kube client, err: %w", err)
	}

	crdCli, err = topohubclient.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to build topohub crd client, err: %w", err)
	}

	return nil
}

func GetKubeClient() kubernetes.Interface {
	return kubeCli
}

func GetTopohubCRDClient() topohubclient.TopohubV1beta1Interface {
	return crdCli
}
