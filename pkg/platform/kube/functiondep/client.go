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

package functiondep

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform/kube/functioncr"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/config"

	"github.com/nuclio/nuclio-sdk"
	apps_v1beta1 "k8s.io/api/apps/v1beta1"
	autos_v1 "k8s.io/api/autoscaling/v1"
	"k8s.io/api/core/v1"
	ext_v1beta1 "k8s.io/api/extensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

const (
	containerHTTPPort         = 8080
	processorConfigVolumeName = "processor-config-volume"
	containerHTTPPortName     = "http"
)

type Client struct {
	logger             nuclio.Logger
	clientSet          *kubernetes.Clientset
	classLabels        map[string]string
	classLabelSelector string
}

func NewClient(parentLogger nuclio.Logger,
	clientSet *kubernetes.Clientset) (*Client, error) {

	newClient := &Client{
		logger:      parentLogger.GetChild("functiondep"),
		clientSet:   clientSet,
		classLabels: make(map[string]string),
	}

	newClient.initClassLabels()

	return newClient, nil
}

func (c *Client) List(namespace string) ([]apps_v1beta1.Deployment, error) {
	listOptions := meta_v1.ListOptions{
		LabelSelector: c.classLabelSelector,
	}

	result, err := c.clientSet.AppsV1beta1().Deployments(namespace).List(listOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to list deployments")
	}

	c.logger.DebugWith("Got deployments", "num", len(result.Items))

	return result.Items, nil
}

func (c *Client) Get(namespace string, name string) (*apps_v1beta1.Deployment, error) {
	var result *apps_v1beta1.Deployment

	result, err := c.clientSet.AppsV1beta1().Deployments(namespace).Get(name, meta_v1.GetOptions{})
	c.logger.DebugWith("Got deployment",
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

	return result, err
}

func (c *Client) CreateOrUpdate(function *functioncr.Function) (*apps_v1beta1.Deployment, error) {

	// get labels from the function and add class labels
	labels := c.getFunctionLabels(function)

	// create or update the applicable configMap
	_, err := c.createOrUpdateConfigMap(function)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create/update configMap")
	}

	// create or update the applicable service
	_, err = c.createOrUpdateService(labels, function)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create/update service")
	}

	// create or update the applicable deployment
	deployment, err := c.createOrUpdateDeployment(labels, function)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create/update deployment")
	}

	// create or update the HPA
	_, err = c.createOrUpdateHorizontalPodAutoscaler(labels, function)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create/update HPA")
	}

	// create or update ingress
	_, err = c.createOrUpdateIngress(labels, function)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create/update ingress")
	}

	c.logger.Debug("Deployment created/updated")

	return deployment, nil
}

func (c *Client) Delete(namespace string, name string) error {
	propogationPolicy := meta_v1.DeletePropagationForeground
	deleteOptions := &meta_v1.DeleteOptions{
		PropagationPolicy: &propogationPolicy,
	}

	// Delete ingress
	err := c.clientSet.ExtensionsV1beta1().Ingresses(namespace).Delete(name, deleteOptions)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrap(err, "Failed to delete ingress")
		}
	} else {
		c.logger.DebugWith("Deleted ingress", "namespace", namespace, "name", name)
	}

	// Delete HPA if exists
	err = c.clientSet.AutoscalingV1().HorizontalPodAutoscalers(namespace).Delete(name, deleteOptions)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrap(err, "Failed to delete HPA")
		}
	} else {
		c.logger.DebugWith("Deleted HPA", "namespace", namespace, "name", name)
	}

	// Delete Service if exists
	err = c.clientSet.CoreV1().Services(namespace).Delete(name, deleteOptions)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrap(err, "Failed to delete service")
		}
	} else {
		c.logger.DebugWith("Deleted service", "namespace", namespace, "name", name)
	}

	// Delete Deployment if exists
	err = c.clientSet.AppsV1beta1().Deployments(namespace).Delete(name, deleteOptions)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrap(err, "Failed to delete deployment")
		}
	} else {
		c.logger.DebugWith("Deleted deployment", "namespace", namespace, "name", name)
	}

	// Delete configMap if exists
	err = c.clientSet.CoreV1().ConfigMaps(namespace).Delete(name, deleteOptions)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrap(err, "Failed to delete configMap")
		}
	} else {
		c.logger.DebugWith("Deleted configMap", "namespace", namespace, "name", name)
	}

	c.logger.DebugWith("Deleted deployed function", "namespace", namespace, "name", name)

	return nil
}

// as a closure so resourceExists can update
func (c *Client) createOrUpdateResource(resourceName string,
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
			c.logger.DebugWith("Resource is deleting, waiting", "name", resourceName)

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

		c.logger.DebugWith("Resource created",
			"resource", resource)

		return resource, nil
	}

	resource, err = updateResource(resource)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to update resource")
	}

	c.logger.DebugWith("Resource updated", "resource", resource)

	return resource, nil
}

func (c *Client) createOrUpdateConfigMap(function *functioncr.Function) (*v1.ConfigMap, error) {

	getConfigMap := func() (interface{}, error) {
		return c.clientSet.CoreV1().ConfigMaps(function.Namespace).Get(c.configMapNameFromFunctionName(function.Name),
			meta_v1.GetOptions{})
	}

	configMapIsDeleting := func(resource interface{}) bool {
		return (resource).(*v1.ConfigMap).ObjectMeta.DeletionTimestamp != nil
	}

	createConfigMap := func() (interface{}, error) {
		configMap := v1.ConfigMap{}
		if err := c.populateConfigMap(nil, function, &configMap); err != nil {
			return nil, errors.Wrap(err, "Failed to populate configMap")
		}

		return c.clientSet.CoreV1().ConfigMaps(function.Namespace).Create(&configMap)
	}

	updateConfigMap := func(resource interface{}) (interface{}, error) {
		configMap := resource.(*v1.ConfigMap)

		// update existing
		if err := c.populateConfigMap(nil, function, configMap); err != nil {
			return nil, errors.Wrap(err, "Failed to populate configMap")
		}

		return c.clientSet.CoreV1().ConfigMaps(function.Namespace).Update(configMap)
	}

	resource, err := c.createOrUpdateResource("configMap",
		getConfigMap,
		configMapIsDeleting,
		createConfigMap,
		updateConfigMap)

	if err != nil {
		return nil, err
	}

	return resource.(*v1.ConfigMap), err
}

func (c *Client) createOrUpdateService(labels map[string]string,
	function *functioncr.Function) (*v1.Service, error) {

	getService := func() (interface{}, error) {
		return c.clientSet.CoreV1().Services(function.Namespace).Get(function.Name, meta_v1.GetOptions{})
	}

	serviceIsDeleting := func(resource interface{}) bool {
		return (resource).(*v1.Service).ObjectMeta.DeletionTimestamp != nil
	}

	createService := func() (interface{}, error) {
		spec := v1.ServiceSpec{}
		c.populateServiceSpec(labels, function, &spec)

		return c.clientSet.CoreV1().Services(function.Namespace).Create(&v1.Service{
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
		c.populateServiceSpec(labels, function, &service.Spec)

		return c.clientSet.CoreV1().Services(function.Namespace).Update(service)
	}

	resource, err := c.createOrUpdateResource("service",
		getService,
		serviceIsDeleting,
		createService,
		updateService)

	if err != nil {
		return nil, err
	}

	return resource.(*v1.Service), err
}

func (c *Client) createOrUpdateDeployment(labels map[string]string,
	function *functioncr.Function) (*apps_v1beta1.Deployment, error) {

	replicas := int32(c.getFunctionReplicas(function))
	annotations, err := c.getFunctionAnnotations(function)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get function annotations")
	}

	getDeployment := func() (interface{}, error) {
		return c.clientSet.AppsV1beta1().Deployments(function.Namespace).Get(function.Name, meta_v1.GetOptions{})
	}

	deploymentIsDeleting := func(resource interface{}) bool {
		return (resource).(*apps_v1beta1.Deployment).ObjectMeta.DeletionTimestamp != nil
	}

	createDeployment := func() (interface{}, error) {
		container := v1.Container{Name: "nuclio"}
		c.populateDeploymentContainer(labels, function, &container)

		volume := v1.Volume{}
		volume.Name = processorConfigVolumeName
		configMapVolumeSource := v1.ConfigMapVolumeSource{}
		configMapVolumeSource.Name = c.configMapNameFromFunctionName(function.Name)
		volume.ConfigMap = &configMapVolumeSource

		return c.clientSet.AppsV1beta1().Deployments(function.Namespace).Create(&apps_v1beta1.Deployment{

			ObjectMeta: meta_v1.ObjectMeta{
				Name:        function.Name,
				Namespace:   function.Namespace,
				Labels:      labels,
				Annotations: annotations,
			},
			Spec: apps_v1beta1.DeploymentSpec{
				Replicas: &replicas,
				Template: v1.PodTemplateSpec{
					ObjectMeta: meta_v1.ObjectMeta{
						Name:      function.Name,
						Namespace: function.Namespace,
						Labels:    labels,
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							container,
						},
						Volumes: []v1.Volume{volume},
					},
				},
			},
		})
	}

	updateDeployment := func(resource interface{}) (interface{}, error) {
		deployment := resource.(*apps_v1beta1.Deployment)

		deployment.Labels = labels
		deployment.Annotations = annotations
		deployment.Spec.Replicas = &replicas
		deployment.Spec.Template.Labels = labels
		c.populateDeploymentContainer(labels, function, &deployment.Spec.Template.Spec.Containers[0])

		return c.clientSet.AppsV1beta1().Deployments(function.Namespace).Update(deployment)
	}

	resource, err := c.createOrUpdateResource("deployment",
		getDeployment,
		deploymentIsDeleting,
		createDeployment,
		updateDeployment)

	if err != nil {
		return nil, err
	}

	return resource.(*apps_v1beta1.Deployment), err
}

func (c *Client) createOrUpdateHorizontalPodAutoscaler(labels map[string]string,
	function *functioncr.Function) (*autos_v1.HorizontalPodAutoscaler, error) {

	maxReplicas := int32(function.Spec.MaxReplicas)
	if maxReplicas == 0 {
		maxReplicas = 4
	}

	minReplicas := int32(function.Spec.MinReplicas)
	if minReplicas == 0 {
		minReplicas = 1
	}

	targetCPU := int32(80)

	getHorizontalPodAutoscaler := func() (interface{}, error) {
		return c.clientSet.AutoscalingV1().HorizontalPodAutoscalers(function.Namespace).Get(function.Name,
			meta_v1.GetOptions{})
	}

	horizontalPodAutoscalerIsDeleting := func(resource interface{}) bool {
		return (resource).(*autos_v1.HorizontalPodAutoscaler).ObjectMeta.DeletionTimestamp != nil
	}

	createHorizontalPodAutoscaler := func() (interface{}, error) {
		return c.clientSet.AutoscalingV1().HorizontalPodAutoscalers(function.Namespace).Create(&autos_v1.HorizontalPodAutoscaler{
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

		return c.clientSet.AutoscalingV1().HorizontalPodAutoscalers(function.Namespace).Update(hpa)
	}

	resource, err := c.createOrUpdateResource("hpa",
		getHorizontalPodAutoscaler,
		horizontalPodAutoscalerIsDeleting,
		createHorizontalPodAutoscaler,
		updateHorizontalPodAutoscaler)

	if err != nil {
		return nil, err
	}

	return resource.(*autos_v1.HorizontalPodAutoscaler), err
}

func (c *Client) createOrUpdateIngress(labels map[string]string,
	function *functioncr.Function) (*ext_v1beta1.Ingress, error) {

	getIngress := func() (interface{}, error) {
		return c.clientSet.ExtensionsV1beta1().Ingresses(function.Namespace).Get(function.Name, meta_v1.GetOptions{})
	}

	ingressIsDeleting := func(resource interface{}) bool {
		return (resource).(*ext_v1beta1.Ingress).ObjectMeta.DeletionTimestamp != nil
	}

	createIngress := func() (interface{}, error) {
		spec := ext_v1beta1.IngressSpec{}

		c.populateIngressSpec(labels, function, &spec)

		return c.clientSet.ExtensionsV1beta1().Ingresses(function.Namespace).Create(&ext_v1beta1.Ingress{
			ObjectMeta: meta_v1.ObjectMeta{
				Name:      function.Name,
				Namespace: function.Namespace,
				Labels:    labels,
			},
			Spec: spec,
		})
	}

	updateIngress := func(resource interface{}) (interface{}, error) {
		ingress := resource.(*ext_v1beta1.Ingress)

		// update existing
		ingress.Labels = labels

		return c.clientSet.ExtensionsV1beta1().Ingresses(function.Namespace).Update(ingress)
	}

	resource, err := c.createOrUpdateResource("ingress",
		getIngress,
		ingressIsDeleting,
		createIngress,
		updateIngress)

	if err != nil {
		return nil, err
	}

	return resource.(*ext_v1beta1.Ingress), err
}

func (c *Client) initClassLabels() {

	// add class labels and prepare a label selector
	c.classLabels["serverless"] = "nuclio"
	c.classLabelSelector = ""

	for classKey, classValue := range c.classLabels {
		c.classLabelSelector += fmt.Sprintf("%s=%s,", classKey, classValue)
	}

	c.classLabelSelector = c.classLabelSelector[:len(c.classLabelSelector)-1]
}

func (c *Client) getFunctionLabels(function *functioncr.Function) map[string]string {
	result := map[string]string{}

	for labelKey, labelValue := range function.Labels {
		result[labelKey] = labelValue
	}

	for labelKey, labelValue := range c.classLabels {
		result[labelKey] = labelValue
	}

	return result
}

func (c *Client) getFunctionReplicas(function *functioncr.Function) int {
	replicas := function.Spec.Replicas

	if function.Spec.Disabled {
		replicas = 0
	} else if replicas == 0 {
		replicas = function.Spec.MinReplicas
	}

	return replicas
}

func (c *Client) getFunctionAnnotations(function *functioncr.Function) (map[string]string, error) {
	annotations := make(map[string]string)

	if function.Spec.Description != "" {
		annotations["description"] = function.Spec.Description
	}

	serializedFunctionJSON, err := c.serializeFunctionJSON(function)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get function as JSON")
	}

	annotations["func_json"] = serializedFunctionJSON
	annotations["func_gen"] = function.ResourceVersion

	return annotations, nil
}

func (c *Client) getFunctionEnvironment(labels map[string]string,
	function *functioncr.Function) []v1.EnvVar {
	env := function.Spec.Env

	env = append(env, v1.EnvVar{Name: "NUCLIO_FUNCTION_NAME", Value: labels["name"]})
	env = append(env, v1.EnvVar{Name: "NUCLIO_FUNCTION_VERSION", Value: labels["version"]})

	return env
}

func (c *Client) serializeFunctionJSON(function *functioncr.Function) (string, error) {
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

func (c *Client) populateServiceSpec(labels map[string]string,
	function *functioncr.Function,
	spec *v1.ServiceSpec) {

	spec.Selector = labels
	spec.Type = v1.ServiceTypeNodePort

	// update the service's node port on the following conditions:
	// 1. this is a new service (spec.Ports is an empty list)
	// 2. this is an existing service (spec.Ports is not an empty list) BUT not if the service already has a node port
	//    and the function specifies 0 (meaning auto assign). This is to prevent cases where service already has a node
	//    port and then updating it causes node port change
	if len(spec.Ports) == 0 || !(spec.Ports[0].NodePort != 0 && function.Spec.HTTPPort == 0) {
		spec.Ports = []v1.ServicePort{
			{Name: containerHTTPPortName, Port: int32(containerHTTPPort), NodePort: int32(function.Spec.HTTPPort)},
		}
	}
}

func (c *Client) populateIngressSpec(labels map[string]string,
	function *functioncr.Function,
	spec *ext_v1beta1.IngressSpec) {

	c.logger.DebugWith("Preparing ingress")

	// register the suffix as a default route
	defaultIngress := &functionconfig.Ingress{
		Paths: []string{fmt.Sprintf("/%s/%s", function.Name, labels["version"])},
	}

	c.addIngressToSpec(function, defaultIngress, spec)

	for _, ingress := range functionconfig.GetIngressesFromTriggers(function.Spec.Triggers) {
		c.addIngressToSpec(function, &ingress, spec)
	}
}

func (c *Client) addIngressToSpec(function *functioncr.Function,
	ingress *functionconfig.Ingress,
	spec *ext_v1beta1.IngressSpec) {

	c.logger.DebugWith("Adding ingress",
		"function", function.Name,
		"host", ingress.Host,
		"paths", ingress.Paths)

	ingressRule := ext_v1beta1.IngressRule{
		Host: ingress.Host,
	}

	ingressRule.IngressRuleValue.HTTP = &ext_v1beta1.HTTPIngressRuleValue{}

	// populate the ingress rule value
	for _, path := range ingress.Paths {
		httpIngressPath := ext_v1beta1.HTTPIngressPath{
			Path: path,
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
}

func (c *Client) populateDeploymentContainer(labels map[string]string,
	function *functioncr.Function,
	container *v1.Container) {

	volumeMount := v1.VolumeMount{}
	volumeMount.Name = processorConfigVolumeName
	volumeMount.MountPath = "/etc/nuclio"

	container.Image = function.Spec.ImageName
	container.Resources = function.Spec.Resources
	container.Env = c.getFunctionEnvironment(labels, function)
	container.VolumeMounts = []v1.VolumeMount{volumeMount}
	container.Ports = []v1.ContainerPort{
		{
			ContainerPort: containerHTTPPort,
		},
	}
}

func (c *Client) populateConfigMap(labels map[string]string,
	function *functioncr.Function,
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
			Name:      c.configMapNameFromFunctionName(function.Name),
			Namespace: function.Namespace,
		},
		Data: map[string]string{
			"processor.yaml": configMapContents.String(),
		},
	}

	return nil
}

func (c *Client) configMapNameFromFunctionName(functionName string) string {
	return functionName
}
