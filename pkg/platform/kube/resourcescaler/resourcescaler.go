package main

import (
	"os"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	nuclioio_client "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"
	// load all sinks
	"github.com/nuclio/nuclio/pkg/platformconfig"
	_ "github.com/nuclio/nuclio/pkg/sinks"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
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

	resourceScalerLogger, err := nucliozap.NewNuclioZap("resource-scaler",
		"console",
		nil,
		os.Stdout,
		os.Stderr,
		nucliozap.DebugLevel)
	if err != nil {
		return nil, errors.Wrap(err, "Failed creating a new logger")
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
		return n.scaleFunctionToZero(n.namespace, resource.Name)
	}
	return n.scaleFunctionFromZero(n.namespace, resource.Name)
}

func (n *NuclioResourceScaler) GetResources() ([]scaler_types.Resource, error) {
	var functionList []scaler_types.Resource
	functions, err := n.nuclioClientSet.NuclioV1beta1().NuclioFunctions(n.namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to list functions")
	}

	// build a list of function names that are candidates to be scaled to zero
	for _, function := range functions.Items {

		// don't include functions that aren't in ready state or that min replicas is larger than zero
		if function.GetComputedMinReplicas() <= 0 && function.Status.State == functionconfig.FunctionStateReady {
			if function.Spec.ScaleToZero == nil {
				n.logger.WarnWith("Function missing scale to zero spec. Continuing", "functionName", function.Name)
				continue
			}

			scaleResources, err := n.parseScaleResources(function)
			if err != nil {
				n.logger.WarnWith("Failed to parse scale resources. Continuing", "functionName", function.Name)
				continue
			}

			lastScaleEvent, lastScaleEventTime, err := n.parseLastScaleEvent(function)
			if err != nil {
				n.logger.WarnWith("Failed to parse last scale event. Continuing", "functionName", function.Name)
				continue
			}

			functionList = append(functionList, scaler_types.Resource{
				Name:               function.Name,
				ScaleResources:     scaleResources,
				LastScaleEvent:     lastScaleEvent,
				LastScaleEventTime: lastScaleEventTime,
			})
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
		return nil, errors.Wrap(err, "Failed to parse scaler interval duration")
	}

	resourceReadinessTimeout, err := time.ParseDuration(platformConfiguration.ScaleToZero.ResourceReadinessTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse resource readiness timeout")
	}

	return &scaler_types.ResourceScalerConfig{
		KubeconfigPath: n.kubeconfigPath,
		AutoScalerOptions: scaler_types.AutoScalerOptions{
			Namespace:     n.namespace,
			ScaleInterval: scaleInterval,
			GroupKind:     "NuclioFunction",
		},
		DLXOptions: scaler_types.DLXOptions{
			Namespace:                n.namespace,
			TargetPort:               8080,
			TargetNameHeader:         "X-Nuclio-Target",
			TargetPathHeader:         "X-Nuclio-Function-Path",
			ListenAddress:            ":8080",
			ResourceReadinessTimeout: resourceReadinessTimeout,
		},
	}, nil
}

func (n *NuclioResourceScaler) parseScaleResources(function nuclioio.NuclioFunction) ([]scaler_types.ScaleResource, error) {
	var scaleResources []scaler_types.ScaleResource
	for _, scaleResource := range function.Spec.ScaleToZero.ScaleResources {
		windowSize, err := time.ParseDuration(scaleResource.WindowSize)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to parse window size")
		}
		scaleResources = append(scaleResources, scaler_types.ScaleResource{
			MetricName: scaleResource.MetricName,
			Threshold:  scaleResource.Threshold,
			WindowSize: windowSize,
		})
	}
	return scaleResources, nil
}

func (n *NuclioResourceScaler) parseLastScaleEvent(function nuclioio.NuclioFunction) (*scaler_types.ScaleEvent, *time.Time, error) {
	if function.Status.ScaleToZero == nil {
		return nil, nil, nil
	}

	if function.Status.ScaleToZero.LastScaleEventTime == nil {
		return nil, nil, errors.New("Function scale to zero status does not contain last scale event time")
	}

	return &function.Status.ScaleToZero.LastScaleEvent, function.Status.ScaleToZero.LastScaleEventTime, nil
}

func (n *NuclioResourceScaler) scaleFunctionToZero(namespace string, functionName string) error {
	n.logger.DebugWith("Scaling to zero", "functionName", functionName)
	err := n.updateFunctionStatus(namespace,
		functionName,
		functionconfig.FunctionStateWaitingForScaleResourcesToZero,
		scaler_types.ScaleToZeroStartedScaleEvent)
	if err != nil {
		return errors.Wrap(err, "Failed to update function status to scale to zero")
	}
	return nil
}

func (n *NuclioResourceScaler) scaleFunctionFromZero(namespace string, functionName string) error {
	n.logger.DebugWith("Scaling from zero", "functionName", functionName)
	err := n.updateFunctionStatus(namespace,
		functionName,
		functionconfig.FunctionStateWaitingForScaleResourcesFromZero,
		scaler_types.ScaleFromZeroStartedScaleEvent)
	if err != nil {
		return errors.Wrap(err, "Failed to update function status to scale from zero")
	}
	return n.waitFunctionReadiness(namespace, functionName)
}

func (n *NuclioResourceScaler) updateFunctionStatus(namespace string,
	functionName string,
	functionState functionconfig.FunctionState,
	functionScaleEvent scaler_types.ScaleEvent) error {
	function, err := n.nuclioClientSet.NuclioV1beta1().NuclioFunctions(namespace).Get(functionName, metav1.GetOptions{})
	if err != nil {
		n.logger.WarnWith("Failed getting nuclio function to update function status", "functionName", functionName, "err", err)
		return errors.Wrap(err, "Failed getting nuclio function to update function status")
	}

	now := time.Now()
	function.Status.State = functionState
	function.Status.ScaleToZero = &functionconfig.ScaleToZeroStatus{
		LastScaleEvent:     functionScaleEvent,
		LastScaleEventTime: &now,
	}
	_, err = n.nuclioClientSet.NuclioV1beta1().NuclioFunctions(namespace).Update(function)
	if err != nil {
		n.logger.WarnWith("Failed to update function", "functionName", functionName, "err", err)
		return errors.Wrap(err, "Failed to update nuclio function")
	}
	return nil
}

func (n *NuclioResourceScaler) waitFunctionReadiness(namespace string, functionName string) error {
	n.logger.DebugWith("Waiting for function readiness", "functionName", functionName)
	for {
		function, err := n.nuclioClientSet.NuclioV1beta1().NuclioFunctions(namespace).Get(functionName, metav1.GetOptions{})
		if err != nil {
			n.logger.WarnWith("Failed getting nuclio function", "functionName", functionName, "err", err)
			return errors.Wrap(err, "Failed getting nuclio function")
		}
		if function.Status.State == functionconfig.FunctionStateReady {
			n.logger.InfoWith("Function is ready", "functionName", functionName)
			return nil
		}

		n.logger.DebugWith("Function not ready yet",
			"functionName", functionName,
			"currentState", function.Status.State)

		time.Sleep(3 * time.Second)
	}
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
