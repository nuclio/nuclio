/*
Copyright 2025 The Nuclio Authors.

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

package nuclio

import (
	"context"
	"time"

	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"github.com/nuclio/nuclio/pkg/platform/kube/clients"
	nuclioioclient "github.com/nuclio/nuclio/pkg/platform/kube/clients/nuclio/clientset/versioned"

	"github.com/nuclio/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

type clientWithRetry struct {
	nuclioioclient.Interface
	retries int
	delay   time.Duration
}

func NewClientWithRetryFromConfig(config *rest.Config) (Client, error) {
	nuclioClient, err := nuclioioclient.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create Nuclio client")
	}
	return &clientWithRetry{
		Interface: nuclioClient,
		retries:   clients.MaxRetries,
		delay:     clients.Delay,
	}, nil
}

func NewClientWithRetryFromClient(nuclioClient nuclioioclient.Interface) Client {
	return &clientWithRetry{
		Interface: nuclioClient,
		retries:   clients.MaxRetries,
		delay:     clients.Delay,
	}
}

// --- NuclioFunction methods ---

func (c *clientWithRetry) GetNuclioFunction(ctx context.Context, namespace, name string) (*nuclioio.NuclioFunction, error) {
	return clients.RequestWithRetry[*nuclioio.NuclioFunction](func() (*nuclioio.NuclioFunction, error) {
		return c.NuclioV1beta1().NuclioFunctions(namespace).Get(ctx, name, metav1.GetOptions{})
	}, c.retries, c.delay)
}

func (c *clientWithRetry) ListNuclioFunctions(ctx context.Context, namespace string, opts metav1.ListOptions) (*nuclioio.NuclioFunctionList, error) {
	return clients.RequestWithRetry[*nuclioio.NuclioFunctionList](func() (*nuclioio.NuclioFunctionList, error) {
		return c.NuclioV1beta1().NuclioFunctions(namespace).List(ctx, opts)
	}, c.retries, c.delay)
}

func (c *clientWithRetry) CreateNuclioFunction(ctx context.Context, namespace string, function *nuclioio.NuclioFunction) (*nuclioio.NuclioFunction, error) {
	return clients.RequestWithRetry[*nuclioio.NuclioFunction](func() (*nuclioio.NuclioFunction, error) {
		return c.NuclioV1beta1().NuclioFunctions(namespace).Create(ctx, function, metav1.CreateOptions{})
	}, c.retries, c.delay)
}

func (c *clientWithRetry) UpdateNuclioFunction(ctx context.Context, namespace string, function *nuclioio.NuclioFunction) (*nuclioio.NuclioFunction, error) {
	return clients.RequestWithRetry[*nuclioio.NuclioFunction](func() (*nuclioio.NuclioFunction, error) {
		return c.NuclioV1beta1().NuclioFunctions(namespace).Update(ctx, function, metav1.UpdateOptions{})
	}, c.retries, c.delay)
}

func (c *clientWithRetry) DeleteNuclioFunction(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) (err error) {
	_, err = clients.RequestWithRetry(func() (any, error) {
		return nil, c.NuclioV1beta1().NuclioFunctions(namespace).Delete(ctx, name, opts)
	}, c.retries, c.delay)
	return
}

// --- NuclioProject methods ---

func (c *clientWithRetry) GetNuclioProject(ctx context.Context, namespace, name string) (*nuclioio.NuclioProject, error) {
	return clients.RequestWithRetry[*nuclioio.NuclioProject](func() (*nuclioio.NuclioProject, error) {
		return c.NuclioV1beta1().NuclioProjects(namespace).Get(ctx, name, metav1.GetOptions{})
	}, c.retries, c.delay)
}

func (c *clientWithRetry) ListNuclioProjects(ctx context.Context, namespace string, opts metav1.ListOptions) (*nuclioio.NuclioProjectList, error) {
	return clients.RequestWithRetry[*nuclioio.NuclioProjectList](func() (*nuclioio.NuclioProjectList, error) {
		return c.NuclioV1beta1().NuclioProjects(namespace).List(ctx, opts)
	}, c.retries, c.delay)
}

func (c *clientWithRetry) CreateNuclioProject(ctx context.Context, namespace string, project *nuclioio.NuclioProject) (*nuclioio.NuclioProject, error) {
	return clients.RequestWithRetry[*nuclioio.NuclioProject](func() (*nuclioio.NuclioProject, error) {
		return c.NuclioV1beta1().NuclioProjects(namespace).Create(ctx, project, metav1.CreateOptions{})
	}, c.retries, c.delay)
}

func (c *clientWithRetry) UpdateNuclioProject(ctx context.Context, namespace string, project *nuclioio.NuclioProject) (*nuclioio.NuclioProject, error) {
	return clients.RequestWithRetry[*nuclioio.NuclioProject](func() (*nuclioio.NuclioProject, error) {
		return c.NuclioV1beta1().NuclioProjects(namespace).Update(ctx, project, metav1.UpdateOptions{})
	}, c.retries, c.delay)
}

func (c *clientWithRetry) DeleteNuclioProject(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) (err error) {
	_, err = clients.RequestWithRetry(func() (any, error) {
		return nil, c.NuclioV1beta1().NuclioProjects(namespace).Delete(ctx, name, opts)
	}, c.retries, c.delay)
	return
}

// --- NuclioAPIGateway methods ---

func (c *clientWithRetry) GetNuclioAPIGateway(ctx context.Context, namespace, name string) (*nuclioio.NuclioAPIGateway, error) {
	return clients.RequestWithRetry[*nuclioio.NuclioAPIGateway](func() (*nuclioio.NuclioAPIGateway, error) {
		return c.NuclioV1beta1().NuclioAPIGateways(namespace).Get(ctx, name, metav1.GetOptions{})
	}, c.retries, c.delay)
}

func (c *clientWithRetry) ListNuclioAPIGateways(ctx context.Context, namespace string, opts metav1.ListOptions) (*nuclioio.NuclioAPIGatewayList, error) {
	return clients.RequestWithRetry[*nuclioio.NuclioAPIGatewayList](func() (*nuclioio.NuclioAPIGatewayList, error) {
		return c.NuclioV1beta1().NuclioAPIGateways(namespace).List(ctx, opts)
	}, c.retries, c.delay)
}

func (c *clientWithRetry) CreateNuclioAPIGateway(ctx context.Context, namespace string, apiGateway *nuclioio.NuclioAPIGateway) (*nuclioio.NuclioAPIGateway, error) {
	return clients.RequestWithRetry[*nuclioio.NuclioAPIGateway](func() (*nuclioio.NuclioAPIGateway, error) {
		return c.NuclioV1beta1().NuclioAPIGateways(namespace).Create(ctx, apiGateway, metav1.CreateOptions{})
	}, c.retries, c.delay)
}

func (c *clientWithRetry) UpdateNuclioAPIGateway(ctx context.Context, namespace string, apiGateway *nuclioio.NuclioAPIGateway) (*nuclioio.NuclioAPIGateway, error) {
	return clients.RequestWithRetry[*nuclioio.NuclioAPIGateway](func() (*nuclioio.NuclioAPIGateway, error) {
		return c.NuclioV1beta1().NuclioAPIGateways(namespace).Update(ctx, apiGateway, metav1.UpdateOptions{})
	}, c.retries, c.delay)
}

func (c *clientWithRetry) DeleteNuclioAPIGateway(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) (err error) {
	_, err = clients.RequestWithRetry(func() (any, error) {
		return nil, c.NuclioV1beta1().NuclioAPIGateways(namespace).Delete(ctx, name, opts)
	}, c.retries, c.delay)
	return
}

// --- NuclioFunctionEvent methods ---

func (c *clientWithRetry) GetNuclioFunctionEvent(ctx context.Context, namespace, name string) (*nuclioio.NuclioFunctionEvent, error) {
	return clients.RequestWithRetry[*nuclioio.NuclioFunctionEvent](func() (*nuclioio.NuclioFunctionEvent, error) {
		return c.NuclioV1beta1().NuclioFunctionEvents(namespace).Get(ctx, name, metav1.GetOptions{})
	}, c.retries, c.delay)
}

func (c *clientWithRetry) ListNuclioFunctionEvents(ctx context.Context, namespace string, opts metav1.ListOptions) (*nuclioio.NuclioFunctionEventList, error) {
	return clients.RequestWithRetry[*nuclioio.NuclioFunctionEventList](func() (*nuclioio.NuclioFunctionEventList, error) {
		return c.NuclioV1beta1().NuclioFunctionEvents(namespace).List(ctx, opts)
	}, c.retries, c.delay)
}

func (c *clientWithRetry) CreateNuclioFunctionEvent(ctx context.Context, namespace string, functionEvent *nuclioio.NuclioFunctionEvent) (*nuclioio.NuclioFunctionEvent, error) {
	return clients.RequestWithRetry[*nuclioio.NuclioFunctionEvent](func() (*nuclioio.NuclioFunctionEvent, error) {
		return c.NuclioV1beta1().NuclioFunctionEvents(namespace).Create(ctx, functionEvent, metav1.CreateOptions{})
	}, c.retries, c.delay)
}

func (c *clientWithRetry) UpdateNuclioFunctionEvent(ctx context.Context, namespace string, functionEvent *nuclioio.NuclioFunctionEvent) (*nuclioio.NuclioFunctionEvent, error) {
	return clients.RequestWithRetry[*nuclioio.NuclioFunctionEvent](func() (*nuclioio.NuclioFunctionEvent, error) {
		return c.NuclioV1beta1().NuclioFunctionEvents(namespace).Update(ctx, functionEvent, metav1.UpdateOptions{})
	}, c.retries, c.delay)
}

func (c *clientWithRetry) DeleteNuclioFunctionEvent(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) (err error) {
	_, err = clients.RequestWithRetry(func() (any, error) {
		return nil, c.NuclioV1beta1().NuclioFunctionEvents(namespace).Delete(ctx, name, opts)
	}, c.retries, c.delay)
	return
}
