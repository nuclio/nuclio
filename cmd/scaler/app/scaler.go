package app

import (
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform/kube/scaler"
	"github.com/nuclio/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsv1 "k8s.io/metrics/pkg/client/clientset_generated/clientset"
	custommetricsv1 "k8s.io/metrics/pkg/client/custom_metrics"
	"time"
)


func Run(kubeconfigPath string, resolvedNamespace string) error {

	newScaler, err := createScaler(kubeconfigPath, resolvedNamespace)
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
	resolvedNamespace string) (*scaler.Scaler, error) {

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

	metricsClientSet, err := metricsv1.NewForConfig(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create k8s client set")
	}

	customMetricsClientSet, err := custommetricsv1.NewForConfig(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed create custom metrics client set")
	}

	newScaler, err := scaler.NewScaler(rootLogger,
		resolvedNamespace,
		kubeClientSet,
		metricsClientSet,
		customMetricsClientSet,
		5*time.Minute)

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

func createLogger() (logger.Logger, error) {
	return nucliozap.NewNuclioZapCmd("scaler", nucliozap.DebugLevel)
}
