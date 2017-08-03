package nucliocli

import (
	"github.com/nuclio/nuclio/pkg/functioncr"
	"github.com/pkg/errors"

	"github.com/nuclio/nuclio-sdk"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type KubeConsumer struct{}

func (kc *KubeConsumer) GetClients(logger nuclio.Logger, kubeconfigPath string) (kubeHost string,
	clientset *kubernetes.Clientset,
	functioncrClient *functioncr.Client,
	clientsErr error) {

	// create REST config
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		clientsErr = errors.Wrap(err, "Failed to create REST config")
		return
	}

	// set kube host
	kubeHost = restConfig.Host

	// create clientset
	clientset, err = kubernetes.NewForConfig(restConfig)
	if err != nil {
		clientsErr = errors.Wrap(err, "Failed to create client set")
		return
	}

	// create a client for function custom resources
	functioncrClient, err = functioncr.NewClient(logger, restConfig, clientset)
	if err != nil {
		clientsErr = errors.Wrap(err, "Failed to create function custom resource client")
		return
	}

	return
}
