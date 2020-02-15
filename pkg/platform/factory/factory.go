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

package factory

import (
	"os"
	"path/filepath"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/containerimagebuilderpusher"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/config"
	"github.com/nuclio/nuclio/pkg/platform/kube"
	"github.com/nuclio/nuclio/pkg/platform/local"

	"github.com/mitchellh/go-homedir"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

// CreatePlatform creates a platform based on a requested type (platformType) and configuration it receives
// and probes
func CreatePlatform(parentLogger logger.Logger,
	platformType string,
	platformConfiguration interface{},
	defaultNamespace string) (platform.Platform, error) {

	var newPlatform platform.Platform
	var err error

	switch platformType {
	case "local":
		newPlatform, err = local.NewPlatform(parentLogger, platformConfiguration)

	case "kube":
		containerBuilderConfiguration := getContainerBuilderConfiguration(platformConfiguration)
		newPlatform, err = kube.NewPlatform(parentLogger, getKubeconfigPath(platformConfiguration), containerBuilderConfiguration, platformConfiguration)

	case "auto":

		// try to get kubeconfig path
		kubeconfigPath := getKubeconfigPath(platformConfiguration)

		if kubeconfigPath != "" || kube.IsInCluster() {

			// call again, but force kube
			newPlatform, err = CreatePlatform(parentLogger, "kube", platformConfiguration, defaultNamespace)
		} else {

			// call again, force local
			newPlatform, err = CreatePlatform(parentLogger, "local", platformConfiguration, defaultNamespace)
		}

	default:
		return nil, errors.Errorf("Can't create platform - unsupported: %s", platformType)
	}

	if err != nil {
		return nil, errors.Errorf("Failed to create %s platform", platformType)
	}

	if err = ensureDefaultProjectExistence(parentLogger, newPlatform, defaultNamespace); err != nil {
		return nil, errors.Wrap(err, "Failed to ensure default project existence")
	}

	return newPlatform, nil
}

func ensureDefaultProjectExistence(parentLogger logger.Logger, p platform.Platform, defaultNamespace string) error {
	resolvedNamespace := p.ResolveDefaultNamespace(defaultNamespace)

	projects, err := p.GetProjects(&platform.GetProjectsOptions{
		Meta: platform.ProjectMeta{
			Name:      platform.DefaultProjectName,
			Namespace: resolvedNamespace,
		},
	})
	if err != nil {
		return errors.Wrap(err, "Failed to get projects")
	}

	if len(projects) == 0 {

		// if we're here the default project doesn't exist. create it
		projectConfig := platform.ProjectConfig{
			Meta: platform.ProjectMeta{
				Name:      platform.DefaultProjectName,
				Namespace: resolvedNamespace,
			},
			Spec: platform.ProjectSpec{},
		}
		newProject, err := platform.NewAbstractProject(parentLogger, p, projectConfig)
		if err != nil {
			return errors.Wrap(err, "Failed to create abstract default project")
		}

		err = p.CreateProject(&platform.CreateProjectOptions{
			ProjectConfig: *newProject.GetConfig(),
		})
		if err != nil {
			return errors.Wrap(err, "Failed to create default project")
		}

	} else if len(projects) > 1 {
		return errors.New("Something went wrong. There's more than one default project")
	}

	return nil
}

func getContainerBuilderConfiguration(platformConfiguration interface{}) *containerimagebuilderpusher.ContainerBuilderConfiguration {
	containerBuilderConfiguration := containerimagebuilderpusher.ContainerBuilderConfiguration{}

	// if kubeconfig is passed in the options, use that
	if platformConfiguration != nil {

		// it might not be a kube configuration
		if _, ok := platformConfiguration.(*config.Configuration); ok {
			containerBuilderConfiguration = platformConfiguration.(*config.Configuration).ContainerBuilderConfiguration
		}
	}

	// if some of the parameters are undefined, try environment variables
	if containerBuilderConfiguration.Kind == "" {
		containerBuilderConfiguration.Kind = common.GetEnvOrDefaultString("NUCLIO_CONTAINER_BUILDER_KIND",
			"docker")
	}
	if containerBuilderConfiguration.BusyBoxImage == "" {
		containerBuilderConfiguration.BusyBoxImage = common.GetEnvOrDefaultString("NUCLIO_BUSYBOX_CONTAINER_IMAGE",
			"busybox:1.31")
	}
	if containerBuilderConfiguration.KanikoImage == "" {
		containerBuilderConfiguration.KanikoImage = common.GetEnvOrDefaultString("NUCLIO_KANIKO_CONTAINER_IMAGE",
			"gcr.io/kaniko-project/executor:v0.17.1")
	}
	if containerBuilderConfiguration.JobPrefix == "" {
		containerBuilderConfiguration.JobPrefix = common.GetEnvOrDefaultString("NUCLIO_DASHBOARD_JOB_NAME_PREFIX",
			"kanikojob")
	}

	containerBuilderConfiguration.InsecurePushRegistry = common.GetEnvOrDefaultBool("NUCLIO_KANIKO_INSECURE_PUSH_REGISTRY", false)
	containerBuilderConfiguration.InsecurePullRegistry = common.GetEnvOrDefaultBool("NUCLIO_KANIKO_INSECURE_PULL_REGISTRY", false)

	containerBuilderConfiguration.DefaultRegistryCredentialsSecretName =
		common.GetEnvOrDefaultString("NUCLIO_REGISTRY_CREDENTIALS_SECRET_NAME", "")

	if containerBuilderConfiguration.DefaultBaseRegistryURL == "" {
		containerBuilderConfiguration.DefaultBaseRegistryURL =
			common.GetEnvOrDefaultString("NUCLIO_DASHBOARD_DEFAULT_BASE_REGISTRY_URL", "quay.io")
	}

	containerBuilderConfiguration.CacheRepo = common.GetEnvOrDefaultString("NUCLIO_DASHBOARD_KANIKO_CACHE_REPO", "")

	return &containerBuilderConfiguration
}

func getKubeconfigPath(platformConfiguration interface{}) string {
	var kubeconfigPath string

	// if kubeconfig is passed in the options, use that
	if platformConfiguration != nil {

		// it might not be a kube configuration
		if _, ok := platformConfiguration.(*config.Configuration); ok {
			kubeconfigPath = platformConfiguration.(*config.Configuration).KubeconfigPath
		}
	}

	// do we still not have a kubeconfig path?
	if kubeconfigPath == "" {
		kubeconfigPath = common.GetEnvOrDefaultString("KUBECONFIG", getKubeconfigFromHomeDir())
	}
	return kubeconfigPath
}

func getKubeconfigFromHomeDir() string {
	homeDir, err := homedir.Dir()
	if err != nil {
		return ""
	}

	homeKubeConfigPath := filepath.Join(homeDir, ".kube", "config")

	// if the file exists @ home, use it
	_, err = os.Stat(homeKubeConfigPath)
	if err == nil {
		return homeKubeConfigPath
	}

	return ""
}
