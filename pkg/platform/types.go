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

package platform

// use k8s structure definitions for now. In the future, duplicate them for cleanliness
import (
	"github.com/nuclio/nuclio/pkg/functionconfig"

	"github.com/nuclio/nuclio-sdk"
)

type BuildOptions struct {
	Logger         nuclio.Logger
	FunctionConfig functionconfig.Config
}

type DeployOptions struct {
	Logger         nuclio.Logger
	FunctionConfig functionconfig.Config
}

type UpdateOptions struct {
	FunctionConfig functionconfig.Config
}

type DeleteOptions struct {
	FunctionConfig functionconfig.Config
}

// BuildResult holds information detected/generated as a result of a build process
type BuildResult struct {
	ImageName             string
	Runtime               string
	Handler               string
	UpdatedFunctionConfig functionconfig.Config
}

// DeployResult holds the results of a deploy
type DeployResult struct {
	BuildResult
	Port        int
	ContainerID string
}

// GetOptions is the base for all platform get options
type GetOptions struct {
	Name      string
	Namespace string
	NotList   bool
	Watch     bool
	Labels    string
	Format    string
}

// InvokeViaType defines via which mechanism the function will be invoked
type InvokeViaType int

const (
	InvokeViaAny InvokeViaType = iota
	InvokeViaExternalIP
	InvokeViaLoadBalancer
)

// InvokeOptions is the base for all platform invoke options
type InvokeOptions struct {
	Name         string
	Namespace    string
	ClusterIP    string
	ContentType  string
	Path         string
	Method       string
	Body         string
	Headers      string
	LogLevelName string
	Via          InvokeViaType
}

// AddressType
type AddressType int

const (
	AddressTypeInternalIP AddressType = iota
	AddressTypeExternalIP
)

// Address
type Address struct {
	Address string
	Type    AddressType
}
