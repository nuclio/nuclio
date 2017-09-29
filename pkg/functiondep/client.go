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
	"github.com/nuclio/nuclio/pkg/functioncr"

	"github.com/nuclio/nuclio-sdk"
	v1beta1 "k8s.io/api/apps/v1beta1"
	autos_v1 "k8s.io/api/autoscaling/v1"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	containerHTTPPort = 8080
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
		logger:      parentLogger.GetChild("functiondep").(nuclio.Logger),
		clientSet:   clientSet,
		classLabels: make(map[string]string),
	}

	newClient.initClassLabels()

	return newClient, nil
}

func (c *Client) List(namespace string) ([]v1beta1.Deployment, error) {
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

func (c *Client) Get(namespace string, name string) (*v1beta1.Deployment, error) {
	var result *v1beta1.Deployment

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

func (c *Client) CreateOrUpdate(function *functioncr.Function) (*v1beta1.Deployment, error) {

	// get labels from the function and add class labels
	labels := c.getFunctionLabels(function)

	// create or update the applicable service
	_, err := c.createOrUpdateService(labels, function)
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

	c.logger.Debug("Deployment created/updated")

	return deployment, nil
}

func (c *Client) Delete(namespace string, name string) error {
	propogationPolicy := meta_v1.DeletePropagationForeground
	deleteOptions := &meta_v1.DeleteOptions{
		PropagationPolicy: &propogationPolicy,
	}

	// Delete HPA if exists
	err := c.clientSet.AutoscalingV1().HorizontalPodAutoscalers(namespace).Delete(name, deleteOptions)
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

	c.logger.DebugWith("Deleted deployed function", "namespace", namespace, "name", name)

	return nil
}

func (c *Client) createOrUpdateService(labels map[string]string,
	function *functioncr.Function) (*v1.Service, error) {

	var service *v1.Service
	var err error

	//
	// TODO: do this nicer
	//
	deadline := time.Now().Add(1 * time.Minute)

	// get the service until it's not deleting
	for {
		service, err = c.clientSet.CoreV1().Services(function.Namespace).Get(function.Name, meta_v1.GetOptions{})

		// if there's no error and the service is deleting, we need to wait
		if err == nil && service.ObjectMeta.DeletionTimestamp != nil {
			c.logger.DebugWith("Service is deleting, waiting", "name", function.Name)

			// we need to wait a bit and try again
			time.Sleep(1 * time.Second)

			// if we passed the deadline
			if time.Now().After(deadline) {
				return nil, errors.New("Timed out waiting for service to delete")
			}

		} else {

			// there was either an error or the service exists and is not being deleted
			break
		}
	}

	// if there's an error
	if err != nil {

		// if there was an error and it wasn't not found - there was an error
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, errors.Wrap(err, "Failed to get service")
		}

		// just for logging
		if err == nil {
			c.logger.DebugWith("Service found, but is deleting", "name", function.Name)
		}

		spec := v1.ServiceSpec{}
		c.populateServiceSpec(labels, function, &spec)

		service, err := c.clientSet.CoreV1().Services(function.Namespace).Create(&v1.Service{
			ObjectMeta: meta_v1.ObjectMeta{
				Name:      function.Name,
				Namespace: function.Namespace,
				Labels:    labels,
			},
			Spec: spec,
		})

		if err != nil {
			return nil, errors.Wrap(err, "Failed to create service")
		}

		c.logger.DebugWith("Service created",
			"service", service,
			"http_port", function.Spec.HTTPPort)

		return service, nil
	}

	// update existing
	service.Labels = labels
	c.populateServiceSpec(labels, function, &service.Spec)

	service, err = c.clientSet.CoreV1().Services(function.Namespace).Update(service)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to update service")
	}

	c.logger.DebugWith("Service updated",
		"service", service,
		"http_port", function.Spec.HTTPPort)

	return service, nil
}

func (c *Client) createOrUpdateDeployment(labels map[string]string,
	function *functioncr.Function) (*v1beta1.Deployment, error) {
	var deployment *v1beta1.Deployment
	var err error

	replicas := int32(c.getFunctionReplicas(function))
	annotations, err := c.getFunctionAnnotations(function)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get function annotations")
	}

	//
	// TODO: do this nicer
	//
	deadline := time.Now().Add(1 * time.Minute)

	// get the service until it's not deleting
	for {
		deployment, err = c.clientSet.AppsV1beta1().Deployments(function.Namespace).Get(function.Name, meta_v1.GetOptions{})

		// if there's no error and the service is deleting, we need to wait
		if err == nil && deployment.ObjectMeta.DeletionTimestamp != nil {
			c.logger.DebugWith("Deployment is deleting, waiting", "name", function.Name)

			// we need to wait a bit and try again
			time.Sleep(1 * time.Second)

			// if we passed the deadline
			if time.Now().After(deadline) {
				return nil, errors.New("Timed out waiting for deployment to delete")
			}

		} else {

			// there was either an error or the service exists and is not being deleted
			break
		}
	}

	// if there's an error or if there's no error but the service is deleting
	if err != nil || deployment.ObjectMeta.DeletionTimestamp != nil {

		// if we got here because there was an error which is not "not found", return error
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, errors.Wrap(err, "Failed to get deployment")
		}

		// just for logging
		if err == nil {
			c.logger.DebugWith("Deployment found, but is deleting", "name", function.Name)
		}

		container := v1.Container{Name: "nuclio"}
		c.populateDeploymentContainer(labels, function, &container)

		deployment, err := c.clientSet.AppsV1beta1().Deployments(function.Namespace).Create(&v1beta1.Deployment{
			ObjectMeta: meta_v1.ObjectMeta{
				Name:        function.Name,
				Namespace:   function.Namespace,
				Labels:      labels,
				Annotations: annotations,
			},
			Spec: v1beta1.DeploymentSpec{
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
					},
				},
			},
		})

		if err != nil {
			return nil, errors.Wrap(err, "Failed to create deployment")
		}

		c.logger.DebugWith("Deployment created", "deployment", deployment)

		return deployment, nil
	}

	deployment.Labels = labels
	deployment.Annotations = annotations
	deployment.Spec.Replicas = &replicas
	deployment.Spec.Template.Labels = labels
	c.populateDeploymentContainer(labels, function, &deployment.Spec.Template.Spec.Containers[0])

	deployment, err = c.clientSet.AppsV1beta1().Deployments(function.Namespace).Update(deployment)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to update deployment")
	}

	c.logger.DebugWith("Deployment updated", "deployment", deployment)

	return deployment, nil
}

func (c *Client) createOrUpdateHorizontalPodAutoscaler(labels map[string]string,
	function *functioncr.Function) (*autos_v1.HorizontalPodAutoscaler, error) {

	hpa, err := c.clientSet.AutoscalingV1().HorizontalPodAutoscalers(function.Namespace).Get(function.Name,
		meta_v1.GetOptions{})

	// TODO: handle HPA deleting
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, errors.Wrap(err, "Failed to get HPA")
		} else {

			// signify that HPA doesn't exist
			hpa = nil
		}
	}

	// if an HPA exists and the replicas is non-zero
	if hpa != nil && function.Spec.Replicas != 0 {
		err = c.clientSet.AutoscalingV1().HorizontalPodAutoscalers(function.Namespace).Delete(function.Name,
			&meta_v1.DeleteOptions{})

		if err != nil {
			return nil, errors.Wrap(err, "Failed to delete unnecessary HPA")
		}

		c.logger.Debug("HPA found, yet not needed. Deleted")

		// HPA existed, but is no longer needed
		return nil, nil
	} else if hpa == nil && function.Spec.Replicas != 0 {
		c.logger.Debug("HPA didn't exist and isn't needed")

		// HPA didn't exist, and isn't needed
		return nil, nil
	}

	maxReplicas := function.Spec.MaxReplicas
	if maxReplicas == 0 {
		maxReplicas = 4
	}

	minReplicas := function.Spec.MinReplicas
	if minReplicas == 0 {
		minReplicas = 1
	}

	targetCPU := int32(80)

	// create new HPA
	if hpa == nil {
		hpa, err = c.clientSet.AutoscalingV1().HorizontalPodAutoscalers(function.Namespace).Create(&autos_v1.HorizontalPodAutoscaler{
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
					APIVersion: "apps/v1beta1",
					Kind:       "Deployment",
					Name:       function.Name,
				},
			},
		})

		if err != nil {
			return nil, errors.Wrap(err, "Failed to create HPA")
		}

		c.logger.DebugWith("Created HPA", "hpa", hpa)

	} else {

		hpa.Labels = labels
		hpa.Spec.MinReplicas = &minReplicas
		hpa.Spec.MaxReplicas = maxReplicas
		hpa.Spec.TargetCPUUtilizationPercentage = &targetCPU
		hpa, err = c.clientSet.AutoscalingV1().HorizontalPodAutoscalers(function.Namespace).Update(hpa)

		if err != nil {
			return nil, errors.Wrap(err, "Failed to update HPA")
		}

		c.logger.DebugWith("Updated HPA", "hpa", hpa)
	}

	return hpa, nil
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
	replicas := int(function.Spec.Replicas)

	if function.Spec.Disabled {
		replicas = 0
	} else if replicas == 0 {
		replicas = int(function.Spec.MinReplicas)
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

	// inject data binding environments
	for dataBindingName, dataBindingConfig := range function.Spec.DataBindings {
		prefix := fmt.Sprintf("NUCLIO_DATA_BINDING_%s_", dataBindingName)

		env = append(env, v1.EnvVar{Name: prefix + "CLASS", Value: dataBindingConfig.Class})
		env = append(env, v1.EnvVar{Name: prefix + "URL", Value: dataBindingConfig.Url})
	}

	// future stuff:
	// env = append(env, v1.EnvVar{Name: "NUCLIO_FUNCTION_MEMORY_SIZE", Value: "TBD"})
	// env = append(env, v1.EnvVar{Name: "NUCLIO_REGION", Value: "local"})
	// env = append(env, v1.EnvVar{Name: "NUCLIO_LOG_STREAM_NAME", Value: "local"})
	// env = append(env, v1.EnvVar{Name: "NUCLIO_DLQ_STREAM_NAME", Value: ""})
	// env = append(env, v1.EnvVar{Name: "IGZ_ACCESS_KEY", Value: "TBD"})
	// env = append(env, v1.EnvVar{Name: "IGZ_ACCESS_SECRET", Value: "TBD"})
	// env = append(env, v1.EnvVar{Name: "IGZ_SESSION_TOKEN", Value: "TBD"})
	// env = append(env, v1.EnvVar{Name: "IGZ_SECURITY_TOKEN", Value: "TBD"})

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

	return string(pbody.Bytes()), nil
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
			{Name: "web", Port: int32(containerHTTPPort), NodePort: function.Spec.HTTPPort},
		}
	}
}

func (c *Client) populateDeploymentContainer(labels map[string]string,
	function *functioncr.Function,
	container *v1.Container) {

	container.Image = function.Spec.Image
	container.Resources = function.Spec.Resources
	container.WorkingDir = function.Spec.WorkingDir
	container.Env = c.getFunctionEnvironment(labels, function)
	container.Ports = []v1.ContainerPort{
		{
			ContainerPort: containerHTTPPort,
		},
	}
}
