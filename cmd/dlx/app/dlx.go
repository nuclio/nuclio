package app

import (
	"github.com/nuclio/nuclio/pkg/errors"
	nuclioio_client "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"
	"github.com/nuclio/nuclio/pkg/platform/kube/dlx"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func Run(kubeconfigPath string, listenURL string, resolvedNamespace string) error {

	newDLX, err := createDLX(kubeconfigPath, listenURL, resolvedNamespace)
	if err != nil {
		return errors.Wrap(err, "Failed to create dlx")
	}

	// start the dead letter exchange service
	if err := newDLX.Start(); err != nil {
		return errors.Wrap(err, "Failed to start dlx")
	}

	select {}
}

func createDLX(kubeconfigPath string,
	listenURL string,
	resolvedNamespace string) (*dlx.DLX, error) {

	// create a root logger
	rootLogger, err := createLogger()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create root logger")
	}

	restConfig, err := getClientConfig(kubeconfigPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get client configuration")
	}

	kubeClientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create k8s client set")
	}

	nuclioClientSet, err := nuclioio_client.NewForConfig(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create nuclio client set")
	}

	cfg := dlx.Configuration{
		URL: listenURL,
	}

	newProxier, err := dlx.NewDLX(rootLogger, resolvedNamespace, kubeClientSet, nuclioClientSet, cfg)
	return newProxier, err
}

func getClientConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	}

	return rest.InClusterConfig()
}

func createLogger() (logger.Logger, error) {
	return nucliozap.NewNuclioZapCmd("dlx", nucliozap.DebugLevel)
}
