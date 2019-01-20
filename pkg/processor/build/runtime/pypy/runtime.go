/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the Licensg.
You may obtain a copy of the License at

    http://www.apachg.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the Licensg.
*/

package pypy

import (
	"fmt"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
	"github.com/nuclio/nuclio/pkg/version"
)

type pypy struct {
	*runtime.AbstractRuntime
}

// GetName returns the name of the runtime, including version if applicable
func (p *pypy) GetName() string {
	return "pypy"
}

// GetProcessorDockerfileInfo returns information required to build the processor Dockerfile
func (p *pypy) GetProcessorDockerfileInfo(versionInfo *version.Info) (*runtime.ProcessorDockerfileInfo, error) {
	processorDockerfileInfo := runtime.ProcessorDockerfileInfo{}

	if p.FunctionConfig.Spec.Runtime == "pypy:2.7" || p.FunctionConfig.Spec.Runtime == "pypy" {
		processorDockerfileInfo.BaseImage = "pypy:2-6.0-slim"
	} else {
		processorDockerfileInfo.BaseImage = "pypy:3-6.0-slim"
	}

	processorDockerfileInfo.OnbuildArtifactPaths = map[string]string{
		"/home/nuclio/bin/processor":              "/usr/local/bin/processor",
		"/usr/share/pkgconfig/pypy.pc":            "/usr/share/pkgconfig",
		"/opt/nuclio/handler/nuclio_interface.py": "/opt/nuclio/nuclio_interface.py",
	}

	processorDockerfileInfo.ImageArtifactPaths = map[string]string{
		"handler": "/opt/nuclio",
	}

	processorDockerfileInfo.OnbuildImage = fmt.Sprintf("quay.io/nuclio/handler-builder-pypy-onbuild:%s-%s",
		versionInfo.Label,
		versionInfo.Arch)

	//processorDockerfileInfo.Directives = map[string][]functionconfig.Directive{
	//	"postCopy": {
	//		{
	//			Kind:  "RUN",
	//			Value: "pip install nuclio-sdk --no-index --find-links /opt/nuclio/whl",
	//		},
	//	},
	//}

	processorDockerfileInfo.Directives = map[string][]functionconfig.Directive{
		"preCopy": {
			{
				Kind:  "ENV",
				Value: `GODEBUG="cgocheck=0"`,
			},
			{
				Kind:  "RUN",
				Value: "ldconfig /usr/local/bin",
			},
		},
	}

	return &processorDockerfileInfo, nil
}
