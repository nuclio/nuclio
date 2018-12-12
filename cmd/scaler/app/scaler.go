package app

import (
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/loggersink"
	nuclioio_client "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"
	"github.com/nuclio/nuclio/pkg/platform/kube/scaler"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	// load all sinks
	_ "github.com/nuclio/nuclio/pkg/sinks"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsv1 "k8s.io/metrics/pkg/client/clientset_generated/clientset"
	custommetricsv1 "k8s.io/metrics/pkg/client/custom_metrics"
)

func Run(kubeconfigPath string,
	resolvedNamespace string,
	platformConfigurationPath string) error {

	newScaler, err := createScaler(kubeconfigPath,
		resolvedNamespace,
		platformConfigurationPath)
	if err != nil {
		return errors.Wrap(err, "Failed to create scaler")
	}

	// start the scaler
	if err := newScaler.Start(); err != nil {
		return errors.Wrap(err, "Failed to start scaler")
	}

	select {}
}

func createScaler(kubeconfigPath string,
	resolvedNamespace string,
	platformConfigurationPath string) (*scaler.Scaler, error) {

	// read platform configuration
	platformConfiguration, err := readPlatformConfiguration(platformConfigurationPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to read platform configuration")
	}

	// create a root logger
	rootLogger, err := loggersink.CreateSystemLogger("scaler", platformConfiguration)
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

	metricsClient, err := metricsv1.NewForConfig(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create k8s client set")
	}

	customMetricsClient, err := custommetricsv1.NewForConfig(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed create custom metrics client set")
	}

	nuclioClientSet, err := nuclioio_client.NewForConfig(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed create custom metrics client set")
	}

	newScaler, err := scaler.NewScaler(rootLogger,
		resolvedNamespace,
		kubeClientSet,
		metricsClient,
		nuclioClientSet,
		customMetricsClient,
		platformConfiguration)

	if err != nil {
		return nil, err
	}

	return newScaler, nil
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
