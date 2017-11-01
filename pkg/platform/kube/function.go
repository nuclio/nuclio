package kube

import (
	"net/url"
	"strings"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/kube/functioncr"

	"github.com/nuclio/nuclio-sdk"
	"k8s.io/api/apps/v1beta1"
	"k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type function struct {
	platform.AbstractFunction
	functioncrInstance *functioncr.Function
	consumer           *consumer
	service            *v1.Service
	deployment         *v1beta1.Deployment
}

func newFunction(parentLogger nuclio.Logger,
	config *functionconfig.Config,
	functioncrInstance *functioncr.Function,
	consumer *consumer) (*function, error) {
	newAbstractFunction, err := platform.NewAbstractFunction(parentLogger, config)
	if err != nil {
		return nil, err
	}

	newFunction := &function{
		AbstractFunction:   *newAbstractFunction,
		functioncrInstance: functioncrInstance,
		consumer:           consumer,
	}

	return newFunction, nil
}

// Initialize loads sub-resources so we can populate our configuration
func (f *function) Initialize([]string) error {
	var err error

	if f.service == nil {
		f.service, err = f.consumer.clientset.CoreV1().Services(f.Config.Meta.Namespace).Get(f.Config.Meta.Name, meta_v1.GetOptions{})
		if err != nil {
			return errors.Wrap(err, "Failed to get service")
		}
	}

	if f.deployment == nil {
		f.deployment, err = f.consumer.clientset.AppsV1beta1().Deployments(f.Config.Meta.Namespace).Get(f.Config.Meta.Name, meta_v1.GetOptions{})
		if err != nil {
			return errors.Wrap(err, "Failed to get deployment")
		}
	}

	// read HTTP port from service
	f.Config.Spec.HTTPPort = int(f.service.Spec.Ports[0].NodePort)

	return nil
}

// GetState returns the state of the function
func (f *function) GetState() string {
	return string(f.functioncrInstance.Status.State)
}

// GetClusterIP gets the IP of the cluster hosting the function
func (f *function) GetClusterIP() string {
	url, err := url.Parse(f.consumer.kubeHost)
	if err == nil && url.Host != "" {
		return strings.Split(url.Host, ":")[0]
	}

	// TODO: ?
	return ""
}

// GetReplicas returns the current # of replicas and the configured # of replicas
func (f *function) GetReplicas() (int, int) {
	if f.deployment == nil {
		return -1, -1
	}

	return int(f.deployment.Status.AvailableReplicas), int(*f.deployment.Spec.Replicas)
}
