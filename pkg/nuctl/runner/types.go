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

package runner

import (
	"github.com/nuclio/nuclio/pkg/functioncr"
	"github.com/nuclio/nuclio/pkg/nuctl"
	"github.com/nuclio/nuclio/pkg/nuctl/builder"
)

// if there's ever another resource that requires building, move this to FunctionOptions and
// have Options contain `function FunctionOptions`
type Options struct {
	Common       *nucliocli.CommonOptions
	Build        builder.Options
	SpecPath     string
	Description  string
	Image        string
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
	DataBindings string
	Spec         functioncr.Function
}
