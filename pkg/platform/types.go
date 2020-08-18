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

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform/kube/ingress"

	"github.com/nuclio/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//
// Auth
//

type AuthConfig struct {
	Token string
}

//
// Function
//

type CreateFunctionBuildOptions struct {
	Logger                     logger.Logger
	FunctionConfig             functionconfig.Config
	PlatformName               string
	OnAfterConfigUpdate        func(*functionconfig.Config) error
	OutputImageFile            string
	DependantImagesRegistryURL string
}

type CreateFunctionOptions struct {
	Logger                     logger.Logger
	FunctionConfig             functionconfig.Config
	CreationStateUpdated       chan bool
	InputImageFile             string
	AuthConfig                 *AuthConfig
	DependantImagesRegistryURL string
}

type UpdateFunctionOptions struct {
	FunctionMeta   *functionconfig.Meta
	FunctionSpec   *functionconfig.Spec
	FunctionStatus *functionconfig.Status
	AuthConfig     *AuthConfig
}

type DeleteFunctionOptions struct {
	FunctionConfig functionconfig.Config
	AuthConfig     *AuthConfig
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
	Name       string
	Namespace  string
	Labels     string
	AuthConfig *AuthConfig
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

const DefaultProjectName string = "default"

type ProjectMeta struct {
	Name        string            `json:"name,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`

	// Can be used to determine whether the object is stale (not used today)
	// more details @ https://kubernetes.io/docs/reference/using-api/api-concepts/#resource-versions
	ResourceVersion string `json:"resourceVersion,omitempty"`
}

type ProjectSpec struct {
	DisplayName string `json:"displayName,omitempty"` // Deprecated. Will be removed in the next major version release
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

//
// APIGateway
//

const DefaultAPIGatewayName string = "default"

type APIGatewayMeta struct {
	Name              string            `json:"name,omitempty"`
	Namespace         string            `json:"namespace,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	CreationTimestamp metav1.Time       `json:"creationTimestamp,omitempty"`
}

type APIGatewayAuthenticationSpec struct {
	BasicAuth *BasicAuth       `json:"basicAuth,omitempty"`
	DexAuth   *ingress.DexAuth `json:"dexAuth,omitempty"`
}

type BasicAuth struct {
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
}

type APIGatewayUpstreamKind string

const (
	APIGatewayUpstreamKindNuclioFunction APIGatewayUpstreamKind = "nucliofunction"
)

type NuclioFunctionAPIGatewaySpec struct {
	Name string `json:"name,omitempty"`
}

type APIGatewayUpstreamSpec struct {
	Kind             APIGatewayUpstreamKind        `json:"kind,omitempty"`
	Nucliofunction   *NuclioFunctionAPIGatewaySpec `json:"nucliofunction,omitempty"`
	Percentage       int                           `json:"percentage,omitempty"`
	RewriteTarget    string                        `json:"rewriteTarget,omitempty"`
	ExtraAnnotations map[string]string             `json:"extraAnnotations,omitempty"`
}

type APIGatewaySpec struct {
	Host               string                        `json:"host,omitempty"`
	Name               string                        `json:"name,omitempty"`
	Description        string                        `json:"description,omitempty"`
	Path               string                        `json:"path,omitempty"`
	AuthenticationMode ingress.AuthenticationMode    `json:"authenticationMode,omitempty"`
	Authentication     *APIGatewayAuthenticationSpec `json:"authentication,omitempty"`
	Upstreams          []APIGatewayUpstreamSpec      `json:"upstreams,omitempty"`
}

type APIGatewayConfig struct {
	Meta   APIGatewayMeta
	Spec   APIGatewaySpec
	Status APIGatewayStatus
}

// APIGatewayState is state of api-gateway
type APIGatewayState string

// Possible api-gateway states
const (
	APIGatewayStateNone                   APIGatewayState = ""
	APIGatewayStateReady                  APIGatewayState = "ready"
	APIGatewayStateError                  APIGatewayState = "error"
	APIGatewayStateWaitingForProvisioning APIGatewayState = "waitingForProvisioning"
)

type APIGatewayStatus struct {
	Name        string          `json:"name,omitempty"`
	LastError   string          `json:"last_error,omitempty"`
	Description string          `json:"description,omitempty"`
	State       APIGatewayState `json:"state,omitempty"`
}

type CreateAPIGatewayOptions struct {
	APIGatewayConfig APIGatewayConfig
}

type UpdateAPIGatewayOptions struct {
	APIGatewayConfig APIGatewayConfig
}

type DeleteAPIGatewayOptions struct {
	Meta APIGatewayMeta
}

type GetAPIGatewaysOptions struct {
	Name       string
	Namespace  string
	Labels     string
}

// to appease k8s
func (s *APIGatewaySpec) DeepCopyInto(out *APIGatewaySpec) {

	// TODO: proper deep copy
	*out = *s
}
