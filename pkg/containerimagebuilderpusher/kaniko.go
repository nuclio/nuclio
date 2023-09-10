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
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Kaniko struct {
	kubeClientSet        kubernetes.Interface
	logger               logger.Logger
	builderConfiguration *ContainerBuilderConfiguration
	jobNameRegex         *regexp.Regexp
	cmdRunner            cmdrunner.CmdRunner
}

func NewKaniko(logger logger.Logger,
	kubeClientSet kubernetes.Interface,
	builderConfiguration *ContainerBuilderConfiguration) (*Kaniko, error) {

	if builderConfiguration == nil {
		return nil, errors.New("Missing kaniko builder configuration")
	}

	// Valid job name is composed of a DNS-1123 subdomains which in turn must contain only lower case
	// alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com')
	jobNameRegex := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)

	shellRunner, err := cmdrunner.NewShellRunner(logger)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create shell runner")
	}

	kanikoBuilder := &Kaniko{
		logger:               logger.GetChild("kaniko"),
		kubeClientSet:        kubeClientSet,
		builderConfiguration: builderConfiguration,
		jobNameRegex:         jobNameRegex,
		cmdRunner:            shellRunner,
	}

	return kanikoBuilder, nil
}

func (k *Kaniko) GetKind() string {
	return "kaniko"
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
	jobSpec := k.compileJobSpec(ctx, namespace, buildOptions, bundleFilename)

	// create job
	k.logger.DebugWithCtx(ctx,
		"Creating job",
		"namespace", namespace,
		"jobSpec", jobSpec,
		"timeoutSeconds", buildOptions.BuildTimeoutSeconds,
	)
	job, err := k.kubeClientSet.
		BatchV1().
		Jobs(namespace).
		Create(ctx, jobSpec, metav1.CreateOptions{})
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
		buildOptions.ReadinessTimeoutSeconds)
}

func (k *Kaniko) GetOnbuildStages(onbuildArtifacts []runtime.Artifact) ([]string, error) {
	onbuildStages := make([]string, len(onbuildArtifacts))
	stage := 0

	for _, artifact := range onbuildArtifacts {
		if artifact.ExternalImage {
			continue
		}

		stage++
		if len(artifact.Name) == 0 {
			artifact.Name = fmt.Sprintf("onbuildStage-%d", stage)
		}

		baseImage := fmt.Sprintf("FROM %s AS %s", artifact.Image, artifact.Name)
		onbuildDockerfileContents := fmt.Sprintf(`%s
ARG NUCLIO_LABEL
ARG NUCLIO_ARCH
`, baseImage)

		onbuildStages = append(onbuildStages, onbuildDockerfileContents)
	}

	return onbuildStages, nil
}

func (k *Kaniko) GetDefaultRegistryCredentialsSecretName() string {
	return k.builderConfiguration.DefaultRegistryCredentialsSecretName
}

func (k *Kaniko) TransformOnbuildArtifactPaths(onbuildArtifacts []runtime.Artifact) (map[string]string, error) {
	stagedArtifactPaths := make(map[string]string)
	for _, artifact := range onbuildArtifacts {
		for source, destination := range artifact.Paths {
			var transformedSource string
			if artifact.ExternalImage {

				// Using external image as "stage"
				// Example: COPY --from=nginx:latest /etc/nginx/nginx.conf /nginx.conf
				transformedSource = fmt.Sprintf("--from=%s %s", artifact.Image, source)
			} else {

				// Using previously build image with index `artifactIndex` as "stage"
				transformedSource = fmt.Sprintf("--from=%s %s", artifact.Name, source)
			}
			stagedArtifactPaths[transformedSource] = destination
		}
	}
	return stagedArtifactPaths, nil
}

func (k *Kaniko) GetBaseImageRegistry(registry string) string {
	return k.builderConfiguration.DefaultBaseRegistryURL
}

func (k *Kaniko) GetRegistryKind() string {
	return k.builderConfiguration.RegistryKind
}

func (k *Kaniko) GetOnbuildImageRegistry(registry string) string {
	return k.builderConfiguration.DefaultOnbuildRegistryURL
}

func (k *Kaniko) createContainerBuildBundle(ctx context.Context,
	image string,
	contextDir string,
	tempDir string) (string, string, error) {

	// Create temp directory to store compressed container build bundle
	buildContainerBundleDir := path.Join(tempDir, "tar")
	if err := os.Mkdir(buildContainerBundleDir, 0744); err != nil {
		return "", "", errors.Wrapf(err, "Failed to create tar dir: %s", buildContainerBundleDir)
	}
	k.logger.DebugWithCtx(ctx, "Created tar dir", "dir", buildContainerBundleDir)

	tarFilename := fmt.Sprintf("%s.tar.gz", strings.ReplaceAll(image, "/", "_"))
	tarFilename = strings.ReplaceAll(tarFilename, ":", "_")
	tarFile, err := os.CreateTemp(buildContainerBundleDir, fmt.Sprintf("*-%s", tarFilename))
	if err != nil {
		return "", "", errors.Wrap(err, "Failed to create tar bundle")
	}

	// allow read on group
	tarFile.Chmod(0744) // nolint: errcheck

	// we do not use its fd
	tarFile.Close() // nolint: errcheck

	k.logger.DebugWithCtx(ctx, "Compressing build bundle", "tarFilePath", tarFile.Name())
	if _, err := k.cmdRunner.Run(&cmdrunner.RunOptions{
		WorkingDir: &buildContainerBundleDir,
	}, "tar -zcvf %s %s", path.Base(tarFile.Name()), contextDir); err != nil {
		return "", "", errors.Wrapf(err, "Failed to compress build bundle")
	}

	buildDir := "/tmp/kaniko-builds"
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return "", "", errors.Wrapf(err, "Failed to ensure directory")
	}

	// Create symlink to bundle tar file in nginx serving directory
	assetPath := path.Join(buildDir, path.Base(tarFile.Name()))
	k.logger.DebugWithCtx(ctx,
		"Creating symlink to bundle tar",
		"tarFileName", tarFile.Name(),
		"assetPath", assetPath)

	if err := os.Link(tarFile.Name(), assetPath); err != nil {
		return "", "", errors.Wrapf(err, "Failed to create symlink to build bundle")
	}

	return path.Base(tarFile.Name()), assetPath, nil
}

func (k *Kaniko) compileJobSpec(ctx context.Context,
	namespace string,
	buildOptions *BuildOptions,
	bundleFilename string) *batchv1.Job {

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

	serviceAccount := k.resolveServiceAccount(buildOptions)

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
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:            "kaniko-executor",
							Image:           k.builderConfiguration.KanikoImage,
							ImagePullPolicy: v1.PullPolicy(k.builderConfiguration.KanikoImagePullPolicy),
							Args:            buildArgs,
							VolumeMounts:    []v1.VolumeMount{tmpFolderVolumeMount},
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
	return kanikoJobSpec
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
	createRepoTemplate := "aws ecr create-repository --repository-name %s --region %s || true"
	createMainRepo := fmt.Sprintf(createRepoTemplate, buildOptions.RepoName, region)
	createCacheRepo := fmt.Sprintf(createRepoTemplate,
		fmt.Sprintf("%s/cache", buildOptions.RepoName),
		region)
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

func (k *Kaniko) compileJobName(ctx context.Context, image string) string {

	functionName := strings.ReplaceAll(image, "/", "")
	functionName = strings.ReplaceAll(functionName, ":", "")
	functionName = strings.ReplaceAll(functionName, "-", "")
	randomSuffix := common.GenerateRandomString(10, common.SmallLettersAndNumbers)
	nuclioPrefix := "nuclio-"

	// Truncate function name so the job name won't exceed k8s limit of 63
	functionNameLimit := 63 - (len(k.builderConfiguration.JobPrefix) + len(randomSuffix) + len(nuclioPrefix) + 2)
	if len(functionName) > functionNameLimit {
		functionName = functionName[0:functionNameLimit]
	}

	jobName := fmt.Sprintf("%s%s.%s.%s", nuclioPrefix, k.builderConfiguration.JobPrefix, functionName, randomSuffix)

	// Fallback
	if !k.jobNameRegex.MatchString(jobName) {
		k.logger.DebugWithCtx(ctx,
			"Job name does not match k8s regex. Won't use function name",
			"jobName", jobName)
		jobName = fmt.Sprintf("%s.%s", k.builderConfiguration.JobPrefix, randomSuffix)
	}

	return jobName
}

func (k *Kaniko) waitForJobCompletion(ctx context.Context,
	namespace string,
	jobName string,
	buildTimeoutSeconds int64,
	readinessTimoutSeconds int) error {
	k.logger.DebugWithCtx(ctx,
		"Waiting for job completion",
		"buildTimeoutSeconds", buildTimeoutSeconds,
		"readinessTimeoutSeconds", readinessTimoutSeconds)
	timeout := time.Now().Add(time.Duration(buildTimeoutSeconds) * time.Second)

	if err := k.resolveFailFast(ctx, namespace, jobName, time.Duration(readinessTimoutSeconds)*time.Second); err != nil {
		return errors.Wrap(err, "Kaniko job failed to run")
	}

	for time.Now().Before(timeout) {
		runningJob, err := k.kubeClientSet.
			BatchV1().
			Jobs(namespace).
			Get(context.Background(), jobName, metav1.GetOptions{})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				k.logger.WarnWithCtx(ctx,
					"Failed to pull kaniko job status",
					"err", err.Error())
			}
			time.Sleep(1 * time.Second)
			continue
		}

		if runningJob.Status.Succeeded > 0 {
			jobLogs, err := k.getJobPodLogs(ctx, jobName, namespace)
			if err != nil {
				k.logger.DebugWithCtx(ctx,
					"Job was completed successfully but failed to retrieve job logs",
					"err", err.Error())
				return nil
			}

			k.logger.DebugWithCtx(ctx,
				"Job was completed successfully",
				"jobLogs", jobLogs)
			return nil
		}
		if runningJob.Status.Failed > 0 {
			jobPod, err := k.getJobPod(ctx, jobName, namespace, false)
			if err != nil {
				return errors.Wrap(err, "Failed to get job pod")
			}
			k.logger.WarnWithCtx(ctx,
				"Build container image job has failed",
				"initContainerStatuses", jobPod.Status.InitContainerStatuses,
				"containerStatuses", jobPod.Status.ContainerStatuses,
				"conditions", jobPod.Status.Conditions,
				"reason", jobPod.Status.Reason,
				"message", jobPod.Status.Message,
				"phase", jobPod.Status.Phase,
				"jobName", jobName)

			jobLogs, err := k.getPodLogs(ctx, jobPod)
			if err != nil {
				k.logger.WarnWithCtx(ctx,
					"Failed to get job logs", "err", err.Error())
				return errors.Wrap(err, "Failed to retrieve kaniko job logs")
			}
			return fmt.Errorf("Job failed. Job logs:\n%s", jobLogs)
		}

		k.logger.DebugWithCtx(ctx,
			"Waiting for job completion",
			"ttl", time.Until(timeout).String(),
			"jobName", jobName)
		time.Sleep(10 * time.Second)
	}

	jobPod, err := k.getJobPod(ctx, jobName, namespace, false)
	if err != nil {
		return errors.Wrap(err, "Job failed and was unable to get job pod")
	}

	k.logger.WarnWithCtx(ctx,
		"Build container image job has timed out",
		"initContainerStatuses", jobPod.Status.InitContainerStatuses,
		"containerStatuses", jobPod.Status.ContainerStatuses,
		"conditions", jobPod.Status.Conditions,
		"reason", jobPod.Status.Reason,
		"message", jobPod.Status.Message,
		"phase", jobPod.Status.Phase,
		"jobName", jobName)

	jobLogs, err := k.getPodLogs(ctx, jobPod)
	if err != nil {
		return errors.Wrap(err, "Job failed and was unable to retrieve job logs")
	}
	return fmt.Errorf("Job has timed out. Job logs:\n%s", jobLogs)
}

func (k *Kaniko) resolveFailFast(ctx context.Context,
	namespace,
	jobName string,
	readinessTimout time.Duration) error {

	// fail fast timeout is max(readinessTimeout, 5 minutes)
	if readinessTimout < 5*time.Minute {
		readinessTimout = 5 * time.Minute
	}
	failFastTimeout := time.After(readinessTimout)

	// fail fast if job pod stuck in Pending or Unknown state
	for {
		select {
		case <-failFastTimeout:
			k.logger.WarnWithCtx(ctx,
				"Kaniko job was not completed in time",
				"jobName", jobName,
				"failFastTimeoutDuration", readinessTimout.String())

			return fmt.Errorf("Job was not completed in time, job name:\n%s", jobName)
		default:
			jobPod, err := k.getJobPod(ctx, jobName, namespace, true)
			if err != nil {
				k.logger.WarnWithCtx(ctx,
					"Failed to get kaniko job pod",
					"jobName", jobName,
					"err", err.Error())
				time.Sleep(5 * time.Second)

				// skip in case job hasn't started yet. it will fail on timeout if getJobPod keeps failing.
				continue
			}
			if jobPod.Status.Phase == v1.PodPending || jobPod.Status.Phase == v1.PodUnknown {
				time.Sleep(5 * time.Second)
				continue
			}
			return nil
		}
	}
}

func (k *Kaniko) getJobPodLogs(ctx context.Context, jobName string, namespace string) (string, error) {
	jobPod, err := k.getJobPod(ctx, jobName, namespace, false)
	if err != nil {
		return "", errors.Wrap(err, "Failed to get job pod")
	}
	return k.getPodLogs(ctx, jobPod)
}

func (k *Kaniko) getPodLogs(ctx context.Context, jobPod *v1.Pod) (string, error) {
	k.logger.DebugWithCtx(ctx,
		"Fetching pod logs",
		"name", jobPod.Name,
		"namespace", jobPod.Namespace)

	// find job pod
	restClientRequest := k.kubeClientSet.
		CoreV1().
		Pods(jobPod.Namespace).
		GetLogs(jobPod.Name, &v1.PodLogOptions{})

	restReadCloser, err := restClientRequest.Stream(ctx)
	if err != nil {
		return "", errors.Wrap(err, "Failed to get log read/closer")
	}

	defer restReadCloser.Close() // nolint: errcheck

	logContents, err := io.ReadAll(restReadCloser)
	if err != nil {
		return "", errors.Wrap(err, "Failed to read logs")
	}

	formattedLogContents := k.prettifyLogContents(string(logContents))

	return formattedLogContents, nil
}

func (k *Kaniko) getJobPod(ctx context.Context, jobName, namespace string, quiet bool) (*v1.Pod, error) {
	if !quiet {
		k.logger.DebugWithCtx(ctx, "Getting job pods", "jobName", jobName)
	}
	jobPods, err := k.kubeClientSet.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})

	if err != nil {
		return nil, errors.Wrapf(err, "Failed to list job's pods")
	}
	if len(jobPods.Items) == 0 {
		return nil, errors.New("No pods found for job")
	}
	if len(jobPods.Items) > 1 {
		return nil, errors.New("Got too many job pods")
	}
	return &jobPods.Items[0], nil
}

func (k *Kaniko) prettifyLogContents(logContents string) string {
	scanner := bufio.NewScanner(strings.NewReader(logContents))

	formattedLogLinesArray := &[]string{}

	for scanner.Scan() {
		logLine := scanner.Text()

		prettifiedLogLine := k.prettifyLogLine(logLine)

		*formattedLogLinesArray = append(*formattedLogLinesArray, prettifiedLogLine)
	}

	return strings.Join(*formattedLogLinesArray, "\n")
}

func (k *Kaniko) prettifyLogLine(logLine string) string {

	// remove ansi color characters generated automatically by kaniko - so the log will be human-readable on the UI
	logLine = common.RemoveANSIColorsFromString(logLine)

	return logLine
}

func (k *Kaniko) deleteJob(ctx context.Context, namespace string, jobName string) error {
	k.logger.DebugWithCtx(ctx, "Deleting job", "namespace", namespace, "job", jobName)

	propagationPolicy := metav1.DeletePropagationBackground
	if err := k.kubeClientSet.BatchV1().Jobs(namespace).Delete(ctx, jobName, metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	}); err != nil {
		k.logger.WarnWithCtx(ctx,
			"Failed to delete kaniko job",
			"namespace", namespace,
			"job", jobName,
			"error", err.Error(),
		)
		return errors.Wrap(err, "Failed to delete job")
	}
	k.logger.DebugWithCtx(ctx, "Successfully deleted job", "namespace", namespace, "job", jobName)
	return nil
}

func (k *Kaniko) matchECRUrl(registryURL string) bool {
	return strings.Contains(registryURL, ".amazonaws.com") && strings.Contains(registryURL, ".ecr.")
}

func (k *Kaniko) resolveAWSRegionFromECR(registryURL string) string {
	return strings.Split(registryURL, ".")[3]
}

func (k *Kaniko) resolveServiceAccount(buildOptions *BuildOptions) string {

	// if a builder service account is provided in build options, use it.
	if buildOptions.BuilderServiceAccount != "" {
		return buildOptions.BuilderServiceAccount
	}
	// otherwise, if default service account is provided in builder configuration, use it.
	if k.builderConfiguration.DefaultServiceAccount != "" {
		return k.builderConfiguration.DefaultServiceAccount
	}
	// otherwise, use function service account.
	return buildOptions.FunctionServiceAccount
}
