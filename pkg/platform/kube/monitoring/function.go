package monitoring

import (
	"runtime/debug"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	nuclioioclient "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"
	kubecommon"github.com/nuclio/nuclio/pkg/platform/kube/common"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"golang.org/x/sync/errgroup"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	PostDeploymentMonitoringBlockingInterval = 60 * time.Second
)

type FunctionMonitor struct {
	logger                     logger.Logger
	namespace                  string
	kubeClientSet              kubernetes.Interface
	nuclioClientSet            nuclioioclient.Interface
	interval                   time.Duration
	stopChan                   chan struct{}
	lastProvisioningTimestamps map[string]time.Time
}

func NewFunctionMonitor(parentLogger logger.Logger,
	namespace string,
	kubeClientSet kubernetes.Interface,
	nuclioClientSet nuclioioclient.Interface,
	interval time.Duration) (*FunctionMonitor, error) {

	newFunctionMonitor := &FunctionMonitor{
		logger:                     parentLogger.GetChild("function_monitor"),
		namespace:                  namespace,
		kubeClientSet:              kubeClientSet,
		nuclioClientSet:            nuclioClientSet,
		interval:                   interval,
		lastProvisioningTimestamps: make(map[string]time.Time),
	}

	newFunctionMonitor.logger.DebugWith("Created function monitor",
		"namespace", namespace,
		"interval", interval)

	return newFunctionMonitor, nil
}

func (fm *FunctionMonitor) Start() error {
	fm.logger.InfoWith("Starting",
		"namespace", fm.namespace)

	// create stop channel
	fm.stopChan = make(chan struct{}, 1)

	// spawn a goroutine for function monitoring
	go func() {
		defer func() {
			if err := recover(); err != nil {
				callStack := debug.Stack()
				fm.logger.ErrorWith("Panic caught while monitoring functions",
					"err", err,
					"stack", string(callStack))
			}
		}()
		for {
			select {
			case <-time.After(fm.interval):
				if err := fm.checkFunctionStatuses(); err != nil {
					fm.logger.WarnWith("Failed check function statuses",
						"namespace", fm.namespace,
						"err", errors.Cause(err))
				}

			case <-fm.stopChan:
				fm.logger.DebugWith("Stopped function monitoring",
					"namespace", fm.namespace)
				return
			}
		}
	}()

	return nil
}

func (fm *FunctionMonitor) Stop() {
	fm.logger.InfoWith("Stopping function monitoring", "namespace", fm.namespace)

	// post to channel
	if fm.stopChan != nil {
		fm.stopChan <- struct{}{}
	}
}

func (fm *FunctionMonitor) checkFunctionStatuses() error {
	functions, err := fm.nuclioClientSet.NuclioV1beta1().NuclioFunctions(fm.namespace).List(metav1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "Failed to list functions")
	}

	var errGroup errgroup.Group
	for _, function := range functions.Items {
		function := function
		errGroup.Go(func() error {
			return fm.updateFunctionStatus(&function)
		})
	}
	return errGroup.Wait()
}

func (fm *FunctionMonitor) updateFunctionStatus(function *nuclioio.NuclioFunction) error {

	// skip check for function status
	if fm.shouldSkipFunctionMonitoring(function) {
		return nil
	}

	fm.logger.DebugWith("Getting function deployment function",
		"functionName", function.Name,
		"functionNamespace", function.Namespace)

	functionDeployment, err := fm.kubeClientSet.
		AppsV1().
		Deployments(function.Namespace).
		Get(kubecommon.DeploymentNameFromFunctionName(function.Name), metav1.GetOptions{})
	if err != nil {
		fm.logger.WarnWith("Failed to get function deployment",
			"functionName", function.Name,
			"functionNamespace", function.Namespace)
		return nil
	}

	stateChanged := false
	functionIsAvailable := fm.isAvailable(functionDeployment)
	if functionIsAvailable && function.Status.State == functionconfig.FunctionStateUnhealthy {
		function.Status.State = functionconfig.FunctionStateReady
		function.Status.Message = ""
		stateChanged = true
	} else if !functionIsAvailable && function.Status.State == functionconfig.FunctionStateReady {
		function.Status.State = functionconfig.FunctionStateUnhealthy
		function.Status.Message = common.FunctionStateMessageUnhealthy
		stateChanged = true
	}

	// return if function did not change
	if !stateChanged {
		return nil
	}

	// function state has changed, update CRD correspondingly
	fm.logger.InfoWith("Function state has changed, updating",
		"functionName", function.Name,
		"functionStatus", function.Status,
		"functionNamespace", function.Namespace,
		"functionDeploymentStatus", functionDeployment.Status,
		"functionIsAvailable", functionIsAvailable)
	if _, err := fm.nuclioClientSet.
		NuclioV1beta1().
		NuclioFunctions(fm.namespace).
		Update(function); err != nil {
		fm.logger.WarnWith("Failed to update function",
			"functionName", function.Name,
			"functionStatus", function.Status,
			"functionNamespace", function.Namespace)
	}
	return nil
}

// consider function deployment available when all pods are available
func (fm *FunctionMonitor) isAvailable(deployment *appsv1.Deployment) bool {
	atLeastOneReplicasRequested := deployment.Spec.Replicas != nil && *deployment.Spec.Replicas > 0
	return atLeastOneReplicasRequested && deployment.Status.UnavailableReplicas == 0
}

// We monitor functions that meet the following conditions:
// - not in provisioning state
// - not recently deployed
// - not in transitional states
// - not disabled / replicas set to 0
func (fm *FunctionMonitor) shouldSkipFunctionMonitoring(function *nuclioio.NuclioFunction) bool {

	// ignore provisioning states
	// ignore recently deployed function
	if fm.resolveFunctionProvisionedOrRecentlyDeployed(function) {
		fm.logger.DebugWith("Function is being provisioned or recently deployed, skipping",
			"functionName", function.Name,
			"functionState", function.Status.State)
		return true
	}

	// ignore provisioned states other than ready / unhealthy
	if !functionconfig.FunctionStateInSlice(function.Status.State, []functionconfig.FunctionState{
		functionconfig.FunctionStateReady,
		functionconfig.FunctionStateUnhealthy,
	}) {
		fm.logger.DebugWith("Function state is not ready or unhealthy, skipping",
			"functionName", function.Name,
			"functionState", function.Status.State)
		return true
	}

	// skip disabled functions / 0-ed replicas functions
	if function.Spec.Disable || (function.Spec.Replicas != nil && *function.Spec.Replicas == 0) {
		fm.logger.DebugWith("Function is disabled or has 0 desired replicas, skipping",
			"functionName", function.Name,
			"functionReplicas", function.Spec.Replicas,
			"functionDisabled", function.Spec.Disable)
		return true
	}

	// do not skip as function should be monitored
	return false
}

func (fm *FunctionMonitor) resolveFunctionProvisionedOrRecentlyDeployed(function *nuclioio.NuclioFunction) bool {
	if functionconfig.FunctionStateProvisioning(function.Status.State) {
		fm.lastProvisioningTimestamps[function.Name] = time.Now()
		fm.logger.DebugWith("Function is in provisioning state",
			"functionState", function.Status.State,
			"functionName", function.Name)
		return true
	} else if lastProvisioningTimestamp, ok := fm.lastProvisioningTimestamps[function.Name]; ok {
		if lastProvisioningTimestamp.Add(PostDeploymentMonitoringBlockingInterval).After(time.Now()) {
			fm.logger.DebugWith("Function was recently deployed",
				"functionName", function.Name,
				"lastProvisioningTimestamp", lastProvisioningTimestamp)
			return true
		}
	}
	return false
}
