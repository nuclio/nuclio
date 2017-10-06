package kube

import (
	"github.com/nuclio/nuclio/pkg/platform/kube/functioncr"
	"github.com/nuclio/nuclio/pkg/errors"

	"k8s.io/api/core/v1"
	"k8s.io/api/apps/v1beta1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type function struct {
	functioncr.Function
	consumer *consumer
	service *v1.Service
	deployment *v1beta1.Deployment
}

// Initialize does nothing, seeing how no fields require lazy loading
func (f *function) Initialize([]string) error {
	var err error

	if f.service == nil {
		f.service, err = f.consumer.clientset.CoreV1().Services(f.Namespace).Get(f.Name, meta_v1.GetOptions{})
		if err != nil {
			return errors.Wrap(err, "Failed to get service")
		}
	}

	if f.deployment == nil {
		f.deployment, err = f.consumer.clientset.AppsV1beta1().Deployments(f.Namespace).Get(f.Name, meta_v1.GetOptions{})
		if err != nil {
			return errors.Wrap(err, "Failed to get deployment")
		}
	}

	return nil
}

// GetNamespace returns the namespace of the function, if its part of a namespace
func (f *function) GetNamespace() string {
	return f.Namespace
}

// GetName returns the name of the function
func (f *function) GetName() string {
	return f.Function.Labels["name"]
}

// GetName returns the name of the function
func (f *function) GetVersion() string {
	return f.Function.Labels["version"]
}

// GetState returns the state of the function
func (f *function) GetState() string {
	return string(f.Status.State)
}

// GetHTTPPort returns the port of the HTTP event source
func (f *function) GetHTTPPort() int {
	if f.service == nil {
		return -1
	}

	return int(f.service.Spec.Ports[0].NodePort)
}

// GetLabels returns the function labels
func (f *function) GetLabels() map[string]string {
	return f.Labels
}

// GetReplicas returns the current # of replicas and the configured # of replicas
func (f *function) GetReplicas() (int, int) {
	if f.deployment == nil {
		return -1, -1
	}

	return int(f.deployment.Status.AvailableReplicas), int(*f.deployment.Spec.Replicas)
}
