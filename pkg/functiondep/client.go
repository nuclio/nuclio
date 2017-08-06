package functiondep

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/nuclio/nuclio-sdk"

	"github.com/nuclio/nuclio/pkg/functioncr"
	"github.com/pkg/errors"
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
	err := c.clientSet.Autoscaling().HorizontalPodAutoscalers(namespace).Delete(name, deleteOptions)
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

	service, err := c.clientSet.CoreV1().Services(function.Namespace).Get(function.Name, meta_v1.GetOptions{})
	if err != nil {

		// if not found, we need to create
		if apierrors.IsNotFound(err) {
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

		return nil, errors.Wrap(err, "Failed to get service")
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

	replicas := int32(c.getFunctionReplicas(function))
	annotations, err := c.getFunctionAnnotations(function)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get function annotations")
	}

	deployment, err := c.clientSet.AppsV1beta1().Deployments(function.Namespace).Get(function.Name, meta_v1.GetOptions{})
	if err != nil {

		if apierrors.IsNotFound(err) {

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

		return nil, errors.Wrap(err, "Failed to get deployment")
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

	c.logger.DebugWith("Service updated", "deployment", deployment)

	return deployment, nil
}

func (c *Client) createOrUpdateHorizontalPodAutoscaler(labels map[string]string,
	function *functioncr.Function) (*autos_v1.HorizontalPodAutoscaler, error) {

	hpa, err := c.clientSet.AutoscalingV1().HorizontalPodAutoscalers(function.Namespace).Get(function.Name,
		meta_v1.GetOptions{})

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

	env = append(env, v1.EnvVar{Name: "NUCLIO_REGION", Value: "local"})
	env = append(env, v1.EnvVar{Name: "NUCLIO_LOG_STREAM_NAME", Value: "local"})
	env = append(env, v1.EnvVar{Name: "NUCLIO_DLQ_STREAM_NAME", Value: ""})
	env = append(env, v1.EnvVar{Name: "NUCLIO_FUNCTION_NAME", Value: labels["function"]})
	env = append(env, v1.EnvVar{Name: "NUCLIO_FUNCTION_VERSION", Value: labels["version"]})
	env = append(env, v1.EnvVar{Name: "NUCLIO_FUNCTION_MEMORY_SIZE", Value: "TBD"})

	env = append(env, v1.EnvVar{Name: "IGZ_ACCESS_KEY", Value: "TBD"})
	env = append(env, v1.EnvVar{Name: "IGZ_ACCESS_SECRET", Value: "TBD"})
	env = append(env, v1.EnvVar{Name: "IGZ_SESSION_TOKEN", Value: "TBD"})
	env = append(env, v1.EnvVar{Name: "IGZ_SECURITY_TOKEN", Value: "TBD"})

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
