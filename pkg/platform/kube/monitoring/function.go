/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package monitoring

import (
	"context"
	"runtime/debug"
	"sync"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errgroup"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform/kube"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	nuclioioclient "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
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
	lastProvisioningTimestamps sync.Map
}

func NewFunctionMonitor(ctx context.Context,
	parentLogger logger.Logger,
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
		lastProvisioningTimestamps: sync.Map{},
	}

	newFunctionMonitor.logger.DebugWithCtx(ctx, "Created function monitor",
		"namespace", namespace,
		"interval", interval)

	return newFunctionMonitor, nil
}

func (fm *FunctionMonitor) Start(ctx context.Context) error {
	fm.logger.InfoWithCtx(ctx, "Starting",
		"interval", fm.interval,
		"namespace", fm.namespace)

	// create stop channel
	fm.stopChan = make(chan struct{}, 1)
	if fm.interval == 0 {
		fm.logger.WarnCtx(ctx, "Function monitoring is disabled")
		return nil
	}

	// spawn a goroutine for function monitoring
	go func() {
		defer func() {
			if err := recover(); err != nil {
				callStack := debug.Stack()
				fm.logger.ErrorWithCtx(ctx, "Panic caught while monitoring functions",
					"err", err,
					"stack", string(callStack))
			}
		}()
		for {
			select {
			case <-time.After(fm.interval):
				if err := fm.checkFunctionStatuses(ctx); err != nil {
					fm.logger.WarnWithCtx(ctx, "Failed check function statuses",
						"namespace", fm.namespace,
						"err", errors.Cause(err))
				}

			case <-fm.stopChan:
				fm.logger.DebugWithCtx(ctx, "Stopped function monitoring",
					"namespace", fm.namespace)
				return
			}
		}
	}()

	return nil
}

func (fm *FunctionMonitor) Stop(ctx context.Context) {
	fm.logger.InfoWithCtx(ctx, "Stopping function monitoring", "namespace", fm.namespace)

	// post to channel
	if fm.stopChan != nil {
		fm.stopChan <- struct{}{}
	}
}

func (fm *FunctionMonitor) checkFunctionStatuses(ctx context.Context) error {
	functions, err := fm.nuclioClientSet.NuclioV1beta1().NuclioFunctions(fm.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "Failed to list functions")
	}

	errGroup, _ := errgroup.WithContext(ctx, fm.logger)
	for _, function := range functions.Items {
		function := function
		errGroup.Go("update-function-status", func() error {
			return fm.updateFunctionStatus(ctx, &function)
		})
	}
	return errGroup.Wait()
}

func (fm *FunctionMonitor) updateFunctionStatus(ctx context.Context, function *nuclioio.NuclioFunction) error {

	// skip check for function status
	if fm.shouldSkipFunctionMonitoring(ctx, function) {
		return nil
	}

	fm.logger.DebugWithCtx(ctx, "Getting function deployment function",
		"functionName", function.Name,
		"functionNamespace", function.Namespace)

	functionDeployment, err := fm.kubeClientSet.
		AppsV1().
		Deployments(function.Namespace).
		Get(ctx, kube.DeploymentNameFromFunctionName(function.Name), metav1.GetOptions{})
	if err != nil {
		fm.logger.WarnWithCtx(ctx, "Failed to get function deployment",
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
		function.Status.Message = string(common.FunctionStateMessageUnhealthy)
		stateChanged = true
	}

	// return if function did not change
	if !stateChanged {
		return nil
	}

	// function state has changed, update CRD correspondingly
	fm.logger.InfoWithCtx(ctx, "Function state has changed, updating",
		"functionName", function.Name,
		"functionStatus", function.Status,
		"functionNamespace", function.Namespace,
		"functionDeploymentStatus", functionDeployment.Status,
		"functionIsAvailable", functionIsAvailable)
	if _, err := fm.nuclioClientSet.
		NuclioV1beta1().
		NuclioFunctions(fm.namespace).
		Update(ctx, function, metav1.UpdateOptions{}); err != nil {
		fm.logger.WarnWithCtx(ctx, "Failed to update function",
			"functionName", function.Name,
			"functionStatus", function.Status,
			"functionNamespace", function.Namespace)
	}
	return nil
}

func (fm *FunctionMonitor) isAvailable(deployment *appsv1.Deployment) bool {

	// require at least one replica
	atLeastOneReplicasRequested := deployment.Spec.Replicas != nil && *deployment.Spec.Replicas > 0
	if !atLeastOneReplicasRequested {
		return false
	}

	// Since we considered function as ready when it reaches its minimum replicas available (see pkg/platform/kube/functionres/lazy.go:240
	// WaitAvailable() for more information.), we might hit a situation where a "ready" function is still in progress,
	// as it reaches its "minimum available replicas" condition from a previous deploy, while still deploying a new replica,
	// and hence we cannot resolve this condition as a failure but rather let it run until the recently
	// deployed replica-set hits a failure (as suggested by the failures below).
	//
	// Iterate over function deployment conditions and "cherry-pick" conditions in which we know the function is no longer available.
	for _, condition := range deployment.Status.Conditions {

		// The errors below are considered errors in which may occur during
		// - nth deployment (n >= 2)
		// - function lifetime

		// - deployment available and status is false - usually when no minimum replica is available
		//   - may occur during function lifetime, at any point
		//   > e.g.: when evicting all pods from a single node
		if condition.Type == appsv1.DeploymentAvailable && condition.Status == v1.ConditionFalse {
			return false
		}

		// - replica failure - usually when failed to populate deployment spec
		//   - may occur during/past 2nd deployment while old replica is still considered as the "minimum available"
		//   > e.g.: failed to find a specific resource specified on deployment spec (configmap / service account, etc)
		if condition.Type == appsv1.DeploymentReplicaFailure {
			return false
		}

		// - deployment is in progress and status is false - insufficient quota / image pull errors, etc
		//   - may occur during/past 2nd deployment or function lifetime
		//   > e.g.: when failing to fulfill function CPU request due to CPU quota limit or image does not exists on registry
		// https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#failed-deployment
		if condition.Type == appsv1.DeploymentProgressing && condition.Status == v1.ConditionFalse {
			return false
		}
	}

	// at this stage, all conditions are not a failure as they are either available or progressing
	return true
}

// We monitor functions that meet the following conditions:
// - not in provisioning state
// - not recently deployed
// - not in transitional states
// - not disabled / replicas set to 0
func (fm *FunctionMonitor) shouldSkipFunctionMonitoring(ctx context.Context, function *nuclioio.NuclioFunction) bool {

	// ignore provisioning states
	// ignore recently deployed function
	if fm.resolveFunctionProvisionedOrRecentlyDeployed(ctx, function) {
		fm.logger.DebugWithCtx(ctx, "Function is being provisioned or recently deployed, skipping",
			"functionName", function.Name,
			"functionState", function.Status.State)
		return true
	}

	// ignore provisioned states other than ready / unhealthy
	if !functionconfig.FunctionStateInSlice(function.Status.State, []functionconfig.FunctionState{
		functionconfig.FunctionStateReady,
		functionconfig.FunctionStateUnhealthy,
	}) {
		fm.logger.DebugWithCtx(ctx, "Function state is not ready or unhealthy, skipping",
			"functionName", function.Name,
			"functionState", function.Status.State)
		return true
	}

	// skip disabled functions / 0-ed replicas functions
	if function.Spec.Disable || (function.Spec.Replicas != nil && *function.Spec.Replicas == 0) {
		fm.logger.DebugWithCtx(ctx, "Function is disabled or has 0 desired replicas, skipping",
			"functionName", function.Name,
			"functionReplicas", function.Spec.Replicas,
			"functionDisabled", function.Spec.Disable)
		return true
	}

	// do not skip as function should be monitored
	return false
}

func (fm *FunctionMonitor) resolveFunctionProvisionedOrRecentlyDeployed(ctx context.Context,
	function *nuclioio.NuclioFunction) bool {
	if functionconfig.FunctionStateProvisioning(function.Status.State) {
		fm.lastProvisioningTimestamps.Store(function.Name, time.Now())
		fm.logger.DebugWithCtx(ctx, "Function is in provisioning state",
			"functionState", function.Status.State,
			"functionName", function.Name)
		return true
	} else if lastProvisioningTimestamp, ok := fm.lastProvisioningTimestamps.Load(function.Name); ok {
		if lastProvisioningTimestamp.(time.Time).Add(PostDeploymentMonitoringBlockingInterval).After(time.Now()) {
			fm.logger.DebugWithCtx(ctx, "Function was recently deployed",
				"functionName", function.Name,
				"lastProvisioningTimestamp", lastProvisioningTimestamp)
			return true
		}
	}

	// cleanup
	fm.lastProvisioningTimestamps.Delete(function.Name)
	return false
}
