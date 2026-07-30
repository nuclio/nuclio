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
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/containerimagebuilderpusher/registryhelpers"
	"github.com/nuclio/nuclio/pkg/platform/kube/clients/kube"
	"github.com/nuclio/nuclio/pkg/platform/kube/utils"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const nuclioBuildsDir = "/tmp/nuclio-builds"

// jobRunner holds the generic Kubernetes-Job-lifecycle machinery shared by every backend.
type jobRunner struct {
	builderName          string
	kubeClientSet        kube.Client
	logger               logger.Logger
	builderConfiguration *ContainerBuilderConfiguration
}

// newJobRunner constructs a jobRunner for the backend named builderName (e.g. "kaniko", "buildah").
func newJobRunner(builderName string,
	parentLogger logger.Logger,
	kubeClientSet kube.Client,
	builderConfiguration *ContainerBuilderConfiguration) (*jobRunner, error) {

	if builderConfiguration == nil {
		return nil, errors.New("Missing builder configuration")
	}

	return &jobRunner{
		builderName:          builderName,
		logger:               parentLogger.GetChild(builderName),
		kubeClientSet:        kubeClientSet,
		builderConfiguration: builderConfiguration,
	}, nil
}

// GetKind returns the backend name this jobRunner was constructed for (e.g. "kaniko", "buildah").
func (r *jobRunner) GetKind() string {
	return r.builderName
}

// GetBaseImageRegistry returns the base image registry.
func (r *jobRunner) GetBaseImageRegistry(registry string) string {
	return r.builderConfiguration.DefaultBaseRegistryURL
}

// GetRegistryKind returns the registry kind (onCluster, offCluster, or empty if not specified).
func (r *jobRunner) GetRegistryKind() string {
	return r.builderConfiguration.RegistryKind
}

// GetOnbuildImageRegistry returns the onbuild base registry.
func (r *jobRunner) GetOnbuildImageRegistry(registry string) string {
	return r.builderConfiguration.DefaultOnbuildRegistryURL
}

// GetOnbuildStages builds the multistage-build FROM directives for onbuildArtifacts. Pure function of
// its argument, shared across backends.
func (r *jobRunner) GetOnbuildStages(onbuildArtifacts []runtime.Artifact) ([]string, error) {
	onbuildStages := make([]string, 0, len(onbuildArtifacts))
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
		onbuildDockerfileContents := fmt.Sprintf("%s\nARG NUCLIO_LABEL\nARG NUCLIO_ARCH\n", baseImage)
		if strings.TrimSpace(artifact.StageCommands) != "" {
			onbuildDockerfileContents += artifact.StageCommands + "\n"
		}

		onbuildStages = append(onbuildStages, onbuildDockerfileContents)
	}

	return onbuildStages, nil
}

// TransformOnbuildArtifactPaths changes onbuild artifact paths depending on the type of the builder
// used. Pure function of its argument, shared across backends.
func (r *jobRunner) TransformOnbuildArtifactPaths(onbuildArtifacts []runtime.Artifact) (map[string]string, error) {
	stagedArtifactPaths := make(map[string]string)
	for _, artifact := range onbuildArtifacts {
		for source, destination := range artifact.Paths {
			var transformedSource string
			if artifact.ExternalImage {

				// External image as stage
				transformedSource = fmt.Sprintf("--from=%s %s", artifact.Image, source)
			} else {

				// Previously built stage
				transformedSource = fmt.Sprintf("--from=%s %s", artifact.Name, source)
			}
			stagedArtifactPaths[transformedSource] = destination
		}
	}
	return stagedArtifactPaths, nil
}

// createContainerBuildBundle tars contextDir and symlinks the result into
// a directory served over HTTP by the dashboard, so the build Job's
// fetch-bundle init container can retrieve it.
func (r *jobRunner) createContainerBuildBundle(ctx context.Context,
	image string,
	contextDir string,
	tempDir string) (string, string, error) {

	// Create temp directory to store compressed container build bundle
	buildContainerBundleDir := path.Join(tempDir, "tar")
	if err := os.Mkdir(buildContainerBundleDir, 0744); err != nil {
		return "", "", errors.Wrapf(err, "Failed to create tar dir: %s", buildContainerBundleDir)
	}
	r.logger.DebugWithCtx(ctx, "Created tar dir", "dir", buildContainerBundleDir)

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

	r.logger.DebugWithCtx(ctx, "Compressing build bundle", "tarFilePath", tarFile.Name())
	tarCmd := exec.CommandContext(ctx, "tar", "-zcvf", path.Base(tarFile.Name()), contextDir)
	tarCmd.Dir = buildContainerBundleDir
	var tarStderr bytes.Buffer
	tarCmd.Stderr = &tarStderr
	if err := tarCmd.Run(); err != nil {
		return "", "", errors.Wrapf(err, "Failed to compress build bundle: %s", tarStderr.String())
	}

	// we need 755 permission to allow running nuclio function with non-root SecurityContext
	if err := os.MkdirAll(nuclioBuildsDir, 0755); err != nil {
		return "", "", errors.Wrapf(err, "Failed to ensure directory")
	}

	// Create symlink to bundle tar file in nginx serving directory
	assetPath := path.Join(nuclioBuildsDir, path.Base(tarFile.Name()))
	r.logger.DebugWithCtx(ctx,
		"Creating symlink to bundle tar",
		"tarFileName", tarFile.Name(),
		"assetPath", assetPath)

	if err := os.Link(tarFile.Name(), assetPath); err != nil {
		return "", "", errors.Wrapf(err, "Failed to create symlink to build bundle")
	}

	return path.Base(tarFile.Name()), assetPath, nil
}

// bundleFetchInitContainers returns the fetch-bundle and extract-bundle init containers shared by every backend.
func (r *jobRunner) bundleFetchInitContainers(bundleFilename string,
	initContainerPullPolicy string,
	tmpFolderVolumeMount v1.VolumeMount,
	resources v1.ResourceRequirements) []v1.Container {

	assetsURL := fmt.Sprintf("http://%s:8070/build/%s", os.Getenv("NUCLIO_DASHBOARD_DEPLOYMENT_NAME"), bundleFilename)
	getAssetCommand := fmt.Sprintf("while true; do wget -T 5 -c %s -P %s && break; sleep 2; done", assetsURL, tmpFolderVolumeMount.MountPath)

	return []v1.Container{
		{
			Name:            "fetch-bundle",
			Image:           r.builderConfiguration.BusyBoxImage,
			ImagePullPolicy: v1.PullPolicy(initContainerPullPolicy),
			Command: []string{
				"/bin/sh",
			},
			Args: []string{
				"-c",
				getAssetCommand,
			},
			VolumeMounts: []v1.VolumeMount{tmpFolderVolumeMount},
			Resources:    resources,
		},
		{
			Name:            "extract-bundle",
			Image:           r.builderConfiguration.BusyBoxImage,
			ImagePullPolicy: v1.PullPolicy(initContainerPullPolicy),
			Command: []string{
				"tar",
				"-xvf",
				fmt.Sprintf("%s/%s", tmpFolderVolumeMount.MountPath, bundleFilename),
				"-C",
				"/",
			},
			VolumeMounts: []v1.VolumeMount{tmpFolderVolumeMount},
			Resources:    resources,
		},
	}
}

// compileBaseJobSpec assembles the Job/Pod skeleton shared by every backend; callers splice in any
// backend-specific volumes/init-containers/podspec tweaks afterwards.
func (r *jobRunner) compileBaseJobSpec(ctx context.Context,
	namespace string,
	buildOptions *BuildOptions,
	bundleFilename string,
	mainContainer v1.Container,
	initContainerPullPolicy string) (*batchv1.Job, error) {

	completions := int32(1)
	backoffLimit := int32(0)

	tmpFolderVolumeMount := v1.VolumeMount{
		Name:      "tmp",
		MountPath: "/tmp",
	}

	jobNamePrefix := fmt.Sprintf("nuclio-%s-", r.builderConfiguration.JobPrefix)
	jobName, err := common.SanitizeKubernetesName(jobNamePrefix, buildOptions.Image, true)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to compile build job name prefix")
	}

	serviceAccount, err := r.enrichAndValidateServiceAccount(ctx, buildOptions, namespace)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to enrich and validate service account")
	}

	// mainContainer is expected to already mount tmpFolderVolumeMount (it needs the bundle contents).
	podSpec := v1.PodSpec{
		Containers: []v1.Container{mainContainer},
		InitContainers: r.bundleFetchInitContainers(bundleFilename,
			initContainerPullPolicy,
			tmpFolderVolumeMount,
			buildOptions.Resources),
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
		SecurityContext:    buildOptions.SecurityContext,
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: jobName,
			Namespace:    namespace,
		},
		Spec: batchv1.JobSpec{
			Completions:           &completions,
			ActiveDeadlineSeconds: &buildOptions.BuildTimeoutSeconds,
			BackoffLimit:          &backoffLimit,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Labels:    common.CopyStringMapOrNil(r.builderConfiguration.PodLabels),
				},
				Spec: podSpec,
			},
		},
	}, nil
}

// submitAndWait creates jobSpec, schedules its delayed deletion, and waits for it to complete.
func (r *jobRunner) submitAndWait(ctx context.Context,
	namespace string,
	buildOptions *BuildOptions,
	jobSpec *batchv1.Job) error {

	r.logger.DebugWithCtx(ctx,
		"Creating job",
		"namespace", namespace,
		"jobSpec", jobSpec,
		"timeoutSeconds", buildOptions.BuildTimeoutSeconds,
	)

	job, err := r.kubeClientSet.CreateJob(ctx, namespace, jobSpec)
	if err != nil {
		return errors.Wrapf(err, "Failed to publish %s job", r.builderName)
	}

	// Cleanup after JobDeletionTimeout, allowing the dev to inspect job/pod information before deletion
	defer time.AfterFunc(r.builderConfiguration.JobDeletionTimeout, func() {

		// Detached context so ctx cancellation doesn't skip the deletion
		detachedCtx := context.WithoutCancel(ctx)
		if err := r.deleteJob(detachedCtx, namespace, job.Name); err != nil {
			r.logger.WarnWithCtx(ctx,
				"Failed to delete job",
				"err", err.Error())
		}
	})

	return r.waitForJobCompletion(ctx,
		namespace,
		job.Name,
		buildOptions.BuildTimeoutSeconds,
		buildOptions.ReadinessTimeoutSeconds,
		buildOptions.BuildLogger)
}

func (r *jobRunner) waitForJobCompletion(ctx context.Context,
	namespace string,
	jobName string,
	buildTimeoutSeconds int64,
	readinessTimoutSeconds int,
	buildLogger logger.Logger) error {
	r.logger.DebugWithCtx(ctx,
		"Waiting for job completion",
		"buildTimeoutSeconds", buildTimeoutSeconds,
		"readinessTimeoutSeconds", readinessTimoutSeconds)
	timeout := time.Now().Add(time.Duration(buildTimeoutSeconds) * time.Second)

	if err := r.resolveFailFast(ctx, buildLogger, namespace, jobName, time.Duration(readinessTimoutSeconds)*time.Second); err != nil {
		return errors.Wrap(err, "Job failed to run")
	}

	for time.Now().Before(timeout) {
		runningJob, err := r.kubeClientSet.GetJob(ctx, namespace, jobName)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				r.logger.WarnWithCtx(ctx,
					"Failed to pull job status",
					"err", err.Error())
			}
			time.Sleep(1 * time.Second)
			continue
		}

		if runningJob.Status.Succeeded > 0 {
			jobLogs, err := r.getJobPodLogs(ctx, jobName, namespace)
			if err != nil {
				r.logger.DebugWithCtx(ctx,
					"Job was completed successfully but failed to retrieve job logs",
					"err", err.Error())
				return nil
			}

			r.logger.DebugWithCtx(ctx,
				"Job was completed successfully",
				"jobLogs", jobLogs)
			return nil
		}
		if runningJob.Status.Failed > 0 {
			jobPod, err := r.getJobPod(ctx, jobName, namespace, false)
			if err != nil {
				return errors.Wrap(err, "Failed to get job pod")
			}
			buildLogger.WarnWithCtx(ctx,
				"Build container image job has failed",
				"initContainerStatuses", jobPod.Status.InitContainerStatuses,
				"containerStatuses", jobPod.Status.ContainerStatuses,
				"conditions", jobPod.Status.Conditions,
				"reason", jobPod.Status.Reason,
				"message", jobPod.Status.Message,
				"phase", jobPod.Status.Phase,
				"jobName", jobName)

			jobLogs, err := r.getPodLogs(ctx, jobPod)
			if err != nil {
				buildLogger.WarnWithCtx(ctx,
					"Failed to get job logs", "err", err.Error())
				return errors.Wrap(err, "Failed to retrieve job logs")
			}
			return errors.Errorf("Job failed. Job logs:\n%s", jobLogs)
		}

		r.logger.DebugWithCtx(ctx,
			"Waiting for job completion",
			"ttl", time.Until(timeout).String(),
			"jobName", jobName)
		time.Sleep(10 * time.Second)
	}

	jobPod, err := r.getJobPod(ctx, jobName, namespace, false)
	if err != nil {
		return errors.Wrap(err, "Job failed and was unable to get job pod")
	}

	r.logger.WarnWithCtx(ctx,
		"Build container image job has timed out",
		"initContainerStatuses", jobPod.Status.InitContainerStatuses,
		"containerStatuses", jobPod.Status.ContainerStatuses,
		"conditions", jobPod.Status.Conditions,
		"reason", jobPod.Status.Reason,
		"message", jobPod.Status.Message,
		"phase", jobPod.Status.Phase,
		"jobName", jobName)

	jobLogs, err := r.getPodLogs(ctx, jobPod)
	if err != nil {
		return errors.Wrap(err, "Job failed and was unable to retrieve job logs")
	}
	return errors.Errorf("Job has timed out. Job logs:\n%s", jobLogs)
}

func (r *jobRunner) resolveFailFast(ctx context.Context,
	buildLogger logger.Logger,
	namespace,
	jobName string,
	readinessTimout time.Duration) error {

	// fail fast timeout is max(readinessTimeout, 5 minutes)
	if readinessTimout < 5*time.Minute {
		readinessTimout = 5 * time.Minute
	}
	failFastTimeout := time.After(readinessTimout)
	var lastError string

	// fail fast if job pod stuck in Pending or Unknown state
	for {
		select {
		case <-failFastTimeout:
			buildLogger.WarnWithCtx(ctx,
				"Job was not completed in time",
				"jobName", jobName,
				"failFastTimeoutDuration", readinessTimout.String())

			if lastError != "" {
				return errors.Errorf("Job was not completed in time, job name: %s. Error: %s ", jobName,
					lastError)
			} else {
				return errors.Errorf("Job was not completed in time, job name: %s", jobName)
			}
		default:
			jobPod, err := r.getJobPod(ctx, jobName, namespace, true)
			if err != nil {
				r.logger.WarnWithCtx(ctx,
					"Failed to get job pod",
					"jobName", jobName,
					"err", err.Error())
				time.Sleep(5 * time.Second)

				// skip in case job hasn't started yet. it will fail on timeout if getJobPod keeps failing.
				continue
			}
			if jobPod.Status.Phase == v1.PodPending || jobPod.Status.Phase == v1.PodUnknown {
				if failure, failed := r.getLastPodWarningEvent(ctx, namespace, jobPod.Name); failed {
					errorMessage := fmt.Sprintf("%s event for pod %s. Message: %s",
						failure.Reason,
						jobPod.Name,
						failure.Message)

					// if an error has changed, print it to the logs
					if errorMessage != lastError {
						buildLogger.WarnWithCtx(ctx,
							"Build pod received a warning event",
							"eventReason", failure.Reason,
							"eventMessage", failure.Message,
							"podName", jobPod.Name)
						lastError = errorMessage
					}
				}
				time.Sleep(5 * time.Second)
				continue
			}
			return nil
		}
	}
}

func (r *jobRunner) getJobPodLogs(ctx context.Context, jobName string, namespace string) (string, error) {
	jobPod, err := r.getJobPod(ctx, jobName, namespace, false)
	if err != nil {
		return "", errors.Wrap(err, "Failed to get job pod")
	}
	return r.getPodLogs(ctx, jobPod)
}

func (r *jobRunner) getPodLogs(ctx context.Context, jobPod *v1.Pod) (string, error) {
	r.logger.DebugWithCtx(ctx,
		"Fetching pod logs",
		"name", jobPod.Name,
		"namespace", jobPod.Namespace)

	restReadCloser, err := r.kubeClientSet.StreamPodLogs(ctx, jobPod.Namespace, jobPod.Name, &v1.PodLogOptions{})
	if err != nil {
		return "", errors.Wrap(err, "Failed to get log read/closer")
	}

	defer restReadCloser.Close() // nolint: errcheck

	logContents, err := io.ReadAll(restReadCloser)
	if err != nil {
		return "", errors.Wrap(err, "Failed to read logs")
	}

	formattedLogContents := r.prettifyLogContents(string(logContents))

	return formattedLogContents, nil
}

// getLastPodWarningEvent returns the last k8s warning event for a given pod
// if event found, then returns (event, true)
// else returns nil, false
func (r *jobRunner) getLastPodWarningEvent(ctx context.Context, namespace, podName string) (*v1.Event, bool) {
	events := r.getPodEvents(ctx, namespace, podName)
	if events == nil {
		return nil, false
	}
	// Iterate over the events and look for warnings
	for i := len(events.Items) - 1; i >= 0; i-- {
		if events.Items[i].Type == v1.EventTypeWarning {
			return &events.Items[i], true
		}
	}
	return nil, false
}

func (r *jobRunner) getPodEvents(ctx context.Context, namespace, podName string) *v1.EventList {

	events, err := r.kubeClientSet.ListEvents(ctx, namespace, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.kind=Pod,involvedObject.name=%s", podName),
	})

	if err != nil {
		r.logger.WarnWithCtx(ctx,
			"Failed to list events for pod",
			"podName", podName,
			"err", err.Error())
		return nil
	}
	return events
}

func (r *jobRunner) getJobPod(ctx context.Context, jobName, namespace string, quiet bool) (*v1.Pod, error) {
	if !quiet {
		r.logger.DebugWithCtx(ctx, "Getting job pods", "jobName", jobName)
	}
	jobPods, err := r.kubeClientSet.ListPods(ctx, namespace, metav1.ListOptions{
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

func (r *jobRunner) prettifyLogContents(logContents string) string {
	scanner := bufio.NewScanner(strings.NewReader(logContents))

	formattedLogLinesArray := &[]string{}

	for scanner.Scan() {
		logLine := scanner.Text()

		prettifiedLogLine := r.prettifyLogLine(logLine)

		*formattedLogLinesArray = append(*formattedLogLinesArray, prettifiedLogLine)
	}

	return strings.Join(*formattedLogLinesArray, "\n")
}

func (r *jobRunner) prettifyLogLine(logLine string) string {

	// remove ansi color characters generated automatically by the build tool - so the log will be human-readable on the UI
	logLine = common.RemoveANSIColorsFromString(logLine)

	return logLine
}

func (r *jobRunner) deleteJob(ctx context.Context, namespace string, jobName string) error {
	r.logger.DebugWithCtx(ctx, "Deleting job", "namespace", namespace, "job", jobName)

	propagationPolicy := metav1.DeletePropagationBackground
	if err := r.kubeClientSet.DeleteJob(ctx, namespace, jobName, metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	}); err != nil {
		r.logger.WarnWithCtx(ctx,
			"Failed to delete job",
			"namespace", namespace,
			"job", jobName,
			"error", err.Error(),
		)
		return errors.Wrap(err, "Failed to delete job")
	}
	r.logger.DebugWithCtx(ctx, "Successfully deleted job", "namespace", namespace, "job", jobName)
	return nil
}

func (r *jobRunner) enrichAndValidateServiceAccount(ctx context.Context, buildOptions *BuildOptions, namespace string) (string, error) {
	// try to enrich service account from builder configuration
	enrichedServiceAccount := r.enrichServiceAccountFromBuilderConfiguration(buildOptions)

	// enrich (from project/platform) and validate service account
	return utils.EnrichAndValidateServiceAccount(ctx,
		r.kubeClientSet,
		buildOptions.DefaultPlatformServiceAccount,
		buildOptions.ProjectSecretTemplate,
		buildOptions.ProjectSecretDefaultServiceAccountKey,
		buildOptions.ProjectSecretAllowedServiceAccountsKey,
		buildOptions.ProjectSecretForbiddenServiceAccountsKey,
		buildOptions.DefaultForbiddenServiceAccounts,
		enrichedServiceAccount,
		buildOptions.ProjectName,
		namespace,
		true,
	)
}

func (r *jobRunner) enrichServiceAccountFromBuilderConfiguration(buildOptions *BuildOptions) string {
	// if a builder service account is provided in build options, use it.
	if buildOptions.BuilderServiceAccount != "" {
		return buildOptions.BuilderServiceAccount
	}
	// otherwise, if default service account is provided in builder configuration, use it.
	if r.builderConfiguration.DefaultServiceAccount != "" {
		return r.builderConfiguration.DefaultServiceAccount
	}
	return buildOptions.FunctionServiceAccount
}

// resolveRegistryAuthSecretNames returns platform default secrets plus the function-level secret, deduped.
func (r *jobRunner) resolveRegistryAuthSecretNames(buildOptions *BuildOptions) []string {
	names := append([]string{}, r.builderConfiguration.DefaultRegistryCredentialsSecretNames...)
	if buildOptions.SecretName != "" {
		names = append(names, buildOptions.SecretName)
	}
	return common.RemoveDuplicatesFromSliceString(names)
}

// configureRegistryAuthentication wires the registry authfile - and, if needed, cloud-provider logins
// and a merge-authfile init container - into podSpec. cloudHosts is nil for kaniko, which uses its
// own bundled cloud credential helpers.
func (r *jobRunner) configureRegistryAuthentication(ctx context.Context,
	namespace string,
	buildOptions *BuildOptions,
	authFileDir string,
	cloudHosts []string,
	imagePullPolicy string,
	podSpec *v1.PodSpec) error {

	authVolumeMount := v1.VolumeMount{Name: registryhelpers.AuthVolumeName, MountPath: authFileDir, ReadOnly: true}
	cloudAuth := registryhelpers.NeedsCloudLogin(cloudHosts)
	secretNames := r.resolveRegistryAuthSecretNames(buildOptions)

	if !cloudAuth {
		// no cloud auth & no secret names, nothing to do
		if len(secretNames) == 0 {
			return nil
		}
		// no cloud auth & one secret name, create a secret volume
		if len(secretNames) == 1 {
			podSpec.Volumes = append(podSpec.Volumes, v1.Volume{
				Name: registryhelpers.AuthVolumeName,
				VolumeSource: v1.VolumeSource{
					Secret: &v1.SecretVolumeSource{
						SecretName: secretNames[0],
						Items:      []v1.KeyToPath{{Key: ".dockerconfigjson", Path: "config.json"}},
					},
				},
			})
			podSpec.Containers[0].VolumeMounts = append(podSpec.Containers[0].VolumeMounts, authVolumeMount)
			return nil
		}
	}

	// cloud auth and/or multiple secrets names: use merge auth init container
	podSpec.Volumes = append(podSpec.Volumes, v1.Volume{
		Name:         registryhelpers.AuthVolumeName,
		VolumeSource: v1.VolumeSource{EmptyDir: &v1.EmptyDirVolumeSource{}},
	})
	podSpec.Containers[0].VolumeMounts = append(podSpec.Containers[0].VolumeMounts, authVolumeMount)

	if cloudAuth {
		loginContainers, loginVolumes, err := registryhelpers.BuildLoginContainers(cloudHosts,
			buildOptions.RegistryURL,
			buildOptions.RepoName,
			r.builderConfiguration.AuthConfig,
			imagePullPolicy)
		if err != nil {
			return err
		}
		tokenDirMount := registryhelpers.TokenDirVolumeMount()
		podSpec.Volumes = append(podSpec.Volumes, v1.Volume{
			Name:         tokenDirMount.Name,
			VolumeSource: v1.VolumeSource{EmptyDir: &v1.EmptyDirVolumeSource{}},
		})
		podSpec.InitContainers = append(podSpec.InitContainers, loginContainers...)
		podSpec.Volumes = append(podSpec.Volumes, loginVolumes...)
	}

	if err := r.ensureMergeScriptConfigMap(ctx, namespace); err != nil {
		return err
	}

	mergeContainer, mergeVolumes := registryhelpers.BuildMergeAuthInitContainer(secretNames,
		authFileDir,
		cloudAuth,
		r.builderConfiguration.AuthConfig)
	podSpec.InitContainers = append(podSpec.InitContainers, mergeContainer)
	podSpec.Volumes = append(podSpec.Volumes, mergeVolumes...)

	return nil
}

// ensureMergeScriptConfigMap applies the namespace-shared ConfigMap carrying the embedded
// merge_authfile.py script. Server-side apply creates or updates it in one call, needing
// no conflict handling since every writer applies the content it embeds.
func (r *jobRunner) ensureMergeScriptConfigMap(ctx context.Context, namespace string) error {
	configMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      registryhelpers.MergeScriptConfigMapName,
			Namespace: namespace,
			Labels:    common.CopyStringMapOrNil(r.builderConfiguration.PodLabels),
		},
		Data: map[string]string{"merge_authfile.py": registryhelpers.MergeScriptContents()},
	}

	if _, err := r.kubeClientSet.ApplyConfigMap(ctx, namespace, configMap); err != nil {
		return errors.Wrap(err, "Failed to apply registry auth merge script config map")
	}
	return nil
}
