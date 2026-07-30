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
	"strings"

	"github.com/nuclio/nuclio/pkg/common"
)

// writeCredentialFileScript emits a shell snippet that writes a three-line credential file
// (host, username, token) to path. tokenCommand must print the bare token on stdout.
func writeCredentialFileScript(path, host, username, tokenCommand string) string {
	return fmt.Sprintf("{ echo %s; echo %s; %s; } > %s",
		common.Quote(host), common.Quote(username), tokenCommand, common.Quote(path))
}

// softFailScript wraps script to log a warning and exit 0 on failure instead of aborting the pod.
func softFailScript(script, host, kind string) string {
	return fmt.Sprintf(`(set -e
%s
) || echo "WARNING: failed to fetch %s login token for %s" >&2`, script, kind, host)
}

// NormalizeHosts strips each url to its bare hostname and drops empty/duplicate values.
func NormalizeHosts(urls ...string) []string {
	hosts := make([]string, 0, len(urls))
	for _, url := range urls {
		if host := hostOf(url); host != "" {
			hosts = append(hosts, host)
		}
	}
	return common.RemoveDuplicatesFromSliceString(hosts)
}

// hostOf returns url's hostname, stripping any repository path (GAR URLs carry one; ACR/ECR don't).
func hostOf(url string) string {
	if i := strings.Index(url, "/"); i != -1 {
		return url[:i]
	}
	return url
}
