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

package client

import (
	"context"
	"fmt"
	"sync"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const FunctionContainerName = "nuclio"

type Function struct {
	platform.AbstractFunction
	function           *nuclioio.NuclioFunction
	consumer           *Consumer
	configuredReplicas int
	availableReplicas  int
	ingressAddress     string
	httpPort           int
	service            *v1.Service
}

func NewFunction(parentLogger logger.Logger,
	parentPlatform platform.Platform,
	nuclioioFunction *nuclioio.NuclioFunction,
	consumer *Consumer) (*Function, error) {

	newFunction := &Function{}

	newAbstractFunction, err := platform.NewAbstractFunction(parentLogger,
		parentPlatform,
		NuclioioToFunctionConfig(nuclioioFunction),
		&nuclioioFunction.Status,
		newFunction)

	if err != nil {
		return nil, err
	}

	newFunction.AbstractFunction = *newAbstractFunction
	newFunction.function = nuclioioFunction
	newFunction.consumer = consumer
	newFunction.httpPort = nuclioioFunction.Status.HTTPPort

	return newFunction, nil
}

func NuclioioToFunctionConfig(nuclioioFunction *nuclioio.NuclioFunction) *functionconfig.Config {
	return &functionconfig.Config{
		Meta: functionconfig.Meta{
			Name:            nuclioioFunction.Name,
			Namespace:       nuclioioFunction.Namespace,
			Labels:          nuclioioFunction.Labels,
			Annotations:     nuclioioFunction.Annotations,
			ResourceVersion: nuclioioFunction.ResourceVersion,
		},
		Spec: nuclioioFunction.Spec,
	}
}

// Initialize loads sub-resources so we can populate our configuration
func (f *Function) Initialize(ctx context.Context, str []string) error {
	var deploymentList *appsv1.DeploymentList
	var ingressList *networkingv1.IngressList
	var serviceList *v1.ServiceList

	var deployment *appsv1.Deployment
	var ingress *networkingv1.Ingress
	var service *v1.Service
	var deploymentErr, ingressErr, serviceErr error

	waitGroup := sync.WaitGroup{}

	// wait for service, ingress and deployment
	waitGroup.Add(3)

	listOptions := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("nuclio.io/function-name=%s", f.Config.Meta.Name),
	}

	// get deployment info
	go func() {
		if deploymentList == nil {
			deploymentList, deploymentErr = f.consumer.KubeClientSet.AppsV1().
				Deployments(f.Config.Meta.Namespace).
				List(ctx, listOptions)

			if deploymentErr != nil {
				return
			}

			// there should be only one
			if len(deploymentList.Items) != 1 {
				deploymentErr = errors.Errorf("Found unexpected number of deployments for function %s: %d",
					f.function.Name,
					len(deploymentList.Items))
			} else {
				deployment = &deploymentList.Items[0]
			}
		}

		waitGroup.Done()
	}()

	// get service info
	go func() {
		if serviceList == nil {
			serviceList, serviceErr = f.consumer.KubeClientSet.CoreV1().
				Services(f.Config.Meta.Namespace).
				List(ctx, listOptions)

			if serviceErr != nil {
				return
			}

			// there should be only one
			if len(serviceList.Items) != 1 {
				serviceErr = errors.Errorf("Found unexpected number of services for function %s: %d",
					f.function.Name,
					len(serviceList.Items))
			} else {
				service = &serviceList.Items[0]
			}
		}

		waitGroup.Done()
	}()

	// get ingress info
	go func() {
		if ingressList == nil {
			ingressList, ingressErr = f.consumer.KubeClientSet.NetworkingV1().
				Ingresses(f.Config.Meta.Namespace).
				List(ctx, listOptions)

			if ingressErr != nil {
				return
			}

			// no more than one
			if len(ingressList.Items) > 1 {
				ingressErr = errors.Errorf("Found unexpected number of ingresses for function %s: %d",
					f.function.Name,
					len(ingressList.Items))

				// there can be 0
			} else if len(ingressList.Items) == 1 {
				ingress = &ingressList.Items[0]
			}
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
	f.service = service

	f.availableReplicas = int(deployment.Status.AvailableReplicas)
	if deployment.Spec.Replicas != nil {
		f.configuredReplicas = int(*deployment.Spec.Replicas)
	}

	if ingressErr == nil && ingress != nil && len(ingress.Status.LoadBalancer.Ingress) > 0 {
		f.ingressAddress = ingress.Status.LoadBalancer.Ingress[0].IP
	}

	return nil
}

// GetInvokeURL returns the URL on which the function can be invoked
func (f *Function) GetInvokeURL(ctx context.Context, invokeViaType platform.InvokeViaType) (string, error) {
	host, port, path, err := f.getInvokeURLFields(ctx, invokeViaType)
	if err != nil {
		return "", errors.Wrap(err, "Failed to get address")
	}

	return fmt.Sprintf("%s:%d%s", host, port, path), nil
}

// GetReplicas returns the current # of replicas and the configured # of replicas
func (f *Function) GetReplicas() (int, int) {
	return f.availableReplicas, f.configuredReplicas
}

func (f *Function) GetConfig() *functionconfig.Config {
	return &functionconfig.Config{
		Meta: functionconfig.Meta{
			Name:            f.function.Name,
			Namespace:       f.function.Namespace,
			Labels:          f.function.Labels,
			Annotations:     f.function.Annotations,
			ResourceVersion: f.function.ResourceVersion,
		},
		Spec: f.function.Spec,
	}
}

func (f *Function) getInvokeURLFields(ctx context.Context, invokeViaType platform.InvokeViaType) (string, int, string, error) {
	var host, path string
	var port int
	var err error

	// user wants a specific invoke via type
	if invokeViaType != platform.InvokeViaAny {

		// if there's an ingress address, use that. otherwise use
		switch invokeViaType {
		case platform.InvokeViaLoadBalancer:
			host, port, path, err = f.getIngressInvokeURL()
		case platform.InvokeViaExternalIP:
			host, port, path, err = f.getExternalIPInvokeURL()
		case platform.InvokeViaDomainName:
			host, port, path, err = f.getDomainNameInvokeURL()
		}

		if err != nil {
			return "", 0, "", errors.Wrap(err, "Failed to get invoke URL")
		}

		// if host is empty and we were configured to a specific via type, return an error
		if host == "" {
			return "", 0, "", errors.New("Couldn't find address for invoke via type")
		}

		return host, port, path, nil
	}

	// try to get host, port, and path in through ingress and then via external ip
	for urlGetterIndex, urlGetter := range []func() (string, int, string, error){
		f.getExternalIPInvokeURL,
		f.getDomainNameInvokeURL,
	} {

		// get the info
		host, port, path, err = urlGetter()

		if err != nil {
			f.Logger.DebugWithCtx(ctx, "Could not get invoke URL with method",
				"index", urlGetterIndex,
				"err", err)
		}

		// if we found something, return it
		if host != "" {
			f.Logger.DebugWithCtx(ctx, "Resolved invoke URL with method",
				"index", urlGetterIndex,
				"host", host,
				"port", port,
				"path", path)

			return host, port, path, nil
		}
	}

	return "", 0, "", errors.New("Could not resolve invoke URL")
}

func (f *Function) getIngressInvokeURL() (string, int, string, error) {
	if f.ingressAddress != "" {

		// 80 seems to be hardcoded in kubectl as well
		return f.ingressAddress,
			80,
			fmt.Sprintf("/%s/%s", f.Config.Meta.Name, f.GetVersion()),
			nil
	}

	return "", 0, "", nil
}

func (f *Function) getExternalIPInvokeURL() (string, int, string, error) {
	host, port, err := f.GetExternalIPInvocationURL()
	if err != nil {
		return "", 0, "", errors.Wrap(err, "Failed to get external IP invocation URL")
	}

	// return it and the port
	return host, port, "", nil
}

func (f *Function) getDomainNameInvokeURL() (string, int, string, error) {
	host, port := GetDomainNameInvokeURL(f.service.Name, f.function.Namespace)
	return host, port, "", nil
}

func GetDomainNameInvokeURL(serviceName, namespace string) (string, int) {
	var domainName string

	if namespace == "" {
		domainName = serviceName
	} else {
		domainName = fmt.Sprintf("%s.%s.svc.cluster.local",
			serviceName,
			namespace)
	}

	return domainName, 8080
}
