package platform

import (
	"github.com/nuclio/nuclio-sdk"
	"github.com/spf13/viper"
	"io"
	"github.com/pkg/errors"
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
	Logger      nuclio.Logger
	Verbose     bool
	Identifier  string
	Namespace   string
	Description string

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
	Path             string            `json:"path,omitempty"`
	OutputType       string            `json:"outputType,omitempty"`
	NuclioSourceDir  string            `json:"nuclioSourceDir,omitempty"`
	NuclioSourceURL  string            `json:"nuclioSourceURL,omitempty"`
	Registry         string            `json:"registry,omitempty"`
	ImageName        string            `json:"imageName,omitempty"`
	ImageVersion     string            `json:"imageVersion,omitempty"`
	Runtime          string            `json:"runtime,omitempty"`
	Handler          string            `json:"handler,omitempty"`
	NoBaseImagesPull bool              `json:"noBaseImagesPull,omitempty"`
	BaseImageName    string            `json:"baseImageName,omitempty"`
	Commands         []string          `json:"commands,omitempty"`
	ScriptPaths      []string          `json:"scriptPaths,omitempty"`
	AddedObjectPaths map[string]string `json:"addedPaths,omitempty"`

	// called when the function configuration is found, either in the directory
	// or through inline
	OnFunctionConfigFound func([]byte) error

	// called before files are copied to staging
	OnBeforeCopyObjectsToStagingDir func() error

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
	Common             *CommonOptions
	Build              BuildOptions
	ImageName          string                 `json:"image,omitempty"`
	FunctionConfigPath string                 `json:"functionConfigPath,omitempty"`
	Description        string                 `json:"description,omitempty"`
	Env                string                 `json:"env,omitempty"`
	Labels             string                 `json:"labels,omitempty"`
	CPU                string                 `json:"cpu,omitempty"`
	Memory             string                 `json:"memory,omitempty"`
	WorkDir            string                 `json:"workingDir,omitempty"`
	Role               string                 `json:"role,omitempty"`
	Secret             string                 `json:"secret,omitempty"`
	Data               string                 `json:"data,omitempty"`
	Disabled           bool                   `json:"disable,omitempty"`
	Publish            bool                   `json:"publish,omitempty"`
	HTTPPort           int                    `json:"httpPort,omitempty"`
	Replicas           int                    `json:"replicas,omitempty"`
	MinReplicas        int                    `json:"minReplicas,omitempty"`
	MaxReplicas        int                    `json:"maxReplicas,omitempty"`
	RunRegistry        string                 `json:"runRegistry,omitempty"`
	DataBindings       map[string]DataBinding `json:"dataBindings,omitempty"`
	Triggers           map[string]Trigger     `json:"triggers,omitempty"`

	// platform specific
	Platform interface{}
}

// InitDefaults will initialize field values to a given default
func (do *DeployOptions) InitDefaults() {
	do.Common.InitDefaults()
	do.Build.Common = do.Common
	do.Build.InitDefaults()
	do.Replicas = 1
}

// ReadFunctionConfig reads a configuration file in either flat format to populate DeployOptions fields
func (do *DeployOptions) ReadFunctionConfig(reader io.Reader) error {

	// if we're completely uninitialized, init
	if do.Common == nil {
		do.Common = &CommonOptions{}
		do.InitDefaults()
	}

	functionConfigViper := viper.New()
	functionConfigViper.SetConfigType("yaml")

	if err := functionConfigViper.ReadConfig(reader); err != nil {
		return errors.Wrap(err, "Failed to read configuration file")
	}

	// check if this is k8s formatting
	if functionConfigViper.IsSet("apiVersion") {
		return errors.New("Kubernetes specfile format not supported yet")
	}

	// unmarshall to a deploy options structure
	if err := functionConfigViper.Unmarshal(do); err != nil {
		return errors.Wrap(err, "Failed to unmarshal specification")
	}

	// write common stuff and things that aren't in natural structured
	if functionConfigViper.IsSet("name") {
		do.Common.Identifier = functionConfigViper.GetString("name")
	}

	if functionConfigViper.IsSet("namespace") {
		do.Common.Namespace = functionConfigViper.GetString("namespace")
	}

	if functionConfigViper.IsSet("description") {
		do.Common.Description = functionConfigViper.GetString("description")
	}

	if functionConfigViper.IsSet("runtime") {
		do.Build.Runtime = functionConfigViper.GetString("runtime")
	}

	if functionConfigViper.IsSet("handler") {
		do.Build.Handler = functionConfigViper.GetString("handler")
	}

	return nil
}

// DeployResult holds the results of a deploy
type DeployResult struct {
	BuildResult
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
