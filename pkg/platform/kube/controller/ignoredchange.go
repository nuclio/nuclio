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

package controller

import "github.com/nuclio/nuclio-sdk"

// the key is the resource versions
type resourceVersions map[string]interface{}

type IgnoredChanges struct {
	logger  nuclio.Logger
	changes map[string]resourceVersions
}

func NewIgnoredChanges(parentLogger nuclio.Logger) *IgnoredChanges {
	return &IgnoredChanges{
		logger:  parentLogger.GetChild("ignored"),
		changes: make(map[string]resourceVersions),
	}
}

func (ic *IgnoredChanges) Push(namespacedName string, resourceVersion string) {
	ignoredResourceVersions, found := ic.changes[namespacedName]
	if !found {

		// add the namespaced name to the ignored changes
		ignoredResourceVersions = resourceVersions{}
		ic.changes[namespacedName] = ignoredResourceVersions
	}

	// add the ignored version
	ignoredResourceVersions[resourceVersion] = nil

	ic.logger.DebugWith("Added ignored change",
		"name", namespacedName,
		"version", resourceVersion)
}

func (ic *IgnoredChanges) Pop(namespacedName string, resourceVersion string) bool {

	ignoredResourceVersions, found := ic.changes[namespacedName]
	if !found {
		return false
	}

	_, found = ignoredResourceVersions[resourceVersion]
	if !found {
		return false
	}

	// delete it
	delete(ignoredResourceVersions, resourceVersion)

	ic.logger.DebugWith("Removed ignored change",
		"name", namespacedName,
		"version", resourceVersion)

	// and indicate we found it
	return true
}
