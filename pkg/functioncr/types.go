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

package functioncr

import (
	"k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type FunctionState string

const (
	FunctionStateCreated   FunctionState = ""
	FunctionStateProcessed FunctionState = "processed"
	FunctionStateError     FunctionState = "error"
	FunctionStateDisabled  FunctionState = "disabled"
	FunctionStateTerminate FunctionState = "terminate"
)

type BuildState string

const (
	BuildStateUnknown BuildState = ""
	BuildStatePending BuildState = "pending"
	BuildStateError   BuildState = "error"
	BuildStateBuild   BuildState = "build"
	BuildStateReady   BuildState = "ready"
)

type FunctionSpec struct {
	Version      int                     `json:"version,omitempty"`
	Description  string                  `json:"description,omitempty"`
	Disabled     bool                    `json:"disable,omitempty"`
	Publish      bool                    `json:"publish,omitempty"`
	Alias        string                  `json:"alias,omitempty"`
	Handler      string                  `json:"handler,omitempty"`
	Runtime      string                  `json:"runtime,omitempty"`
	WorkingDir   string                  `json:"workingDir,omitempty"`
	Env          []v1.EnvVar             `json:"env,omitempty"`
	Resources    v1.ResourceRequirements `json:"resources,omitempty"`
	DlqStream    string                  `json:"dlqStream,omitempty"`
	Role         string                  `json:"role,omitempty"`
	Secret       string                  `json:"secret,omitempty"`
	Image        string                  `json:"image,omitempty"`
	Code         FunctionCode            `json:"code,omitempty"`
	NetPolicy    string                  `json:"netPolicy,omitempty"`
	LogLevel     string                  `json:"logLevel,omitempty"`
	Replicas     int32                   `json:"replicas,omitempty"`
	MinReplicas  int32                   `json:"minReplicas,omitempty"`
	MaxReplicas  int32                   `json:"maxReplicas,omitempty"`
	DataBindings []DataBinding           `json:"dataBindings,omitempty"`
	HTTPPort     int32                   `json:"httpPort,omitempty"`
}

type FunctionStatus struct {
	State       FunctionState `json:"state,omitempty"`
	Message     string        `json:"message,omitempty"`
	BuildState  BuildState    `json:"build,omitempty"`
	ObservedGen string        `json:"observedVer,omitempty"`
}

type FunctionList struct {
	meta_v1.TypeMeta `json:",inline"`
	meta_v1.ListMeta `json:"metadata"`
	Items            []Function `json:"items"`
}

type FunctionCode struct {
	Path  string `json:"path,omitempty"`
	Key   string `json:"key,omitempty"`
	Code  string `json:"code,omitempty"`
	Watch bool   `json:"watch,omitempty"`
}

// Connection between function and data source/output
// TBD: need to specify requiered network ?  r/o vs r/w ?
type DataBinding struct {
	Name    string            `json:"name"`
	Class   string            `json:"class"`
	Url     string            `json:"url"`
	Path    string            `json:"path,omitempty"`
	Query   string            `json:"query,omitempty"`
	Secret  string            `json:"secret,omitempty"`
	Options map[string]string `json:"options,omitempty"`
}
