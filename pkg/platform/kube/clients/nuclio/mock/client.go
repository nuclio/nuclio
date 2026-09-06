/*
Copyright 2026 The Nuclio Authors.

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

package mock

import (
	"context"

	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"

	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Client mocks nuclioclient.Client.
type Client struct {
	mock.Mock
}

func (c *Client) GetNuclioFunction(ctx context.Context, namespace, name string) (*nuclioio.NuclioFunction, error) {
	args := c.Called(ctx, namespace, name)
	function, _ := args.Get(0).(*nuclioio.NuclioFunction)
	return function, args.Error(1)
}

func (c *Client) ListNuclioFunctions(ctx context.Context, namespace string, opts metav1.ListOptions) (*nuclioio.NuclioFunctionList, error) {
	args := c.Called(ctx, namespace, opts)
	list, _ := args.Get(0).(*nuclioio.NuclioFunctionList)
	return list, args.Error(1)
}

func (c *Client) CreateNuclioFunction(ctx context.Context, namespace string, function *nuclioio.NuclioFunction) (*nuclioio.NuclioFunction, error) {
	args := c.Called(ctx, namespace, function)
	created, _ := args.Get(0).(*nuclioio.NuclioFunction)
	return created, args.Error(1)
}

func (c *Client) UpdateNuclioFunction(ctx context.Context, namespace string, function *nuclioio.NuclioFunction) (*nuclioio.NuclioFunction, error) {
	args := c.Called(ctx, namespace, function)
	updated, _ := args.Get(0).(*nuclioio.NuclioFunction)
	return updated, args.Error(1)
}

func (c *Client) DeleteNuclioFunction(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) error {
	args := c.Called(ctx, namespace, name, opts)
	return args.Error(0)
}

func (c *Client) GetNuclioProject(ctx context.Context, namespace, name string) (*nuclioio.NuclioProject, error) {
	args := c.Called(ctx, namespace, name)
	project, _ := args.Get(0).(*nuclioio.NuclioProject)
	return project, args.Error(1)
}

func (c *Client) ListNuclioProjects(ctx context.Context, namespace string, opts metav1.ListOptions) (*nuclioio.NuclioProjectList, error) {
	args := c.Called(ctx, namespace, opts)
	list, _ := args.Get(0).(*nuclioio.NuclioProjectList)
	return list, args.Error(1)
}

func (c *Client) CreateNuclioProject(ctx context.Context, namespace string, project *nuclioio.NuclioProject) (*nuclioio.NuclioProject, error) {
	args := c.Called(ctx, namespace, project)
	created, _ := args.Get(0).(*nuclioio.NuclioProject)
	return created, args.Error(1)
}

func (c *Client) UpdateNuclioProject(ctx context.Context, namespace string, project *nuclioio.NuclioProject) (*nuclioio.NuclioProject, error) {
	args := c.Called(ctx, namespace, project)
	updated, _ := args.Get(0).(*nuclioio.NuclioProject)
	return updated, args.Error(1)
}

func (c *Client) DeleteNuclioProject(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) error {
	args := c.Called(ctx, namespace, name, opts)
	return args.Error(0)
}

func (c *Client) GetNuclioAPIGateway(ctx context.Context, namespace, name string) (*nuclioio.NuclioAPIGateway, error) {
	args := c.Called(ctx, namespace, name)
	apiGateway, _ := args.Get(0).(*nuclioio.NuclioAPIGateway)
	return apiGateway, args.Error(1)
}

func (c *Client) ListNuclioAPIGateways(ctx context.Context, namespace string, opts metav1.ListOptions) (*nuclioio.NuclioAPIGatewayList, error) {
	args := c.Called(ctx, namespace, opts)
	list, _ := args.Get(0).(*nuclioio.NuclioAPIGatewayList)
	return list, args.Error(1)
}

func (c *Client) CreateNuclioAPIGateway(ctx context.Context, namespace string, apiGateway *nuclioio.NuclioAPIGateway) (*nuclioio.NuclioAPIGateway, error) {
	args := c.Called(ctx, namespace, apiGateway)
	created, _ := args.Get(0).(*nuclioio.NuclioAPIGateway)
	return created, args.Error(1)
}

func (c *Client) UpdateNuclioAPIGateway(ctx context.Context, namespace string, apiGateway *nuclioio.NuclioAPIGateway) (*nuclioio.NuclioAPIGateway, error) {
	args := c.Called(ctx, namespace, apiGateway)
	updated, _ := args.Get(0).(*nuclioio.NuclioAPIGateway)
	return updated, args.Error(1)
}

func (c *Client) DeleteNuclioAPIGateway(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) error {
	args := c.Called(ctx, namespace, name, opts)
	return args.Error(0)
}

func (c *Client) GetNuclioFunctionEvent(ctx context.Context, namespace, name string) (*nuclioio.NuclioFunctionEvent, error) {
	args := c.Called(ctx, namespace, name)
	functionEvent, _ := args.Get(0).(*nuclioio.NuclioFunctionEvent)
	return functionEvent, args.Error(1)
}

func (c *Client) ListNuclioFunctionEvents(ctx context.Context, namespace string, opts metav1.ListOptions) (*nuclioio.NuclioFunctionEventList, error) {
	args := c.Called(ctx, namespace, opts)
	list, _ := args.Get(0).(*nuclioio.NuclioFunctionEventList)
	return list, args.Error(1)
}

func (c *Client) CreateNuclioFunctionEvent(ctx context.Context, namespace string, functionEvent *nuclioio.NuclioFunctionEvent) (*nuclioio.NuclioFunctionEvent, error) {
	args := c.Called(ctx, namespace, functionEvent)
	created, _ := args.Get(0).(*nuclioio.NuclioFunctionEvent)
	return created, args.Error(1)
}

func (c *Client) UpdateNuclioFunctionEvent(ctx context.Context, namespace string, functionEvent *nuclioio.NuclioFunctionEvent) (*nuclioio.NuclioFunctionEvent, error) {
	args := c.Called(ctx, namespace, functionEvent)
	updated, _ := args.Get(0).(*nuclioio.NuclioFunctionEvent)
	return updated, args.Error(1)
}

func (c *Client) DeleteNuclioFunctionEvent(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) error {
	args := c.Called(ctx, namespace, name, opts)
	return args.Error(0)
}
