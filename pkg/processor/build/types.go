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
