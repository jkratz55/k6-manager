package internal

import (
	"os"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func InitKubernetesClient() (*KubernetesClients, error) {
	k8sCfg, err := buildK8sConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(k8sCfg)
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(k8sCfg)
	if err != nil {
		return nil, err
	}

	return &KubernetesClients{
		Client:        clientset,
		DynamicClient: dynamicClient,
	}, nil
}

func buildK8sConfig() (*rest.Config, error) {
	cfg, err := rest.InClusterConfig()
	if err == nil {
		return cfg, nil
	}
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, _ := os.UserHomeDir()
		kubeconfig = home + "/.kube/config"
	}
	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}

type KubernetesClients struct {
	Client        *kubernetes.Clientset
	DynamicClient dynamic.Interface
}
