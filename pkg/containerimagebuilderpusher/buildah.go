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

package containerimagebuilderpusher

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/containerimagebuilderpusher/registryhelpers"
	"github.com/nuclio/nuclio/pkg/platform/kube/clients/kube"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/api/core/v1"
)

const (
	BuildahKind = "buildah"

	buildahAuthDir  = "/var/lib/containers/auth"
	buildahAuthFile = buildahAuthDir + "/config.json"
)

type Buildah struct {
	*jobRunner
}

func NewBuildah(logger logger.Logger,
	kubeClientSet kube.Client,
	builderConfiguration *ContainerBuilderConfiguration) (*Buildah, error) {

	jobRunner, err := newJobRunner(BuildahKind, logger, kubeClientSet, builderConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create buildah job runner")
	}

	return &Buildah{jobRunner: jobRunner}, nil
}

func (b *Buildah) BuildAndPushContainerImage(ctx context.Context,
	buildOptions *BuildOptions,
	namespace string) error {
	bundleFilename, assetPath, err := b.createContainerBuildBundle(ctx,
		buildOptions.Image,
		buildOptions.ContextDir,
		buildOptions.TempDir)
	if err != nil {
		return errors.Wrap(err, "Failed to create container build bundle")
	}

	// Remove bundle file from assets once we are done
	defer os.Remove(assetPath) // nolint: errcheck

	// Generate job spec
	jobSpec, err := b.compileJobSpec(ctx, namespace, buildOptions, bundleFilename)
	if err != nil {
		return errors.Wrap(err, "Failed to compile buildah job spec")
	}

	return b.submitAndWait(ctx, namespace, buildOptions, jobSpec)
}

func (b *Buildah) compileJobSpec(ctx context.Context,
	namespace string,
	buildOptions *BuildOptions,
	bundleFilename string) (*batchv1.Job, error) {

	buildahContainer := b.compileBuildahContainer(buildOptions)

	jobSpec, err := b.compileBaseJobSpec(ctx,
		namespace,
		buildOptions,
		bundleFilename,
		buildahContainer,
		b.builderConfiguration.Buildah.ImagePullPolicy)
	if err != nil {
		return nil, err
	}

	podSpec := &jobSpec.Spec.Template.Spec
	if err := b.configureRegistryAuthentication(ctx, namespace, buildOptions, podSpec); err != nil {
		return nil, errors.Wrap(err, "Failed to configure registry auth volumes and init containers")
	}
	b.configureRootlessMode(podSpec)
	b.configureAppArmorProfile(jobSpec)

	return jobSpec, nil
}

// configureAppArmorProfile relaxes AppArmor for the buildah container, if configured.
func (b *Buildah) configureAppArmorProfile(jobSpec *batchv1.Job) {
	profile := b.builderConfiguration.Buildah.AppArmorProfile
	if profile == "" {
		return
	}

	podTemplate := &jobSpec.Spec.Template
	if podTemplate.Annotations == nil {
		podTemplate.Annotations = map[string]string{}
	}
	containerName := podTemplate.Spec.Containers[0].Name
	podTemplate.Annotations["container.apparmor.security.beta.kubernetes.io/"+containerName] = profile
}

// configureRegistryAuthentication mounts the registry auth secret(s) and cloud-provider credentials
// into the buildah container.
func (b *Buildah) configureRegistryAuthentication(ctx context.Context, namespace string, buildOptions *BuildOptions, podSpec *v1.PodSpec) error {
	cloudHosts := registryhelpers.NormalizeHosts(buildOptions.RegistryURL,
		buildOptions.BaseImageRegistry,
		buildOptions.OnbuildImageRegistry)

	return b.jobRunner.configureRegistryAuthentication(ctx,
		namespace,
		buildOptions,
		buildahAuthDir,
		cloudHosts,
		b.builderConfiguration.Buildah.ImagePullPolicy,
		podSpec)
}

// configureRootlessMode wires the buildah container's security context per Buildah.RootlessMode.
func (b *Buildah) configureRootlessMode(podSpec *v1.PodSpec) {
	if b.builderConfiguration.Buildah.RootlessMode == "hostusers" {
		hostUsers := false
		podSpec.HostUsers = &hostUsers
		return
	}

	allowPrivilegeEscalation := true
	podSpec.Containers[0].SecurityContext = &v1.SecurityContext{
		Capabilities: &v1.Capabilities{
			Add: []v1.Capability{"SETUID", "SETGID"},
		},
		AllowPrivilegeEscalation: &allowPrivilegeEscalation,
	}
}

// compileBuildahBudArgs assembles the "buildah bud" argument list.
func (b *Buildah) compileBuildahBudArgs(buildOptions *BuildOptions, destination string, envRef func(string) string) []string {
	buildArgs := []string{
		"--layers",
		fmt.Sprintf("--storage-driver=%s", envRef(b.builderConfiguration.Buildah.StorageDriver)),
		fmt.Sprintf("--isolation=%s", envRef(b.builderConfiguration.Buildah.Isolation)),
		fmt.Sprintf("--file=%s", envRef(buildOptions.DockerfileInfo.DockerfilePath)),
		fmt.Sprintf("--tag=%s", envRef(destination)),
	}

	if b.builderConfiguration.InsecurePullRegistry {
		buildArgs = append(buildArgs, "--tls-verify=false")
	}

	if buildOptions.NoCache {
		buildArgs = append(buildArgs, "--no-cache")
	} else {
		cacheRepo := b.builderConfiguration.CacheRepo
		if cacheRepo == "" {
			// Mirror Kaniko's default cache repo when none is configured
			cacheRepo = common.StripImageTag(destination) + "/cache"
		}
		envCacheRepo := envRef(cacheRepo)
		buildArgs = append(buildArgs,
			fmt.Sprintf("--cache-to=%s", envCacheRepo),
			fmt.Sprintf("--cache-from=%s", envCacheRepo))
	}

	for buildArgName, buildArgValue := range buildOptions.BuildArgs {
		buildArg := fmt.Sprintf("%s=%s", buildArgName, buildArgValue)
		buildArgs = append(buildArgs, fmt.Sprintf("--build-arg=%s", envRef(buildArg)))
	}

	buildArgs = append(buildArgs, envRef(buildOptions.ContextDir))

	return buildArgs
}

// compileBuildahPushArgs assembles the "buildah push" argument list.
func (b *Buildah) compileBuildahPushArgs(destination string, envRef func(string) string) []string {
	pushArgs := []string{
		fmt.Sprintf("--storage-driver=%s", envRef(b.builderConfiguration.Buildah.StorageDriver)),
		fmt.Sprintf("--retry=%d", b.builderConfiguration.PushImagesRetries),
	}

	if b.builderConfiguration.InsecurePushRegistry {
		pushArgs = append(pushArgs, "--tls-verify=false")
	}

	envDestination := envRef(destination)
	pushArgs = append(pushArgs, envDestination, fmt.Sprintf("docker://%s", envDestination))

	return pushArgs
}

func (b *Buildah) compileBuildahContainer(buildOptions *BuildOptions) v1.Container {

	tmpFolderVolumeMount := v1.VolumeMount{
		Name:      "tmp",
		MountPath: "/tmp",
	}

	destination := common.CompileImageName(buildOptions.RegistryURL, buildOptions.Image)

	envVars := []v1.EnvVar{
		{
			Name:  "REGISTRY_AUTH_FILE",
			Value: buildahAuthFile,
		},
	}

	// envRef avoids shell injection of free-text values into the command line.
	envRef := func(value string) string {
		name := fmt.Sprintf("BUILDAH_ARG_%d", len(envVars))
		envVars = append(envVars, v1.EnvVar{Name: name, Value: value})
		return fmt.Sprintf(`"$%s"`, name)
	}

	buildArgs := b.compileBuildahBudArgs(buildOptions, destination, envRef)
	pushArgs := b.compileBuildahPushArgs(destination, envRef)

	budCmd := "buildah bud " + strings.Join(buildArgs, " ")
	pushCmd := "buildah push " + strings.Join(pushArgs, " ")
	buildahCommand := strings.Join([]string{"set -eu", budCmd, pushCmd}, "\n")

	return v1.Container{
		Name:            "buildah",
		Image:           b.builderConfiguration.Buildah.Image,
		ImagePullPolicy: v1.PullPolicy(b.builderConfiguration.Buildah.ImagePullPolicy),
		Command:         []string{"/bin/sh"},
		Args:            []string{"-c", buildahCommand},
		Env:             envVars,
		VolumeMounts:    []v1.VolumeMount{tmpFolderVolumeMount},
		Resources:       buildOptions.Resources,
	}
}
