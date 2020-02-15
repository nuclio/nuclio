package main

import (
	"fmt"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/loggersink"
	nuclioio_client "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"
	// load all sinks
	"github.com/nuclio/nuclio/pkg/platformconfig"
	_ "github.com/nuclio/nuclio/pkg/sinks"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/v3io/scaler-types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// A plugin for github.com/v3io/scaler, allowing to extend it to scale to zero and from zero function resources in k8s
type NuclioResourceScaler struct {
	logger          logger.Logger
	nuclioClientSet nuclioio_client.Interface
	kubeconfigPath  string
	namespace       string
}

// New is called when plugin loaded on scaler, so it's considered "dead code" for the linter
func New(kubeconfigPath string, namespace string) (scaler_types.ResourceScaler, error) { // nolint: deadcode
	platformConfiguration, err := readPlatformConfiguration()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to read platform configuration")
	}

	// create a root logger
	resourceScalerLogger, err := loggersink.CreateSystemLogger("resource-scaler", platformConfiguration)
	if err != nil {
		fmt.Println(err)
		return nil, errors.Wrap(err, "Failed to create logger")
	}

	restConfig, err := getClientConfig(kubeconfigPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get client configuration")
	}

	nuclioClientSet, err := nuclioio_client.NewForConfig(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create nuclio client set")
	}

	resourceScalerLogger.DebugWith("Initialized resource scaler",
		"platformconfig", platformConfiguration,
		"namespace", namespace,
		"kubeconfigPath", kubeconfigPath)

	return &NuclioResourceScaler{
		logger:          resourceScalerLogger,
		nuclioClientSet: nuclioClientSet,
		kubeconfigPath:  kubeconfigPath,
		namespace:       namespace,
	}, nil
}

func (n *NuclioResourceScaler) SetScale(resource scaler_types.Resource, scale int) error {
	if scale == 0 {
		return n.scaleFunctionToZero(n.namespace, string(resource))
	}
	return n.scaleFunctionFromZero(n.namespace, string(resource))
}

func (n *NuclioResourceScaler) GetResources() ([]scaler_types.Resource, error) {
	functions, err := n.nuclioClientSet.NuclioV1beta1().NuclioFunctions(n.namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to list functions")
	}

	var functionList []scaler_types.Resource

	// build a list of function names that are potential to be scaled to zero
	for _, function := range functions.Items {

		// don't include functions that aren't in ready state or that min replicas is larger than zero
		if function.GetComputedMinReplicas() <= 0 && function.Status.State == functionconfig.FunctionStateReady {
			functionList = append(functionList, scaler_types.Resource(function.Name))
		}
	}
	return functionList, nil
}

func (n *NuclioResourceScaler) GetConfig() (*scaler_types.ResourceScalerConfig, error) {
	platformConfiguration, err := readPlatformConfiguration()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to read platform configuration")
	}

	scaleInterval, err := time.ParseDuration(platformConfiguration.ScaleToZero.ScalerInterval)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to read scale interval")
	}

	scaleWindow, err := time.ParseDuration(platformConfiguration.ScaleToZero.WindowSize)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse scale window")
	}

	pollerInterval, err := time.ParseDuration(platformConfiguration.ScaleToZero.PollerInterval)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse poller interval")
	}

	return &scaler_types.ResourceScalerConfig{
		KubeconfigPath: n.kubeconfigPath,
		AutoScalerOptions: scaler_types.AutoScalerOptions{
			Namespace:     n.namespace,
			ScaleInterval: scaleInterval,
			ScaleWindow:   scaleWindow,
			MetricName:    platformConfiguration.ScaleToZero.MetricName,
			Threshold:     0,
		},
		DLXOptions: scaler_types.DLXOptions{
			Namespace:        n.namespace,
			TargetPort:       8080,
			TargetNameHeader: "X-Nuclio-Target",
			TargetPathHeader: "X-Nuclio-Function-Path",
			ListenAddress:    ":8080",
		},
		PollerOptions: scaler_types.PollerOptions{
			MetricInterval: pollerInterval,
			MetricName:     platformConfiguration.ScaleToZero.MetricName,
			Namespace:      n.namespace,
			GroupKind:      "NuclioFunction",
		},
	}, nil
}

func (n *NuclioResourceScaler) scaleFunctionToZero(namespace string, functionName string) error {
	n.logger.DebugWith("Scaling to zero", "functionName", functionName)
	function, err := n.nuclioClientSet.NuclioV1beta1().NuclioFunctions(namespace).Get(functionName, metav1.GetOptions{})
	if err != nil {
		n.logger.WarnWith("Failed to get nuclio function", "functionName", functionName, "err", err)
		return errors.Wrap(err, "Failed to get function")
	}

	// this has the nice property of disabling hpa as well
	function.Status.State = functionconfig.FunctionStateScaledToZero
	_, err = n.nuclioClientSet.NuclioV1beta1().NuclioFunctions(namespace).Update(function)
	if err != nil {
		n.logger.WarnWith("Failed to update function", "functionName", functionName, "err", err)
		return errors.Wrap(err, "Failed to update function")
	}
	return nil
}

func (n *NuclioResourceScaler) scaleFunctionFromZero(namespace string, functionName string) error {
	err := n.updateFunctionStatus(namespace, functionName)
	if err != nil {
		return errors.Wrap(err, "Failed to change function status to waitingForResourceConfiguration")
	}
	return n.waitFunctionReadiness(namespace, functionName)
}

func (n *NuclioResourceScaler) updateFunctionStatus(namespace string, functionName string) error {
	function, err := n.nuclioClientSet.NuclioV1beta1().NuclioFunctions(namespace).Get(functionName, metav1.GetOptions{})
	if err != nil {
		n.logger.WarnWith("Failed to get nuclio function", "functionName", functionName, "err", err)
		return errors.Wrap(err, "Failed to get nuclio function")
	}

	function.Status.State = functionconfig.FunctionStateWaitingForResourceConfiguration
	_, err = n.nuclioClientSet.NuclioV1beta1().NuclioFunctions(namespace).Update(function)
	if err != nil {
		n.logger.WarnWith("Failed to update function", "functionName", functionName, "err", err)
		return errors.Wrap(err, "Failed to update nuclio function")
	}
	return nil
}

func (n *NuclioResourceScaler) waitFunctionReadiness(namespace string, functionName string) error {
	for {
		function, err := n.nuclioClientSet.NuclioV1beta1().NuclioFunctions(namespace).Get(functionName, metav1.GetOptions{})
		if err != nil {
			n.logger.WarnWith("Failed to get nuclio function", "functionName", functionName, "err", err)
			return errors.Wrap(err, "Failed to get nuclio function")
		}
		n.logger.DebugWith("Started function", "state", function.Status.State)
		if function.Status.State != functionconfig.FunctionStateReady {
			time.Sleep(3 * time.Second)
		} else {
			break
		}
	}
	return nil
}

func readPlatformConfiguration() (*platformconfig.Config, error) {
	configurationPath := "/etc/nuclio/config/platform/platform.yaml"
	platformConfigurationReader, err := platformconfig.NewReader()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create platform configuration reader")
	}

	return platformConfigurationReader.ReadFileOrDefault(configurationPath)
}

func getClientConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	}

	return rest.InClusterConfig()
}
