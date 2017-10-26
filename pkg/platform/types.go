package platform

import (
	"github.com/nuclio/nuclio-sdk"
)

// DataBinding holds configuration for a databinding
type DataBinding struct {
	Name    string            `json:"name,omitempty"`
	Class   string            `json:"class"`
	URL     string            `json:"url"`
	Path    string            `json:"path,omitempty"`
	Query   string            `json:"query,omitempty"`
	Secret  string            `json:"secret,omitempty"`
	Options map[string]string `json:"options,omitempty"`
}

// DataBinding holds configuration for a trigger
type Trigger struct {
	Class         string                 `json:"class"`
	Kind          string                 `json:"kind"`
	Disabled      bool                   `json:"disabled,omitempty"`
	MaxWorkers    int                    `json:"max_workers,omitempty"`
	URL           string                 `json:"url,omitempty"`
	Paths         []string               `json:"paths,omitempty"`
	NumPartitions int                    `json:"num_partitions,omitempty"`
	User          string                 `json:"user,omitempty"`
	Secret        string                 `json:"secret,omitempty"`
	Attributes    map[string]interface{} `json:"attributes,omitempty"`
}

// CommonOptions is the base for all platform options. It's never instantiated directly
type CommonOptions struct {
	Logger     nuclio.Logger
	Verbose    bool
	Identifier string
	Namespace  string

	// platform specific configuration
	PlatformConfiguration interface{}
}

// InitDefaults will initialize field values to a given default
func (co *CommonOptions) InitDefaults() {
	co.Namespace = "default"
}

// GetLogger returns the common.Logger if it's valid, or defaultLogger if it's not
func (co *CommonOptions) GetLogger(defaultLogger nuclio.Logger) nuclio.Logger {
	if co.Logger == nil {
		return defaultLogger
	}

	return co.Logger
}

// BuildOptions is the base for all platform build options
type BuildOptions struct {
	Common           *CommonOptions
	Path             string
	OutputType       string
	NuclioSourceDir  string
	NuclioSourceURL  string
	Registry         string
	ImageName        string
	ImageVersion     string
	Runtime          string
	Handler          string
	NoBaseImagesPull bool
	BaseImageName    string
	Commands         []string
	ScriptPaths      []string
	AddedFilePaths   []string

	// platform specific
	Platform interface{}
}

// InitDefaults will initialize field values to a given default
func (bo *BuildOptions) InitDefaults() {
	bo.Common.InitDefaults()
	bo.NuclioSourceURL = "https://github.com/nuclio/nuclio.git"
	bo.OutputType = "docker"
	bo.ImageVersion = "latest"
}

// DeployOptions is the base for all platform deploy options
type DeployOptions struct {
	Common       *CommonOptions
	Build        BuildOptions
	ImageName    string
	SpecPath     string
	Description  string
	Env          string
	Labels       string
	CPU          string
	Memory       string
	WorkDir      string
	Role         string
	Secret       string
	Events       string
	Data         string
	Disabled     bool
	Publish      bool
	HTTPPort     int32
	Scale        string
	MinReplicas  int32
	MaxReplicas  int32
	RunRegistry  string
	DataBindings map[string]DataBinding
	Triggers     map[string]Trigger

	// platform specific
	Platform interface{}
}

// InitDefaults will initialize field values to a given default
func (do *DeployOptions) InitDefaults() {
	do.Common.InitDefaults()
	do.Build.Common = do.Common
	do.Build.InitDefaults()
	do.Scale = "1"
}

// DeployResult holds the results of a deploy
type DeployResult struct {
	Port int
}

// InvokeOptions is the base for all platform invoke options
type InvokeOptions struct {
	Common       *CommonOptions
	ClusterIP    string
	ContentType  string
	URL          string
	Method       string
	Body         string
	Headers      string
	LogLevelName string
}

// GetOptions is the base for all platform get options
type GetOptions struct {
	Common  *CommonOptions
	NotList bool
	Watch   bool
	Labels  string
	Format  string
}

// DeleteOptions is the base for all platform delete options
type DeleteOptions struct {
	Common *CommonOptions
}

// UpdateOptions is the base for all platform update options
type UpdateOptions struct {
	Common *CommonOptions
	Deploy DeployOptions
	Alias  string
}
