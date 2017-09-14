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

package builder

import "github.com/nuclio/nuclio/pkg/nuctl"

// if there's ever another resource that requires building, move this to FunctionOptions and
// have Options contain `function FunctionOptions`
type Options struct {
	Common           *nuctl.CommonOptions
	Path             string
	Registry         string
	ImageName        string
	ImageVersion     string
	ProcessorURL     string
	PythonWrapperURL string
	PushProcessor    bool
}

func (o *Options) InitDefaults() {
	o.Common.InitDefaults()
	// TODO: Find right URLs
	o.ProcessorURL = "http://localhost:8000/processor.bz2"
	o.PythonWrapperURL = "http://localhost:8000/wrapper.py"
	o.ImageVersion = "latest"
}
