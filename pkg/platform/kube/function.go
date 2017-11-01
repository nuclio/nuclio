package kube

import (
	"net/url"
	"strings"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/kube/functioncr"

	"k8s.io/api/apps/v1beta1"
	"k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type function struct {
	platform.AbstractFunction
	functioncrInstance functioncr.Function
	consumer   *consumer
	service    *v1.Service
	deployment *v1beta1.Deployment
}

// Initialize does nothing, seeing how no fields require lazy loading
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
