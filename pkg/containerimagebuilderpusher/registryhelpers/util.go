/*
Copyright 2026 The Nuclio Authors.

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

package registryhelpers

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
)

// writeCredentialFileScript emits a shell snippet that writes a three-line credential file
// (host, username, token) to the envVarRegistryTokenFile path. tokenCommand must print the bare
// token on stdout. Values come from env vars, so none of them can be parsed as shell code.
func writeCredentialFileScript(tokenCommand string) string {
	return fmt.Sprintf(`{ echo "$%s"; echo "$%s"; %s; } > "$%s"`,
		envVarRegistryHost,
		envVarRegistryUsername,
		tokenCommand,
		envVarRegistryTokenFile)
}

// credentialFileEnv returns the env vars every login script reads.
func credentialFileEnv(host, username, tokenFilePath string) []v1.EnvVar {
	return []v1.EnvVar{
		{Name: envVarRegistryHost, Value: host},
		{Name: envVarRegistryUsername, Value: username},
		{Name: envVarRegistryTokenFile, Value: tokenFilePath},
	}
}

// softFailScript wraps script to log a warning and exit 0 on failure instead of aborting the pod.
func softFailScript(script, kind string) string {
	return fmt.Sprintf(`(set -e
%s
) || echo "WARNING: failed to fetch %s login token for $%s" >&2`, script, kind, envVarRegistryHost)
}
