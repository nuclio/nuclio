package app

import (
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform/kube/dlx"
	"github.com/nuclio/zap"
	"k8s.io/client-go/kubernetes"
	nuclioio_client "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)


func Run(kubeconfigPath string, resolvedNamespace string) error {

	newProxier, err := createProxier(kubeconfigPath, resolvedNamespace)
	if err != nil {
		return errors.Wrap(err, "Failed to create scaler")
	}

	// start the scaler
	if err := newProxier.Start(); err != nil {
		return errors.Wrap(err, "Failed to start scaler")
	}

	select {}
}

func createProxier(kubeconfigPath string,
	resolvedNamespace string) (*dlx.Proxier, error) {

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
		URL: ":9091",
		ReadBufferSize: 1111,
	}

	newProxier, err := dlx.NewProxier(rootLogger, resolvedNamespace, kubeClientSet, nuclioClientSet, cfg)
	return newProxier, err
}

func getClientConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	}

	return rest.InClusterConfig()
}

func createLogger() (logger.Logger, error) {
	return nucliozap.NewNuclioZapCmd("scaler", nucliozap.DebugLevel)
}