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

package functionconfig

import (
	"fmt"
	"strings"
)

func ParseHandler(handler string) (string, string, error) {

	// take the handler name, if a module was provided
	moduleAndEntrypoint := strings.Split(handler, ":")
	switch len(moduleAndEntrypoint) {

	// entrypoint only
	case 1:
		return "", moduleAndEntrypoint[0], nil

		// module:entrypoint
	case 2:
		return moduleAndEntrypoint[0], moduleAndEntrypoint[1], nil

	default:
		return "", "", fmt.Errorf("Invalid handler name %s", handler)
	}
}
