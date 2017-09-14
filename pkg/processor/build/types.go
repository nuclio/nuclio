package build

import (
	"path"

	"github.com/nuclio/nuclio/pkg/util/common"
)

type Options struct {
	FunctionName    string
	FunctionPath    string
	NuclioSourceDir string
	NuclioSourceURL string
	PushRegistry    string
	Runtime         string
	Verbose         bool
	OutputName      string
	OutputType      string
	OutputVersion   string
}

// returns the directory the function is in
func (o *Options) getFunctionDir() string {

	// if the function directory was passed, just return that. if the function path was passed, return the directory
	// the function is in
	if common.IsDir(o.FunctionPath) {
		return o.FunctionPath
	}

	return path.Dir(o.FunctionPath)
}
