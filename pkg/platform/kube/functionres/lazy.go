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

package functionres

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errgroup"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform/abstract"
	"github.com/nuclio/nuclio/pkg/platform/kube"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"github.com/nuclio/nuclio/pkg/platform/kube/client"
	nuclioioclient "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/config"
	"github.com/nuclio/nuclio/pkg/processor/trigger/cron"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http"

	"github.com/ghodss/yaml"
	"github.com/imdario/mergo"
	"github.com/mitchellh/mapstructure"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/v3io/version-go"
	appsv1 "k8s.io/api/apps/v1"
	autosv2 "k8s.io/api/autoscaling/v2beta1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	"k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apiresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

const (
	ContainerHTTPPortName         = "http"
	containerMetricPort           = 8090
	containerMetricPortName       = "metrics"
	nginxIngressUpdateGracePeriod = 5 * time.Second
)

type deploymentResourceMethod string

const (
	createDeploymentResourceMethod deploymentResourceMethod = "create"
	updateDeploymentResourceMethod deploymentResourceMethod = "update"
)

//
// Client
//

type lazyClient struct {
	logger                        logger.Logger
	kubeClientSet                 kubernetes.Interface
	nuclioClientSet               nuclioioclient.Interface
	classLabels                   labels.Set
	platformConfigurationProvider PlatformConfigurationProvider
	nginxIngressUpdateGracePeriod time.Duration
	nodeScaleUpSleepTimeout       time.Duration
}

func NewLazyClient(parentLogger logger.Logger,
	kubeClientSet kubernetes.Interface,
	nuclioClientSet nuclioioclient.Interface) (Client, error) {

	newClient := lazyClient{
		logger:                        parentLogger.GetChild("functionres"),
		kubeClientSet:                 kubeClientSet,
		nuclioClientSet:               nuclioClientSet,
		classLabels:                   make(labels.Set),
		nginxIngressUpdateGracePeriod: nginxIngressUpdateGracePeriod,

		//  autoscale cycle is at least 10s
		nodeScaleUpSleepTimeout: 15 * time.Second,
	}

	newClient.initClassLabels()

	return &newClient, nil
}

func (lc *lazyClient) List(ctx context.Context, namespace string) ([]Resources, error) {
	listOptions := metav1.ListOptions{
		LabelSelector: "nuclio.io/class=function",
	}

	result, err := lc.kubeClientSet.AppsV1().Deployments(namespace).List(ctx, listOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to list deployments")
	}

	lc.logger.DebugWithCtx(ctx, "Got deployments", "num", len(result.Items))

	var resources []Resources

	// iterate over items, create a lazy resources
	for _, item := range result.Items {

		// create a lazy resource and append to resources - populating deployment
		resources = append(resources, &lazyResources{
			deployment: &item,
		})
	}

	return resources, nil
}

func (lc *lazyClient) Get(ctx context.Context, namespace string, name string) (Resources, error) {
	var result *appsv1.Deployment
	deploymentName := kube.DeploymentNameFromFunctionName(name)
	result, err := lc.kubeClientSet.AppsV1().
		Deployments(namespace).
		Get(ctx, deploymentName, metav1.GetOptions{})
	lc.logger.DebugWithCtx(ctx,
		"Got deployment",
		"namespace", namespace,
		"deploymentName", deploymentName,
		"result", result,
		"err", err)

	// if we didn't find it
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, err
	}

	// create a lazy resources instance, populating deployment
	return &lazyResources{
		deployment: result,
	}, err
}

func (lc *lazyClient) CreateOrUpdate(ctx context.Context,
	function *nuclioio.NuclioFunction,
	imagePullSecrets string) (Resources, error) {
	var err error

	// get labels from the function and add class labels
	functionLabels := lc.getFunctionLabels(function)

	// set a few constants
	functionLabels["nuclio.io/function-name"] = function.Name

	// TODO: remove when versioning is back in
	function.Spec.Version = -1
	function.Spec.Alias = "latest"
	functionLabels["nuclio.io/function-version"] = "latest"

	resources := lazyResources{}

	platformConfig := lc.platformConfigurationProvider.GetPlatformConfiguration()
	for _, augmentedConfig := range platformConfig.FunctionAugmentedConfigs {

		selector, err := metav1.LabelSelectorAsSelector(&augmentedConfig.LabelSelector)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get selector from label selector")
		}

		// if the label matches any of the function labels, augment the function with provided function config
		if selector.Matches(functionLabels) {
			encodedFunctionConfig, err := yaml.Marshal(augmentedConfig.FunctionConfig)
			if err != nil {
				return nil, errors.Wrap(err, "Failed to marshal augmented function config")
			}

			err = yaml.Unmarshal(encodedFunctionConfig, function)
			if err != nil {
				return nil, errors.Wrap(err, "Failed to join augmented function config into target function")
			}
		}
	}

	// create or update the applicable configMap
	if resources.configMap, err = lc.createOrUpdateConfigMap(ctx, function); err != nil {
		return nil, errors.Wrap(err, "Failed to create/update configMap")
	}

	// create or update the applicable service
	if resources.service, err = lc.createOrUpdateService(ctx, functionLabels, function); err != nil {
		return nil, errors.Wrap(err, "Failed to create/update service")
	}

	// create or update the applicable deployment
	if resources.deployment, err = lc.createOrUpdateDeployment(ctx,
		functionLabels,
		imagePullSecrets,
		function); err != nil {
		return nil, errors.Wrap(err, "Failed to create/update deployment")
	}

	// create or update the HPA
	if resources.horizontalPodAutoscaler, err = lc.createOrUpdateHorizontalPodAutoscaler(ctx,
		functionLabels,
		function); err != nil {
		return nil, errors.Wrap(err, "Failed to create/update HPA")
	}

	// create or update ingress
	if resources.ingress, err = lc.createOrUpdateIngress(ctx, functionLabels, function); err != nil {
		return nil, errors.Wrap(err, "Failed to create/update ingress")
	}

	// whether to use kubernetes cron job to invoke nuclio function cron trigger
	if lc.platformConfigurationProvider.GetPlatformConfiguration().CronTriggerCreationMode == platformconfig.KubeCronTriggerCreationMode {
		if resources.cronJobs, err = lc.createOrUpdateCronJobs(ctx, functionLabels, function, &resources); err != nil {
			return nil, errors.Wrap(err, "Failed to create cron jobs from cron triggers")
		}
	}

	lc.logger.DebugWithCtx(ctx, "Successfully created/updated resources",
		"functionName", function.Name,
		"functionNamespace", function.Namespace)
	return &resources, nil
}

func (lc *lazyClient) WaitAvailable(ctx context.Context,
	namespace string,
	name string,
	functionResourcesCreateOrUpdateTimestamp time.Time) (error, functionconfig.FunctionState) {
	deploymentName := kube.DeploymentNameFromFunctionName(name)
	lc.logger.DebugWithCtx(ctx, "Waiting for deployment to be available",
		"namespace", namespace,
		"functionName", name,
		"deploymentName", deploymentName)

	waitMs := 250

	for {

		// wait a bit
		time.Sleep(time.Duration(waitMs) * time.Millisecond)

		// exponentially wait more next time, up to 2 seconds
		waitMs *= 2
		if waitMs > 2000 {
			waitMs = 2000
		}

		// check if context is still OK
		if err := ctx.Err(); err != nil {
			return err, functionconfig.FunctionStateUnhealthy
		}

		// get the deployment. if it doesn't exist yet, retry a bit later
		result, err := lc.kubeClientSet.AppsV1().
			Deployments(namespace).
			Get(ctx, deploymentName, metav1.GetOptions{})
		if err != nil {
			continue
		}

		// find the condition whose type is Available - that's the one we want to examine
		for _, deploymentCondition := range result.Status.Conditions {

			// when we find the right condition, check its Status to see if it's true.
			// a DeploymentCondition whose Type == Available and Status == True means the deployment is available
			if deploymentCondition.Type == appsv1.DeploymentAvailable {
				available := deploymentCondition.Status == v1.ConditionTrue

				if available && result.Status.UnavailableReplicas == 0 {
					lc.logger.DebugWithCtx(ctx, "Deployment is available",
						"reason", deploymentCondition.Reason,
						"deploymentName", deploymentName)
					return nil, functionconfig.FunctionStateReady
				}

				lc.logger.DebugWithCtx(ctx, "Deployment not available yet",
					"reason", deploymentCondition.Reason,
					"unavailableReplicas", result.Status.UnavailableReplicas,
					"deploymentName", deploymentName)

				// we found the condition, wasn't available
				break
			}
		}

		// get the deployment pods. if it doesn't exist yet, retry a bit later
		podsList, err := lc.kubeClientSet.CoreV1().
			Pods(namespace).
			List(ctx, metav1.ListOptions{
				LabelSelector: common.CompileListFunctionPodsLabelSelector(name),
			})
		if err != nil {
			continue
		}

		// fail-fast mechanism
		if err := lc.resolveFailFast(ctx,
			podsList,
			functionResourcesCreateOrUpdateTimestamp); err != nil {

			return errors.Wrapf(err, "NuclioFunction deployment failed"), functionconfig.FunctionStateError
		}
	}
}

func (lc *lazyClient) Delete(ctx context.Context, namespace string, name string) error {
	propagationPolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	}

	// Delete ingress
	ingressName := kube.IngressNameFromFunctionName(name)
	err := lc.kubeClientSet.NetworkingV1().Ingresses(namespace).Delete(ctx, ingressName, deleteOptions)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrap(err, "Failed to delete ingress")
		}
	} else {
		lc.logger.DebugWithCtx(ctx, "Deleted ingress", "namespace", namespace, "ingressName", ingressName)
	}

	// Delete HPA if exists
	hpaName := kube.HPANameFromFunctionName(name)
	err = lc.kubeClientSet.AutoscalingV2beta1().HorizontalPodAutoscalers(namespace).Delete(ctx, hpaName, deleteOptions)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrap(err, "Failed to delete HPA")
		}
	} else {
		lc.logger.DebugWithCtx(ctx, "Deleted HPA", "namespace", namespace, "hpaName", hpaName)
	}

	// Delete Service if exists
	serviceName := kube.ServiceNameFromFunctionName(name)
	err = lc.kubeClientSet.CoreV1().Services(namespace).Delete(ctx, serviceName, deleteOptions)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrap(err, "Failed to delete service")
		}
	} else {
		lc.logger.DebugWithCtx(ctx, "Deleted service", "namespace", namespace, "serviceName", serviceName)
	}

	// Delete Deployment if exists
	deploymentName := kube.DeploymentNameFromFunctionName(name)
	err = lc.kubeClientSet.AppsV1().Deployments(namespace).Delete(ctx, deploymentName, deleteOptions)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrap(err, "Failed to delete deployment")
		}
	} else {
		lc.logger.DebugWithCtx(ctx, "Deleted deployment",
			"namespace", namespace,
			"deploymentName", deploymentName)
	}

	// Delete configMap if exists
	configMapName := kube.ConfigMapNameFromFunctionName(name)
	err = lc.kubeClientSet.CoreV1().ConfigMaps(namespace).Delete(ctx, configMapName, deleteOptions)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrap(err, "Failed to delete configMap")
		}
	} else {
		lc.logger.DebugWithCtx(ctx, "Deleted configMap", "namespace", namespace, "configMapName", configMapName)
	}

	if err = lc.deleteFunctionEvents(ctx, name, namespace); err != nil {
		return errors.Wrap(err, "Failed to delete function events")
	}

	if lc.platformConfigurationProvider.GetPlatformConfiguration().CronTriggerCreationMode == platformconfig.KubeCronTriggerCreationMode {
		if err = lc.deleteCronJobs(ctx, name, namespace); err != nil {
			return errors.Wrap(err, "Failed to delete function cron jobs")
		}
	}

	lc.logger.DebugWithCtx(ctx, "Deleted deployed function", "namespace", namespace, "name", name)

	return nil
}

// SetPlatformConfigurationProvider sets the provider of the platform configuration for any future access
func (lc *lazyClient) SetPlatformConfigurationProvider(platformConfigurationProvider PlatformConfigurationProvider) {
	lc.platformConfigurationProvider = platformConfigurationProvider
}

func (lc *lazyClient) createOrUpdateCronJobs(ctx context.Context,
	functionLabels labels.Set,
	function *nuclioio.NuclioFunction,
	resources Resources) ([]*batchv1beta1.CronJob, error) {
	var cronJobs []*batchv1beta1.CronJob
	var suspendCronJobs bool

	// if function was paused - suspend all cron jobs
	if function.Spec.Disable {
		suspendCronJobs = true
	}

	cronTriggerCronJobs, err := lc.createOrUpdateCronTriggerCronJobs(ctx, functionLabels, function, resources, suspendCronJobs)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create cron trigger cron jobs")
	}

	cronJobs = append(cronJobs, cronTriggerCronJobs...)
	return cronJobs, nil
}

// create cron triggers as k8s cron jobs instead of creating them inside the processor
// these k8s cron jobs will invoke the function's default http trigger on their schedule/interval
// this will enable using the scale to zero functionality of http triggers for cron triggers
func (lc *lazyClient) createOrUpdateCronTriggerCronJobs(ctx context.Context,
	functionLabels labels.Set,
	function *nuclioio.NuclioFunction,
	resources Resources,
	suspendCronJobs bool) ([]*batchv1beta1.CronJob, error) {
	var cronJobs []*batchv1beta1.CronJob

	cronTriggers := functionconfig.GetTriggersByKind(function.Spec.Triggers, "cron")

	// first, remove all cron-trigger-cron-jobs that are irrelevant - exists but doesn't appear on function spec (removed on update)
	if err := lc.deleteRemovedCronTriggersCronJob(ctx, functionLabels, function, cronTriggers); err != nil {
		return nil, errors.Wrap(err, "Failed to delete removed cron triggers cron job")
	}

	for triggerName, cronTrigger := range cronTriggers {
		cronJobSpec, err := lc.generateCronTriggerCronJobSpec(functionLabels, function, resources, cronTrigger)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to generate cron job spec from cron trigger. Trigger name: %s", triggerName)
		}

		extraMetaLabels := labels.Set{
			"nuclio.io/component":                  "cron-trigger",
			"nuclio.io/function-cron-trigger-name": triggerName,
		}
		cronJob, err := lc.createOrUpdateCronJob(ctx,
			functionLabels,
			extraMetaLabels,
			function,
			triggerName,
			cronJobSpec,
			suspendCronJobs)
		if err != nil {

			go func() {
				if deleteCronJobsErr := lc.deleteCronJobs(ctx, function.Name, function.Namespace); deleteCronJobsErr != nil {
					lc.logger.WarnWithCtx(ctx, "Failed to delete cron jobs on cron job creation failure",
						"deleteCronJobsErr", deleteCronJobsErr)
				}
			}()

			return nil, errors.Wrapf(err, "Failed to create/update cron job for trigger: %s", triggerName)
		}

		cronJobs = append(cronJobs, cronJob)
	}

	return cronJobs, nil
}

// delete every cron-trigger-cron-job of the function that has been removed from the function's triggers
func (lc *lazyClient) deleteRemovedCronTriggersCronJob(ctx context.Context,
	functionLabels labels.Set,
	function *nuclioio.NuclioFunction,
	newCronTriggers map[string]functionconfig.Trigger) error {

	// make a list of all the new cron trigger cron job names
	var newCronTriggerNames []string
	for newCronTriggerName := range newCronTriggers {
		newCronTriggerNames = append(newCronTriggerNames, newCronTriggerName)
	}

	cronTriggerInNewCronTriggers, err := lc.compileCronTriggerNotInSliceLabels(newCronTriggerNames)
	if err != nil {
		return errors.Wrap(err, "Failed to compile cron trigger not in slice labels")
	}

	// retrieve all the cron jobs that aren't inside the new cron triggers, so they can be deleted
	cronJobsToDelete, err := lc.kubeClientSet.BatchV1beta1().CronJobs(function.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: lc.compileCronTriggerLabelSelector(function.Name, cronTriggerInNewCronTriggers),
	})
	if err != nil {
		return errors.Wrap(err, "Failed to list cron jobs")
	}

	// if there's none to delete return
	if len(cronJobsToDelete.Items) == 0 {
		return nil
	}

	lc.logger.DebugWithCtx(ctx, "Deleting removed cron trigger cron job",
		"cronJobsToDelete", cronJobsToDelete)

	errGroup, _ := errgroup.WithContext(ctx, lc.logger)
	for _, cronJobToDelete := range cronJobsToDelete.Items {
		cronJobToDelete := cronJobToDelete
		errGroup.Go("DeleteCronTrigger", func() error {

			// delete this removed cron trigger cron job
			err := lc.kubeClientSet.BatchV1beta1().
				CronJobs(function.Namespace).
				Delete(ctx, cronJobToDelete.Name, metav1.DeleteOptions{})

			if err != nil {
				return errors.Wrapf(err, "Failed to delete removed cron trigger cron job: %s", cronJobToDelete.Name)
			}

			return nil
		})
	}

	return errGroup.Wait()
}

// as a closure so resourceExists can update
func (lc *lazyClient) createOrUpdateResource(ctx context.Context,
	resourceName string,
	getResource func() (interface{}, error),
	resourceIsDeleting func(interface{}) bool,
	createResource func() (interface{}, error),
	updateResource func(interface{}) (interface{}, error)) (interface{}, error) {

	var resource interface{}
	var err error

	updateDeadline := time.Now().Add(2 * time.Minute)

	for {
		waitingForDeletionDeadline := time.Now().Add(1 * time.Minute)

		// get the resource until it's not deleting
		for {

			// get resource will return the resource
			resource, err = getResource()

			// if the resource is deleting, wait for it to complete deleting
			if err == nil && resourceIsDeleting(resource) {
				lc.logger.DebugWithCtx(ctx, "Resource is deleting, waiting", "name", resourceName)

				// we need to wait a bit and try again
				time.Sleep(1 * time.Second)

				// if we passed the deadline
				if time.Now().After(waitingForDeletionDeadline) {
					return nil, errors.New("Timed out waiting for service to delete")
				}

			} else {

				// there was either an error or the resource exists and is not being deleted
				break
			}
		}

		// if there's an error
		if err != nil {

			// bail if error is not "not found"
			if !apierrors.IsNotFound(err) {
				return nil, errors.Wrapf(err, "Failed to get resource")
			}

			// create the resource
			resource, err = createResource()
			if err != nil {
				if !apierrors.IsAlreadyExists(err) {
					return nil, errors.Wrap(err, "Failed to create resource")
				}

				// this case could happen if several controllers are running in parallel. (may happen on rolling upgrade of the controller)
				lc.logger.WarnWithCtx(ctx, "Got \"resource already exists\" error on creation. Retrying (Perhaps more than 1 controller is running?)",
					"name", resourceName,
					"err", err.Error())
				continue
			}

			lc.logger.DebugWithCtx(ctx, "Resource created", "name", resourceName)
			return resource, nil
		}

		resource, err = updateResource(resource)
		if err != nil {

			// if there was an error and it wasn't conflict - there was an error. Bail
			if !apierrors.IsConflict(err) {
				return nil, errors.Wrapf(err, "Failed to update resource")
			}

			// if we passed the deadline
			if time.Now().After(updateDeadline) {
				return nil, errors.Errorf("Timed out updating resource: %s", resourceName)
			}

			lc.logger.DebugWithCtx(ctx, "Got conflict while trying to update resource. Retrying", "name", resourceName)
			continue
		}

		lc.logger.DebugWithCtx(ctx, "Resource updated", "name", resourceName)
		return resource, nil
	}
}

func (lc *lazyClient) createOrUpdateConfigMap(ctx context.Context, function *nuclioio.NuclioFunction) (*v1.ConfigMap, error) {

	getConfigMap := func() (interface{}, error) {
		return lc.kubeClientSet.CoreV1().
			ConfigMaps(function.Namespace).
			Get(ctx, kube.ConfigMapNameFromFunctionName(function.Name), metav1.GetOptions{})
	}

	configMapIsDeleting := func(resource interface{}) bool {
		return (resource).(*v1.ConfigMap).ObjectMeta.DeletionTimestamp != nil
	}

	createConfigMap := func() (interface{}, error) {
		configMap := v1.ConfigMap{}
		if err := lc.populateConfigMap(nil, function, &configMap); err != nil {
			return nil, errors.Wrap(err, "Failed to populate configMap")
		}

		return lc.kubeClientSet.CoreV1().ConfigMaps(function.Namespace).Create(ctx, &configMap, metav1.CreateOptions{})
	}

	updateConfigMap := func(resource interface{}) (interface{}, error) {
		configMap := resource.(*v1.ConfigMap)

		// update existing
		if err := lc.populateConfigMap(nil, function, configMap); err != nil {
			return nil, errors.Wrap(err, "Failed to populate configMap")
		}

		return lc.kubeClientSet.CoreV1().ConfigMaps(function.Namespace).Update(ctx, configMap, metav1.UpdateOptions{})
	}

	resource, err := lc.createOrUpdateResource(ctx,
		"configMap",
		getConfigMap,
		configMapIsDeleting,
		createConfigMap,
		updateConfigMap)

	if err != nil {
		return nil, err
	}

	return resource.(*v1.ConfigMap), err
}

func (lc *lazyClient) createOrUpdateService(ctx context.Context,
	functionLabels labels.Set,
	function *nuclioio.NuclioFunction) (*v1.Service, error) {

	getService := func() (interface{}, error) {
		return lc.kubeClientSet.CoreV1().
			Services(function.Namespace).
			Get(ctx, kube.ServiceNameFromFunctionName(function.Name), metav1.GetOptions{})
	}

	serviceIsDeleting := func(resource interface{}) bool {
		return (resource).(*v1.Service).ObjectMeta.DeletionTimestamp != nil
	}

	createService := func() (interface{}, error) {
		spec := v1.ServiceSpec{}
		lc.populateServiceSpec(ctx, functionLabels, function, &spec)

		return lc.kubeClientSet.CoreV1().Services(function.Namespace).Create(ctx,
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      kube.ServiceNameFromFunctionName(function.Name),
					Namespace: function.Namespace,
					Labels:    functionLabels,
				},
				Spec: spec,
			},
			metav1.CreateOptions{})
	}

	updateService := func(resource interface{}) (interface{}, error) {
		service := resource.(*v1.Service)

		// update existing
		service.Labels = functionLabels
		lc.populateServiceSpec(ctx, functionLabels, function, &service.Spec)

		return lc.kubeClientSet.CoreV1().Services(function.Namespace).Update(ctx, service, metav1.UpdateOptions{})
	}

	resource, err := lc.createOrUpdateResource(ctx,
		"service",
		getService,
		serviceIsDeleting,
		createService,
		updateService)

	if err != nil {
		return nil, err
	}

	return resource.(*v1.Service), err
}

func (lc *lazyClient) createOrUpdateDeployment(ctx context.Context,
	functionLabels labels.Set,
	imagePullSecrets string,
	function *nuclioio.NuclioFunction) (*appsv1.Deployment, error) {

	// to make sure the pod re-pulls the image, we need to specify a unique string here
	podAnnotations, err := lc.getPodAnnotations(function)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get pod annotations")
	}

	replicas := function.GetComputedReplicas()
	if replicas != nil {
		lc.logger.DebugWithCtx(ctx, "Got replicas", "replicas", *replicas, "functionName", function.Name)
	}
	deploymentAnnotations, err := lc.getDeploymentAnnotations(function)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get function annotations")
	}

	// get volumes and volumeMounts from configuration
	volumes, volumeMounts := lc.getFunctionVolumeAndMounts(ctx, function)

	getDeployment := func() (interface{}, error) {
		return lc.kubeClientSet.AppsV1().
			Deployments(function.Namespace).
			Get(ctx, kube.DeploymentNameFromFunctionName(function.Name), metav1.GetOptions{})
	}

	deploymentIsDeleting := func(resource interface{}) bool {
		return (resource).(*appsv1.Deployment).ObjectMeta.DeletionTimestamp != nil
	}

	if function.Spec.ImagePullSecrets != "" {
		imagePullSecrets = function.Spec.ImagePullSecrets
	}

	createDeployment := func() (interface{}, error) {
		method := createDeploymentResourceMethod
		container := v1.Container{Name: client.FunctionContainerName}
		lc.populateDeploymentContainer(functionLabels, function, &container)
		container.VolumeMounts = volumeMounts

		deploymentSpec := appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: functionLabels,
			},
			Replicas: replicas,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:        kube.PodNameFromFunctionName(function.Name),
					Namespace:   function.Namespace,
					Labels:      functionLabels,
					Annotations: podAnnotations,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						container,
					},
					Volumes:            volumes,
					ServiceAccountName: function.Spec.ServiceAccount,
					SecurityContext:    function.Spec.SecurityContext,
					Affinity:           function.Spec.Affinity,
					Tolerations:        function.Spec.Tolerations,
					NodeSelector:       function.Spec.NodeSelector,
					NodeName:           function.Spec.NodeName,
					PriorityClassName:  function.Spec.PriorityClassName,
					PreemptionPolicy:   function.Spec.PreemptionPolicy,
					HostIPC:            function.Spec.HostIPC,
				},
			},
		}

		// apply when provided
		if imagePullSecrets != "" {
			deploymentSpec.Template.Spec.ImagePullSecrets = []v1.LocalObjectReference{
				{Name: imagePullSecrets},
			}
		}

		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:        kube.DeploymentNameFromFunctionName(function.Name),
				Namespace:   function.Namespace,
				Labels:      functionLabels,
				Annotations: deploymentAnnotations,
			},
			Spec: deploymentSpec,
		}

		// enrich deployment spec with default fields that were passed inside the platform configuration
		if err := lc.enrichDeploymentFromPlatformConfiguration(function, deployment, method); err != nil {
			return nil, err
		}
		return lc.kubeClientSet.AppsV1().Deployments(function.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
	}

	updateDeployment := func(resource interface{}) (interface{}, error) {
		deployment := resource.(*appsv1.Deployment)
		method := updateDeploymentResourceMethod

		// If we got nil replicas it means leave as is (in order to prevent unwanted scale down)
		// but need to make sure the current replicas is not less than the min replicas
		if replicas == nil {
			minReplicas := function.GetComputedMinReplicas()
			maxReplicas := function.GetComputedMaxReplicas()
			deploymentReplicas := deployment.Status.Replicas
			lc.logger.DebugWithCtx(ctx, "Verifying current replicas not lower than minReplicas or higher than max",
				"functionName", function.Name,
				"maxReplicas", maxReplicas,
				"minReplicas", minReplicas,
				"deploymentReplicas", deploymentReplicas)
			switch {
			case deploymentReplicas > maxReplicas:
				replicas = &maxReplicas
			case deploymentReplicas < minReplicas:
				replicas = &minReplicas
			default:

				// if we're within the valid range - and want to leave as is (since replicas == nil) - use current value
				// NOTE: since we're using the existing deployment (given by our get function) ResourceVersion is set
				// meaning the update will fail with conflict if something has changed in the meanwhile (e.g. HPA
				// changed the replicas count) - retry is handled by the createOrUpdateResource wrapper
				replicas = &deploymentReplicas

			}
		}

		deployment.Annotations = deploymentAnnotations
		deployment.Spec.Replicas = replicas
		deployment.Spec.Template.Annotations = podAnnotations
		lc.populateDeploymentContainer(functionLabels, function, &deployment.Spec.Template.Spec.Containers[0])
		deployment.Spec.Template.Spec.Volumes = volumes
		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts
		deployment.Spec.Template.Spec.SecurityContext = function.Spec.SecurityContext

		if function.Spec.ServiceAccount != "" {
			deployment.Spec.Template.Spec.ServiceAccountName = function.Spec.ServiceAccount
		}

		deployment.Spec.Template.Spec.Tolerations = function.Spec.Tolerations
		deployment.Spec.Template.Spec.Affinity = function.Spec.Affinity
		deployment.Spec.Template.Spec.NodeSelector = function.Spec.NodeSelector
		deployment.Spec.Template.Spec.NodeName = function.Spec.NodeName
		deployment.Spec.Template.Spec.PriorityClassName = function.Spec.PriorityClassName
		deployment.Spec.Template.Spec.PreemptionPolicy = function.Spec.PreemptionPolicy

		// enrich deployment spec with default fields that were passed inside the platform configuration
		// performed on update too, in case the platform config has been modified after the creation of this deployment
		if err := lc.enrichDeploymentFromPlatformConfiguration(function, deployment, method); err != nil {
			return nil, err
		}

		return lc.kubeClientSet.AppsV1().Deployments(function.Namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	}

	resource, err := lc.createOrUpdateResource(ctx,
		"deployment",
		getDeployment,
		deploymentIsDeleting,
		createDeployment,
		updateDeployment)

	if err != nil {
		return nil, err
	}

	return resource.(*appsv1.Deployment), err
}

func (lc *lazyClient) resolveDeploymentStrategy(function *nuclioio.NuclioFunction) appsv1.DeploymentStrategyType {

	// Since k8s (ATM) does not support rolling update for GPU
	// redeploying a Nuclio function will get stuck if no GPU is available
	// to overcome it, we simply change the update strategy to recreate
	// so k8s will kill the existing pod\function and create the new one
	if function.Spec.PositiveGPUResourceLimit() {

		// requested a gpu resource, change to recreate
		return appsv1.RecreateDeploymentStrategyType
	}
	// no gpu resources requested, set to rollingUpdate (default)
	return appsv1.RollingUpdateDeploymentStrategyType
}

func (lc *lazyClient) enrichDeploymentFromPlatformConfiguration(function *nuclioio.NuclioFunction,
	deployment *appsv1.Deployment,
	method deploymentResourceMethod) error {

	var allowResolvingDeploymentStrategy = true

	// explicit deployment strategy given on function spec
	if function.Spec.DeploymentStrategy != nil {
		allowResolvingDeploymentStrategy = false
		deployment.Spec.Strategy = *function.Spec.DeploymentStrategy
	}

	// get deployment augmented configurations
	deploymentAugmentedConfigs, err := lc.getDeploymentAugmentedConfigs(function)
	if err != nil {
		return errors.Wrap(err, "Failed to get deployment augmented configs")
	}

	// merge
	for _, augmentedConfig := range deploymentAugmentedConfigs {
		if augmentedConfig.Kubernetes.Deployment != nil {
			if augmentedConfig.Kubernetes.Deployment.Spec.Strategy.Type != "" ||
				augmentedConfig.Kubernetes.Deployment.Spec.Strategy.RollingUpdate != nil {
				allowResolvingDeploymentStrategy = false
			}
			if err := mergo.Merge(&deployment.Spec, &augmentedConfig.Kubernetes.Deployment.Spec); err != nil {
				return errors.Wrap(err, "Failed to merge deployment spec")
			}
		}
	}

	switch method {

	// on create, change inplace the deployment strategy
	case createDeploymentResourceMethod:
		if allowResolvingDeploymentStrategy {
			deployment.Spec.Strategy.Type = lc.resolveDeploymentStrategy(function)
		}
	case updateDeploymentResourceMethod:
		if allowResolvingDeploymentStrategy {
			newDeploymentStrategyType := lc.resolveDeploymentStrategy(function)
			if newDeploymentStrategyType != deployment.Spec.Strategy.Type {

				// if current strategy is rolling update, in order to change it to `Recreate`
				// we must remove `rollingUpdate` field
				if deployment.Spec.Strategy.Type == appsv1.RollingUpdateDeploymentStrategyType &&
					newDeploymentStrategyType == appsv1.RecreateDeploymentStrategyType {
					deployment.Spec.Strategy.RollingUpdate = nil
				}
				deployment.Spec.Strategy.Type = newDeploymentStrategyType
			}
		}
	}
	return nil
}

func (lc *lazyClient) getDeploymentAugmentedConfigs(function *nuclioio.NuclioFunction) (
	[]platformconfig.LabelSelectorAndConfig, error) {
	var configs []platformconfig.LabelSelectorAndConfig

	// get the function labels
	functionLabels := lc.getFunctionLabels(function)

	// get platform config
	platformConfig := lc.platformConfigurationProvider.GetPlatformConfiguration()

	for _, augmentedConfig := range platformConfig.FunctionAugmentedConfigs {

		selector, err := metav1.LabelSelectorAsSelector(&augmentedConfig.LabelSelector)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get selector from label selector")
		}

		// if the label matches any of the function labels, augment the deployment with provided function config
		// NOTE: supports spec only for now. in the future we can remove .Spec and try to merge both meta and spec
		if selector.Matches(functionLabels) {
			configs = append(configs, augmentedConfig)
		}
	}

	return configs, nil
}

func (lc *lazyClient) createOrUpdateHorizontalPodAutoscaler(ctx context.Context,
	functionLabels labels.Set,
	function *nuclioio.NuclioFunction) (*autosv2.HorizontalPodAutoscaler, error) {

	minReplicas := function.GetComputedMinReplicas()
	maxReplicas := function.GetComputedMaxReplicas()
	lc.logger.DebugWithCtx(ctx, "Create/Update hpa",
		"functionName", function.Name,
		"minReplicas", minReplicas,
		"maxReplicas", maxReplicas)

	// hpa min replicas must be equal or greater than 1
	if minReplicas < 1 {
		minReplicas = int32(1)
	}

	// hpa max replicas must be equal or greater than 1
	if maxReplicas < 1 {
		maxReplicas = int32(1)
	}

	getHorizontalPodAutoscaler := func() (interface{}, error) {
		return lc.kubeClientSet.AutoscalingV2beta1().
			HorizontalPodAutoscalers(function.Namespace).
			Get(ctx, kube.HPANameFromFunctionName(function.Name), metav1.GetOptions{})
	}

	horizontalPodAutoscalerIsDeleting := func(resource interface{}) bool {
		return (resource).(*autosv2.HorizontalPodAutoscaler).ObjectMeta.DeletionTimestamp != nil
	}

	createHorizontalPodAutoscaler := func() (interface{}, error) {
		if minReplicas == maxReplicas {
			return nil, nil
		}

		metricSpecs, err := lc.GetFunctionMetricSpecs(function)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get function metric specs")
		}

		hpa := autosv2.HorizontalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      kube.HPANameFromFunctionName(function.Name),
				Namespace: function.Namespace,
				Labels:    functionLabels,
			},
			Spec: autosv2.HorizontalPodAutoscalerSpec{
				MinReplicas: &minReplicas,
				MaxReplicas: maxReplicas,
				Metrics:     metricSpecs,
				ScaleTargetRef: autosv2.CrossVersionObjectReference{
					APIVersion: "apps/apps_v1",
					Kind:       "Deployment",
					Name:       kube.DeploymentNameFromFunctionName(function.Name),
				},
			},
		}

		return lc.kubeClientSet.AutoscalingV2beta1().HorizontalPodAutoscalers(function.Namespace).Create(ctx, &hpa, metav1.CreateOptions{})
	}

	updateHorizontalPodAutoscaler := func(resourceToUpdate interface{}) (interface{}, error) {
		hpa := resourceToUpdate.(*autosv2.HorizontalPodAutoscaler)

		metricSpecs, err := lc.GetFunctionMetricSpecs(function)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get function metric specs")
		}

		hpa.Spec.Metrics = metricSpecs
		hpa.Labels = functionLabels
		hpa.Spec.MinReplicas = &minReplicas
		hpa.Spec.MaxReplicas = maxReplicas

		// when the min replicas equal the max replicas, there's no need for hpa resourceToUpdate
		if minReplicas == maxReplicas {
			propogationPolicy := metav1.DeletePropagationForeground
			deleteOptions := &metav1.DeleteOptions{
				PropagationPolicy: &propogationPolicy,
			}

			lc.logger.DebugWithCtx(ctx, "Deleting hpa - min replicas and max replicas are equal",
				"functionName", function.Name,
				"name", hpa.Name)

			err := lc.kubeClientSet.AutoscalingV2beta1().
				HorizontalPodAutoscalers(function.Namespace).
				Delete(ctx, hpa.Name, *deleteOptions)
			return nil, err
		}

		return lc.kubeClientSet.AutoscalingV2beta1().HorizontalPodAutoscalers(function.Namespace).Update(ctx, hpa, metav1.UpdateOptions{})
	}

	resource, err := lc.createOrUpdateResource(ctx,
		"hpa",
		getHorizontalPodAutoscaler,
		horizontalPodAutoscalerIsDeleting,
		createHorizontalPodAutoscaler,
		updateHorizontalPodAutoscaler)

	// a resource can be nil if it didn't meet preconditions and wasn't created
	if err != nil || resource == nil {
		return nil, err
	}

	return resource.(*autosv2.HorizontalPodAutoscaler), err
}

func (lc *lazyClient) createOrUpdateIngress(ctx context.Context,
	functionLabels labels.Set,
	function *nuclioio.NuclioFunction) (*networkingv1.Ingress, error) {

	getIngress := func() (interface{}, error) {
		return lc.kubeClientSet.NetworkingV1().
			Ingresses(function.Namespace).
			Get(ctx, kube.IngressNameFromFunctionName(function.Name), metav1.GetOptions{})
	}

	ingressIsDeleting := func(resource interface{}) bool {
		return (resource).(*networkingv1.Ingress).ObjectMeta.DeletionTimestamp != nil
	}

	createIngress := func() (interface{}, error) {
		ingressMeta := metav1.ObjectMeta{
			Name:      kube.IngressNameFromFunctionName(function.Name),
			Namespace: function.Namespace,
			Labels:    functionLabels,
		}

		ingressSpec := networkingv1.IngressSpec{}

		if err := lc.populateIngressConfig(ctx, functionLabels, function, &ingressMeta, &ingressSpec); err != nil {
			return nil, errors.Wrap(err, "Failed to populate ingress spec")
		}

		// if there are no rules, don't create an ingress
		if len(ingressSpec.Rules) == 0 {
			return nil, nil
		}

		resultIngress, err := lc.kubeClientSet.NetworkingV1().
			Ingresses(function.Namespace).
			Create(ctx,
				&networkingv1.Ingress{
					ObjectMeta: ingressMeta,
					Spec:       ingressSpec,
				},
				metav1.CreateOptions{})
		if err == nil {
			lc.waitForNginxIngressToStabilize(ctx, resultIngress)
		}

		return resultIngress, err
	}

	updateIngress := func(resource interface{}) (interface{}, error) {
		ingress := resource.(*networkingv1.Ingress)

		// save to bool if there are current rules
		ingressRulesExist := len(ingress.Spec.Rules) > 0

		if err := lc.populateIngressConfig(ctx, functionLabels, function, &ingress.ObjectMeta, &ingress.Spec); err != nil {
			return nil, errors.Wrap(err, "Failed to populate ingress spec")
		}

		if len(ingress.Spec.Rules) == 0 {

			// if there are no rules and previously were, delete the ingress resource
			if ingressRulesExist {
				propagationPolicy := metav1.DeletePropagationForeground
				deleteOptions := &metav1.DeleteOptions{
					PropagationPolicy: &propagationPolicy,
				}

				err := lc.kubeClientSet.NetworkingV1().
					Ingresses(function.Namespace).
					Delete(ctx, kube.IngressNameFromFunctionName(function.Name), *deleteOptions)
				return nil, err

			}

			// there's nothing to update
			return nil, nil
		}

		resultIngress, err := lc.kubeClientSet.NetworkingV1().Ingresses(function.Namespace).Update(ctx, ingress, metav1.UpdateOptions{})
		if err == nil {
			lc.waitForNginxIngressToStabilize(ctx, ingress)
		}

		return resultIngress, err
	}

	resource, err := lc.createOrUpdateResource(ctx,
		"ingress",
		getIngress,
		ingressIsDeleting,
		createIngress,
		updateIngress)

	if err != nil {
		return nil, err
	}

	if resource == nil {
		return nil, nil
	}

	return resource.(*networkingv1.Ingress), err
}

func (lc *lazyClient) deleteCronJobs(ctx context.Context, functionName, functionNamespace string) error {
	lc.logger.InfoWithCtx(ctx, "Deleting function cron jobs", "functionName", functionName)

	functionNameLabel := fmt.Sprintf("nuclio.io/function-name=%s", functionName)

	return lc.kubeClientSet.BatchV1beta1().
		CronJobs(functionNamespace).
		DeleteCollection(ctx, metav1.DeleteOptions{},
			metav1.ListOptions{LabelSelector: functionNameLabel})
}

func (lc *lazyClient) createOrUpdateCronJob(ctx context.Context,
	functionLabels labels.Set,
	extraMetaLabels labels.Set,
	function *nuclioio.NuclioFunction,
	jobName string,
	cronJobSpec *batchv1beta1.CronJobSpec,
	suspendCronJob bool) (*batchv1beta1.CronJob, error) {

	// should cron job be suspended or not (true when function is paused)
	cronJobSpec.Suspend = &suspendCronJob

	// prepare cron job meta labels
	cronJobMetaLabels := labels.Merge(functionLabels, extraMetaLabels)

	getCronJob := func() (interface{}, error) {
		cronJobs, err := lc.kubeClientSet.BatchV1beta1().
			CronJobs(function.Namespace).
			List(ctx, metav1.ListOptions{
				LabelSelector: cronJobMetaLabels.String(),
			})
		if err != nil {
			return nil, errors.Wrapf(err, "Failed getting cron jobs for function %s", function.Name)
		}
		if len(cronJobs.Items) == 0 {

			// purposefully return a k8s NotFound because the `createOrUpdateResource` checks the err type
			return nil, apierrors.NewNotFound(nuclioio.Resource("cronjob"), jobName)
		}
		return &cronJobs.Items[0], nil
	}

	cronJobIsDeleting := func(resource interface{}) bool {
		return (resource).(*batchv1beta1.CronJob).ObjectMeta.DeletionTimestamp != nil
	}

	// Prepare the new cron job object

	// prepare cron job meta
	cronJobMeta := metav1.ObjectMeta{
		Name:      kube.CronJobName(),
		Namespace: function.Namespace,
		Labels:    cronJobMetaLabels,
	}

	// prepare pod template labels
	podTemplateLabels := labels.Set{
		"nuclio.io/function-cron-job-pod": "true",
	}
	podTemplateLabels = labels.Merge(podTemplateLabels, functionLabels)
	cronJobSpec.JobTemplate.Spec.Template.Labels = podTemplateLabels

	// this new object will be used both on creation/update
	newCronJob := batchv1beta1.CronJob{
		ObjectMeta: cronJobMeta,
		Spec:       *cronJobSpec,
	}

	createCronJob := func() (interface{}, error) {
		resultCronJob, err := lc.kubeClientSet.BatchV1beta1().
			CronJobs(function.Namespace).
			Create(ctx, &newCronJob, metav1.CreateOptions{})

		return resultCronJob, err
	}

	updateCronJob := func(resource interface{}) (interface{}, error) {
		cronJob := resource.(*batchv1beta1.CronJob)

		// Use the original name of the CronJob
		newCronJob.Name = cronJob.Name

		// set the contents of the cron job pointer to be the updated cron job
		*cronJob = newCronJob

		resultCronJob, err := lc.kubeClientSet.BatchV1beta1().CronJobs(function.Namespace).Update(ctx, cronJob, metav1.UpdateOptions{})
		if err != nil {
			return nil, errors.Wrap(err, "Failed to update cron job")
		}

		return resultCronJob, nil
	}

	resource, err := lc.createOrUpdateResource(ctx,
		"cronJob",
		getCronJob,
		cronJobIsDeleting,
		createCronJob,
		updateCronJob)

	if err != nil {
		return nil, err
	}

	if resource == nil {
		return nil, nil
	}

	return resource.(*batchv1beta1.CronJob), err
}

func (lc *lazyClient) compileCronTriggerLabelSelector(functionName, additionalLabels string) string {
	labelSelector := labels.Set{
		"nuclio.io/component":     "cron-trigger",
		"nuclio.io/function-name": functionName,
	}.String()

	if additionalLabels != "" {
		labelSelector += fmt.Sprintf(",%s", additionalLabels)
	}
	return labelSelector
}

func (lc *lazyClient) compileCronTriggerNotInSliceLabels(slice []string) (string, error) {
	if len(slice) == 0 {
		return "", nil
	}

	labelSet, err := labels.NewRequirement("nuclio.io/function-cron-trigger-name",
		selection.NotIn,
		slice)
	if err != nil {
		return "", errors.Wrap(err, "Failed to create cron trigger list requirement label")
	}
	return labelSet.String(), nil
}

// nginx ingress controller might need a grace period to stabilize after an update, otherwise it might respond with 503
func (lc *lazyClient) waitForNginxIngressToStabilize(ctx context.Context, ingress *networkingv1.Ingress) {
	lc.logger.DebugWithCtx(ctx, "Waiting for nginx ingress to stabilize",
		"nginxIngressUpdateGracePeriod", lc.nginxIngressUpdateGracePeriod,
		"ingressNamespace", ingress.Namespace,
		"ingressName", ingress.Name)

	time.Sleep(lc.nginxIngressUpdateGracePeriod)
	lc.logger.DebugWithCtx(ctx, "Finished waiting for nginx ingress to stabilize",
		"ingressNamespace", ingress.Namespace,
		"ingressName", ingress.Name)
}

func (lc *lazyClient) initClassLabels() {
	lc.classLabels["nuclio.io/class"] = "function"
	lc.classLabels["nuclio.io/app"] = "functionres"
}

func (lc *lazyClient) getFunctionLabels(function *nuclioio.NuclioFunction) labels.Set {
	result := labels.Set{}

	for labelKey, labelValue := range function.Labels {
		result[labelKey] = labelValue
	}

	for labelKey, labelValue := range lc.classLabels {
		result[labelKey] = labelValue
	}

	return result
}

func (lc *lazyClient) getPodAnnotations(function *nuclioio.NuclioFunction) (map[string]string, error) {
	annotations := map[string]string{
		"nuclio.io/image-hash": function.Spec.ImageHash,
	}

	// add annotations for prometheus pull
	if lc.functionsHaveMetricSink(lc.platformConfigurationProvider.GetPlatformConfiguration(), "prometheusPull") {
		annotations["nuclio.io/prometheus_pull"] = "true"
		annotations["nuclio.io/prometheus_pull_port"] = strconv.Itoa(containerMetricPort)
	}

	// add function annotations
	for annotationKey, annotationValue := range function.Annotations {
		annotations[annotationKey] = annotationValue
	}

	return annotations, nil
}

func (lc *lazyClient) getDeploymentAnnotations(function *nuclioio.NuclioFunction) (map[string]string, error) {
	annotations := make(map[string]string)

	if function.Spec.Description != "" {
		annotations["description"] = function.Spec.Description
	}

	serializedFunctionConfigJSON, err := lc.serializeFunctionJSON(function)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get function as JSON")
	}

	var nuclioVersion string

	// get version
	nuclioVersion = version.Get().Label
	if nuclioVersion == "" {
		nuclioVersion = "unknown"
	}
	annotations["nuclio.io/function-config"] = serializedFunctionConfigJSON
	annotations["nuclio.io/controller-version"] = nuclioVersion

	// add function annotations
	for annotationKey, annotationValue := range function.Annotations {
		annotations[annotationKey] = annotationValue
	}

	return annotations, nil
}

func (lc *lazyClient) getFunctionEnvironment(functionLabels labels.Set,
	function *nuclioio.NuclioFunction) []v1.EnvVar {
	env := function.Spec.Env

	env = append(env, v1.EnvVar{Name: "NUCLIO_FUNCTION_NAME", Value: functionLabels["nuclio.io/function-name"]})
	env = append(env, v1.EnvVar{Name: "NUCLIO_FUNCTION_VERSION", Value: functionLabels["nuclio.io/function-version"]})
	env = append(env, v1.EnvVar{
		Name: "NUCLIO_FUNCTION_INSTANCE",
		ValueFrom: &v1.EnvVarSource{
			FieldRef: &v1.ObjectFieldSelector{
				FieldPath: "metadata.name",
			},
		},
	})

	return env
}

func (lc *lazyClient) serializeFunctionJSON(function *nuclioio.NuclioFunction) (string, error) {
	body, err := json.Marshal(function.Spec)
	if err != nil {
		return "", errors.Wrap(err, "Failed to marshal JSON")
	}

	var pbody bytes.Buffer
	err = json.Compact(&pbody, body)
	if err != nil {
		return "", errors.Wrap(err, "Failed to compact JSON")
	}

	return pbody.String(), nil
}

func (lc *lazyClient) populateServiceSpec(ctx context.Context,
	functionLabels labels.Set,
	function *nuclioio.NuclioFunction,
	spec *v1.ServiceSpec) {

	if function.Status.State == functionconfig.FunctionStateScaledToZero ||
		function.Status.State == functionconfig.FunctionStateWaitingForScaleResourcesToZero {

		// pass all further requests to DLX service
		spec.Selector = map[string]string{"nuclio.io/app": "dlx"}
	} else {
		spec.Selector = functionLabels
	}

	spec.Type = functionconfig.ResolveFunctionServiceType(
		&function.Spec,
		lc.platformConfigurationProvider.GetPlatformConfiguration().Kube.DefaultServiceType)
	serviceTypeIsNodePort := spec.Type == v1.ServiceTypeNodePort
	functionHTTPPort := function.Spec.GetHTTPPort()

	// update the service's node port on the following conditions:
	// 1. this is a new service (spec.Ports is an empty list)
	// 2. this is an existing service (spec.Ports is not an empty list) BUT not if the service already has a node port
	//    and the function specifies 0 (meaning auto assign). This is to prevent cases where service already has a node
	//    port and then updating it causes node port change
	// 3. this is an existing service (spec.Ports is not an empty list) and node port was previously configured, but
	//    the trigger type has been updated to ClusterIP(or any other type which isn't NodePort).
	if len(spec.Ports) == 0 ||
		!(spec.Ports[0].NodePort != 0 && function.Spec.GetHTTPPort() == 0) ||
		(spec.Ports[0].NodePort != 0 && !serviceTypeIsNodePort) {

		spec.Ports = []v1.ServicePort{
			{
				Name: ContainerHTTPPortName,
				Port: int32(abstract.FunctionContainerHTTPPort),
			},
		}
		if serviceTypeIsNodePort {
			spec.Ports[0].NodePort = int32(functionHTTPPort)
		} else {
			spec.Ports[0].NodePort = 0
		}
		lc.logger.DebugWithCtx(ctx, "Updating service node port",
			"functionName", function.Name,
			"ports", spec.Ports)
	}

	// check if platform requires additional ports
	platformServicePorts := lc.getServicePortsFromPlatform(lc.platformConfigurationProvider.GetPlatformConfiguration())

	// make sure the ports exist (add if not)
	spec.Ports = lc.ensureServicePortsExist(spec.Ports, platformServicePorts)
}

func (lc *lazyClient) getServicePortsFromPlatform(platformConfiguration *platformconfig.Config) []v1.ServicePort {
	var servicePorts []v1.ServicePort

	if lc.functionsHaveMetricSink(platformConfiguration, "prometheusPull") {
		servicePorts = append(servicePorts, v1.ServicePort{
			Name: containerMetricPortName,
			Port: int32(containerMetricPort),
		})
	}

	return servicePorts
}

func (lc *lazyClient) functionsHaveMetricSink(platformConfiguration *platformconfig.Config, kind string) bool {
	metricSinks, err := platformConfiguration.GetFunctionMetricSinks()
	if err != nil {
		return false
	}

	for _, metricSink := range metricSinks {
		if metricSink.Kind == kind {
			return true
		}
	}

	return false
}

func (lc *lazyClient) functionsHaveAutoScaleMetrics(platformConfiguration *platformconfig.Config) bool {
	autoScaleMetrics := platformConfiguration.AutoScale
	if autoScaleMetrics.MetricName == "" || autoScaleMetrics.TargetValue == "" {
		return false
	}

	return true
}

func (lc *lazyClient) ensureServicePortsExist(to []v1.ServicePort, from []v1.ServicePort) []v1.ServicePort {

	// iterate over from and check that it's in to
	for _, fromServicePort := range from {
		found := false

		for _, toServicePort := range to {
			if toServicePort.Name == fromServicePort.Name {
				found = true
				break
			}
		}

		if !found {
			to = append(to, fromServicePort)
		}
	}

	return to
}

func (lc *lazyClient) getCronTriggerInvocationURL(resources Resources, namespace string) (string, error) {
	functionService, err := resources.Service()
	if err != nil {
		return "", errors.Wrap(err, "Failed to get function service")
	}
	host, port := client.GetDomainNameInvokeURL(functionService.Name, namespace)

	return fmt.Sprintf("%s:%d", host, port), nil
}

func (lc *lazyClient) generateCronTriggerCronJobSpec(functionLabels labels.Set,
	function *nuclioio.NuclioFunction,
	resources Resources,
	cronTrigger functionconfig.Trigger) (*batchv1beta1.CronJobSpec, error) {
	var err error
	one := int32(1)
	spec := batchv1beta1.CronJobSpec{}

	type cronAttributes struct {
		Schedule          string
		Interval          string
		ConcurrencyPolicy string
		JobBackoffLimit   int32
		Event             cron.Event
	}

	// get the attributes from the cron trigger
	var attributes cronAttributes
	if err = mapstructure.Decode(cronTrigger.Attributes, &attributes); err != nil {
		return nil, errors.Wrap(err, "Failed to decode cron trigger attributes")
	}

	// populate schedule
	if attributes.Interval != "" {
		spec.Schedule = fmt.Sprintf("@every %s", attributes.Interval)
	} else {
		spec.Schedule, err = lc.normalizeCronTriggerScheduleInput(attributes.Schedule)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to normalize cron schedule")
		}
	}

	// generate a string containing all of the headers with --header flag as prefix, to be used by curl later
	headersAsCurlArg := ""
	for headerKey := range attributes.Event.Headers {
		headerValue := attributes.Event.GetHeaderString(headerKey)
		headersAsCurlArg = fmt.Sprintf("%s --header \"%s: %s\"", headersAsCurlArg, headerKey, headerValue)
	}

	// add default header
	headersAsCurlArg = fmt.Sprintf("%s --header \"%s: %s\"", headersAsCurlArg, "x-nuclio-invoke-trigger", "cron")

	functionAddress, err := lc.getCronTriggerInvocationURL(resources, function.Namespace)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get cron trigger invocation URL")
	}

	// generate the curl command to be run by the CronJob to invoke the function
	// invoke the function (retry for 10 seconds)
	curlCommand := fmt.Sprintf("curl --silent %s %s --retry 10 --retry-delay 1 --retry-max-time 10 --retry-connrefused",
		headersAsCurlArg,
		functionAddress)

	if attributes.Event.Body != "" {
		eventBody := attributes.Event.Body

		// if a body exists - dump it into a file, and pass this file as argument (done to support JSON body)
		eventBodyFilePath := "/tmp/eventbody.out"
		eventBodyCurlArg := fmt.Sprintf("--data '@%s'", eventBodyFilePath)

		// try compact as JSON (will fail if it's not a valid JSON)
		eventBodyAsCompactedJSON := bytes.NewBuffer([]byte{})
		if err := json.Compact(eventBodyAsCompactedJSON, []byte(eventBody)); err == nil {

			// set the compacted JSON as event body
			eventBody = eventBodyAsCompactedJSON.String()
		}

		curlCommand = fmt.Sprintf("echo %s > %s && %s %s",
			strconv.Quote(eventBody),
			eventBodyFilePath,
			curlCommand,
			eventBodyCurlArg)
	}

	// get cron job retries until failing a job (default=2)
	jobBackoffLimit := attributes.JobBackoffLimit
	if jobBackoffLimit == 0 {
		jobBackoffLimit = 2
	}

	spec.JobTemplate = batchv1beta1.JobTemplateSpec{
		Spec: batchv1.JobSpec{
			BackoffLimit: &jobBackoffLimit,
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:            "function-invocator",
							Image:           common.GetEnvOrDefaultString("NUCLIO_CONTROLLER_CRON_TRIGGER_CRON_JOB_IMAGE_NAME", "appropriate/curl:latest"),
							Args:            []string{"/bin/sh", "-c", curlCommand},
							ImagePullPolicy: v1.PullPolicy(common.GetEnvOrDefaultString("NUCLIO_CONTROLLER_CRON_TRIGGER_CRON_JOB_IMAGE_PULL_POLICY", "IfNotPresent")),
						},
					},
					RestartPolicy: v1.RestartPolicyNever,
				},
			},
		},
	}

	// set concurrency policy if given (default to forbid - to protect the user from overdose of cron jobs)
	concurrencyPolicy := batchv1beta1.ForbidConcurrent
	if attributes.ConcurrencyPolicy != "" {
		concurrencyPolicy = batchv1beta1.ConcurrencyPolicy(strings.Title(attributes.ConcurrencyPolicy))
	}
	spec.ConcurrencyPolicy = concurrencyPolicy

	// set default history limit (no need for more than one - makes kube jobs api clearer)
	spec.SuccessfulJobsHistoryLimit = &one
	spec.FailedJobsHistoryLimit = &one

	return &spec, nil
}

func (lc *lazyClient) normalizeCronTriggerScheduleInput(schedule string) (string, error) {

	splittedSchedule := strings.Split(schedule, " ")

	// if schedule is of length 5, do nothing
	if len(splittedSchedule) == 5 {
		return schedule, nil
	}

	// normalizes cron schedules of length 6 to be of length 5 (removes the seconds slot)
	if len(splittedSchedule) != 6 {
		return "", errors.New(fmt.Sprintf("Unexpected cron schedule syntax: %s. (expects standard UNIX cron schedule)", schedule))
	}

	return strings.Join(splittedSchedule[1:6], " "), nil
}

func (lc *lazyClient) populateIngressConfig(ctx context.Context,
	functionLabels labels.Set,
	function *nuclioio.NuclioFunction,
	meta *metav1.ObjectMeta,
	spec *networkingv1.IngressSpec) error {
	meta.Annotations = make(map[string]string)

	platformConfig := lc.platformConfigurationProvider.GetPlatformConfiguration()

	// get the first HTTP trigger and look for annotations that we shove to the ingress
	// there should only be 0 or 1. if there are more, just take the first
	for _, httpTrigger := range functionconfig.GetTriggersByKind(function.Spec.Triggers, "http") {

		// set annotations
		if httpTrigger.Annotations != nil {
			meta.Annotations = httpTrigger.Annotations
		}

		// ignore any other http triggers, validation should catch that
		break
	}

	// set nuclio target header on ingress
	meta.Annotations["nginx.ingress.kubernetes.io/configuration-snippet"] = fmt.Sprintf(
		`proxy_set_header X-Nuclio-Target "%s";`, function.Name)

	// Check if function is a scale to zero candidate
	//			is not disabled
	//			is not in imported state
	//			has minimum replicas == 0
	//			has maximum replicas >  0
	if !function.Spec.Disable &&
		function.Status.State != functionconfig.FunctionStateImported &&
		function.GetComputedMinReplicas() == 0 &&
		function.GetComputedMaxReplicas() > 0 {
		platformConfiguration := lc.platformConfigurationProvider.GetPlatformConfiguration()

		// enrich if not exists
		for key, value := range platformConfiguration.ScaleToZero.HTTPTriggerIngressAnnotations {
			if _, ok := meta.Annotations[key]; !ok {
				meta.Annotations[key] = value
			}
		}
	}

	// enrich with default ingress annotations
	for key, value := range platformConfig.Kube.DefaultHTTPIngressAnnotations {

		// selectively take only undefined annotations
		if _, found := meta.Annotations[key]; !found {
			meta.Annotations[key] = value
		}
	}

	// clear out existing so that we don't keep adding rules
	spec.Rules = []networkingv1.IngressRule{}
	spec.TLS = []networkingv1.IngressTLS{}

	ingresses := functionconfig.GetFunctionIngresses(client.NuclioioToFunctionConfig(function))
	for _, ingress := range ingresses {
		if err := lc.addIngressToSpec(ctx, &ingress, functionLabels, function, spec); err != nil {
			return errors.Wrap(err, "Failed to add ingress to spec")
		}
	}
	return nil
}

func (lc *lazyClient) formatIngressPattern(ingressPattern string,
	functionLabels labels.Set,
	function *nuclioio.NuclioFunction) (string, error) {

	if !strings.HasPrefix(ingressPattern, "/") {
		ingressPattern = "/" + ingressPattern
	}

	parsedTemplate, err := template.New("test").Parse(ingressPattern)
	if err != nil {
		return "", errors.Wrap(err, "Failed to parse template")
	}

	ingressPatternBuffer := bytes.Buffer{}

	templateVars := struct {
		Name      string
		Namespace string
		Version   string
	}{
		Name:      function.Name,
		Namespace: function.Namespace,
		Version:   functionLabels["nuclio.io/function-version"],
	}

	if err := parsedTemplate.Execute(&ingressPatternBuffer, templateVars); err != nil {
		return "", errors.Wrap(err, "Failed to execute parsed template")
	}

	return ingressPatternBuffer.String(), nil
}

func (lc *lazyClient) addIngressToSpec(ctx context.Context,
	ingress *functionconfig.Ingress,
	functionLabels labels.Set,
	function *nuclioio.NuclioFunction,
	spec *networkingv1.IngressSpec) error {

	lc.logger.DebugWithCtx(ctx,
		"Adding ingress",
		"functionName", function.Name,
		"ingressName", kube.IngressNameFromFunctionName(function.Name),
		"labels", functionLabels,
		"host", ingress.Host,
		"paths", ingress.Paths,
		"TLS", ingress.TLS)

	ingressRule := networkingv1.IngressRule{
		Host: ingress.Host,
	}

	ingressRule.IngressRuleValue.HTTP = &networkingv1.HTTPIngressRuleValue{}

	// populate the ingress rule value
	for _, path := range ingress.Paths {
		formattedPath, err := lc.formatIngressPattern(path, functionLabels, function)
		if err != nil {
			return errors.Wrap(err, "Failed to format ingress pattern")
		}

		// TODO: make pathType configurable - via function (or trigger) annotation or platform configuration
		if ingress.PathType == "" {
			ingress.PathType = networkingv1.PathTypeImplementationSpecific
		}

		httpIngressPath := networkingv1.HTTPIngressPath{
			Path:     formattedPath,
			PathType: &ingress.PathType,
			Backend: networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: kube.ServiceNameFromFunctionName(function.Name),
					Port: networkingv1.ServiceBackendPort{
						Name: ContainerHTTPPortName,
					},
				},
			},
		}

		// add path
		ingressRule.IngressRuleValue.HTTP.Paths = append(ingressRule.IngressRuleValue.HTTP.Paths, httpIngressPath)

		// add TLS if such exists
		if ingress.TLS.SecretName != "" {
			ingressTLS := networkingv1.IngressTLS{}
			ingressTLS.SecretName = ingress.TLS.SecretName
			ingressTLS.Hosts = ingress.TLS.Hosts

			spec.TLS = append(spec.TLS, ingressTLS)
		}
	}

	spec.Rules = append(spec.Rules, ingressRule)

	return nil
}

func (lc *lazyClient) populateDeploymentContainer(functionLabels labels.Set,
	function *nuclioio.NuclioFunction,
	container *v1.Container) {

	container.Image = function.Spec.Image
	container.Resources = function.Spec.Resources
	if container.Resources.Requests == nil {
		container.Resources.Requests = make(v1.ResourceList)

		// the default is 500 milli cpu
		cpuQuantity, err := apiresource.ParseQuantity("25m") // nolint: errcheck
		if err == nil {
			container.Resources.Requests["cpu"] = cpuQuantity
		}
	}
	container.Env = lc.getFunctionEnvironment(functionLabels, function)
	container.Ports = []v1.ContainerPort{
		{
			Name:          ContainerHTTPPortName,
			ContainerPort: abstract.FunctionContainerHTTPPort,
			Protocol:      "TCP",
		},
	}

	// iterate through metric sinks. if prometheus pull is configured, add containerMetricPort
	if lc.functionsHaveMetricSink(lc.platformConfigurationProvider.GetPlatformConfiguration(), "prometheusPull") {
		container.Ports = append(container.Ports, v1.ContainerPort{
			Name:          containerMetricPortName,
			ContainerPort: containerMetricPort,
			Protocol:      "TCP",
		})
	}

	container.ReadinessProbe = &v1.Probe{
		Handler: v1.Handler{
			HTTPGet: &v1.HTTPGetAction{
				Port: intstr.FromInt(abstract.FunctionContainerHTTPPort),
				Path: http.InternalHealthPath,
			},
		},
		InitialDelaySeconds: 1,
		TimeoutSeconds:      1,
		PeriodSeconds:       1,
	}

	container.LivenessProbe = &v1.Probe{
		Handler: v1.Handler{
			HTTPGet: &v1.HTTPGetAction{
				Port: intstr.FromInt(abstract.FunctionContainerHealthCheckHTTPPort),
				Path: "/live",
			},
		},
		InitialDelaySeconds: 10,
		TimeoutSeconds:      3,
		PeriodSeconds:       5,
	}

	// always pull is the default since each create / update will trigger a rollingupdate including
	// pulling the image. this is because the tag of the image doesn't change between revisions of the function
	if function.Spec.ImagePullPolicy == "" {
		container.ImagePullPolicy = v1.PullAlways
	} else {
		container.ImagePullPolicy = function.Spec.ImagePullPolicy
	}
}

func (lc *lazyClient) populateConfigMap(functionLabels labels.Set,
	function *nuclioio.NuclioFunction,
	configMap *v1.ConfigMap) error {

	// create a processor configMap writer
	// TODO: abstract this so that controller isn't bound to a processor?
	configWriter, err := processorconfig.NewWriter()
	if err != nil {
		return errors.Wrap(err, "Failed to create processor configuration writer")
	}

	// create configMap contents - generate a processor configuration based on the function CR
	configMapContents := bytes.Buffer{}

	if err := configWriter.Write(&configMapContents, &processor.Configuration{
		Config: functionconfig.Config{
			Meta: functionconfig.Meta{
				Name:        function.Name,
				Namespace:   function.Namespace,
				Labels:      functionLabels,
				Annotations: function.Annotations,
			},
			Spec: function.Spec,
		},
	}); err != nil {

		return errors.Wrap(err, "Failed to write configuration")
	}

	*configMap = v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kube.ConfigMapNameFromFunctionName(function.Name),
			Namespace: function.Namespace,
		},
		Data: map[string]string{
			"processor.yaml": configMapContents.String(),
		},
	}

	return nil
}

func (lc *lazyClient) getFunctionVolumeAndMounts(ctx context.Context,
	function *nuclioio.NuclioFunction) ([]v1.Volume, []v1.VolumeMount) {
	trueVal := true
	var configVolumes []functionconfig.Volume
	var filteredFunctionVolumes []functionconfig.Volume

	processorConfigVolumeName := "processor-config-volume"
	platformConfigVolumeName := "platform-config-volume"

	// processor configuration
	processorConfigVolume := functionconfig.Volume{}
	processorConfigVolume.Volume.Name = processorConfigVolumeName
	processorConfigMapVolumeSource := v1.ConfigMapVolumeSource{}
	processorConfigMapVolumeSource.Name = kube.ConfigMapNameFromFunctionName(function.Name)
	processorConfigVolume.Volume.ConfigMap = &processorConfigMapVolumeSource
	processorConfigVolume.VolumeMount.Name = processorConfigVolumeName
	processorConfigVolume.VolumeMount.MountPath = "/etc/nuclio/config/processor"

	// platform configuration
	platformConfigVolume := functionconfig.Volume{}
	platformConfigVolume.Volume.Name = platformConfigVolumeName
	platformConfigMapVolumeSource := v1.ConfigMapVolumeSource{}
	platformConfigMapVolumeSource.Name = lc.platformConfigurationProvider.GetPlatformConfigurationName()
	platformConfigMapVolumeSource.Optional = &trueVal
	platformConfigVolume.Volume.ConfigMap = &platformConfigMapVolumeSource
	platformConfigVolume.VolumeMount.Name = platformConfigVolumeName
	platformConfigVolume.VolumeMount.MountPath = "/etc/nuclio/config/platform"

	// ignore HostPath volumes
	for _, configVolume := range function.Spec.Volumes {
		if configVolume.Volume.HostPath != nil {
			lc.logger.WarnWithCtx(ctx, "Ignoring volume. HostPath volumes are now deprecated",
				"configVolume",
				configVolume)

		} else {
			filteredFunctionVolumes = append(filteredFunctionVolumes, configVolume)
		}
	}
	function.Spec.Volumes = filteredFunctionVolumes

	// merge from functionconfig and injected configuration
	configVolumes = append(configVolumes, function.Spec.Volumes...)
	configVolumes = append(configVolumes, processorConfigVolume)
	configVolumes = append(configVolumes, platformConfigVolume)

	var volumes []v1.Volume
	var volumeMounts []v1.VolumeMount

	// using maps to eliminate duplicates where user use the same volume to be mounted by multiple volume mounts
	// e.g.: volume config-map X, mount it twice to different paths
	volumeNameToVolume := map[string]v1.Volume{}
	volumeNameToVolumeMounts := map[string][]v1.VolumeMount{}

	for _, configVolume := range configVolumes {
		if configVolume.Volume.FlexVolume != nil && configVolume.Volume.FlexVolume.Driver == "v3io/fuse" {

			// make sure the given sub path matches the needed structure. fix in case it doesn't
			subPath, subPathExists := configVolume.Volume.FlexVolume.Options["subPath"]
			if subPathExists && len(subPath) != 0 {

				// insert slash in the beginning in case it wasn't given (example: "my/path" -> "/my/path")
				if !filepath.IsAbs(subPath) {
					subPath = "/" + subPath
				}

				subPath = filepath.Clean(subPath)
				if subPath == "/" {
					subPath = ""
				}

				configVolume.Volume.FlexVolume.Options["subPath"] = subPath
			}
		}

		lc.logger.DebugWithCtx(ctx, "Adding volume",
			"configVolume", configVolume,
			"functionName", function.Name)

		// volume name is unique per its volume instance
		volumeNameToVolume[configVolume.Volume.Name] = configVolume.Volume

		// same volume name can be shared by n volume mounts
		volumeNameToVolumeMounts[configVolume.Volume.Name] = append(volumeNameToVolumeMounts[configVolume.Volume.Name],
			configVolume.VolumeMount)
	}

	for _, volume := range volumeNameToVolume {
		volumes = append(volumes, volume)
	}

	for _, volumeVolumeMounts := range volumeNameToVolumeMounts {
		volumeMounts = append(volumeMounts, volumeVolumeMounts...)
	}

	// kubernetes is sensitive to list order.
	// avoid deployment from being re-applied by order volumes and volume mounts by name
	sort.Slice(volumes, func(i, j int) bool {
		return volumes[i].Name < volumes[j].Name
	})
	sort.Slice(volumeMounts, func(i, j int) bool {
		return volumeMounts[i].Name < volumeMounts[j].Name
	})

	// flatten and return as list of instances
	return volumes, volumeMounts
}

func (lc *lazyClient) deleteFunctionEvents(ctx context.Context, functionName string, namespace string) error {

	// create error group
	errGroup, _ := errgroup.WithContext(ctx, lc.logger)

	listOptions := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("nuclio.io/function-name=%s", functionName),
	}

	result, err := lc.nuclioClientSet.NuclioV1beta1().NuclioFunctionEvents(namespace).List(ctx, listOptions)
	if err != nil {
		return errors.Wrap(err, "Failed to list function events")
	}

	lc.logger.DebugWithCtx(ctx, "Got function events", "num", len(result.Items))

	for _, functionEvent := range result.Items {
		functionEvent := functionEvent
		errGroup.Go("DeleteEvents", func() error {
			err = lc.nuclioClientSet.NuclioV1beta1().
				NuclioFunctionEvents(namespace).
				Delete(ctx, functionEvent.Name, metav1.DeleteOptions{})
			if err != nil {
				return errors.Wrap(err, "Failed to delete function event")
			}
			return nil
		})
	}

	// wait for all errgroup goroutines
	if err := errGroup.Wait(); err != nil {
		return errors.Wrap(err, "Failed to delete function events")
	}

	return nil
}

func (lc *lazyClient) GetFunctionMetricSpecs(function *nuclioio.NuclioFunction) ([]autosv2.MetricSpec, error) {
	if function.Spec.CustomScalingMetricSpecs != nil {
		return function.Spec.CustomScalingMetricSpecs, nil
	}

	targetCPU := int32(function.Spec.TargetCPU)
	if targetCPU == 0 {
		targetCPU = abstract.DefaultTargetCPU
	}

	var metricSpecs []autosv2.MetricSpec
	platformConfig := lc.platformConfigurationProvider.GetPlatformConfiguration()
	if lc.functionsHaveAutoScaleMetrics(platformConfig) {
		targetValue, err := apiresource.ParseQuantity(platformConfig.AutoScale.TargetValue)
		if err != nil {
			return metricSpecs, errors.Wrap(err, "Failed to parse target value for auto scale")
		}

		// special cases for k8s resources that are supplied by regular metric server, excluding cpu
		if lc.getMetricResourceByName(platformConfig.AutoScale.MetricName) != "" {
			metricSpecs = []autosv2.MetricSpec{
				{
					Type: "Resource",
					Resource: &autosv2.ResourceMetricSource{
						Name:               lc.getMetricResourceByName(platformConfig.AutoScale.MetricName),
						TargetAverageValue: &targetValue,
					},
				},
			}
		} else {
			metricSpecs = []autosv2.MetricSpec{
				{
					Type: "Pods",
					Pods: &autosv2.PodsMetricSource{
						MetricName:         platformConfig.AutoScale.MetricName,
						TargetAverageValue: targetValue,
					},
				},
			}
		}

		// a bug/unexpected feature in hpa doesn't allow for both custom metrics and resource metrics
	} else {

		// special case, keep support for target cpu in percentage
		metricSpecs = append(metricSpecs, autosv2.MetricSpec{
			Type: "Resource",
			Resource: &autosv2.ResourceMetricSource{
				Name:                     v1.ResourceCPU,
				TargetAverageUtilization: &targetCPU,
			},
		})
	}

	return metricSpecs, nil
}

func (lc *lazyClient) getMetricResourceByName(resourceName string) v1.ResourceName {
	switch resourceName {
	case "memory":
		return v1.ResourceMemory
	case "alpha.kubernetes.io/nvidia-gpu":
		return v1.ResourceName(resourceName)
	case functionconfig.NvidiaGPUResourceName:
		return v1.ResourceName(resourceName)
	case "ephemeral-storage":
		return v1.ResourceEphemeralStorage
	case "storage":
		return v1.ResourceStorage
	default:
		return ""
	}
}

func (lc *lazyClient) resolveFailFast(ctx context.Context,
	podsList *v1.PodList,
	functionResourcesCreateOrUpdateTimestamp time.Time) error {

	var pods []v1.Pod
	for _, pod := range podsList.Items {

		// skip irrelevant pods (leftovers of previous function deployments)
		// (subtract 2 seconds from create/update timestamp because of ms accuracy loss of pod.creationTimestamp)
		if !pod.GetCreationTimestamp().After(functionResourcesCreateOrUpdateTimestamp.Add(-2 * time.Second)) {
			continue
		}
		pods = append(pods, pod)
	}

	// infer from the pod statuses if the function deployment had failed
	// failure of one pod is enough to tell that the deployment had failed
	for _, pod := range pods {

		for _, containerStatus := range pod.Status.ContainerStatuses {

			if pod.Status.ContainerStatuses[0].State.Waiting != nil {

				// check if the pod is on a crashLoopBackoff
				if containerStatus.State.Waiting.Reason == "CrashLoopBackOff" {

					return errors.Errorf("NuclioFunction pod (%s) is in a crash loop", pod.Name)
				}
			}
		}
	}

	// check each pod's conditions to determine if there is at least one unschedulable pod
	errGroup, errGroupCtx := errgroup.WithContext(ctx, lc.logger)
	lock := sync.Mutex{}
	scaleUpOccurred := false
	for _, pod := range pods {
		pod := pod
		for _, condition := range pod.Status.Conditions {

			// check if the pod is in pending state, and the reason is that it is unschedulable
			// (meaning no k8s node can currently run it, because of insufficient resources etc..)
			if pod.Status.Phase == v1.PodPending && condition.Reason == "Unschedulable" {

				errGroup.Go("WaitAndCheckAutoScaleEvents", func() error {

					lc.logger.DebugWithCtx(errGroupCtx,
						"Waiting for autoscale evaluation",
						"nodeScaleUpSleepTimeout", lc.nodeScaleUpSleepTimeout,
						"podName", pod.Name)
					time.Sleep(lc.nodeScaleUpSleepTimeout)

					// check if the pod is unschedulable due to scaling up
					triggeredScaleUp, err := lc.isPodAutoScaledUp(errGroupCtx, pod)
					if err != nil {

						// log the error and keep waiting for deployment
						lc.logger.WarnWithCtx(errGroupCtx,
							"Failed to resolve pod autoscaling",
							"podName", pod.Name,
							"err", errors.RootCause(err).Error())
						return nil
					}
					if !triggeredScaleUp {
						return errors.Errorf("NuclioFunction pod (%s) is unschedulable", pod.Name)
					}
					lock.Lock()
					scaleUpOccurred = true
					lock.Unlock()
					return nil
				})
			}
		}
	}
	if err := errGroup.Wait(); err != nil {
		return errors.Wrap(err, "Failed to verify at least one pod schedulability")
	}
	if scaleUpOccurred {
		lc.logger.DebugWithCtx(ctx, "Pod triggered a scale up. Still waiting for deployment to be available")
	}
	return nil
}

func (lc *lazyClient) isPodAutoScaledUp(ctx context.Context, pod v1.Pod) (bool, error) {

	// get pod events to check if pod triggered auto scale
	podEvents, err := lc.kubeClientSet.CoreV1().Events(pod.Namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s", pod.Name),
	})
	if err != nil {
		return false, errors.Wrap(err, "Failed to list pod events")
	}
	lc.logger.DebugWithCtx(ctx, "Received pod events", "podEventsLength", len(podEvents.Items))

	for _, event := range podEvents.Items {

		if event.Source.Component == "cluster-autoscaler" {

			// check autoscaler event reasons according to:
			// https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/FAQ.md#what-events-are-emitted-by-ca
			switch event.Reason {
			case "TriggeredScaleUp":
				lc.logger.InfoWithCtx(ctx,
					"Nuclio function pod has triggered a node scale up",
					"podName", pod.Name)
				return true, nil
			case "NotTriggerScaleUp":
				lc.logger.DebugWithCtx(ctx,
					"Couldn't find node group that can be scaled up to make this pod schedulable",
					"podName", pod.Name)
				return false, nil
			case "ScaleDown":
				lc.logger.DebugWithCtx(ctx,
					"Pod is evicted as part of scale down",
					"podName", pod.Name)
				return false, nil
			}
		}
	}
	return false, nil
}

//
// Resources
//

type lazyResources struct {
	deployment              *appsv1.Deployment
	configMap               *v1.ConfigMap
	service                 *v1.Service
	horizontalPodAutoscaler *autosv2.HorizontalPodAutoscaler
	ingress                 *networkingv1.Ingress
	cronJobs                []*batchv1beta1.CronJob
}

// Deployment returns the deployment
func (lr *lazyResources) Deployment() (*appsv1.Deployment, error) {
	return lr.deployment, nil
}

// ConfigMap returns the configmap
func (lr *lazyResources) ConfigMap() (*v1.ConfigMap, error) {
	return lr.configMap, nil
}

// Service returns the service
func (lr *lazyResources) Service() (*v1.Service, error) {
	return lr.service, nil
}

// HorizontalPodAutoscaler returns the hpa
func (lr *lazyResources) HorizontalPodAutoscaler() (*autosv2.HorizontalPodAutoscaler, error) {
	return lr.horizontalPodAutoscaler, nil
}

// Ingress returns the ingress
func (lr *lazyResources) Ingress() (*networkingv1.Ingress, error) {
	return lr.ingress, nil
}

// CronJobs returns the cron job
func (lr *lazyResources) CronJobs() ([]*batchv1beta1.CronJob, error) {
	return lr.cronJobs, nil
}
