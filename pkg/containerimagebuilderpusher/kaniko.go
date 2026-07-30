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

package containerimagebuilderpusher

import (
	"context"
	"fmt"
	"os"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/containerimagebuilderpusher/registryhelpers"
	"github.com/nuclio/nuclio/pkg/platform/kube/clients/kube"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/api/core/v1"
)

const (
	KanikoKind = "kaniko"

	kanikoAuthDir = "/kaniko/.docker"
)

type Kaniko struct {
	*jobRunner
}

func NewKaniko(logger logger.Logger,
	kubeClientSet kube.Client,
	builderConfiguration *ContainerBuilderConfiguration) (*Kaniko, error) {

	jobRunner, err := newJobRunner(KanikoKind, logger, kubeClientSet, builderConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create kaniko job runner")
	}

	return &Kaniko{jobRunner: jobRunner}, nil
}

func (k *Kaniko) BuildAndPushContainerImage(ctx context.Context,
	buildOptions *BuildOptions,
	namespace string) error {
	bundleFilename, assetPath, err := k.createContainerBuildBundle(ctx,
		buildOptions.Image,
		buildOptions.ContextDir,
		buildOptions.TempDir)
	if err != nil {
		return errors.Wrap(err, "Failed to create container build bundle")
	}

	// Remove bundle file from NGINX assets once we are done
	defer os.Remove(assetPath) // nolint: errcheck

	// Generate job spec
	jobSpec, err := k.compileJobSpec(ctx, namespace, buildOptions, bundleFilename)
	if err != nil {
		return errors.Wrap(err, "Failed to compile kaniko job spec")
	}

	return k.submitAndWait(ctx, namespace, buildOptions, jobSpec)
}

func (k *Kaniko) compileJobSpec(ctx context.Context,
	namespace string,
	buildOptions *BuildOptions,
	bundleFilename string) (*batchv1.Job, error) {

	kanikoContainer := k.compileKanikoContainer(buildOptions)

	jobSpec, err := k.compileBaseJobSpec(ctx,
		namespace,
		buildOptions,
		bundleFilename,
		kanikoContainer,
		k.builderConfiguration.Kaniko.ImagePullPolicy)
	if err != nil {
		return nil, err
	}

	if err := k.configureRegistryAuthentication(ctx, namespace, buildOptions, jobSpec); err != nil {
		return nil, errors.Wrap(err, "Failed to configure registry auth volume mount")
	}
	return jobSpec, nil
}

func (k *Kaniko) compileKanikoContainer(buildOptions *BuildOptions) v1.Container {

	tmpFolderVolumeMount := v1.VolumeMount{
		Name:      "tmp",
		MountPath: "/tmp",
	}

	buildArgs := []string{
		fmt.Sprintf("--dockerfile=%s", buildOptions.DockerfileInfo.DockerfilePath),
		fmt.Sprintf("--context=%s", buildOptions.ContextDir),
		fmt.Sprintf("--destination=%s", common.CompileImageName(buildOptions.RegistryURL, buildOptions.Image)),
		fmt.Sprintf("--push-retry=%d", k.builderConfiguration.PushImagesRetries),
		fmt.Sprintf("--image-fs-extract-retry=%d", k.builderConfiguration.Kaniko.ImageFSExtractionRetries),
	}

	if !buildOptions.NoCache {
		buildArgs = append(buildArgs, "--cache=true")
	}

	if _, ok := buildOptions.BuildFlags["--insecure"]; !ok && k.builderConfiguration.InsecurePushRegistry {
		buildArgs = append(buildArgs, "--insecure")
	}

	if _, ok := buildOptions.BuildFlags["--insecure-pull"]; !ok && k.builderConfiguration.InsecurePullRegistry {
		buildArgs = append(buildArgs, "--insecure-pull")
	}

	// Add user's custom flags
	for flag := range buildOptions.BuildFlags {
		buildArgs = append(buildArgs, flag)
	}

	if k.builderConfiguration.CacheRepo != "" {
		buildArgs = append(buildArgs, fmt.Sprintf("--cache-repo=%s", k.builderConfiguration.CacheRepo))
	}

	// Add build options args
	for buildArgName, buildArgValue := range buildOptions.BuildArgs {
		buildArgs = append(buildArgs, fmt.Sprintf("--build-arg=%s=%s", buildArgName, buildArgValue))
	}

	return v1.Container{
		Name:            "kaniko-executor",
		Image:           k.builderConfiguration.Kaniko.Image,
		ImagePullPolicy: v1.PullPolicy(k.builderConfiguration.Kaniko.ImagePullPolicy),
		Args:            buildArgs,
		VolumeMounts:    []v1.VolumeMount{tmpFolderVolumeMount},
		Resources:       buildOptions.Resources,
	}
}

// configureRegistryAuthentication wires the registry authfile into the kaniko container. Kaniko has
// its own bundled cloud credential helpers, hence the nil cloudHosts - no login containers needed.
func (k *Kaniko) configureRegistryAuthentication(ctx context.Context, namespace string, buildOptions *BuildOptions, kanikoJobSpec *batchv1.Job) error {
	if registryhelpers.IsECRHost(buildOptions.RegistryURL) {
		k.configureECRInitContainerAndMount(buildOptions, kanikoJobSpec)
		return nil
	}

	podSpec := &kanikoJobSpec.Spec.Template.Spec

	return k.jobRunner.configureRegistryAuthentication(ctx,
		namespace,
		buildOptions,
		kanikoAuthDir,
		nil,
		k.builderConfiguration.Kaniko.ImagePullPolicy,
		podSpec)
}

func (k *Kaniko) configureECRInitContainerAndMount(buildOptions *BuildOptions, kanikoJobSpec *batchv1.Job) {

	// Add init container to create the main and cache repositories
	// fail silently in order to ignore "repository already exists" errors
	// if any other error occurs - kaniko will fail similarly
	region := registryhelpers.ECRRegion(buildOptions.RegistryURL)
	registryID := registryhelpers.ECRRegistryID(buildOptions.RegistryURL)
	createRepoTemplate := "aws ecr create-repository --repository-name %s --region %s --registry-id %s || true"
	createMainRepo := fmt.Sprintf(createRepoTemplate, buildOptions.RepoName, region, registryID)
	createCacheRepo := fmt.Sprintf(createRepoTemplate,
		fmt.Sprintf("%s/cache", buildOptions.RepoName),
		region,
		registryID)
	createReposCommand := fmt.Sprintf("%s && %s",
		createMainRepo,
		createCacheRepo)

	initContainer := v1.Container{
		Name:            "create-repos",
		Image:           k.builderConfiguration.AWSCLIImage,
		ImagePullPolicy: v1.PullPolicy(k.builderConfiguration.Kaniko.ImagePullPolicy),
		Command: []string{
			"/bin/sh",
		},
		Args: []string{
			"-c",
			createReposCommand,
		},
	}

	if k.builderConfiguration.RegistryProviderSecretName != "" {

		// mount AWS credentials file to /tmp for permissions reasons
		initContainer.Env = []v1.EnvVar{
			{
				Name:  "AWS_SHARED_CREDENTIALS_FILE",
				Value: "/tmp/credentials",
			},
		}
		initContainer.VolumeMounts = []v1.VolumeMount{
			{
				Name:      k.builderConfiguration.RegistryProviderSecretName,
				MountPath: "/tmp",
			},
		}

		// volume aws secret to kaniko
		kanikoJobSpec.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			kanikoJobSpec.Spec.Template.Spec.Containers[0].VolumeMounts,
			v1.VolumeMount{
				Name:      k.builderConfiguration.RegistryProviderSecretName,
				MountPath: "/root/.aws/",
			})
		kanikoJobSpec.Spec.Template.Spec.Volumes = append(kanikoJobSpec.Spec.Template.Spec.Volumes,
			v1.Volume{
				Name: k.builderConfiguration.RegistryProviderSecretName,
				VolumeSource: v1.VolumeSource{
					Secret: &v1.SecretVolumeSource{
						SecretName: k.builderConfiguration.RegistryProviderSecretName,
					},
				},
			})
	} else {

		// assume instance role has permissions to register and store a container image
		// https://github.com/GoogleContainerTools/kaniko#pushing-to-amazon-ecr
		kanikoJobSpec.Spec.Template.Spec.Containers[0].Env = append(kanikoJobSpec.Spec.Template.Spec.Containers[0].Env,
			v1.EnvVar{
				Name:  "AWS_SDK_LOAD_CONFIG",
				Value: "true",
			})
	}
	kanikoJobSpec.Spec.Template.Spec.InitContainers = append(kanikoJobSpec.Spec.Template.Spec.InitContainers, initContainer)
}
