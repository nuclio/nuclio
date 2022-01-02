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
	"reflect"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dashboard/auth"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/opa"
	"github.com/nuclio/nuclio/pkg/platform/kube/ingress"
	"github.com/nuclio/nuclio/pkg/platformconfig"

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
	PermissionOptions          opa.PermissionOptions
	AuthSession                auth.Session
}

type UpdateFunctionOptions struct {
	FunctionMeta      *functionconfig.Meta
	FunctionSpec      *functionconfig.Spec
	FunctionStatus    *functionconfig.Status
	AuthConfig        *AuthConfig
	PermissionOptions opa.PermissionOptions
	AuthSession       auth.Session
}

type DeleteFunctionOptions struct {
	FunctionConfig    functionconfig.Config
	AuthConfig        *AuthConfig
	PermissionOptions opa.PermissionOptions
	AuthSession       auth.Session

	// whether to ignore the validation where functions being provisioned cannot be deleted
	IgnoreFunctionStateValidation bool
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
	FunctionStatus functionconfig.Status
	Port           int
	ContainerID    string
}

// GetFunctionsOptions is the base for all platform get options
type GetFunctionsOptions struct {
	Name              string
	Namespace         string
	Labels            string
	ResourceVersion   string
	AuthConfig        *AuthConfig
	PermissionOptions opa.PermissionOptions
	AuthSession       auth.Session

	// Enrich functions with their api gateways
	EnrichWithAPIGateways bool
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
	Timeout      time.Duration
	Via          InvokeViaType
	URL          string

	PermissionOptions opa.PermissionOptions
	AuthSession       auth.Session
}

const FunctionInvocationDefaultTimeout = time.Minute

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

func (pm ProjectMeta) IsEqual(other ProjectMeta) bool {
	labels := common.GetStringToStringMapOrEmpty(pm.Labels)
	otherLabels := common.GetStringToStringMapOrEmpty(other.Labels)
	annotations := common.GetStringToStringMapOrEmpty(pm.Annotations)
	otherAnnotations := common.GetStringToStringMapOrEmpty(other.Annotations)

	return pm.Name == other.Name &&
		pm.Namespace == other.Namespace &&
		reflect.DeepEqual(labels, otherLabels) &&
		reflect.DeepEqual(annotations, otherAnnotations)
}

type ProjectSpec struct {
	Description string `json:"description,omitempty"`
	Owner       string `json:"owner,omitempty"`
}

func (ps ProjectSpec) IsEqual(other ProjectSpec) bool {
	return ps == other
}

type ProjectStatus struct {
	AdminStatus       string     `json:"adminStatus,omitempty"`
	OperationalStatus string     `json:"operationalStatus,omitempty"`
	UpdatedAt         *time.Time `json:"updatedAt,omitempty"`
}

func (pst ProjectStatus) IsEqual(other ProjectStatus) bool {
	return pst == other
}

type ProjectConfig struct {
	Meta   ProjectMeta   `json:"meta"`
	Spec   ProjectSpec   `json:"spec"`
	Status ProjectStatus `json:"status,omitempty"`
}

func (pc *ProjectConfig) IsEqual(other *ProjectConfig, ignoreStatus bool) bool {
	return pc.Meta.IsEqual(other.Meta) && pc.Spec.IsEqual(other.Spec) && (ignoreStatus || pc.Status.IsEqual(other.Status))
}

func (pc *ProjectConfig) Scrub() {
	pc.Meta.ResourceVersion = ""
}

type CreateProjectOptions struct {
	ProjectConfig           *ProjectConfig
	RequestOrigin           platformconfig.ProjectsLeaderKind
	SessionCookie           *http.Cookie
	PermissionOptions       opa.PermissionOptions
	AuthSession             auth.Session
	WaitForCreateCompletion bool
}

type UpdateProjectOptions struct {
	ProjectConfig     ProjectConfig
	RequestOrigin     platformconfig.ProjectsLeaderKind
	SessionCookie     *http.Cookie
	PermissionOptions opa.PermissionOptions
	AuthSession       auth.Session
}

type DeleteProjectStrategy string

const (

	// DeleteProjectStrategyCascading - delete sub resources prior to project deletion, leaving no orphans behind
	DeleteProjectStrategyCascading DeleteProjectStrategy = "cascading"

	// DeleteProjectStrategyRestricted - avoid deleting when project contains related resources (e.g.: functions)
	DeleteProjectStrategyRestricted DeleteProjectStrategy = "restricted"

	// DeleteProjectStrategyCheck - check pre-conditions for project deletion. does not perform deletion.
	DeleteProjectStrategyCheck DeleteProjectStrategy = "check"
)

func ResolveProjectDeletionStrategyOrDefault(projectDeletionStrategy string) DeleteProjectStrategy {
	switch strategy := DeleteProjectStrategy(projectDeletionStrategy); strategy {
	case DeleteProjectStrategyCascading, DeleteProjectStrategyRestricted, DeleteProjectStrategyCheck:
		return strategy
	default:

		// default
		return DeleteProjectStrategyRestricted
	}
}

type DeleteProjectOptions struct {
	Meta              ProjectMeta
	Strategy          DeleteProjectStrategy
	RequestOrigin     platformconfig.ProjectsLeaderKind
	SessionCookie     *http.Cookie
	PermissionOptions opa.PermissionOptions
	AuthSession       auth.Session

	// allowing us to "block" until related resources are removed.
	// used in testings
	WaitForResourcesDeletionCompletion         bool
	WaitForResourcesDeletionCompletionDuration time.Duration
}

type GetProjectsOptions struct {
	Meta              ProjectMeta
	PermissionOptions opa.PermissionOptions
	RequestOrigin     platformconfig.ProjectsLeaderKind
	SessionCookie     *http.Cookie
	AuthSession       auth.Session
}

// to appease k8s
func (ps *ProjectSpec) DeepCopyInto(out *ProjectSpec) {

	// TODO: proper deep copy
	*out = *ps
}

func (pst *ProjectStatus) DeepCopyInto(out *ProjectStatus) {

	// TODO: proper deep copy
	*out = *pst
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

type FunctionEventTriggerKind string

const FunctionEventTriggerKindHTTP = "http"
const DefaultFunctionEventTriggerKind = FunctionEventTriggerKindHTTP

type FunctionEventSpec struct {
	DisplayName string                 `json:"displayName,omitempty"`
	TriggerName string                 `json:"triggerName,omitempty"`
	TriggerKind string                 `json:"triggerKind,omitempty"`
	Body        string                 `json:"body,omitempty"`
	Attributes  map[string]interface{} `json:"attributes,omitempty"`
}

type FunctionEventConfig struct {
	Meta FunctionEventMeta `json:"meta"`
	Spec FunctionEventSpec `json:"spec"`
}

type CreateFunctionEventOptions struct {
	FunctionEventConfig FunctionEventConfig
	PermissionOptions   opa.PermissionOptions
	AuthSession         auth.Session
}

type UpdateFunctionEventOptions struct {
	FunctionEventConfig FunctionEventConfig
	PermissionOptions   opa.PermissionOptions
	AuthSession         auth.Session
}

type DeleteFunctionEventOptions struct {
	Meta              FunctionEventMeta
	PermissionOptions opa.PermissionOptions
	AuthSession       auth.Session
}

type GetFunctionEventsOptions struct {
	Meta              FunctionEventMeta
	FunctionNames     []string
	PermissionOptions opa.PermissionOptions
	AuthSession       auth.Session
}

// DeepCopyInto to appease k8s
func (s *FunctionEventSpec) DeepCopyInto(out *FunctionEventSpec) {

	// TODO: proper deep copy
	*out = *s
}

//
// APIGateway
//

type APIGatewayMeta struct {
	Name              string            `json:"name,omitempty"`
	Namespace         string            `json:"namespace,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	CreationTimestamp *metav1.Time      `json:"creationTimestamp,omitempty"`
}

func (agc *APIGatewayConfig) PrepareAPIGatewayForExport(noScrub bool) {
	if !noScrub {
		agc.scrubAPIGatewayData()
	}
}

func (agc *APIGatewayConfig) scrubAPIGatewayData() {

	// scrub namespace from api-gateway meta
	agc.Meta.Namespace = ""

	// creation timestamp won't be relevant on export
	agc.Meta.CreationTimestamp = nil

	// empty status
	agc.Status = APIGatewayStatus{}
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
	NuclioFunction   *NuclioFunctionAPIGatewaySpec `json:"nucliofunction,omitempty"`
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
	Meta   APIGatewayMeta   `json:"metadata,omitempty"`
	Spec   APIGatewaySpec   `json:"spec,omitempty"`
	Status APIGatewayStatus `json:"status,omitempty"`
}

// APIGatewayState is state of api gateway
type APIGatewayState string

// Possible api gateway states
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
	APIGatewayConfig           *APIGatewayConfig
	AuthSession                auth.Session
	ValidateFunctionsExistence bool
}

type UpdateAPIGatewayOptions struct {
	APIGatewayConfig           *APIGatewayConfig
	AuthSession                auth.Session
	ValidateFunctionsExistence bool
}

type DeleteAPIGatewayOptions struct {
	Meta        APIGatewayMeta
	AuthSession auth.Session
}

type GetAPIGatewaysOptions struct {
	Name        string
	Namespace   string
	Labels      string
	AuthSession auth.Session
}

// to appease k8s
func (s *APIGatewaySpec) DeepCopyInto(out *APIGatewaySpec) {

	// TODO: proper deep copy
	*out = *s
}

type GetFunctionReplicaLogsStreamOptions struct {

	// The replica (pod / container) name
	Name string

	// The replica (pod / container) namespace
	Namespace string

	// Whether to log stream of the replica
	Follow bool

	// A relative time in seconds before the current time from which to show logs.
	SinceSeconds *int64

	// Number of lines to show from the end of the logs
	TailLines *int64
}
