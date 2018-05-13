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
	"encoding/json"
	"strings"
	"text/template"
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/config"
	"github.com/nuclio/nuclio/pkg/version"

	"github.com/nuclio/logger"
	apps_v1beta1 "k8s.io/api/apps/v1beta1"
	autos_v1 "k8s.io/api/autoscaling/v1"
	"k8s.io/api/core/v1"
	ext_v1beta1 "k8s.io/api/extensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

//
// Client
//

type lazyClient struct {
	logger        logger.Logger
	kubeClientSet kubernetes.Interface
	classLabels   map[string]string
}

func NewLazyClient(parentLogger logger.Logger,
	kubeClientSet kubernetes.Interface) (Client, error) {

	newClient := lazyClient{
		logger:        parentLogger.GetChild("functionres"),
		kubeClientSet: kubeClientSet,
		classLabels:   make(map[string]string),
	}

	newClient.initClassLabels()

	return &newClient, nil
}

func (lc *lazyClient) List(namespace string) ([]Resources, error) {
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

func (lc *lazyClient) Get(namespace string, name string) (Resources, error) {
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

func (lc *lazyClient) CreateOrUpdate(function *nuclioio.Function, imagePullSecrets string) (Resources, error) {
	var err error

	// get labels from the function and add class labels
	labels := lc.getFunctionLabels(function)

	// set a few constants
	labels["nuclio.io/function-name"] = function.Name

	// TODO: remove when versioning is back in
	function.Spec.Version = -1
	function.Spec.Alias = "latest"
	labels["nuclio.io/function-version"] = "latest"

	resources := lazyResources{}

	// create or update the applicable configMap
	resources.configMap, err = lc.createOrUpdateConfigMap(function)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create/update configMap")
	}

	// create or update the applicable service
	resources.service, err = lc.createOrUpdateService(labels, function)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create/update service")
	}

	// create or update the applicable deployment
	resources.deployment, err = lc.createOrUpdateDeployment(labels, imagePullSecrets, function)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create/update deployment")
	}

	// create or update the HPA
	resources.horizontalPodAutoscaler, err = lc.createOrUpdateHorizontalPodAutoscaler(labels, function)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create/update HPA")
	}

	// create or update ingress
	resources.ingress, err = lc.createOrUpdateIngress(labels, function)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create/update ingress")
	}

	lc.logger.Debug("Deployment created/updated")

	return &resources, nil
}

func (lc *lazyClient) WaitAvailable(namespace string, name string) error {
	lc.logger.DebugWith("Waiting for deployment to be available", "namespace", namespace, "name", name)

	// TODO: maybe want to take the readiness timeout from DeployOptions for this timeout
	return wait.Poll(400*time.Millisecond, 30*time.Second, func() (bool, error) {

		// get the deployment
		result, err := lc.kubeClientSet.AppsV1beta1().Deployments(namespace).Get(name, meta_v1.GetOptions{})
		if err != nil {
			return false, nil
		}

		// find the condition whose type is Available - that's the one we want to examine
		for _, deploymentCondition := range result.Status.Conditions {

			// when we find the right condition, check its Status to see if it's true.
			// a DeploymentCondition whose Type == Available and Status == True means the deployment is available
			if deploymentCondition.Type == apps_v1beta1.DeploymentAvailable {
				available := deploymentCondition.Status == v1.ConditionTrue

				if available {
					lc.logger.DebugWith("Deployment is available", "reason", deploymentCondition.Reason)
					return true, nil
				}

				lc.logger.DebugWith("Deployment not available yet", "reason", deploymentCondition.Reason)
				return false, nil
			}
		}

		// if we got here it means we didn't find the deployment's Availability condition - shouldn't happen
		return false, errors.New("Failed to find the deployment's availability condition")
	})
}

func (lc *lazyClient) Delete(namespace string, name string) error {
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
	err = lc.kubeClientSet.AutoscalingV1().HorizontalPodAutoscalers(namespace).Delete(name, deleteOptions)
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

	lc.logger.DebugWith("Deleted deployed function", "namespace", namespace, "name", name)

	return nil
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

		lc.logger.DebugWith("Resource created",
			"resource", resource)

		return resource, nil
	}

	resource, err = updateResource(resource)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to update resource")
	}

	lc.logger.DebugWith("Resource updated", "resource", resource)

	return resource, nil
}

func (lc *lazyClient) createOrUpdateConfigMap(function *nuclioio.Function) (*v1.ConfigMap, error) {

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

func (lc *lazyClient) createOrUpdateService(labels map[string]string,
	function *nuclioio.Function) (*v1.Service, error) {

	getService := func() (interface{}, error) {
		return lc.kubeClientSet.CoreV1().Services(function.Namespace).Get(function.Name, meta_v1.GetOptions{})
	}

	serviceIsDeleting := func(resource interface{}) bool {
		return (resource).(*v1.Service).ObjectMeta.DeletionTimestamp != nil
	}

	createService := func() (interface{}, error) {
		spec := v1.ServiceSpec{}
		lc.populateServiceSpec(labels, function, &spec)

		return lc.kubeClientSet.CoreV1().Services(function.Namespace).Create(&v1.Service{
			ObjectMeta: meta_v1.ObjectMeta{
				Name:      function.Name,
				Namespace: function.Namespace,
				Labels:    labels,
			},
			Spec: spec,
		})
	}

	updateService := func(resource interface{}) (interface{}, error) {
		service := resource.(*v1.Service)

		// update existing
		service.Labels = labels
		lc.populateServiceSpec(labels, function, &service.Spec)

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

func (lc *lazyClient) createOrUpdateDeployment(labels map[string]string,
	imagePullSecrets string,
	function *nuclioio.Function) (*apps_v1beta1.Deployment, error) {

	// to make sure the pod re-pulls the image, we need to specify a unique string here
	podAnnotations := map[string]string{
		"nuclio.io/image-hash": function.Spec.ImageHash,
	}

	replicas := int32(lc.getFunctionReplicas(function))
	deploymentAnnotations, err := lc.getDeploymentAnnotations(function)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get function annotations")
	}

	getDeployment := func() (interface{}, error) {
		return lc.kubeClientSet.AppsV1beta1().Deployments(function.Namespace).Get(function.Name, meta_v1.GetOptions{})
	}

	deploymentIsDeleting := func(resource interface{}) bool {
		return (resource).(*apps_v1beta1.Deployment).ObjectMeta.DeletionTimestamp != nil
	}

	createDeployment := func() (interface{}, error) {
		container := v1.Container{Name: "nuclio"}
		lc.populateDeploymentContainer(labels, function, &container)

		volumes := lc.getConfigurationVolumes(function)

		return lc.kubeClientSet.AppsV1beta1().Deployments(function.Namespace).Create(&apps_v1beta1.Deployment{

			ObjectMeta: meta_v1.ObjectMeta{
				Name:        function.Name,
				Namespace:   function.Namespace,
				Labels:      labels,
				Annotations: deploymentAnnotations,
			},
			Spec: apps_v1beta1.DeploymentSpec{
				Replicas: &replicas,
				Template: v1.PodTemplateSpec{
					ObjectMeta: meta_v1.ObjectMeta{
						Name:        function.Name,
						Namespace:   function.Namespace,
						Labels:      labels,
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

		deployment.Labels = labels
		deployment.Annotations = deploymentAnnotations
		deployment.Spec.Replicas = &replicas
		deployment.Spec.Template.Annotations = podAnnotations
		deployment.Spec.Template.Labels = labels
		lc.populateDeploymentContainer(labels, function, &deployment.Spec.Template.Spec.Containers[0])

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

func (lc *lazyClient) createOrUpdateHorizontalPodAutoscaler(labels map[string]string,
	function *nuclioio.Function) (*autos_v1.HorizontalPodAutoscaler, error) {

	if function.Spec.MinReplicas == function.Spec.MaxReplicas {
		return nil, nil
	}

	maxReplicas := int32(function.Spec.MaxReplicas)
	if maxReplicas == 0 {
		maxReplicas = 4
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
		return lc.kubeClientSet.AutoscalingV1().HorizontalPodAutoscalers(function.Namespace).Get(function.Name,
			meta_v1.GetOptions{})
	}

	horizontalPodAutoscalerIsDeleting := func(resource interface{}) bool {
		return (resource).(*autos_v1.HorizontalPodAutoscaler).ObjectMeta.DeletionTimestamp != nil
	}

	createHorizontalPodAutoscaler := func() (interface{}, error) {
		return lc.kubeClientSet.AutoscalingV1().HorizontalPodAutoscalers(function.Namespace).Create(&autos_v1.HorizontalPodAutoscaler{
			ObjectMeta: meta_v1.ObjectMeta{
				Name:      function.Name,
				Namespace: function.Namespace,
				Labels:    labels,
			},
			Spec: autos_v1.HorizontalPodAutoscalerSpec{
				MinReplicas:                    &minReplicas,
				MaxReplicas:                    maxReplicas,
				TargetCPUUtilizationPercentage: &targetCPU,
				ScaleTargetRef: autos_v1.CrossVersionObjectReference{
					APIVersion: "apps/apps_v1beta1",
					Kind:       "Deployment",
					Name:       function.Name,
				},
			},
		})
	}

	updateHorizontalPodAutoscaler := func(resource interface{}) (interface{}, error) {
		hpa := resource.(*autos_v1.HorizontalPodAutoscaler)

		hpa.Labels = labels
		hpa.Spec.MinReplicas = &minReplicas
		hpa.Spec.MaxReplicas = maxReplicas
		hpa.Spec.TargetCPUUtilizationPercentage = &targetCPU

		return lc.kubeClientSet.AutoscalingV1().HorizontalPodAutoscalers(function.Namespace).Update(hpa)
	}

	resource, err := lc.createOrUpdateResource("hpa",
		getHorizontalPodAutoscaler,
		horizontalPodAutoscalerIsDeleting,
		createHorizontalPodAutoscaler,
		updateHorizontalPodAutoscaler)

	if err != nil {
		return nil, err
	}

	return resource.(*autos_v1.HorizontalPodAutoscaler), err
}

func (lc *lazyClient) createOrUpdateIngress(labels map[string]string,
	function *nuclioio.Function) (*ext_v1beta1.Ingress, error) {

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
			Labels:    labels,
		}

		ingressSpec := ext_v1beta1.IngressSpec{}

		if err := lc.populateIngressConfig(labels, function, &ingressMeta, &ingressSpec); err != nil {
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

		if err := lc.populateIngressConfig(labels, function, &ingress.ObjectMeta, &ingress.Spec); err != nil {
			return nil, errors.Wrap(err, "Failed to populate ingress spec")
		}

		// if there are no rules, don't create an ingress
		if len(ingress.Spec.Rules) == 0 {
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

func (lc *lazyClient) getFunctionLabels(function *nuclioio.Function) map[string]string {
	result := map[string]string{}

	for labelKey, labelValue := range function.Labels {
		result[labelKey] = labelValue
	}

	for labelKey, labelValue := range lc.classLabels {
		result[labelKey] = labelValue
	}

	return result
}

func (lc *lazyClient) getFunctionReplicas(function *nuclioio.Function) int {
	replicas := function.Spec.Replicas

	if function.Spec.Disabled {
		replicas = 0
	} else if replicas == 0 {
		replicas = function.Spec.MinReplicas
	}

	return replicas
}

func (lc *lazyClient) getDeploymentAnnotations(function *nuclioio.Function) (map[string]string, error) {
	annotations := make(map[string]string)

	if function.Spec.Description != "" {
		annotations["description"] = function.Spec.Description
	}

	serializedFunctionJSON, err := lc.serializeFunctionJSON(function)
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

	annotations["nuclio.io/function-config"] = serializedFunctionJSON
	annotations["nuclio.io/controller-version"] = nuclioVersion

	return annotations, nil
}

func (lc *lazyClient) getFunctionEnvironment(labels map[string]string,
	function *nuclioio.Function) []v1.EnvVar {
	env := function.Spec.Env

	env = append(env, v1.EnvVar{Name: "NUCLIO_FUNCTION_NAME", Value: labels["nuclio.io/function-name"]})
	env = append(env, v1.EnvVar{Name: "NUCLIO_FUNCTION_VERSION", Value: labels["nuclio.io/function-version"]})

	return env
}

func (lc *lazyClient) serializeFunctionJSON(function *nuclioio.Function) (string, error) {
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

func (lc *lazyClient) populateServiceSpec(labels map[string]string,
	function *nuclioio.Function,
	spec *v1.ServiceSpec) {

	spec.Selector = labels
	spec.Type = v1.ServiceTypeNodePort

	// update the service's node port on the following conditions:
	// 1. this is a new service (spec.Ports is an empty list)
	// 2. this is an existing service (spec.Ports is not an empty list) BUT not if the service already has a node port
	//    and the function specifies 0 (meaning auto assign). This is to prevent cases where service already has a node
	//    port and then updating it causes node port change
	if len(spec.Ports) == 0 || !(spec.Ports[0].NodePort != 0 && function.Spec.GetHTTPPort() == 0) {
		spec.Ports = []v1.ServicePort{
			{Name: containerHTTPPortName, Port: int32(containerHTTPPort), NodePort: int32(function.Spec.GetHTTPPort())},
		}
	}
}

func (lc *lazyClient) populateIngressConfig(labels map[string]string,
	function *nuclioio.Function,
	meta *meta_v1.ObjectMeta,
	spec *ext_v1beta1.IngressSpec) error {

	lc.logger.DebugWith("Preparing ingress")

	// get the first HTTP trigger and look for annotations that we shove to the ingress
	// there should only be 0 or 1. if there are more, just take the first
	for _, httpTrigger := range functionconfig.GetTriggersByKind(function.Spec.Triggers, "http") {

		// set annotations
		meta.Annotations = httpTrigger.Annotations

		// ignore any other http triggers, validation should catch that
		break
	}

	// clear out existing so that we don't keep adding rules
	spec.Rules = []ext_v1beta1.IngressRule{}

	for _, ingress := range functionconfig.GetIngressesFromTriggers(function.Spec.Triggers) {
		if err := lc.addIngressToSpec(&ingress, labels, function, spec); err != nil {
			return errors.Wrap(err, "Failed to add ingress to spec")
		}
	}

	return nil
}

func (lc *lazyClient) formatIngressPattern(ingressPattern string,
	labels map[string]string,
	function *nuclioio.Function) (string, error) {

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
		Version:   labels["nuclio.io/function-version"],
	}

	if err := parsedTemplate.Execute(&ingressPatternBuffer, templateVars); err != nil {
		return "", errors.Wrap(err, "Failed to execute parsed template")
	}

	return ingressPatternBuffer.String(), nil
}

func (lc *lazyClient) addIngressToSpec(ingress *functionconfig.Ingress,
	labels map[string]string,
	function *nuclioio.Function,
	spec *ext_v1beta1.IngressSpec) error {

	lc.logger.DebugWith("Adding ingress",
		"function", function.Name,
		"labels", labels,
		"host", ingress.Host,
		"paths", ingress.Paths)

	ingressRule := ext_v1beta1.IngressRule{
		Host: ingress.Host,
	}

	ingressRule.IngressRuleValue.HTTP = &ext_v1beta1.HTTPIngressRuleValue{}

	// populate the ingress rule value
	for _, path := range ingress.Paths {
		formattedPath, err := lc.formatIngressPattern(path, labels, function)
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
	}

	spec.Rules = append(spec.Rules, ingressRule)

	return nil
}

func (lc *lazyClient) populateDeploymentContainer(labels map[string]string,
	function *nuclioio.Function,
	container *v1.Container) {

	processorConfigVolumeMount := v1.VolumeMount{}
	processorConfigVolumeMount.Name = processorConfigVolumeName
	processorConfigVolumeMount.MountPath = "/etc/nuclio/config/processor"

	platformConfigVolumeMount := v1.VolumeMount{}
	platformConfigVolumeMount.Name = platformConfigVolumeName
	platformConfigVolumeMount.MountPath = "/etc/nuclio/config/platform"

	container.Image = function.Spec.Image
	container.Resources = function.Spec.Resources
	container.Env = lc.getFunctionEnvironment(labels, function)
	container.VolumeMounts = []v1.VolumeMount{processorConfigVolumeMount, platformConfigVolumeMount}
	container.Ports = []v1.ContainerPort{
		{
			ContainerPort: containerHTTPPort,
		},
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

	// always pull so that each create / update will trigger a rolling update including pulling the image. this is
	// because the tag of the image doesn't change between revisions of the function
	container.ImagePullPolicy = v1.PullAlways
}

func (lc *lazyClient) populateConfigMap(labels map[string]string,
	function *nuclioio.Function,
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

func (lc *lazyClient) getConfigurationVolumes(function *nuclioio.Function) []v1.Volume {
	trueVal := true

	// processor configuration
	processorConfigVolume := v1.Volume{}
	processorConfigVolume.Name = processorConfigVolumeName
	processorConfigMapVolumeSource := v1.ConfigMapVolumeSource{}
	processorConfigMapVolumeSource.Name = lc.configMapNameFromFunctionName(function.Name)
	processorConfigVolume.ConfigMap = &processorConfigMapVolumeSource

	// platform configuration
	platformConfigVolume := v1.Volume{}
	platformConfigVolume.Name = platformConfigVolumeName
	platformConfigMapVolumeSource := v1.ConfigMapVolumeSource{}
	platformConfigMapVolumeSource.Name = "platform-config"
	platformConfigMapVolumeSource.Optional = &trueVal
	platformConfigVolume.ConfigMap = &platformConfigMapVolumeSource

	return []v1.Volume{
		processorConfigVolume,
		platformConfigVolume,
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
	horizontalPodAutoscaler *autos_v1.HorizontalPodAutoscaler
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
func (lr *lazyResources) HorizontalPodAutoscaler() (*autos_v1.HorizontalPodAutoscaler, error) {
	return lr.horizontalPodAutoscaler, nil
}

// Ingress returns the ingress
func (lr *lazyResources) Ingress() (*ext_v1beta1.Ingress, error) {
	return lr.ingress, nil
}
