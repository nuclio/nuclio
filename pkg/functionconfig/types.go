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

package functionconfig

import (
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/nuclio/nuclio/pkg/common"

	"github.com/nuclio/errors"
	"github.com/v3io/scaler/pkg/scalertypes"
	appsv1 "k8s.io/api/apps/v1"
	autosv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	NvidiaGPUResourceName = "nvidia.com/gpu"
)

// DataBinding holds configuration for a databinding
type DataBinding struct {
	Name       string                 `json:"name,omitempty"`
	Class      string                 `json:"class"`
	Kind       string                 `json:"kind"`
	URL        string                 `json:"url"`
	Path       string                 `json:"path,omitempty"`
	Query      string                 `json:"query,omitempty"`
	Secret     string                 `json:"secret,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// Checkpoint is a partition checkpoint
type Checkpoint *string

// Partition is a partition information
type Partition struct {
	ID         string     `json:"id"`
	Checkpoint Checkpoint `json:"checkpoint,omitempty"`
}

// Volume stores simple volume and mount
type Volume struct {
	Volume      v1.Volume      `json:"volume,omitempty"`
	VolumeMount v1.VolumeMount `json:"volumeMount,omitempty"`
}

// Trigger holds configuration for a trigger
type Trigger struct {
	Class                                 string            `json:"class"`
	Kind                                  string            `json:"kind"`
	Name                                  string            `json:"name"`
	Disabled                              bool              `json:"disabled,omitempty"`
	NumWorkers                            int               `json:"numWorkers,omitempty"`
	URL                                   string            `json:"url,omitempty"`
	Paths                                 []string          `json:"paths,omitempty"`
	Username                              string            `json:"username,omitempty"`
	Password                              string            `json:"password,omitempty"`
	Secret                                string            `json:"secret,omitempty"`
	Partitions                            []Partition       `json:"partitions,omitempty"`
	Annotations                           map[string]string `json:"annotations,omitempty"`
	WorkerAvailabilityTimeoutMilliseconds *int              `json:"workerAvailabilityTimeoutMilliseconds,omitempty"`
	WorkerAllocatorName                   string            `json:"workerAllocatorName,omitempty"`
	ExplicitAckMode                       ExplicitAckMode   `json:"explicitAckMode,omitempty"`
	WaitExplicitAckDuringRebalanceTimeout string            `json:"waitExplicitAckDuringRebalanceTimeout,omitempty"`
	WorkerTerminationTimeout              string            `json:"workerTerminationTimeout,omitempty"`

	// Dealer Information
	TotalTasks        int `json:"total_tasks,omitempty"`
	MaxTaskAllocation int `json:"max_task_allocation,omitempty"`

	// General attributes
	Attributes map[string]interface{} `json:"attributes,omitempty"`

	// Deprecated: MaxWorkers is replaced by NumWorkers, and will be removed in 1.15.x
	// TODO: remove in 1.15.x
	MaxWorkers int `json:"maxWorkers,omitempty"`
}

type ExplicitAckMode string

const (

	// ExplicitAckModeEnable allows explicit and implicit ack according to the "x-nuclio-stream-no-ack" header
	ExplicitAckModeEnable ExplicitAckMode = "enable"

	// ExplicitAckModeDisable disables the explicit ack feature and allows only implicit acks (default)
	ExplicitAckModeDisable ExplicitAckMode = "disable"

	// ExplicitAckModeExplicitOnly allows only explicit acks and disables implicit acks
	ExplicitAckModeExplicitOnly ExplicitAckMode = "explicitOnly"

	// DefaultWorkerTerminationTimeout wait time for workers to drop or ack events before rebalance initiates
	DefaultWorkerTerminationTimeout string = "10s"
)

func ExplicitAckModeInSlice(ackMode ExplicitAckMode, ackModes []ExplicitAckMode) bool {
	for _, mode := range ackModes {
		if ackMode == mode {
			return true
		}
	}
	return false
}

func ExplicitAckEnabled(mode ExplicitAckMode) bool {
	return ExplicitAckModeInSlice(mode,
		[]ExplicitAckMode{
			ExplicitAckModeEnable,
			ExplicitAckModeExplicitOnly,
		})
}

// GetTriggersByKind returns a map of triggers by their kind
func GetTriggersByKind(triggers map[string]Trigger, kind string) map[string]Trigger {
	matchingTrigger := map[string]Trigger{}

	for triggerName, trigger := range triggers {
		if trigger.Kind == kind {
			matchingTrigger[triggerName] = trigger
		}
	}

	return matchingTrigger
}

// GetTriggersByKinds returns a map of triggers by their kinds
func GetTriggersByKinds(triggers map[string]Trigger, kinds []string) map[string]Trigger {
	matchingTrigger := map[string]Trigger{}

	for triggerName, trigger := range triggers {
		if common.StringSliceContainsString(kinds, trigger.Kind) {
			matchingTrigger[triggerName] = trigger
		}
	}

	return matchingTrigger
}

func ResolveFunctionServiceType(functionSpec *Spec, defaultServiceType v1.ServiceType) v1.ServiceType {
	functionHTTPTriggers := GetTriggersByKind(functionSpec.Triggers, "http")

	// if the http trigger has a configured service type, return that.
	for _, trigger := range functionHTTPTriggers {
		if serviceTypeInterface, serviceTypeExists := trigger.Attributes["serviceType"]; serviceTypeExists {
			if serviceType, serviceTypeIsString := serviceTypeInterface.(string); serviceTypeIsString {
				return v1.ServiceType(serviceType)
			}
		}
	}

	// otherwise, if the function spec has a service type, return that (for backwards compatibility)
	if functionSpec.ServiceType != "" {
		return functionSpec.ServiceType
	}

	// otherwise return default
	return defaultServiceType
}

func GetFunctionIngresses(config *Config) map[string]Ingress {

	ingresses := map[string]Ingress{}
	for _, httpTrigger := range GetTriggersByKind(config.Spec.Triggers, "http") {

		// if there are attributes
		if encodedIngresses, found := httpTrigger.Attributes["ingresses"]; found {

			// iterate over the encoded ingresses map and created ingress structures
			encodedIngresses := encodedIngresses.(map[string]interface{})
			for encodedIngressName, encodedIngress := range encodedIngresses {
				encodedIngressMap := encodedIngress.(map[string]interface{})

				var ingress Ingress

				if host, ok := encodedIngressMap["host"].(string); ok {
					ingress.Host = host
				}

				// try to convert paths - this can arrive as []string or []interface{}
				switch typedPaths := encodedIngressMap["paths"].(type) {
				case []string:
					ingress.Paths = typedPaths
				case []interface{}:
					for _, path := range typedPaths {
						ingress.Paths = append(ingress.Paths, path.(string))
					}
				}

				// try to convert secretName and create a matching ingressTLS
				var ingressTLS IngressTLS
				if secretName, ok := encodedIngressMap["secretName"].(string); ok {
					hostsList := []string{ingress.Host}

					ingressTLS.Hosts = hostsList
					ingressTLS.SecretName = secretName
				}
				ingress.TLS = ingressTLS

				// enrich ingress pathType if not present
				ingress.PathType = networkingv1.PathTypeImplementationSpecific
				if ingressPathType, ok := encodedIngressMap["pathType"].(networkingv1.PathType); ok {
					ingress.PathType = ingressPathType
				}

				ingresses[encodedIngressName] = ingress
			}
		}
	}
	return ingresses
}

func GetDefaultHTTPTrigger() Trigger {
	return Trigger{
		Kind:       "http",
		Name:       "default-http",
		NumWorkers: 1,
	}
}

// Ingress holds configuration for an ingress - an entity that can route HTTP requests
// to the function
type Ingress struct {
	Host     string                `json:"host,omitempty"`
	Paths    []string              `json:"paths,omitempty"`
	PathType networkingv1.PathType `json:"pathType,omitempty"`
	TLS      IngressTLS            `json:"tls,omitempty"`
}

// IngressTLS holds configuration for an ingress's TLS
type IngressTLS struct {
	Hosts      []string `json:"hosts,omitempty"`
	SecretName string   `json:"secretName,omitempty"`
}

// LoggerSink overrides the default platform configuration for function loggers
type LoggerSink struct {
	Level string `json:"level,omitempty"`
	Sink  string `json:"sink,omitempty"`
}

// Platform holds platform specific attributes
type Platform struct {
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// Directive is injected into the image file (e.g. Dockerfile) generated during build
type Directive struct {
	Kind  string `json:"kind,omitempty"`
	Value string `json:"value,omitempty"`
}

type Metric struct {
	SourceType     string `json:"sourceType,omitempty"`
	ThresholdValue int64  `json:"thresholdValue,omitempty"`
	WindowSize     string `json:"windowSize,omitempty"`
}

type AutoScaleDisplayType string

const (
	AutoScaleMetricTypeInt        AutoScaleDisplayType = "int"
	AutoScaleMetricTypePercentage AutoScaleDisplayType = "percentage"
)

type AutoScaleMetric struct {
	ScaleResource `json:",inline"`
	SourceType    autosv2.MetricSourceType `json:"sourceType,omitempty"`
	DisplayType   AutoScaleDisplayType     `json:"displayType,omitempty"`
}

type BuildMode string

const (
	NeverBuild  BuildMode = "neverBuild"
	AlwaysBuild BuildMode = "alwaysBuild"
)

// Build holds all configuration parameters related to building a function
type Build struct {
	Path                  string                 `json:"path,omitempty"`
	FunctionSourceCode    string                 `json:"functionSourceCode,omitempty"`
	FunctionConfigPath    string                 `json:"functionConfigPath,omitempty"`
	TempDir               string                 `json:"tempDir,omitempty"`
	Registry              string                 `json:"registry,omitempty"`
	BaseImageRegistry     string                 `json:"baseImageRegistry,omitempty"`
	Image                 string                 `json:"image,omitempty"`
	NoBaseImagesPull      bool                   `json:"noBaseImagesPull,omitempty"`
	NoCache               bool                   `json:"noCache,omitempty"`
	NoCleanup             bool                   `json:"noCleanup,omitempty"`
	BaseImage             string                 `json:"baseImage,omitempty"`
	Commands              []string               `json:"commands,omitempty"`
	Directives            map[string][]Directive `json:"directives,omitempty"`
	ScriptPaths           []string               `json:"scriptPaths,omitempty"`
	AddedObjectPaths      map[string]string      `json:"addedPaths,omitempty"`
	Dependencies          []string               `json:"dependencies,omitempty"`
	OnbuildImage          string                 `json:"onbuildImage,omitempty"`
	Offline               bool                   `json:"offline,omitempty"`
	RuntimeAttributes     map[string]interface{} `json:"runtimeAttributes,omitempty"`
	CodeEntryType         string                 `json:"codeEntryType,omitempty"`
	CodeEntryAttributes   map[string]interface{} `json:"codeEntryAttributes,omitempty"`
	Timestamp             int64                  `json:"timestamp,omitempty"`
	BuildTimeoutSeconds   *int64                 `json:"buildTimeoutSeconds,omitempty"`
	Mode                  BuildMode              `json:"mode,omitempty"`
	Args                  map[string]string      `json:"args,omitempty"`
	Flags                 []string               `json:"flags,omitempty"`
	BuilderServiceAccount string                 `json:"builderServiceAccount,omitempty"`
}

// Spec holds all parameters related to a function's configuration
type Spec struct {
	Description             string                  `json:"description,omitempty"`
	Disable                 bool                    `json:"disable,omitempty"`
	Publish                 bool                    `json:"publish,omitempty"`
	Handler                 string                  `json:"handler,omitempty"`
	Runtime                 string                  `json:"runtime,omitempty"`
	Env                     []v1.EnvVar             `json:"env,omitempty"`
	EnvFrom                 []v1.EnvFromSource      `json:"envFrom,omitempty"`
	Resources               v1.ResourceRequirements `json:"resources,omitempty"`
	Image                   string                  `json:"image,omitempty"`
	ImageHash               string                  `json:"imageHash,omitempty"`
	Replicas                *int                    `json:"replicas,omitempty"`
	MinReplicas             *int                    `json:"minReplicas,omitempty"`
	MaxReplicas             *int                    `json:"maxReplicas,omitempty"`
	TargetCPU               int                     `json:"targetCPU,omitempty"`
	DataBindings            map[string]DataBinding  `json:"dataBindings,omitempty"`
	Triggers                map[string]Trigger      `json:"triggers,omitempty"`
	Volumes                 []Volume                `json:"volumes,omitempty"`
	Version                 int                     `json:"version,omitempty"`
	Alias                   string                  `json:"alias,omitempty"`
	Build                   Build                   `json:"build,omitempty"`
	RunRegistry             string                  `json:"runRegistry,omitempty"`
	ImagePullSecrets        string                  `json:"imagePullSecrets,omitempty"`
	RuntimeAttributes       map[string]interface{}  `json:"runtimeAttributes,omitempty"`
	LoggerSinks             []LoggerSink            `json:"loggerSinks,omitempty"`
	DealerURI               string                  `json:"dealerURI,omitempty"`
	Platform                Platform                `json:"platform,omitempty"`
	ReadinessTimeoutSeconds int                     `json:"readinessTimeoutSeconds,omitempty"`
	ServiceType             v1.ServiceType          `json:"serviceType,omitempty"`
	ImagePullPolicy         v1.PullPolicy           `json:"imagePullPolicy,omitempty"`
	SecurityContext         *v1.PodSecurityContext  `json:"securityContext,omitempty"`
	ServiceAccount          string                  `json:"serviceAccount,omitempty"`
	ScaleToZero             *ScaleToZeroSpec        `json:"scaleToZero,omitempty"`

	// If set to nil, the value is taken from the platform configuration. When set explicitly in function config, it has a priority
	DisableDefaultHTTPTrigger *bool `json:"disableDefaultHTTPTrigger,omitempty"`

	// When set to true, the function spec would not be scrubbed
	DisableSensitiveFieldsMasking bool `json:"disableSensitiveFieldsMasking,omitempty"`

	// Used for local platform functions mounting specific devices
	// https://docs.docker.com/engine/reference/run/#runtime-privilege-and-linux-capabilities
	// E.g.: /dev/video0:/dev/video0 or /dev/video0:/dev/video0:rwm or /dev/fuse
	Devices []string `json:"devices,omitempty"`

	// Run function on a particular set of node(s)
	// https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/
	Affinity     *v1.Affinity      `json:"affinity,omitempty"`
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	NodeName     string            `json:"nodeName,omitempty"`

	// Allow function to run on a node with matching taint
	// https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/
	Tolerations []v1.Toleration `json:"tolerations,omitempty"`

	// Priority and Preemption
	PriorityClassName string               `json:"priorityClassName,omitempty"`
	PreemptionPolicy  *v1.PreemptionPolicy `json:"preemptionPolicy,omitempty"`

	// How to replace existing function pods with new ones
	DeploymentStrategy *appsv1.DeploymentStrategy `json:"deploymentStrategy,omitempty"`

	// Use the host's ipc namespace
	HostIPC bool `json:"hostIPC,omitempty"`

	// Scale function's replica (when min < max replicas) based on given custom metric specs
	CustomScalingMetricSpecs []autosv2.MetricSpec `json:"customScalingMetricSpecs,omitempty"`
	AutoScaleMetrics         []AutoScaleMetric    `json:"autoScaleMetrics,omitempty"`

	// WaitReadinessTimeoutBeforeFailure is relevant only for k8s platform
	// if true - wait the whole ReadinessTimeoutSeconds before marking this function as unhealthy
	// otherwise, fail the function instantly when there is indication of deployment failure (e.g. pod stuck on crash
	// loop, pod container exited with an error, pod is unschedulable).
	// Default: false
	WaitReadinessTimeoutBeforeFailure bool `json:"waitReadinessTimeoutBeforeFailure,omitempty"`

	// We're letting users write "20s" and not the default marshalled time.Duration
	// (Which is in nanoseconds)
	EventTimeout string `json:"eventTimeout"`

	// PreemptionMode is a mode to allow the user to allow running function pods on preemptible nodes
	// When filled, tolerations, node labels, and affinity would be populated correspondingly to
	// the platformconfig.PreemptibleNodes values.
	PreemptionMode RunOnPreemptibleNodeMode `json:"preemptionMode,omitempty"`

	// Sidecars are containers that run alongside the function container in the same pod
	// the configuration for each sidecar is the same as k8s containers
	Sidecars []*v1.Container `json:"sidecars,omitempty"`

	// InitContainers are specialized containers that run before app containers in a Pod
	// Init containers can contain utilities or setup scripts not present in an app image
	InitContainers []*v1.Container `json:"initContainers,omitempty"`
}

type RunOnPreemptibleNodeMode string

const (

	// RunOnPreemptibleNodesAllow makes function pods be able to run on preemptible nodes
	RunOnPreemptibleNodesAllow RunOnPreemptibleNodeMode = "allow"

	// RunOnPreemptibleNodesConstrain makes the function pods run on preemtible nodes only
	RunOnPreemptibleNodesConstrain RunOnPreemptibleNodeMode = "constrain"

	// RunOnPreemptibleNodesPrevent prevents the function pods from running on preemptible nodes
	RunOnPreemptibleNodesPrevent RunOnPreemptibleNodeMode = "prevent"

	// RunOnPreemptibleNodesNone do nothing
	RunOnPreemptibleNodesNone RunOnPreemptibleNodeMode = "none"
)

type ScaleToZeroSpec struct {
	ScaleResources []ScaleResource `json:"scaleResources,omitempty"`
}

type ScaleResource struct {
	MetricName string `json:"metricName,omitempty"`
	WindowSize string `json:"windowSize,omitempty"`
	Threshold  int    `json:"threshold"`
}

// DeepCopyInto to appease k8s
func (s *Spec) DeepCopyInto(out *Spec) {

	// TODO: proper deep copy
	*out = *s
}

// GetHTTPPort returns the HTTP port
func (s *Spec) GetHTTPPort() int {
	if s.Triggers == nil {
		return 0
	}

	for _, trigger := range s.Triggers {
		if trigger.Kind == "http" {
			httpPort, httpPortValid := trigger.Attributes["port"]
			if httpPortValid {
				switch typedHTTPPort := httpPort.(type) {
				case int8:
					return int(typedHTTPPort)
				case int16:
					return int(typedHTTPPort)
				case int32:
					return int(typedHTTPPort)
				case int64:
					return int(typedHTTPPort)
				case uint:
					return int(typedHTTPPort)
				case uint8:
					return int(typedHTTPPort)
				case uint16:
					return int(typedHTTPPort)
				case uint32:
					return int(typedHTTPPort)
				case uint64:
					return int(typedHTTPPort)
				case float32:
					return int(typedHTTPPort)
				case float64:
					return int(typedHTTPPort)
				case int:
					return typedHTTPPort
				}
			}
		}
	}

	return 0
}

// GetEventTimeout returns the event timeout as time.Duration
func (s *Spec) GetEventTimeout() (time.Duration, error) {
	timeout, err := time.ParseDuration(s.EventTimeout)
	if err == nil && timeout <= 0 {
		err = fmt.Errorf("eventTimeout <= 0 (%s)", timeout)
	}

	return timeout, err
}

// PositiveGPUResourceLimit returns whether function requested at least one GPU
func (s *Spec) PositiveGPUResourceLimit() bool {
	if gpuResourceLimit, found := s.Resources.Limits[NvidiaGPUResourceName]; found {
		return !gpuResourceLimit.IsZero()
	}
	return false
}

const (
	FunctionAnnotationSkipBuild   = "skip-build"
	FunctionAnnotationSkipDeploy  = "skip-deploy"
	FunctionAnnotationPrevState   = "nuclio.io/previous-state"
	FunctionAnnotationForceUpdate = "nuclio.io/force-update"
)

// Meta identifies a function
type Meta struct {
	Name        string            `json:"name,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`

	// Used to determine whether the object is stale
	// more details @ https://kubernetes.io/docs/reference/using-api/api-concepts/#resource-versions
	ResourceVersion string `json:"resourceVersion,omitempty"`
}

// GetUniqueID return unique id
func (m *Meta) GetUniqueID() string {
	return m.Namespace + ":" + m.Name
}

func (m *Meta) AddSkipDeployAnnotation() {
	m.Annotations[FunctionAnnotationSkipDeploy] = strconv.FormatBool(true)
}

func (m *Meta) AddSkipBuildAnnotation() {
	m.Annotations[FunctionAnnotationSkipBuild] = strconv.FormatBool(true)
}

func (m *Meta) RemoveSkipDeployAnnotation() {
	delete(m.Annotations, FunctionAnnotationSkipDeploy)
}

func (m *Meta) RemoveSkipBuildAnnotation() {
	delete(m.Annotations, FunctionAnnotationSkipBuild)
}

func ShouldSkipDeploy(annotations map[string]string) bool {
	var skipFunctionDeploy bool
	if skipFunctionBuildDeploy, ok := annotations[FunctionAnnotationSkipDeploy]; ok {
		skipFunctionDeploy, _ = strconv.ParseBool(skipFunctionBuildDeploy)
	}
	return skipFunctionDeploy
}

func ShouldSkipBuild(annotations map[string]string) bool {
	var skipFunctionBuild bool
	if skipFunctionBuildStr, ok := annotations[FunctionAnnotationSkipBuild]; ok {
		skipFunctionBuild, _ = strconv.ParseBool(skipFunctionBuildStr)
	}
	return skipFunctionBuild
}

// Config holds the configuration of a function - meta and spec
type Config struct {
	Meta Meta `json:"metadata,omitempty"`
	Spec Spec `json:"spec,omitempty"`
}

// NewConfig creates a new configuration structure
func NewConfig() *Config {
	return &Config{
		Meta: Meta{
			Namespace: "default",
		},
	}
}

func (c *Config) CleanFunctionSpec() {

	// artifacts are created unique to the cluster not needed to be returned to any client of nuclio REST API
	c.Spec.RunRegistry = ""
	c.Spec.Build.Registry = ""
	if c.Spec.Build.FunctionSourceCode != "" {
		c.Spec.Image = ""
	}
}

func (c *Config) GetProjectName() (string, error) {
	if c.Meta.Labels == nil {
		c.Meta.Labels = make(map[string]string)
	}
	if name, ok := c.Meta.Labels[common.NuclioResourceLabelKeyProjectName]; ok {
		return name, nil
	}
	return "", errors.New("Project label not found")
}

func (c *Config) PrepareFunctionForExport(exportOptions *common.ExportFunctionOptions) {
	if !exportOptions.NoScrub {
		c.scrubFunctionData()
	}

	if exportOptions.CleanupSpec {
		c.CleanFunctionSpec()
	}

	// resource version should not be exported anyway, as it's a k8s thing
	c.Meta.ResourceVersion = ""

	c.AddSkipAnnotations()
	c.AddPrevStateAnnotation(exportOptions.PrevState)
}

func (c *Config) AddSkipAnnotations() {

	if c.Meta.Annotations == nil {
		c.Meta.Annotations = map[string]string{}
	}

	// add annotations for not deploying or building on import
	c.Meta.AddSkipBuildAnnotation()
	c.Meta.AddSkipDeployAnnotation()
}

func (c *Config) AddPrevStateAnnotation(state string) {
	if c.Meta.Annotations == nil {
		c.Meta.Annotations = map[string]string{}
	}
	c.Meta.Annotations[FunctionAnnotationPrevState] = state
}

func (c *Config) scrubFunctionData() {
	// scrub namespace from function meta
	c.Meta.Namespace = ""

	// remove secrets and passwords from triggers
	newTriggers := c.Spec.Triggers
	for triggerName, trigger := range newTriggers {
		trigger.Password = ""
		trigger.Secret = ""
		newTriggers[triggerName] = trigger
	}
	c.Spec.Triggers = newTriggers
}

func (c *Config) EnrichWithNodeSelectors(nodeSelector map[string]string) {
	if nodeSelector == nil {
		return
	}

	// merge node selectors - precedence to existing node selector
	c.Spec.NodeSelector = labels.Merge(c.Spec.NodeSelector, nodeSelector)
}

func (c *Config) EnrichWithTolerations(tolerations []v1.Toleration) {
	if len(tolerations) == 0 {
		return
	}

	var tolerationsToAdd []v1.Toleration

	// only add non-matching toleratinos to avoid duplications
	for _, functionToleration := range c.Spec.Tolerations {
		for _, preemptibleNodeTolerations := range tolerations {
			if !functionToleration.MatchToleration(&preemptibleNodeTolerations) {
				tolerationsToAdd = append(tolerationsToAdd, preemptibleNodeTolerations)
			}
		}
	}

	// in case function has no toleration, take all from input
	if len(c.Spec.Tolerations) == 0 {
		tolerationsToAdd = tolerations
	}

	if len(tolerationsToAdd) > 0 {
		c.Spec.Tolerations = append(c.Spec.Tolerations, tolerationsToAdd...)
	}
}

// PruneAffinityNodeSelectorRequirement prunes given affinity node selector requirement from function spec
func (c *Config) PruneAffinityNodeSelectorRequirement(nodeSelectorRequirements []v1.NodeSelectorRequirement, mode string) {

	// nothing to do here
	if c.Spec.Affinity == nil || nodeSelectorRequirements == nil {
		return
	}

	if nodeAffinity := c.Spec.Affinity.NodeAffinity; nodeAffinity != nil {
		if nodeSelector := nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution; nodeSelector != nil {

			// for all term expressions on function spec
			var newNodeSelectorTerms []v1.NodeSelectorTerm
			for termIdx, term := range nodeSelector.NodeSelectorTerms {

				// check if its key matches the anti affinity
				// if it does, we want to remove this expression, so it won't block us
				// by default, prunes "one of"
				var forcePruneAll bool
				var matchingExpressionFound bool
				var newNodeSelectorRequirements []v1.NodeSelectorRequirement
				for _, expression := range term.MatchExpressions {
					for _, nodeSelectorRequirement := range nodeSelectorRequirements {
						if nodeSelectorRequirement.Key == expression.Key &&
							nodeSelectorRequirement.Operator == expression.Operator &&
							reflect.DeepEqual(nodeSelectorRequirement.Values, expression.Values) {
							matchingExpressionFound = true
							if mode == "matchAll" {
								forcePruneAll = true
							}
							break
						}
					}

					// prunes all requirements
					if forcePruneAll {
						newNodeSelectorRequirements = newNodeSelectorRequirements[:0]
						break
					}

					// no matching expression found against the node selector requirements to prune
					if !matchingExpressionFound {

						// preserve it
						newNodeSelectorRequirements = append(newNodeSelectorRequirements, expression)
					}
				}

				// assign list
				term.MatchExpressions = newNodeSelectorRequirements

				// if terms at termIdx has no matching expressions / matching fields - slice it (object has
				//   meaning)
				if len(term.MatchExpressions) > 0 || len(term.MatchFields) > 0 {
					newNodeSelectorTerms = append(newNodeSelectorTerms, term)
				}

				// update term
				nodeSelector.NodeSelectorTerms[termIdx] = term
			}
			nodeSelector.NodeSelectorTerms = newNodeSelectorTerms

			// clean if empty
			if len(nodeSelector.NodeSelectorTerms) == 0 {
				nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = nil
			}
		}

		if nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil &&
			nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution == nil {
			c.Spec.Affinity.NodeAffinity = nil
		}

		if c.Spec.Affinity.NodeAffinity == nil &&
			c.Spec.Affinity.PodAffinity == nil &&
			c.Spec.Affinity.PodAntiAffinity == nil {
			c.Spec.Affinity = nil
		}
	}
}

// PruneTolerations prunes given tolerations from function spec
func (c *Config) PruneTolerations(tolerations []v1.Toleration) {
	if len(tolerations) == 0 {
		return
	}

	// prune matching tolerations
	for _, preemptibleNodeTolerations := range tolerations {
		for index, functionToleration := range c.Spec.Tolerations {
			if functionToleration.MatchToleration(&preemptibleNodeTolerations) {
				c.Spec.Tolerations = append(c.Spec.Tolerations[:index], c.Spec.Tolerations[index+1:]...)
				break
			}
		}
	}
}

// PruneNodeSelector prunes given node selector key from function spec if their key, value are matching
func (c *Config) PruneNodeSelector(nodeSelector map[string]string) {
	if nodeSelector == nil {
		return
	}

	for key, value := range nodeSelector {
		if specValue, exists := c.Spec.NodeSelector[key]; exists && value == specValue {
			delete(c.Spec.NodeSelector, key)
		}
	}
}

// FunctionState is state of function
type FunctionState string

// Possible function states
const (
	FunctionStateWaitingForBuild                  FunctionState = "waitingForBuild"
	FunctionStateBuilding                         FunctionState = "building"
	FunctionStateWaitingForResourceConfiguration  FunctionState = "waitingForResourceConfiguration"
	FunctionStateWaitingForScaleResourcesFromZero FunctionState = "waitingForScaleResourceFromZero"
	FunctionStateWaitingForScaleResourcesToZero   FunctionState = "waitingForScaleResourceToZero"
	FunctionStateConfiguringResources             FunctionState = "configuringResources"
	FunctionStateReady                            FunctionState = "ready"
	FunctionStateError                            FunctionState = "error"
	FunctionStateUnhealthy                        FunctionState = "unhealthy"
	FunctionStateScaledToZero                     FunctionState = "scaledToZero"
	FunctionStateImported                         FunctionState = "imported"
)

func FunctionStateInSlice(functionState FunctionState, functionStates []FunctionState) bool {
	for _, state := range functionStates {
		if functionState == state {
			return true
		}
	}
	return false
}

func FunctionStateProvisioned(functionState FunctionState) bool {
	return FunctionStateInSlice(functionState,
		[]FunctionState{
			FunctionStateReady,
			FunctionStateError,
			FunctionStateUnhealthy,
			FunctionStateScaledToZero,
			FunctionStateImported,
		})
}

func FunctionStateProvisioning(functionState FunctionState) bool {
	return !FunctionStateProvisioned(functionState)
}

func IsPreviousFunctionStateAllowedToBeSet(prevState FunctionState) bool {
	allowedPreviousStates := []FunctionState{
		FunctionStateScaledToZero,
		FunctionStateReady,
		FunctionStateImported,
	}
	return FunctionStateInSlice(prevState, allowedPreviousStates)
}

// Status holds the status of the function
type Status struct {
	State       FunctionState            `json:"state,omitempty"`
	Message     string                   `json:"message,omitempty"`
	Logs        []map[string]interface{} `json:"logs,omitempty"`
	ScaleToZero *ScaleToZeroStatus       `json:"scaleToZero,omitempty"`
	APIGateways []string                 `json:"apiGateways,omitempty"`
	HTTPPort    int                      `json:"httpPort,omitempty"`

	// the built and pushed image name, populated by the function operator after the function has been deployed
	ContainerImage string `json:"containerImage,omitempty"`

	// list of internal urls
	// e.g.:
	//		Kubernetes 	-	[ my-namespace.my-function.svc.cluster.local:8080 ]
	//		Docker 		-	[ function-container-name:8080 ]
	InternalInvocationURLs []string `json:"internalInvocationUrls,omitempty"`

	// list of external urls, containing ingresses and external-ip:function-port
	// e.g.: [ my-function.some-domain.com/pathA, other-ingress.some-domain.co, 1.2.3.4:3000 ]
	ExternalInvocationURLs []string `json:"externalInvocationUrls,omitempty"`
}

func (s *Status) InvocationURLs() []string {
	return append(s.InternalInvocationURLs, s.ExternalInvocationURLs...)
}

type ScaleToZeroStatus struct {
	LastScaleEvent     scalertypes.ScaleEvent `json:"lastScaleEvent,omitempty"`
	LastScaleEventTime *time.Time             `json:"lastScaleEventTime,omitempty"`
}

// DeepCopyInto copies to appease k8s
func (s *Status) DeepCopyInto(out *Status) {

	// TODO: proper deep copy
	*out = *s
}

// ConfigWithStatus holds the config and status of a function
type ConfigWithStatus struct {
	Config `json:",inline" yaml:",inline"`
	Status Status `json:"status,omitempty"`
}

var FixableValidationErrors = []string{"V3IO Stream trigger does not support autoscaling"}
