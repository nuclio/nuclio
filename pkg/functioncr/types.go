package functioncr

import (
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/api/v1"
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

type Function struct {
	meta_v1.TypeMeta   `json:",inline"`
	meta_v1.ObjectMeta `json:"metadata"`
	Spec               FunctionSpec   `json:"spec"`
	Status             FunctionStatus `json:"status,omitempty"`
}

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
