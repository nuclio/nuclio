package controller

import "github.com/nuclio/nuclio-sdk/logger"

// the key is the resource versions
type resourceVersions map[string]interface{}

type IgnoredChanges struct {
	logger  logger.Logger
	changes map[string]resourceVersions
}

func NewIgnoredChanges(parentLogger logger.Logger) *IgnoredChanges {
	return &IgnoredChanges{
		logger:  parentLogger.GetChild("ignored").(logger.Logger),
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
