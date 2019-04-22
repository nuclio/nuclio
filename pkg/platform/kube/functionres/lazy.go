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
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	nuclioio_client "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/config"
	"github.com/nuclio/nuclio/pkg/version"

	"github.com/ghodss/yaml"
	"github.com/nuclio/logger"
	"golang.org/x/sync/errgroup"
	apps_v1beta1 "k8s.io/api/apps/v1beta1"
	autos_v2 "k8s.io/api/autoscaling/v2beta1"
	"k8s.io/api/core/v1"
	ext_v1beta1 "k8s.io/api/extensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

const (
	containerHTTPPort       = 8080
	containerHTTPPortName   = "http"
	containerMetricPort     = 8090
	containerMetricPortName = "metrics"
)

//
// Client
//

type lazyClient struct {
	logger                        logger.Logger
	kubeClientSet                 kubernetes.Interface
	nuclioClientSet               nuclioio_client.Interface
	classLabels                   labels.Set
	platformConfigurationProvider PlatformConfigurationProvider
}

func NewLazyClient(parentLogger logger.Logger,
	kubeClientSet kubernetes.Interface,
	nuclioClientSet nuclioio_client.Interface) (Client, error) {

	newClient := lazyClient{
		logger:          parentLogger.GetChild("functionres"),
		kubeClientSet:   kubeClientSet,
		nuclioClientSet: nuclioClientSet,
		classLabels:     make(labels.Set),
	}

	newClient.initClassLabels()

	return &newClient, nil
}

func (lc *lazyClient) List(ctx context.Context, namespace string) ([]Resources, error) {
	listOptions := meta_v1.ListOptions{
		LabelSelector: "nuclio.io/class=function",
	}

	result, err := lc.kubeClientSet.AppsV1beta1().Deployments(namespace).List(listOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to list deployments")
	}

	lc.logger.DebugWith("Got deployments", "num", len(result.Items))

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
	var result *apps_v1beta1.Deployment

	result, err := lc.kubeClientSet.AppsV1beta1().Deployments(namespace).Get(name, meta_v1.GetOptions{})
	lc.logger.DebugWith("Got deployment",
		"namespace", namespace,
		"name", name,
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

func (lc *lazyClient) CreateOrUpdate(ctx context.Context, function *nuclioio.NuclioFunction, imagePullSecrets string) (Resources, error) {
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

		selector, err := meta_v1.LabelSelectorAsSelector(&augmentedConfig.LabelSelector)
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

	// set a default
	if function.Spec.ServiceType == v1.ServiceType("") {
		function.Spec.ServiceType = v1.ServiceTypeNodePort
	}

	// create or update the applicable configMap
	resources.configMap, err = lc.createOrUpdateConfigMap(function)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create/update configMap")
	}

	// create or update the applicable service
	resources.service, err = lc.createOrUpdateService(functionLabels, function)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create/update service")
	}

	// create or update the applicable deployment
	resources.deployment, err = lc.createOrUpdateDeployment(functionLabels, imagePullSecrets, function)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create/update deployment")
	}

	// create or update the HPA
	resources.horizontalPodAutoscaler, err = lc.createOrUpdateHorizontalPodAutoscaler(functionLabels, function)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create/update HPA")
	}

	// create or update ingress
	resources.ingress, err = lc.createOrUpdateIngress(functionLabels, function)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create/update ingress")
	}

	lc.logger.Debug("Deployment created/updated")

	return &resources, nil
}

func (lc *lazyClient) WaitAvailable(ctx context.Context, namespace string, name string) error {
	lc.logger.DebugWith("Waiting for deployment to be available", "namespace", namespace, "name", name)

	waitMs := 250

	for {

		// wait a bit
		time.Sleep(time.Duration(waitMs) * time.Millisecond)

		// expenentially wait more next time, up to 2 seconds
		waitMs *= 2
		if waitMs > 2000 {
			waitMs = 2000
		}

		// check if context is still OK
		if err := ctx.Err(); err != nil {
			return err
		}

		// get the deployment. if it doesn't exist yet, retry a bit later
		result, err := lc.kubeClientSet.AppsV1beta1().Deployments(namespace).Get(name, meta_v1.GetOptions{})
		if err != nil {
			continue
		}

		// find the condition whose type is Available - that's the one we want to examine
		for _, deploymentCondition := range result.Status.Conditions {

			// when we find the right condition, check its Status to see if it's true.
			// a DeploymentCondition whose Type == Available and Status == True means the deployment is available
			if deploymentCondition.Type == apps_v1beta1.DeploymentAvailable {
				available := deploymentCondition.Status == v1.ConditionTrue

				if available && result.Status.UnavailableReplicas == 0 {
					lc.logger.DebugWith("Deployment is available", "reason", deploymentCondition.Reason)
					return nil
				}

				lc.logger.DebugWith("Deployment not available yet",
					"reason", deploymentCondition.Reason,
					"unavailableReplicas", result.Status.UnavailableReplicas)

				// we found the condition, wasn't available
				break
			}
		}
	}
}

func (lc *lazyClient) Delete(ctx context.Context, namespace string, name string) error {
	propogationPolicy := meta_v1.DeletePropagationForeground
	deleteOptions := &meta_v1.DeleteOptions{
		PropagationPolicy: &propogationPolicy,
	}

	// Delete ingress
	err := lc.kubeClientSet.ExtensionsV1beta1().Ingresses(namespace).Delete(name, deleteOptions)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrap(err, "Failed to delete ingress")
		}
	} else {
		lc.logger.DebugWith("Deleted ingress", "namespace", namespace, "name", name)
	}

	// Delete HPA if exists
	err = lc.kubeClientSet.AutoscalingV2beta1().HorizontalPodAutoscalers(namespace).Delete(name, deleteOptions)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrap(err, "Failed to delete HPA")
		}
	} else {
		lc.logger.DebugWith("Deleted HPA", "namespace", namespace, "name", name)
	}

	// Delete Service if exists
	err = lc.kubeClientSet.CoreV1().Services(namespace).Delete(name, deleteOptions)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrap(err, "Failed to delete service")
		}
	} else {
		lc.logger.DebugWith("Deleted service", "namespace", namespace, "name", name)
	}

	// Delete Deployment if exists
	err = lc.kubeClientSet.AppsV1beta1().Deployments(namespace).Delete(name, deleteOptions)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrap(err, "Failed to delete deployment")
		}
	} else {
		lc.logger.DebugWith("Deleted deployment", "namespace", namespace, "name", name)
	}

	// Delete configMap if exists
	err = lc.kubeClientSet.CoreV1().ConfigMaps(namespace).Delete(name, deleteOptions)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrap(err, "Failed to delete configMap")
		}
	} else {
		lc.logger.DebugWith("Deleted configMap", "namespace", namespace, "name", name)
	}

	err = lc.deleteFunctionEvents(ctx, name, namespace)
	if err != nil {
		return errors.Wrap(err, "Failed to delete function events")
	}

	lc.logger.DebugWith("Deleted deployed function", "namespace", namespace, "name", name)

	return nil
}

// SetPlatformConfigurationProvider sets the provider of the platform configuration for any future access
func (lc *lazyClient) SetPlatformConfigurationProvider(platformConfigurationProvider PlatformConfigurationProvider) {
	lc.platformConfigurationProvider = platformConfigurationProvider
}

// as a closure so resourceExists can update
func (lc *lazyClient) createOrUpdateResource(resourceName string,
	getResource func() (interface{}, error),
	resourceIsDeleting func(interface{}) bool,
	createResource func() (interface{}, error),
	updateResource func(interface{}) (interface{}, error)) (interface{}, error) {

	var resource interface{}
	var err error

	deadline := time.Now().Add(1 * time.Minute)

	// get the resource until it's not deleting
	for {

		// get resource will return the resource
		resource, err = getResource()

		// if the resource is deleting, wait for it to complete deleting
		if err == nil && resourceIsDeleting(resource) {
			lc.logger.DebugWith("Resource is deleting, waiting", "name", resourceName)

			// we need to wait a bit and try again
			time.Sleep(1 * time.Second)

			// if we passed the deadline
			if time.Now().After(deadline) {
				return nil, errors.New("Timed out waiting for service to delete")
			}

		} else {

			// there was either an error or the resource exists and is not being deleted
			break
		}
	}

	// if there's an error
	if err != nil {

		// if there was an error and it wasn't not found - there was an error. bail
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, errors.Wrapf(err, "Failed to get resource")
		}

		// create the resource
		resource, err = createResource()

		if err != nil {
			return nil, errors.Wrap(err, "Failed to create resource")
		}

		return resource, nil
	}

	resource, err = updateResource(resource)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to update resource")
	}

	lc.logger.DebugWith("Resource updated")

	return resource, nil
}

func (lc *lazyClient) createOrUpdateConfigMap(function *nuclioio.NuclioFunction) (*v1.ConfigMap, error) {

	getConfigMap := func() (interface{}, error) {
		return lc.kubeClientSet.CoreV1().ConfigMaps(function.Namespace).Get(lc.configMapNameFromFunctionName(function.Name),
			meta_v1.GetOptions{})
	}

	configMapIsDeleting := func(resource interface{}) bool {
		return (resource).(*v1.ConfigMap).ObjectMeta.DeletionTimestamp != nil
	}

	createConfigMap := func() (interface{}, error) {
		configMap := v1.ConfigMap{}
		if err := lc.populateConfigMap(nil, function, &configMap); err != nil {
			return nil, errors.Wrap(err, "Failed to populate configMap")
		}

		return lc.kubeClientSet.CoreV1().ConfigMaps(function.Namespace).Create(&configMap)
	}

	updateConfigMap := func(resource interface{}) (interface{}, error) {
		configMap := resource.(*v1.ConfigMap)

		// update existing
		if err := lc.populateConfigMap(nil, function, configMap); err != nil {
			return nil, errors.Wrap(err, "Failed to populate configMap")
		}

		return lc.kubeClientSet.CoreV1().ConfigMaps(function.Namespace).Update(configMap)
	}

	resource, err := lc.createOrUpdateResource("configMap",
		getConfigMap,
		configMapIsDeleting,
		createConfigMap,
		updateConfigMap)

	if err != nil {
		return nil, err
	}

	return resource.(*v1.ConfigMap), err
}

func (lc *lazyClient) createOrUpdateService(functionLabels labels.Set,
	function *nuclioio.NuclioFunction) (*v1.Service, error) {

	getService := func() (interface{}, error) {
		return lc.kubeClientSet.CoreV1().Services(function.Namespace).Get(function.Name, meta_v1.GetOptions{})
	}

	serviceIsDeleting := func(resource interface{}) bool {
		return (resource).(*v1.Service).ObjectMeta.DeletionTimestamp != nil
	}

	createService := func() (interface{}, error) {
		spec := v1.ServiceSpec{}
		lc.populateServiceSpec(functionLabels, function, &spec)

		return lc.kubeClientSet.CoreV1().Services(function.Namespace).Create(&v1.Service{
			ObjectMeta: meta_v1.ObjectMeta{
				Name:      function.Name,
				Namespace: function.Namespace,
				Labels:    functionLabels,
			},
			Spec: spec,
		})
	}

	updateService := func(resource interface{}) (interface{}, error) {
		service := resource.(*v1.Service)

		// update existing
		service.Labels = functionLabels
		lc.populateServiceSpec(functionLabels, function, &service.Spec)

		return lc.kubeClientSet.CoreV1().Services(function.Namespace).Update(service)
	}

	resource, err := lc.createOrUpdateResource("service",
		getService,
		serviceIsDeleting,
		createService,
		updateService)

	if err != nil {
		return nil, err
	}

	return resource.(*v1.Service), err
}

func (lc *lazyClient) createOrUpdateDeployment(functionLabels labels.Set,
	imagePullSecrets string,
	function *nuclioio.NuclioFunction) (*apps_v1beta1.Deployment, error) {

	// to make sure the pod re-pulls the image, we need to specify a unique string here
	podAnnotations, err := lc.getPodAnnotations(function)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get pod annotations")
	}

	replicas := int32(lc.getFunctionReplicas(function))
	lc.logger.DebugWith("Got replicas", "replicas", replicas)
	deploymentAnnotations, err := lc.getDeploymentAnnotations(function)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get function annotations")
	}

	// get volumes and volumeMounts from configuration
	volumes, volumeMounts := lc.getFunctionVolumeAndMounts(function)

	getDeployment := func() (interface{}, error) {
		return lc.kubeClientSet.AppsV1beta1().Deployments(function.Namespace).Get(function.Name, meta_v1.GetOptions{})
	}

	deploymentIsDeleting := func(resource interface{}) bool {
		return (resource).(*apps_v1beta1.Deployment).ObjectMeta.DeletionTimestamp != nil
	}

	createDeployment := func() (interface{}, error) {
		container := v1.Container{Name: "nuclio"}

		lc.populateDeploymentContainer(functionLabels, function, &container)
		container.VolumeMounts = volumeMounts

		return lc.kubeClientSet.AppsV1beta1().Deployments(function.Namespace).Create(&apps_v1beta1.Deployment{

			ObjectMeta: meta_v1.ObjectMeta{
				Name:        function.Name,
				Namespace:   function.Namespace,
				Labels:      functionLabels,
				Annotations: deploymentAnnotations,
			},
			Spec: apps_v1beta1.DeploymentSpec{
				Replicas: &replicas,
				Template: v1.PodTemplateSpec{
					ObjectMeta: meta_v1.ObjectMeta{
						Name:        function.Name,
						Namespace:   function.Namespace,
						Labels:      functionLabels,
						Annotations: podAnnotations,
					},
					Spec: v1.PodSpec{
						ImagePullSecrets: []v1.LocalObjectReference{
							{Name: imagePullSecrets},
						},
						Containers: []v1.Container{
							container,
						},
						Volumes: volumes,
					},
				},
			},
		})
	}

	updateDeployment := func(resource interface{}) (interface{}, error) {
		deployment := resource.(*apps_v1beta1.Deployment)

		deployment.Labels = functionLabels
		deployment.Annotations = deploymentAnnotations
		deployment.Spec.Replicas = &replicas
		deployment.Spec.Template.Annotations = podAnnotations
		deployment.Spec.Template.Labels = functionLabels
		lc.populateDeploymentContainer(functionLabels, function, &deployment.Spec.Template.Spec.Containers[0])
		deployment.Spec.Template.Spec.Volumes = volumes
		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts

		return lc.kubeClientSet.AppsV1beta1().Deployments(function.Namespace).Update(deployment)
	}

	resource, err := lc.createOrUpdateResource("deployment",
		getDeployment,
		deploymentIsDeleting,
		createDeployment,
		updateDeployment)

	if err != nil {
		return nil, err
	}

	return resource.(*apps_v1beta1.Deployment), err
}

func (lc *lazyClient) createOrUpdateHorizontalPodAutoscaler(functionLabels labels.Set,
	function *nuclioio.NuclioFunction) (*autos_v2.HorizontalPodAutoscaler, error) {

	maxReplicas := int32(function.Spec.MaxReplicas)
	if maxReplicas == 0 {
		maxReplicas = 10
	}

	minReplicas := int32(function.Spec.MinReplicas)
	if minReplicas == 0 {
		minReplicas = 1
	}

	targetCPU := int32(function.Spec.TargetCPU)
	if targetCPU == 0 {
		targetCPU = 75
	}

	getHorizontalPodAutoscaler := func() (interface{}, error) {
		return lc.kubeClientSet.AutoscalingV2beta1().HorizontalPodAutoscalers(function.Namespace).Get(function.Name,
			meta_v1.GetOptions{})
	}

	horizontalPodAutoscalerIsDeleting := func(resource interface{}) bool {
		return (resource).(*autos_v2.HorizontalPodAutoscaler).ObjectMeta.DeletionTimestamp != nil
	}

	createHorizontalPodAutoscaler := func() (interface{}, error) {
		if minReplicas == maxReplicas {
			return nil, nil
		}

		metricSpecs, err := lc.GetFunctionMetricSpecs(function.Name, targetCPU)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get function metric specs")
		}

		hpa := autos_v2.HorizontalPodAutoscaler{
			ObjectMeta: meta_v1.ObjectMeta{
				Name:      function.Name,
				Namespace: function.Namespace,
				Labels:    functionLabels,
			},
			Spec: autos_v2.HorizontalPodAutoscalerSpec{
				MinReplicas: &minReplicas,
				MaxReplicas: maxReplicas,
				Metrics:     metricSpecs,
				ScaleTargetRef: autos_v2.CrossVersionObjectReference{
					APIVersion: "apps/apps_v1beta1",
					Kind:       "Deployment",
					Name:       function.Name,
				},
			},
		}

		return lc.kubeClientSet.AutoscalingV2beta1().HorizontalPodAutoscalers(function.Namespace).Create(&hpa)
	}

	updateHorizontalPodAutoscaler := func(resourceToUpdate interface{}) (interface{}, error) {
		hpa := resourceToUpdate.(*autos_v2.HorizontalPodAutoscaler)

		metricSpecs, err := lc.GetFunctionMetricSpecs(function.Name, targetCPU)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get function metric specs")
		}

		hpa.Spec.Metrics = metricSpecs
		hpa.Labels = functionLabels
		hpa.Spec.MinReplicas = &minReplicas
		hpa.Spec.MaxReplicas = maxReplicas

		// when the min replicas equal the max replicas, there's no need for hpa resourceToUpdate
		if function.Spec.MinReplicas == function.Spec.MaxReplicas {
			propogationPolicy := meta_v1.DeletePropagationForeground
			deleteOptions := &meta_v1.DeleteOptions{
				PropagationPolicy: &propogationPolicy,
			}

			err := lc.kubeClientSet.AutoscalingV2beta1().HorizontalPodAutoscalers(function.Namespace).Delete(hpa.Name, deleteOptions)
			return nil, err
		}

		return lc.kubeClientSet.AutoscalingV2beta1().HorizontalPodAutoscalers(function.Namespace).Update(hpa)
	}

	resource, err := lc.createOrUpdateResource("hpa",
		getHorizontalPodAutoscaler,
		horizontalPodAutoscalerIsDeleting,
		createHorizontalPodAutoscaler,
		updateHorizontalPodAutoscaler)

	// a resource can be nil if it didn't met preconditions and wasn't created
	if err != nil || resource == nil {
		return nil, err
	}

	return resource.(*autos_v2.HorizontalPodAutoscaler), err
}

func (lc *lazyClient) createOrUpdateIngress(functionLabels labels.Set,
	function *nuclioio.NuclioFunction) (*ext_v1beta1.Ingress, error) {

	getIngress := func() (interface{}, error) {
		return lc.kubeClientSet.ExtensionsV1beta1().Ingresses(function.Namespace).Get(function.Name, meta_v1.GetOptions{})
	}

	ingressIsDeleting := func(resource interface{}) bool {
		return (resource).(*ext_v1beta1.Ingress).ObjectMeta.DeletionTimestamp != nil
	}

	createIngress := func() (interface{}, error) {
		ingressMeta := meta_v1.ObjectMeta{
			Name:      function.Name,
			Namespace: function.Namespace,
			Labels:    functionLabels,
		}

		ingressSpec := ext_v1beta1.IngressSpec{}

		if err := lc.populateIngressConfig(functionLabels, function, &ingressMeta, &ingressSpec); err != nil {
			return nil, errors.Wrap(err, "Failed to populate ingress spec")
		}

		// if there are no rules, don't create an ingress
		if len(ingressSpec.Rules) == 0 {
			return nil, nil
		}

		return lc.kubeClientSet.ExtensionsV1beta1().Ingresses(function.Namespace).Create(&ext_v1beta1.Ingress{
			ObjectMeta: ingressMeta,
			Spec:       ingressSpec,
		})
	}

	updateIngress := func(resource interface{}) (interface{}, error) {
		ingress := resource.(*ext_v1beta1.Ingress)

		// save to bool if there are current rules
		ingressRulesExist := len(ingress.Spec.Rules) > 0

		if err := lc.populateIngressConfig(functionLabels, function, &ingress.ObjectMeta, &ingress.Spec); err != nil {
			return nil, errors.Wrap(err, "Failed to populate ingress spec")
		}

		if len(ingress.Spec.Rules) == 0 {

			// if there are no rules and previously were, delete the ingress resource
			if ingressRulesExist {
				propogationPolicy := meta_v1.DeletePropagationForeground
				deleteOptions := &meta_v1.DeleteOptions{
					PropagationPolicy: &propogationPolicy,
				}

				err := lc.kubeClientSet.ExtensionsV1beta1().Ingresses(function.Namespace).Delete(function.Name, deleteOptions)
				return nil, err

			}

			// there's nothing to update
			return nil, nil
		}

		return lc.kubeClientSet.ExtensionsV1beta1().Ingresses(function.Namespace).Update(ingress)
	}

	resource, err := lc.createOrUpdateResource("ingress",
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

	return resource.(*ext_v1beta1.Ingress), err
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

func (lc *lazyClient) getFunctionReplicas(function *nuclioio.NuclioFunction) int {
	replicas := function.Spec.Replicas

	// only when function is scaled to zero, allow for replicas to be set to zero
	if function.Spec.Disabled || function.Status.State == functionconfig.FunctionStateScaledToZero {
		replicas = 0
	} else if replicas == 0 {

		// in this path, there's a always a minimum of one replica than needs to be available
		if function.Spec.MinReplicas > 0 {
			replicas = function.Spec.MinReplicas
		} else {
			replicas = 1
		}
	}

	return replicas
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
	if info, err := version.Get(); err == nil {
		nuclioVersion = info.Label
	} else {
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

func (lc *lazyClient) populateServiceSpec(functionLabels labels.Set,
	function *nuclioio.NuclioFunction,
	spec *v1.ServiceSpec) {

	if function.Status.State == functionconfig.FunctionStateScaledToZero {

		// pass all further requests to DLX service
		spec.Selector = map[string]string{"nuclio.io/app": "dlx"}
	} else {
		spec.Selector = functionLabels
	}

	spec.Type = function.Spec.ServiceType
	serviceTypeIsNodePort := spec.Type == v1.ServiceTypeNodePort

	// update the service's node port on the following conditions:
	// 1. this is a new service (spec.Ports is an empty list)
	// 2. this is an existing service (spec.Ports is not an empty list) BUT not if the service already has a node port
	//    and the function specifies 0 (meaning auto assign). This is to prevent cases where service already has a node
	//    port and then updating it causes node port change
	if len(spec.Ports) == 0 || !(spec.Ports[0].NodePort != 0 && function.Spec.GetHTTPPort() == 0) {
		spec.Ports = []v1.ServicePort{
			{
				Name: containerHTTPPortName,
				Port: int32(containerHTTPPort),
			},
		}
		if serviceTypeIsNodePort {
			spec.Ports[0].NodePort = int32(function.Spec.GetHTTPPort())
		} else {
			spec.Ports[0].NodePort = 0
		}
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

func (lc *lazyClient) populateIngressConfig(functionLabels labels.Set,
	function *nuclioio.NuclioFunction,
	meta *meta_v1.ObjectMeta,
	spec *ext_v1beta1.IngressSpec) error {
	meta.Annotations = make(map[string]string)

	lc.logger.DebugWith("Preparing ingress")

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

	// clear out existing so that we don't keep adding rules
	spec.Rules = []ext_v1beta1.IngressRule{}
	spec.TLS = []ext_v1beta1.IngressTLS{}

	for _, ingress := range functionconfig.GetIngressesFromTriggers(function.Spec.Triggers) {
		if err := lc.addIngressToSpec(&ingress, functionLabels, function, spec); err != nil {
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

func (lc *lazyClient) addIngressToSpec(ingress *functionconfig.Ingress,
	functionLabels labels.Set,
	function *nuclioio.NuclioFunction,
	spec *ext_v1beta1.IngressSpec) error {

	lc.logger.DebugWith("Adding ingress",
		"function", function.Name,
		"labels", functionLabels,
		"host", ingress.Host,
		"paths", ingress.Paths,
		"TLS", ingress.TLS)

	ingressRule := ext_v1beta1.IngressRule{
		Host: ingress.Host,
	}

	ingressRule.IngressRuleValue.HTTP = &ext_v1beta1.HTTPIngressRuleValue{}

	// populate the ingress rule value
	for _, path := range ingress.Paths {
		formattedPath, err := lc.formatIngressPattern(path, functionLabels, function)
		if err != nil {
			return errors.Wrap(err, "Failed to format ingress pattern")
		}

		httpIngressPath := ext_v1beta1.HTTPIngressPath{
			Path: formattedPath,
			Backend: ext_v1beta1.IngressBackend{
				ServiceName: function.Name,
				ServicePort: intstr.IntOrString{
					Type:   intstr.String,
					StrVal: containerHTTPPortName,
				},
			},
		}

		// add path
		ingressRule.IngressRuleValue.HTTP.Paths = append(ingressRule.IngressRuleValue.HTTP.Paths, httpIngressPath)

		// add TLS if such exists
		if ingress.TLS.SecretName != "" {
			ingressTLS := ext_v1beta1.IngressTLS{}
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
	healthCheckHTTPPort := 8082

	container.Image = function.Spec.Image
	container.Resources = function.Spec.Resources
	if container.Resources.Requests == nil {
		container.Resources.Requests = make(v1.ResourceList)

		// the default is 500 milli cpu
		cpuQuantity, err := resource.ParseQuantity("25m") // nolint: errcheck
		if err == nil {
			container.Resources.Requests["cpu"] = cpuQuantity
		}
	}
	container.Env = lc.getFunctionEnvironment(functionLabels, function)
	container.Ports = []v1.ContainerPort{
		{
			Name:          containerHTTPPortName,
			ContainerPort: containerHTTPPort,
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
				Port: intstr.FromInt(healthCheckHTTPPort),
				Path: "/ready",
			},
		},
		InitialDelaySeconds: 1,
		TimeoutSeconds:      1,
		PeriodSeconds:       1,
	}

	container.LivenessProbe = &v1.Probe{
		Handler: v1.Handler{
			HTTPGet: &v1.HTTPGetAction{
				Port: intstr.FromInt(healthCheckHTTPPort),
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
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      lc.configMapNameFromFunctionName(function.Name),
			Namespace: function.Namespace,
		},
		Data: map[string]string{
			"processor.yaml": configMapContents.String(),
		},
	}

	return nil
}

func (lc *lazyClient) configMapNameFromFunctionName(functionName string) string {
	return functionName
}

func (lc *lazyClient) getFunctionVolumeAndMounts(function *nuclioio.NuclioFunction) ([]v1.Volume, []v1.VolumeMount) {
	trueVal := true
	var configVolumes []functionconfig.Volume

	processorConfigVolumeName := "processor-config-volume"
	platformConfigVolumeName := "platform-config-volume"

	// processor configuration
	processorConfigVolume := functionconfig.Volume{}
	processorConfigVolume.Volume.Name = processorConfigVolumeName
	processorConfigMapVolumeSource := v1.ConfigMapVolumeSource{}
	processorConfigMapVolumeSource.Name = lc.configMapNameFromFunctionName(function.Name)
	processorConfigVolume.Volume.ConfigMap = &processorConfigMapVolumeSource
	processorConfigVolume.VolumeMount.Name = processorConfigVolumeName
	processorConfigVolume.VolumeMount.MountPath = "/etc/nuclio/config/processor"

	// platform configuration
	platformConfigVolume := functionconfig.Volume{}
	platformConfigVolume.Volume.Name = platformConfigVolumeName
	platformConfigMapVolumeSource := v1.ConfigMapVolumeSource{}
	platformConfigMapVolumeSource.Name = "platform-config"
	platformConfigMapVolumeSource.Optional = &trueVal
	platformConfigVolume.Volume.ConfigMap = &platformConfigMapVolumeSource
	platformConfigVolume.VolumeMount.Name = platformConfigVolumeName
	platformConfigVolume.VolumeMount.MountPath = "/etc/nuclio/config/platform"

	// merge from functionconfig and injected configuration
	configVolumes = append(configVolumes, function.Spec.Volumes...)
	configVolumes = append(configVolumes, processorConfigVolume)
	configVolumes = append(configVolumes, platformConfigVolume)

	var volumes []v1.Volume
	var volumeMounts []v1.VolumeMount

	for _, configVolume := range configVolumes {

		// ignore if it's a HostPath volume
		if configVolume.Volume.HostPath != nil {
			lc.logger.WarnWith("Ignoring volume. HostPath volumes are now deprecated",
				"configVolume",
				configVolume)

			continue
		}

		if configVolume.Volume.FlexVolume != nil && configVolume.Volume.FlexVolume.Driver == "v3io/fuse" {

			// make sure the given sub path matches the needed structure. fix in case it doesn't
			subPath, subPathExists := configVolume.Volume.FlexVolume.Options["subPath"]
			if subPathExists && len(subPath) != 0 {

				// insert slash in the beginning in case it wasn't given (example: "my/path" -> "/my/path")
				if subPath[0] != '/' {
					configVolume.Volume.FlexVolume.Options["subPath"] = "/" + subPath
				}

				// remove ending slash in case it was given (example: "/my/path/" -> "/my/path")
				if subPath[len(subPath)-1] == '/' {
					configVolume.Volume.FlexVolume.Options["subPath"] = subPath[:len(subPath)-1]

				}
			}
		}

		lc.logger.DebugWith("Adding volume",
			"configVolume",
			configVolume)

		volumes = append(volumes, configVolume.Volume)
		volumeMounts = append(volumeMounts, configVolume.VolumeMount)
	}

	return volumes, volumeMounts
}

func (lc *lazyClient) deleteFunctionEvents(ctx context.Context, functionName string, namespace string) error {

	// create error group
	errGroup, _ := errgroup.WithContext(ctx)

	listOptions := meta_v1.ListOptions{
		LabelSelector: fmt.Sprintf("nuclio.io/function-name=%s", functionName),
	}

	result, err := lc.nuclioClientSet.NuclioV1beta1().NuclioFunctionEvents(namespace).List(listOptions)
	if err != nil {
		return errors.Wrap(err, "Failed to list function events")
	}

	lc.logger.DebugWith("Got function events", "num", len(result.Items))

	for _, functionEvent := range result.Items {
		errGroup.Go(func() error {
			err = lc.nuclioClientSet.NuclioV1beta1().NuclioFunctionEvents(namespace).Delete(functionEvent.Name, &meta_v1.DeleteOptions{})
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

func (lc *lazyClient) GetFunctionMetricSpecs(functionName string, targetCPU int32) ([]autos_v2.MetricSpec, error) {
	var metricSpecs []autos_v2.MetricSpec
	config := lc.platformConfigurationProvider.GetPlatformConfiguration()
	if lc.functionsHaveAutoScaleMetrics(config) {
		targetValue, err := resource.ParseQuantity(config.AutoScale.TargetValue)
		if err != nil {
			return metricSpecs, errors.Wrap(err, "Failed to parse target value for auto scale")
		}

		// special cases for k8s resources that are supplied by regular metric server, excluding cpu
		if lc.getMetricResourceByName(config.AutoScale.MetricName) != "" {
			metricSpecs = []autos_v2.MetricSpec{
				{
					Type: "Resource",
					Resource: &autos_v2.ResourceMetricSource{
						Name:               lc.getMetricResourceByName(config.AutoScale.MetricName),
						TargetAverageValue: &targetValue,
					},
				},
			}
		} else {
			metricSpecs = []autos_v2.MetricSpec{
				{
					Type: "Pods",
					Pods: &autos_v2.PodsMetricSource{
						MetricName:         config.AutoScale.MetricName,
						TargetAverageValue: targetValue,
					},
				},
			}
		}

		// a bug/unexpected feature in hpa doesn't allow for both custom metrics and resource metrics
	} else {

		// special case, keep support for target cpu in percentage
		metricSpecs = append(metricSpecs, autos_v2.MetricSpec{
			Type: "Resource",
			Resource: &autos_v2.ResourceMetricSource{
				Name: v1.ResourceCPU,
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
		return v1.ResourceNvidiaGPU
	case "ephemeral-storage":
		return v1.ResourceEphemeralStorage
	case "storage":
		return v1.ResourceStorage
	default:
		return v1.ResourceName("")
	}
}

//
// Resources
//

type lazyResources struct {
	logger                  logger.Logger
	deployment              *apps_v1beta1.Deployment
	configMap               *v1.ConfigMap
	service                 *v1.Service
	horizontalPodAutoscaler *autos_v2.HorizontalPodAutoscaler
	ingress                 *ext_v1beta1.Ingress
}

// Deployment returns the deployment
func (lr *lazyResources) Deployment() (*apps_v1beta1.Deployment, error) {
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
func (lr *lazyResources) HorizontalPodAutoscaler() (*autos_v2.HorizontalPodAutoscaler, error) {
	return lr.horizontalPodAutoscaler, nil
}

// Ingress returns the ingress
func (lr *lazyResources) Ingress() (*ext_v1beta1.Ingress, error) {
	return lr.ingress, nil
}
