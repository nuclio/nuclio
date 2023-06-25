/*
Copyright 2023 The Nuclio Authors.

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
