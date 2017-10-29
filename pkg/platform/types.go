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
	MaxWorkers    int                    `json:"maxWorkers,omitempty"`
	URL           string                 `json:"url,omitempty"`
	Paths         []string               `json:"paths,omitempty"`
	NumPartitions int                    `json:"numPartitions,omitempty"`
	User          string                 `json:"user,omitempty"`
	Secret        string                 `json:"secret,omitempty"`
	Attributes    map[string]interface{} `json:"attributes,omitempty"`
}

// Ingress holds configuration for an ingress - an entity that can route HTTP requests
// to the function
type Ingress struct {
	Host  string
	Paths []string
}

// CommonOptions is the base for all platform options. It's never instantiated directly
type CommonOptions struct {
	Logger      nuclio.Logger
	Verbose     bool   `json:"verbose"`
	Identifier  string `json:"name"`
	Namespace   string `json:"namespace"`
	Description string `json:"description"`

	// platform specific configuration
	PlatformConfiguration interface{}
}

// NewCommonOptions creates and initializes a CommonOptions structure
func NewCommonOptions() *CommonOptions {
	newCommonOptions := CommonOptions{}
	newCommonOptions.InitDefaults()

	return &newCommonOptions
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
	*CommonOptions
	Path               string            `json:"path,omitempty"`
	FunctionConfigPath string            `json:"functionConfigPath,omitempty"`
	OutputType         string            `json:"outputType,omitempty"`
	NuclioSourceDir    string            `json:"nuclioSourceDir,omitempty"`
	NuclioSourceURL    string            `json:"nuclioSourceURL,omitempty"`
	Registry           string            `json:"registry,omitempty"`
	ImageName          string            `json:"imageName,omitempty"`
	ImageVersion       string            `json:"imageVersion,omitempty"`
	Runtime            string            `json:"runtime,omitempty"`
	Handler            string            `json:"handler,omitempty"`
	NoBaseImagesPull   bool              `json:"noBaseImagesPull,omitempty"`
	BaseImageName      string            `json:"baseImageName,omitempty"`
	Commands           []string          `json:"commands,omitempty"`
	ScriptPaths        []string          `json:"scriptPaths,omitempty"`
	AddedObjectPaths   map[string]string `json:"addedPaths,omitempty"`

	// platform specific
	Platform interface{}
}

// NewBuildOptions creates and initializes a BuildOptions structure
func NewBuildOptions(commonOptions *CommonOptions) *BuildOptions {
	newBuildOptions := BuildOptions{
		CommonOptions: commonOptions,
	}

	// create common options instance
	if newBuildOptions.CommonOptions == nil {
		newBuildOptions.CommonOptions = NewCommonOptions()
	}

	return &newBuildOptions
}

// InitDefaults will initialize field values to a given default
func (bo *BuildOptions) InitDefaults() {
	bo.NuclioSourceURL = "https://github.com/nuclio/nuclio.git"
	bo.OutputType = "docker"
	bo.ImageVersion = "latest"
}

// DeployOptions is the base for all platform deploy options
type DeployOptions struct {
	*CommonOptions
	Build        BuildOptions
	ImageName    string                 `json:"image,omitempty"`
	Description  string                 `json:"description,omitempty"`
	Env          string                 `json:"env,omitempty"`
	Labels       string                 `json:"labels,omitempty"`
	CPU          string                 `json:"cpu,omitempty"`
	Memory       string                 `json:"memory,omitempty"`
	WorkDir      string                 `json:"workingDir,omitempty"`
	Role         string                 `json:"role,omitempty"`
	Secret       string                 `json:"secret,omitempty"`
	Data         string                 `json:"data,omitempty"`
	Disabled     bool                   `json:"disable,omitempty"`
	Publish      bool                   `json:"publish,omitempty"`
	HTTPPort     int                    `json:"httpPort,omitempty"`
	Replicas     int                    `json:"replicas,omitempty"`
	MinReplicas  int                    `json:"minReplicas,omitempty"`
	MaxReplicas  int                    `json:"maxReplicas,omitempty"`
	RunRegistry  string                 `json:"runRegistry,omitempty"`
	DataBindings map[string]DataBinding `json:"dataBindings,omitempty"`
	Triggers     map[string]Trigger     `json:"triggers,omitempty"`
	Ingresses    map[string]Ingress     `json:"ingresses,omitempty"`

	// platform specific
	Platform interface{}
}

// NewDeployOptions creates and initializes a DeployOptions structure
func NewDeployOptions(commonOptions *CommonOptions) *DeployOptions {
	newDeployOptions := DeployOptions{
		CommonOptions: commonOptions,
	}

	// create common options instance
	if newDeployOptions.CommonOptions == nil {
		newDeployOptions.CommonOptions = NewCommonOptions()
	}

	// initialize build options defaults
	newDeployOptions.Build.CommonOptions = newDeployOptions.CommonOptions
	newDeployOptions.Build.InitDefaults()

	return &newDeployOptions
}

// InitDefaults will initialize field values to a given default
func (do *DeployOptions) InitDefaults() {
	do.Build.InitDefaults()

	do.Replicas = 1
}

// DeployResult holds the results of a deploy
type DeployResult struct {
	BuildResult
	Port int
}

// InvokeOptions is the base for all platform invoke options
type InvokeOptions struct {
	*CommonOptions
	ClusterIP    string
	ContentType  string
	URL          string
	Method       string
	Body         string
	Headers      string
	LogLevelName string
}

// NewInvokeOptions creates and initializes a InvokeOptions structure
func NewInvokeOptions(commonOptions *CommonOptions) *InvokeOptions {
	newInvokeOptions := InvokeOptions{
		CommonOptions: commonOptions,
	}

	// create common options instance
	if newInvokeOptions.CommonOptions == nil {
		newInvokeOptions.CommonOptions = NewCommonOptions()
	}

	return &newInvokeOptions
}

// GetOptions is the base for all platform get options
type GetOptions struct {
	*CommonOptions
	NotList bool
	Watch   bool
	Labels  string
	Format  string
}

// NewGetOptions creates and initializes a GetOptions structure
func NewGetOptions(commonOptions *CommonOptions) *GetOptions {
	newGetOptions := GetOptions{
		CommonOptions: commonOptions,
	}

	// create common options instance
	if newGetOptions.CommonOptions == nil {
		newGetOptions.CommonOptions = NewCommonOptions()
	}

	return &newGetOptions
}

// DeleteOptions is the base for all platform delete options
type DeleteOptions struct {
	*CommonOptions
}

// NewDeleteOptions creates and initializes a DeleteOptions structure
func NewDeleteOptions(commonOptions *CommonOptions) *DeleteOptions {
	newDeleteOptions := DeleteOptions{
		CommonOptions: commonOptions,
	}

	// create common options instance
	if newDeleteOptions.CommonOptions == nil {
		newDeleteOptions.CommonOptions = NewCommonOptions()
	}

	return &newDeleteOptions
}

// UpdateOptions is the base for all platform update options
type UpdateOptions struct {
	*CommonOptions
	Deploy DeployOptions
	Alias  string
}

// NewUpdateOptions creates and initializes a UpdateOptions structure
func NewUpdateOptions(commonOptions *CommonOptions) *UpdateOptions {
	newUpdateOptions := UpdateOptions{
		CommonOptions: commonOptions,
	}

	// create common options instance
	if newUpdateOptions.CommonOptions == nil {
		newUpdateOptions.CommonOptions = NewCommonOptions()
	}

	// initialize deploy options defaults
	newUpdateOptions.Deploy.InitDefaults()
	newUpdateOptions.Deploy.CommonOptions = commonOptions
	newUpdateOptions.Deploy.Build.CommonOptions = commonOptions

	return &newUpdateOptions
}

// BuildResult holds information detected/generated as a result of a build process
type BuildResult struct {
	ImageName          string
	Runtime            string
	Handler            string
	FunctionConfigPath string
}
