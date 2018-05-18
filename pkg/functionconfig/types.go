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

package functionconfig

import (
	"strings"

	"k8s.io/api/core/v1"
)

// DataBinding holds configuration for a databinding
type DataBinding struct {
	Name       string            `json:"name,omitempty"`
	Class      string            `json:"class"`
	Kind       string            `json:"kind"`
	URL        string            `json:"url"`
	Path       string            `json:"path,omitempty"`
	Query      string            `json:"query,omitempty"`
	Secret     string            `json:"secret,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// Checkpoint is a partition checkpoint
type Checkpoint *string

// Partition is a partition information
type Partition struct {
	ID         string     `json:"id"`
	Checkpoint Checkpoint `json:"checkpoint,omitempty"`
}

// VolumeAndMount stores simple volume and mount
type Volume struct {
	Volume      v1.Volume      `json:"volume,omitempty"`
	VolumeMount v1.VolumeMount `json:"volumeMount,omitempty"`
}

// Trigger holds configuration for a trigger
type Trigger struct {
	Class       string            `json:"class"`
	Kind        string            `json:"kind"`
	Disabled    bool              `json:"disabled,omitempty"`
	MaxWorkers  int               `json:"maxWorkers,omitempty"`
	URL         string            `json:"url,omitempty"`
	Paths       []string          `json:"paths,omitempty"`
	User        string            `json:"user,omitempty"`
	Secret      string            `json:"secret,omitempty"`
	Partitions  []Partition       `json:"partitions,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`

	// Dealer Information
	TotalTasks        int `json:"total_tasks,omitempty"`
	MaxTaskAllocation int `json:"max_task_allocation,omitempty"`

	// General attributes
	Attributes map[string]interface{} `json:"attributes,omitempty"`
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

// GetIngressesFromTriggers returns all ingresses from a map of triggers
func GetIngressesFromTriggers(triggers map[string]Trigger) map[string]Ingress {
	ingresses := map[string]Ingress{}

	for _, trigger := range GetTriggersByKind(triggers, "http") {

		// if there are attributes
		if encodedIngresses, found := trigger.Attributes["ingresses"]; found {

			// iterate over the encoded ingresses map and created ingress structures
			for encodedIngressName, encodedIngress := range encodedIngresses.(map[string]interface{}) {
				encodedIngressMap := encodedIngress.(map[string]interface{})

				ingress := Ingress{}

				// try to convert host
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

				ingresses[encodedIngressName] = ingress
			}
		}
	}

	return ingresses
}

// Ingress holds configuration for an ingress - an entity that can route HTTP requests
// to the function
type Ingress struct {
	Host  string   `json:"host,omitempty"`
	Paths []string `json:"paths,omitempty"`
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

// Build holds all configuration parameters related to building a function
type Build struct {
	Path               string                 `json:"path,omitempty"`
	FunctionSourceCode string                 `json:"functionSourceCode,omitempty"`
	FunctionConfigPath string                 `json:"functionConfigPath,omitempty"`
	TempDir            string                 `json:"tempDir,omitempty"`
	Registry           string                 `json:"registry,omitempty"`
	Image              string                 `json:"image,omitempty"`
	NoBaseImagesPull   bool                   `json:"noBaseImagesPull,omitempty"`
	NoCache            bool                   `json:"noCache,omitempty"`
	NoCleanup          bool                   `json:"noCleanup,omitempty"`
	BaseImage          string                 `json:"baseImage,omitempty"`
	Commands           []string               `json:"commands,omitempty"`
	ScriptPaths        []string               `json:"scriptPaths,omitempty"`
	AddedObjectPaths   map[string]string      `json:"addedPaths,omitempty"`
	Dependencies       []string               `json:"dependencies,omitempty"`
	OnbuildImage       string                 `json:"onbuildImage,omitempty"`
	Offline            bool                   `json:"offline,omitempty"`
	RuntimeAttributes  map[string]interface{} `json:"offline,omitempty"`
}

// Spec holds all parameters related to a function's configuration
type Spec struct {
	Description       string                  `json:"description,omitempty"`
	Disabled          bool                    `json:"disable,omitempty"`
	Publish           bool                    `json:"publish,omitempty"`
	Handler           string                  `json:"handler,omitempty"`
	Runtime           string                  `json:"runtime,omitempty"`
	Env               []v1.EnvVar             `json:"env,omitempty"`
	Resources         v1.ResourceRequirements `json:"resources,omitempty"`
	Image             string                  `json:"image,omitempty"`
	ImageHash         string                  `json:"imageHash,omitempty"`
	Replicas          int                     `json:"replicas,omitempty"`
	MinReplicas       int                     `json:"minReplicas,omitempty"`
	MaxReplicas       int                     `json:"maxReplicas,omitempty"`
	TargetCPU         int                     `json:"targetCPU,omitempty"`
	DataBindings      map[string]DataBinding  `json:"dataBindings,omitempty"`
	Triggers          map[string]Trigger      `json:"triggers,omitempty"`
	Volumes           []Volume                `json:"volumes,omitempty"`
	Version           int                     `json:"version,omitempty"`
	Alias             string                  `json:"alias,omitempty"`
	Build             Build                   `json:"build,omitempty"`
	RunRegistry       string                  `json:"runRegistry,omitempty"`
	RuntimeAttributes map[string]interface{}  `json:"runtimeAttributes,omitempty"`
	LoggerSinks       []LoggerSink            `json:"loggerSinks,omitempty"`
	DealerURI         string                  `json:"dealer_uri,omitempty"`
	Platform          Platform                `json:"platform,omitempty"`
}

// to appease k8s
func (s *Spec) DeepCopyInto(out *Spec) {

	// TODO: proper deep copy
	*out = *s
}

func (s *Spec) GetRuntimeNameAndVersion() (string, string) {
	runtimeAndVersion := strings.Split(s.Runtime, ":")

	switch len(runtimeAndVersion) {
	case 1:
		return runtimeAndVersion[0], ""
	case 2:
		return runtimeAndVersion[0], runtimeAndVersion[1]
	default:
		return "", ""
	}
}

func (s *Spec) GetHTTPPort() int {
	if s.Triggers == nil {
		return 0
	}

	for _, trigger := range s.Triggers {
		if trigger.Kind == "http" {
			httpPort, httpPortValid := trigger.Attributes["port"]
			if httpPortValid {
				switch typedHTTPPort := httpPort.(type) {
				case uint64:
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

// Meta identifies a function
type Meta struct {
	Name        string            `json:"name,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

func (m *Meta) GetUniqueID() string {
	return m.Namespace + ":" + m.Name
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
		Spec: Spec{
			Replicas: 1,
		},
	}
}

type FunctionState string

const (
	FunctionStateWaitingForBuild                 FunctionState = "waitingForBuild"
	FunctionStateBuilding                        FunctionState = "building"
	FunctionStateWaitingForResourceConfiguration FunctionState = "waitingForResourceConfiguration"
	FunctionStateConfiguringResources            FunctionState = "configuringResources"
	FunctionStateReady                           FunctionState = "ready"
	FunctionStateError                           FunctionState = "error"
)

// Status holds the status of the function
type Status struct {
	State    FunctionState            `json:"state,omitempty"`
	Message  string                   `json:"message,omitempty"`
	Logs     []map[string]interface{} `json:"logs,omitempty"`
	HTTPPort int                      `json:"httpPort,omitempty"`
}

// to appease k8s
func (s *Status) DeepCopyInto(out *Status) {

	// TODO: proper deep copy
	*out = *s
}

// ConfigWithStatus holds the config and status of a function
type ConfigWithStatus struct {
	Config
	Status Status `json:"status,omitempty"`
}
