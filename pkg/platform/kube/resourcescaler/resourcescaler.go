package resourcescaler

import (
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform/kube"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	nuclioioclient "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/v3io/scaler/pkg/scalertypes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// NuclioResourceScaler leverages github.com/v3io/scaler
// to allow extending scale to zero and from zero nuclio functions
type NuclioResourceScaler struct {
	logger                logger.Logger
	nuclioClientSet       nuclioioclient.Interface
	kubeconfigPath        string
	namespace             string
	platformConfiguration *platformconfig.Config
}

func New(logger logger.Logger,
	platformConfiguration *platformconfig.Config,
	kubeconfigPath string,
	namespace string) (scalertypes.ResourceScaler, error) {

	restConfig, err := common.GetClientConfig(kubeconfigPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get client configuration")
	}

	nuclioClientSet, err := nuclioioclient.NewForConfig(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create nuclio client set")
	}

	logger.DebugWith("Initialized resource scaler",
		"platformconfig", platformConfiguration,
		"namespace", namespace,
		"kubeconfigPath", kubeconfigPath)

	return &NuclioResourceScaler{
		logger:                logger,
		nuclioClientSet:       nuclioClientSet,
		kubeconfigPath:        kubeconfigPath,
		namespace:             namespace,
		platformConfiguration: platformConfiguration,
	}, nil
}

func (n *NuclioResourceScaler) SetScale(resources []scalertypes.Resource, scale int) error {
	functionNames := make([]string, 0)
	for _, resource := range resources {
		functionNames = append(functionNames, resource.Name)
	}
	if scale == 0 {
		return n.scaleFunctionsToZero(n.namespace, functionNames)
	}
	return n.scaleFunctionsFromZero(n.namespace, functionNames)
}

func (n *NuclioResourceScaler) GetResources() ([]scalertypes.Resource, error) {
	var functionList []scalertypes.Resource
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

			functionList = append(functionList, scalertypes.Resource{
				Name:               function.Name,
				ScaleResources:     scaleResources,
				LastScaleEvent:     lastScaleEvent,
				LastScaleEventTime: lastScaleEventTime,
			})
		}
	}
	return functionList, nil
}

func (n *NuclioResourceScaler) GetConfig() (*scalertypes.ResourceScalerConfig, error) {

	// enrich
	if n.platformConfiguration.ScaleToZero.ResourceReadinessTimeout == "" {
		n.platformConfiguration.ScaleToZero.ResourceReadinessTimeout = "2m"
	}

	if n.platformConfiguration.ScaleToZero.ScalerInterval == "" {
		n.platformConfiguration.ScaleToZero.ScalerInterval = "1m"
	}

	scaleInterval, err := time.ParseDuration(n.platformConfiguration.ScaleToZero.ScalerInterval)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse scaler interval duration")
	}

	resourceReadinessTimeout, err := time.ParseDuration(n.platformConfiguration.ScaleToZero.ResourceReadinessTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse resource readiness timeout")
	}

	return &scalertypes.ResourceScalerConfig{
		KubeconfigPath: n.kubeconfigPath,
		AutoScalerOptions: scalertypes.AutoScalerOptions{
			Namespace:     n.namespace,
			ScaleInterval: scalertypes.Duration{Duration: scaleInterval},
			GroupKind: schema.GroupKind{
				Group: "nuclio.io",
				Kind:  "NuclioFunction",
			},
		},
		DLXOptions: scalertypes.DLXOptions{
			Namespace:                n.namespace,
			TargetPort:               8080,
			TargetNameHeader:         "X-Nuclio-Target",
			TargetPathHeader:         "X-Nuclio-Function-Path",
			ListenAddress:            ":8080",
			ResourceReadinessTimeout: scalertypes.Duration{Duration: resourceReadinessTimeout},
		},
	}, nil
}

func (n *NuclioResourceScaler) ResolveServiceName(resource scalertypes.Resource) (string, error) {
	return kube.ServiceNameFromFunctionName(resource.Name), nil
}

func (n *NuclioResourceScaler) parseScaleResources(function nuclioio.NuclioFunction) ([]scalertypes.ScaleResource, error) {
	var scaleResources []scalertypes.ScaleResource
	for _, scaleResource := range function.Spec.ScaleToZero.ScaleResources {
		windowSize, err := time.ParseDuration(scaleResource.WindowSize)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to parse window size")
		}
		scaleResources = append(scaleResources, scalertypes.ScaleResource{
			MetricName: scaleResource.MetricName,
			Threshold:  scaleResource.Threshold,
			WindowSize: scalertypes.Duration{Duration: windowSize},
		})
	}
	return scaleResources, nil
}

func (n *NuclioResourceScaler) parseLastScaleEvent(
	function nuclioio.NuclioFunction) (*scalertypes.ScaleEvent, *time.Time, error) {
	if function.Status.ScaleToZero == nil {
		return nil, nil, nil
	}

	if function.Status.ScaleToZero.LastScaleEventTime == nil {
		return nil, nil, errors.New("Function scale to zero status does not contain last scale event time")
	}

	return &function.Status.ScaleToZero.LastScaleEvent, function.Status.ScaleToZero.LastScaleEventTime, nil
}

func (n *NuclioResourceScaler) scaleFunctionsToZero(namespace string, functionNames []string) error {
	n.logger.DebugWith("Scaling to zero", "functionNames", functionNames)
	failedFunctionNames := make([]string, 0)
	for _, functionName := range functionNames {
		err := n.updateFunctionStatus(namespace,
			functionName,
			functionconfig.FunctionStateWaitingForScaleResourcesToZero,
			scalertypes.ScaleToZeroStartedScaleEvent)
		if err != nil {
			failedFunctionNames = append(failedFunctionNames, functionName)
			n.logger.WarnWith("Failed to update function status to scale to zero", "functionName", functionName)
			continue
		}
	}

	if len(failedFunctionNames) > 0 {
		return errors.Errorf("Failed to scale some functions to zero: %v", failedFunctionNames)
	}
	return nil
}

func (n *NuclioResourceScaler) scaleFunctionsFromZero(namespace string, functionNames []string) error {
	n.logger.DebugWith("Scaling from zero", "functionNames", functionNames)
	failedFunctionNames := make([]string, 0)
	for _, functionName := range functionNames {
		err := n.updateFunctionStatus(namespace,
			functionName,
			functionconfig.FunctionStateWaitingForScaleResourcesFromZero,
			scalertypes.ScaleFromZeroStartedScaleEvent)
		if err != nil {
			failedFunctionNames = append(failedFunctionNames, functionName)
			n.logger.WarnWith("Failed to update function status to scale from zero", "functionName", functionName)
			continue
		}
		if err := n.waitFunctionReadiness(namespace, functionName); err != nil {
			failedFunctionNames = append(failedFunctionNames, functionName)
			n.logger.WarnWith("Failed waiting for function readiness", "functionName", functionName)
			continue
		}
	}
	if len(failedFunctionNames) > 0 {
		return errors.Errorf("Failed to scale some functions from zero: %v", failedFunctionNames)
	}
	return nil
}

func (n *NuclioResourceScaler) updateFunctionStatus(namespace string,
	functionName string,
	functionState functionconfig.FunctionState,
	functionScaleEvent scalertypes.ScaleEvent) error {
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

		n.logger.DebugWith("Function is not ready yet",
			"functionName", functionName,
			"currentState", function.Status.State)

		time.Sleep(3 * time.Second)
	}
}
