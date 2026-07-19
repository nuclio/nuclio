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
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/platform/kube/clients/kube"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const kanikoKind = "kaniko"

type Kaniko struct {
	*jobRunner
}

func NewKaniko(logger logger.Logger,
	kubeClientSet kube.Client,
	builderConfiguration *ContainerBuilderConfiguration) (*Kaniko, error) {

	jr, err := newJobRunner(kanikoKind, logger, kubeClientSet, builderConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create kaniko job runner")
	}

	return &Kaniko{jobRunner: jr}, nil
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
	// create job
	k.logger.DebugWithCtx(ctx,
		"Creating job",
		"namespace", namespace,
		"jobSpec", jobSpec,
		"timeoutSeconds", buildOptions.BuildTimeoutSeconds,
	)
	job, err := k.kubeClientSet.CreateJob(ctx, namespace, jobSpec)
	if err != nil {
		return errors.Wrap(err, "Failed to publish kaniko job")
	}

	// Cleanup after 30 minutes, allowing to dev to inspect job / pod information before getting deleted
	defer time.AfterFunc(k.builderConfiguration.JobDeletionTimeout, func() {

		// Create a detached context to avoid cancellation of the deletion process
		detachedCtx := context.WithoutCancel(ctx)
		if err := k.deleteJob(detachedCtx, namespace, job.Name); err != nil {
			k.logger.WarnWithCtx(ctx,
				"Failed to delete job",
				"err", err.Error())
		}
	})

	// Wait for kaniko to finish
	return k.waitForJobCompletion(ctx,
		namespace,
		job.Name,
		buildOptions.BuildTimeoutSeconds,
		buildOptions.ReadinessTimeoutSeconds,
		buildOptions.BuildLogger)
}

func (k *Kaniko) compileJobSpec(ctx context.Context,
	namespace string,
	buildOptions *BuildOptions,
	bundleFilename string) (*batchv1.Job, error) {

	completions := int32(1)
	backoffLimit := int32(0)
	buildArgs := []string{
		fmt.Sprintf("--dockerfile=%s", buildOptions.DockerfileInfo.DockerfilePath),
		fmt.Sprintf("--context=%s", buildOptions.ContextDir),
		fmt.Sprintf("--destination=%s", common.CompileImageName(buildOptions.RegistryURL, buildOptions.Image)),
		fmt.Sprintf("--push-retry=%d", k.builderConfiguration.PushImagesRetries),
		fmt.Sprintf("--image-fs-extract-retry=%d", k.builderConfiguration.ImageFSExtractionRetries),
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

	tmpFolderVolumeMount := v1.VolumeMount{
		Name:      "tmp",
		MountPath: "/tmp",
	}
	jobName := k.compileJobName(ctx, buildOptions.Image)

	assetsURL := fmt.Sprintf("http://%s:8070/kaniko/%s", os.Getenv("NUCLIO_DASHBOARD_DEPLOYMENT_NAME"), bundleFilename)
	getAssetCommand := fmt.Sprintf("while true; do wget -T 5 -c %s -P %s && break; done", assetsURL, tmpFolderVolumeMount.MountPath)

	serviceAccount, err := k.enrichAndValidateServiceAccount(ctx, buildOptions, namespace)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to enrich and validate service account")
	}

	kanikoJobSpec := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
		},
		Spec: batchv1.JobSpec{
			Completions:           &completions,
			ActiveDeadlineSeconds: &buildOptions.BuildTimeoutSeconds,
			BackoffLimit:          &backoffLimit,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      jobName,
					Namespace: namespace,
					Labels:    common.CopyStringMapOrNil(k.builderConfiguration.KanikoPodLabels),
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:            "kaniko-executor",
							Image:           k.builderConfiguration.KanikoImage,
							ImagePullPolicy: v1.PullPolicy(k.builderConfiguration.KanikoImagePullPolicy),
							Args:            buildArgs,
							VolumeMounts:    []v1.VolumeMount{tmpFolderVolumeMount},
							Resources:       buildOptions.Resources,
						},
					},
					InitContainers: []v1.Container{
						{
							Name:            "fetch-bundle",
							Image:           k.builderConfiguration.BusyBoxImage,
							ImagePullPolicy: v1.PullPolicy(k.builderConfiguration.KanikoImagePullPolicy),
							Command: []string{
								"/bin/sh",
							},
							Args: []string{
								"-c",
								getAssetCommand,
							},
							VolumeMounts: []v1.VolumeMount{tmpFolderVolumeMount},
							Resources:    buildOptions.Resources,
						},
						{
							Name:            "extract-bundle",
							Image:           k.builderConfiguration.BusyBoxImage,
							ImagePullPolicy: v1.PullPolicy(k.builderConfiguration.KanikoImagePullPolicy),
							Command: []string{
								"tar",
								"-xvf",
								fmt.Sprintf("%s/%s", tmpFolderVolumeMount.MountPath, bundleFilename),
								"-C",
								"/",
							},
							VolumeMounts: []v1.VolumeMount{tmpFolderVolumeMount},
							Resources:    buildOptions.Resources,
						},
					},
					Volumes: []v1.Volume{
						{
							Name: tmpFolderVolumeMount.Name,
							VolumeSource: v1.VolumeSource{
								EmptyDir: &v1.EmptyDirVolumeSource{},
							},
						},
					},
					RestartPolicy:      v1.RestartPolicyNever,
					NodeSelector:       buildOptions.NodeSelector,
					NodeName:           buildOptions.NodeName,
					Affinity:           buildOptions.Affinity,
					PriorityClassName:  buildOptions.PriorityClassName,
					Tolerations:        buildOptions.Tolerations,
					ServiceAccountName: serviceAccount,
				},
			},
		},
	}

	k.configureSecretVolumeMount(buildOptions, kanikoJobSpec)
	return kanikoJobSpec, nil
}

func (k *Kaniko) configureSecretVolumeMount(buildOptions *BuildOptions, kanikoJobSpec *batchv1.Job) {
	if k.matchECRUrl(buildOptions.RegistryURL) {
		k.configureECRInitContainerAndMount(buildOptions, kanikoJobSpec)

		// if SecretName is defined - configure mount with docker credentials
	} else if len(buildOptions.SecretName) > 0 {

		// configure mount with docker credentials
		kanikoJobSpec.Spec.Template.Spec.Containers[0].VolumeMounts =
			append(kanikoJobSpec.Spec.Template.Spec.Containers[0].VolumeMounts, v1.VolumeMount{
				Name:      "docker-config",
				MountPath: "/kaniko/.docker",
				ReadOnly:  true,
			})

		kanikoJobSpec.Spec.Template.Spec.Volumes = append(kanikoJobSpec.Spec.Template.Spec.Volumes, v1.Volume{
			Name: "docker-config",
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName: buildOptions.SecretName,
					Items: []v1.KeyToPath{
						{
							Key:  ".dockerconfigjson",
							Path: "config.json",
						},
					},
				},
			},
		})
	}
}

func (k *Kaniko) configureECRInitContainerAndMount(buildOptions *BuildOptions, kanikoJobSpec *batchv1.Job) {

	// Add init container to create the main and cache repositories
	// fail silently in order to ignore "repository already exists" errors
	// if any other error occurs - kaniko will fail similarly
	region := k.resolveAWSRegionFromECR(buildOptions.RegistryURL)
	registryID := k.resolveAWSRegistryId(buildOptions.RegistryURL)
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
		ImagePullPolicy: v1.PullPolicy(k.builderConfiguration.KanikoImagePullPolicy),
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

func (k *Kaniko) matchECRUrl(registryURL string) bool {
	return strings.Contains(registryURL, ".amazonaws.com") && strings.Contains(registryURL, ".ecr.")
}

func (k *Kaniko) resolveAWSRegionFromECR(registryURL string) string {
	return strings.Split(registryURL, ".")[3]
}

// resolveAWSRegistryId extracts the AWS account ID (registry ID) from an ECR registry URL
// Example: "123456789012.dkr.ecr.us-east-1.amazonaws.com" -> "123456789012"
func (k *Kaniko) resolveAWSRegistryId(registryURL string) string {
	return strings.Split(registryURL, ".")[0]
}
