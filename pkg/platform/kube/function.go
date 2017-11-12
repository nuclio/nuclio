package kube

import (
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/kube/functioncr"

	"github.com/nuclio/nuclio-sdk"
	"k8s.io/api/apps/v1beta1"
	"k8s.io/api/core/v1"
	ext_v1beta1 "k8s.io/api/extensions/v1beta1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type function struct {
	platform.AbstractFunction
	functioncrInstance *functioncr.Function
	consumer           *consumer
	configuredReplicas int
	availableReplicas  int
	ingressAddress     string
}

func newFunction(parentLogger nuclio.Logger,
	parentPlatform platform.Platform,
	config *functionconfig.Config,
	functioncrInstance *functioncr.Function,
	consumer *consumer) (*function, error) {
	newAbstractFunction, err := platform.NewAbstractFunction(parentLogger, parentPlatform, config)
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
	var service *v1.Service
	var deployment *v1beta1.Deployment
	var ingress *ext_v1beta1.Ingress
	var serviceErr, deploymentErr, ingressErr error

	waitGroup := sync.WaitGroup{}

	// wait for service, ingress and deployment
	waitGroup.Add(3)

	// get service info
	go func() {
		if service == nil {
			service, serviceErr = f.consumer.clientset.CoreV1().
				Services(f.Config.Meta.Namespace).
				Get(f.Config.Meta.Name, meta_v1.GetOptions{})
		}

		// update HTTP port
		f.Config.Spec.HTTPPort = int(service.Spec.Ports[0].NodePort)

		waitGroup.Done()
	}()

	// get deployment info
	go func() {
		if deployment == nil {
			deployment, deploymentErr = f.consumer.clientset.AppsV1beta1().
				Deployments(f.Config.Meta.Namespace).
				Get(f.Config.Meta.Name, meta_v1.GetOptions{})
		}

		waitGroup.Done()
	}()

	go func() {
		if ingress == nil {
			ingress, ingressErr = f.consumer.clientset.ExtensionsV1beta1().
				Ingresses(f.Config.Meta.Namespace).
				Get(f.Config.Meta.Name, meta_v1.GetOptions{})
		}

		waitGroup.Done()
	}()

	// wait for all to finish
	waitGroup.Wait()

	// return the first error
	for _, err := range []error{
		serviceErr, deploymentErr, ingressErr,
	} {
		if err != nil {
			return err
		}
	}

	// update fields
	f.Config.Spec.HTTPPort = int(service.Spec.Ports[0].NodePort)
	f.availableReplicas = int(deployment.Status.AvailableReplicas)
	if deployment.Spec.Replicas != nil {
		f.configuredReplicas = int(*deployment.Spec.Replicas)
	}

	if len(ingress.Status.LoadBalancer.Ingress) > 0 {
		f.ingressAddress = ingress.Status.LoadBalancer.Ingress[0].IP
	}

	return nil
}

// GetState returns the state of the function
func (f *function) GetState() string {
	return string(f.functioncrInstance.Status.State)
}

// GetInvokeURL returns the URL on which the function can be invoked
func (f *function) GetInvokeURL(invokeViaType platform.InvokeViaType) (string, error) {
	host, port, path, err := f.getInvokeURLFields(invokeViaType)
	if err != nil {
		return "", errors.Wrap(err, "Failed to get address")
	}

	return fmt.Sprintf("%s:%d%s", host, port, path), nil
}

// GetReplicas returns the current # of replicas and the configured # of replicas
func (f *function) GetReplicas() (int, int) {
	return f.availableReplicas, f.configuredReplicas
}

func (f *function) getInvokeURLFields(invokeViaType platform.InvokeViaType) (string, int, string, error) {
	var host, path string
	var port int

	// user wants a specific invoke via type
	if invokeViaType != platform.InvokeViaAny {

		// if there's an ingress address, use that. otherwise use
		switch invokeViaType {
		case platform.InvokeViaLoadBalancer:
			host, port, path = f.getIngressInvokeURL()
		case platform.InvokeViaExternalIP:
			host, port, path = f.getExternalIPInvokeURL()
		}

		// if host is empty and we were configured to a specific via type, return an error
		if host == "" {
			return "", 0, "", errors.New("Couldn't find address for invoke via type")
		}

		return host, port, path, nil
	}

	// try to get host, port, and path in through ingress and then via external ip
	for _, urlGetter := range []func() (string, int, string){
		f.getIngressInvokeURL,
		f.getExternalIPInvokeURL,
	} {

		// get the info
		host, port, path = urlGetter()

		// if we found something, return it
		if host != "" {
			return host, port, path, nil
		}
	}

	return "", 0, "", errors.New("Could not find address")
}

func (f *function) getIngressInvokeURL() (string, int, string) {
	if f.ingressAddress != "" {

		// 80 seems to be hardcoded in kubectl as well
		return f.ingressAddress,
			80,
			fmt.Sprintf("/%s/%s", f.Config.Meta.Name, f.GetVersion())
	}

	return "", 0, ""
}

func (f *function) getExternalIPInvokeURL() (string, int, string) {
	nodes, err := f.Platform.GetNodes()
	if err != nil {
		return "", 0, ""
	}

	// try to get an external IP address from one of the nodes. if that doesn't work,
	// try to get an internal IP
	for _, addressType := range []platform.AddressType{
		platform.AddressTypeExternalIP,
		platform.AddressTypeInternalIP} {

		for _, node := range nodes {
			for _, address := range node.GetAddresses() {
				if address.Type == addressType {
					return address.Address, f.Config.Spec.HTTPPort, ""
				}
			}
		}
	}

	// try to take from kube host as configured
	kubeURL, err := url.Parse(f.consumer.kubeHost)
	if err == nil && kubeURL.Host != "" {
		return strings.Split(kubeURL.Host, ":")[0], f.Config.Spec.HTTPPort, ""
	}

	return "", 0, ""
}
