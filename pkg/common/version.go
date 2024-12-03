/*
Copyright 2023 The Nuclio Authors.

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

package common

import (
	"github.com/v3io/version-go"
	"runtime"
)

// SetVersionFromEnv is being used by tests to override linker injected values
func SetVersionFromEnv() {
	version.Set(&version.Info{
		Label:     GetEnvOrDefaultString("NUCLIO_LABEL", version.Get().Label),
		GitCommit: "c",
		OS:        GetEnvOrDefaultString("NUCLIO_OS", "linux"),
		Arch:      GetEnvOrDefaultString("NUCLIO_ARCH", runtime.GOARCH),
		GoVersion: version.Get().GoVersion,
	})
}
