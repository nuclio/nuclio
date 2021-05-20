package project

import (
	"sync"

	"github.com/nuclio/nuclio/pkg/platform"
)

type SafeCache struct {
	mu            sync.Mutex
	projectsCache []platform.Project
}

func NewSafeCache() *SafeCache {
	return &SafeCache{projectsCache: []platform.Project{}}
}

func (c *SafeCache) Add(projectInstance platform.Project) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.add(projectInstance)
}

func (c *SafeCache) AddMany(projectInstances []platform.Project) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, projectInstance := range projectInstances {
		c.add(projectInstance)
	}
}

func (c *SafeCache) Update(projectInstance platform.Project) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// delete project and re-add it to update the cache
	c.delete(projectInstance.GetConfig().Meta.Namespace, projectInstance.GetConfig().Meta.Name)
	c.add(projectInstance)
}

func (c *SafeCache) Delete(namespace, name string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.delete(namespace, name)
}

func (c *SafeCache) Get(getProjectOptions *platform.GetProjectsOptions) []platform.Project {
	matchingProjects := []platform.Project{}

	for _, projectInstance := range c.projectsCache {
		projectConfig := projectInstance.GetConfig()

		// if a specific namespace was requested and this project is not in it - skip it
		if projectConfig.Meta.Namespace != getProjectOptions.Meta.Namespace {
			continue
		}

		// if a specific namespace and name were requested - return this project (can't be more than one)
		if getProjectOptions.Meta.Name != "" {
			if projectConfig.Meta.Name != getProjectOptions.Meta.Name {
				continue
			}

			// name matches - return the matching project
			return []platform.Project{projectInstance}
		}

		matchingProjects = append(matchingProjects, projectInstance)
	}

	return matchingProjects
}

func (c *SafeCache) add(projectInstance platform.Project) {
	c.projectsCache = append(c.projectsCache, projectInstance)
}

func (c *SafeCache) delete(namespace, name string) {
	newProjectsCache := []platform.Project{}

	for _, projectInstance := range c.projectsCache {
		projectConfig := projectInstance.GetConfig()
		if projectConfig.Meta.Namespace == namespace && projectConfig.Meta.Name == name {
			continue
		}
		newProjectsCache = append(newProjectsCache, projectInstance)
	}

	c.projectsCache = newProjectsCache
}
