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

package kube

import (
	"fmt"
	"sync"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"

	"github.com/nuclio/logger"
	"k8s.io/api/apps/v1beta1"
	ext_v1beta1 "k8s.io/api/extensions/v1beta1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type function struct {
	platform.AbstractFunction
	function           *nuclioio.Function
	consumer           *consumer
	configuredReplicas int
	availableReplicas  int
	ingressAddress     string
	httpPort           int
}

func newFunction(parentLogger logger.Logger,
	parentPlatform platform.Platform,
	nuclioioFunction *nuclioio.Function,
	consumer *consumer) (*function, error) {

	newFunction := &function{}

	// create a config from function
	functionConfig := functionconfig.Config{
		Meta: functionconfig.Meta{
			Name:      nuclioioFunction.Name,
			Namespace: nuclioioFunction.Namespace,
			Labels:    nuclioioFunction.Labels,
		},
		Spec: nuclioioFunction.Spec,
	}

	newAbstractFunction, err := platform.NewAbstractFunction(parentLogger,
		parentPlatform,
		&functionConfig,
		&nuclioioFunction.Status,
		newFunction)

	if err != nil {
		return nil, err
	}

	newFunction.AbstractFunction = *newAbstractFunction
	newFunction.function = nuclioioFunction
	newFunction.consumer = consumer

	return newFunction, nil
}

// Initialize loads sub-resources so we can populate our configuration
func (f *function) Initialize([]string) error {
	var deployment *v1beta1.Deployment
	var ingress *ext_v1beta1.Ingress
	var deploymentErr, ingressErr error

	waitGroup := sync.WaitGroup{}

	// wait for service, ingress and deployment
	waitGroup.Add(2)

	// get deployment info
	go func() {
		if deployment == nil {
			deployment, deploymentErr = f.consumer.kubeClientSet.AppsV1beta1().
				Deployments(f.Config.Meta.Namespace).
				Get(f.Config.Meta.Name, meta_v1.GetOptions{})
		}

		waitGroup.Done()
	}()

	go func() {
		if ingress == nil {
			ingress, ingressErr = f.consumer.kubeClientSet.ExtensionsV1beta1().
				Ingresses(f.Config.Meta.Namespace).
				Get(f.Config.Meta.Name, meta_v1.GetOptions{})
		}

		waitGroup.Done()
	}()

	// wait for all to finish
	waitGroup.Wait()

	// return the first error
	for _, err := range []error{
		deploymentErr,
	} {
		if err != nil {
			return err
		}
	}

	// update fields
	f.availableReplicas = int(deployment.Status.AvailableReplicas)
	if deployment.Spec.Replicas != nil {
		f.configuredReplicas = int(*deployment.Spec.Replicas)
	}

	if ingressErr != nil && ingress != nil && len(ingress.Status.LoadBalancer.Ingress) > 0 {
		f.ingressAddress = ingress.Status.LoadBalancer.Ingress[0].IP
	}

	return nil
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

func (f *function) GetConfig() *functionconfig.Config {
	return &functionconfig.Config{
		Meta: functionconfig.Meta{
			Name:        f.function.Name,
			Namespace:   f.function.Namespace,
			Labels:      f.function.Labels,
			Annotations: f.function.Annotations,
		},
		Spec: f.function.Spec,
	}
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
		case platform.InvokeViaDomainName:
			host, port, path = f.getDomainNameInvokeURL()
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
	host, port, err := f.GetExternalIPInvocationURL()
	if err != nil {
		return "", 0, ""
	}

	// return it and the port
	return host, port, ""
}

func (f *function) getDomainNameInvokeURL() (string, int, string) {
	namespace := f.function.ObjectMeta.Namespace
	if namespace == "" {
		namespace = "nuclio"
	}

	domainName := fmt.Sprintf("%s.%s.svc.cluster.local", f.function.ObjectMeta.Name, namespace)

	return domainName, 8080, ""
}
