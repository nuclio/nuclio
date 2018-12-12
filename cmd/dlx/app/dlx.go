package app

import (
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/loggersink"
	nuclioio_client "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"
	"github.com/nuclio/nuclio/pkg/platform/kube/dlx"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	// load all sinks
	_ "github.com/nuclio/nuclio/pkg/sinks"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func Run(kubeconfigPath string,
	listenURL string,
	namespace string,
	platformConfigurationPath string) error {

	newDLX, err := createDLX(kubeconfigPath, listenURL, namespace, platformConfigurationPath)
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
	namespace string,
	platformConfigurationPath string) (*dlx.DLX, error) {

	// read platform configuration
	platformConfiguration, err := readPlatformConfiguration(platformConfigurationPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to read platform configuration")
	}

	// create a root logger
	rootLogger, err := loggersink.CreateSystemLogger("dlx", platformConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create logger")
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

	newProxier, err := dlx.NewDLX(rootLogger, namespace, kubeClientSet, nuclioClientSet, cfg)
	return newProxier, err
}

func getClientConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	}

	return rest.InClusterConfig()
}

func readPlatformConfiguration(configurationPath string) (*platformconfig.Config, error) {
	platformConfigurationReader, err := platformconfig.NewReader()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create platform configuration reader")
	}

	return platformConfigurationReader.ReadFileOrDefault(configurationPath)
}
