package platform

// use k8s structure definitions for now. In the future, duplicate them for cleanliness
import (
	"github.com/nuclio/nuclio/pkg/functionconfig"

	"github.com/nuclio/nuclio-sdk"
)

type BuildOptions struct {
	Logger         nuclio.Logger
	FunctionConfig functionconfig.Config
}

type DeployOptions struct {
	Logger         nuclio.Logger
	FunctionConfig functionconfig.Config
}

type UpdateOptions struct {
	FunctionConfig functionconfig.Config
}

type DeleteOptions struct {
	FunctionConfig functionconfig.Config
}

// BuildResult holds information detected/generated as a result of a build process
type BuildResult struct {
	ImageName             string
	Runtime               string
	Handler               string
	UpdatedFunctionConfig functionconfig.Config
}

// DeployResult holds the results of a deploy
type DeployResult struct {
	BuildResult
	Port int
}

// GetOptions is the base for all platform get options
type GetOptions struct {
	Name      string
	Namespace string
	NotList   bool
	Watch     bool
	Labels    string
	Format    string
}

// InvokeOptions is the base for all platform invoke options
type InvokeOptions struct {
	Name         string
	Namespace    string
	ClusterIP    string
	ContentType  string
	URL          string
	Method       string
	Body         string
	Headers      string
	LogLevelName string
}
