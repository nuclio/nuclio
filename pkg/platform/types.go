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
	"net/http"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"

	"github.com/nuclio/logger"
)

//
// Function
//

type CreateFunctionBuildOptions struct {
	Logger              logger.Logger
	FunctionConfig      functionconfig.Config
	PlatformName        string
	OnAfterConfigUpdate func(*functionconfig.Config) error
}

type CreateFunctionOptions struct {
	Logger               logger.Logger
	FunctionConfig       functionconfig.Config
	ReadinessTimeout     *time.Duration
	CreationStateUpdated chan bool
}

type UpdateFunctionOptions struct {
	FunctionMeta     *functionconfig.Meta
	FunctionSpec     *functionconfig.Spec
	FunctionStatus   *functionconfig.Status
	ReadinessTimeout *time.Duration
}

type DeleteFunctionOptions struct {
	FunctionConfig functionconfig.Config
}

// CreateFunctionBuildResult holds information detected/generated as a result of a build process
type CreateFunctionBuildResult struct {
	Image string

	// the function configuration read by the builder either from function.yaml or inline configuration
	UpdatedFunctionConfig functionconfig.Config
}

// CreateFunctionResult holds the results of a deploy
type CreateFunctionResult struct {
	CreateFunctionBuildResult
	Port        int
	ContainerID string
}

// GetFunctionsOptions is the base for all platform get options
type GetFunctionsOptions struct {
	Name      string
	Namespace string
	Labels    string
}

// InvokeViaType defines via which mechanism the function will be invoked
type InvokeViaType int

const (
	InvokeViaAny InvokeViaType = iota
	InvokeViaExternalIP
	InvokeViaLoadBalancer
	InvokeViaDomainName
)

// CreateFunctionInvocationOptions is the base for all platform invoke options
type CreateFunctionInvocationOptions struct {
	Name         string
	Namespace    string
	Path         string
	Method       string
	Body         []byte
	Headers      http.Header
	LogLevelName string
	Via          InvokeViaType
}

// CreateFunctionInvocationResult holds the result of a single invocation
type CreateFunctionInvocationResult struct {
	Headers    http.Header
	Body       []byte
	StatusCode int
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

//
// Project
//

type ProjectMeta struct {
	Name        string            `json:"name,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type ProjectSpec struct {
	DisplayName string `json:"displayName,omitempty"`
	Description string `json:"description,omitempty"`
}

type ProjectConfig struct {
	Meta ProjectMeta
	Spec ProjectSpec
}

type CreateProjectOptions struct {
	ProjectConfig ProjectConfig
}

type UpdateProjectOptions struct {
	ProjectConfig ProjectConfig
}

type DeleteProjectOptions struct {
	Meta ProjectMeta
}

type GetProjectsOptions struct {
	Meta ProjectMeta
}

// to appease k8s
func (s *ProjectSpec) DeepCopyInto(out *ProjectSpec) {

	// TODO: proper deep copy
	*out = *s
}

//
// FunctionEvent
//

type FunctionEventMeta struct {
	Name        string            `json:"name,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type FunctionEventSpec struct {
	DisplayName string                 `json:"displayName,omitempty"`
	TriggerName string                 `json:"triggerName,omitempty"`
	TriggerKind string                 `json:"triggerKind,omitempty"`
	Body        string                 `json:"body,omitempty"`
	Attributes  map[string]interface{} `json:"attributes,omitempty"`
}

type FunctionEventConfig struct {
	Meta FunctionEventMeta
	Spec FunctionEventSpec
}

type CreateFunctionEventOptions struct {
	FunctionEventConfig FunctionEventConfig
}

type UpdateFunctionEventOptions struct {
	FunctionEventConfig FunctionEventConfig
}

type DeleteFunctionEventOptions struct {
	Meta FunctionEventMeta
}

type GetFunctionEventsOptions struct {
	Meta FunctionEventMeta
}

// to appease k8s
func (s *FunctionEventSpec) DeepCopyInto(out *FunctionEventSpec) {

	// TODO: proper deep copy
	*out = *s
}
