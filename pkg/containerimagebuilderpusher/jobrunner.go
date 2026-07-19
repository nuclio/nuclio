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
	"regexp"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/platform/kube/clients/kube"
	"github.com/nuclio/nuclio/pkg/platform/kube/utils"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// jobRunner holds the generic Kubernetes-Job-lifecycle machinery shared by every backend.
type jobRunner struct {
	builderName          string
	kubeClientSet        kube.Client
	logger               logger.Logger
	builderConfiguration *ContainerBuilderConfiguration
	jobNameRegex         *regexp.Regexp
}

// newJobRunner constructs a jobRunner for the backend named builderName (e.g. "kaniko", "buildah").
func newJobRunner(builderName string,
	parentLogger logger.Logger,
	kubeClientSet kube.Client,
	builderConfiguration *ContainerBuilderConfiguration) (*jobRunner, error) {

	if builderConfiguration == nil {
		return nil, errors.New("Missing builder configuration")
	}

	// Valid job name is composed of a DNS-1123 subdomains which in turn must contain only lower case
	// alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com')
	jobNameRegex := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)

	return &jobRunner{
		builderName:          builderName,
		logger:               parentLogger.GetChild(builderName),
		kubeClientSet:        kubeClientSet,
		builderConfiguration: builderConfiguration,
		jobNameRegex:         jobNameRegex,
	}, nil
}

// GetKind returns the backend name this jobRunner was constructed for (e.g. "kaniko", "buildah").
func (r *jobRunner) GetKind() string {
	return r.builderName
}

// GetDefaultRegistryCredentialsSecretName returns the secret with credentials to push/pull from the
// docker registry. Shared across backends: they all read the same builderConfiguration field.
func (r *jobRunner) GetDefaultRegistryCredentialsSecretName() string {
	return r.builderConfiguration.DefaultRegistryCredentialsSecretName
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
// /tmp/<builderName>-builds (a directory served over HTTP by the dashboard),
// so the build Job's fetch-bundle init container can retrieve it.
func (r *jobRunner) createContainerBuildBundle(ctx context.Context,
	image string,
	contextDir string,
	tempDir string) (string, string, error) {

	buildDir := fmt.Sprintf("/tmp/%s-builds", r.builderName)

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
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return "", "", errors.Wrapf(err, "Failed to ensure directory")
	}

	// Create symlink to bundle tar file in nginx serving directory
	assetPath := path.Join(buildDir, path.Base(tarFile.Name()))
	r.logger.DebugWithCtx(ctx,
		"Creating symlink to bundle tar",
		"tarFileName", tarFile.Name(),
		"assetPath", assetPath)

	if err := os.Link(tarFile.Name(), assetPath); err != nil {
		return "", "", errors.Wrapf(err, "Failed to create symlink to build bundle")
	}

	return path.Base(tarFile.Name()), assetPath, nil
}

func (r *jobRunner) compileJobName(ctx context.Context, image string) string {

	functionName := strings.ReplaceAll(image, "/", "")
	functionName = strings.ReplaceAll(functionName, ":", "")
	functionName = strings.ReplaceAll(functionName, "-", "")
	randomSuffix := common.GenerateRandomString(10, common.SmallLettersAndNumbers)
	nuclioPrefix := "nuclio-"

	// Truncate function name so the job name won't exceed k8s limit of 63
	functionNameLimit := 63 - (len(r.builderConfiguration.JobPrefix) + len(randomSuffix) + len(nuclioPrefix) + 2)
	if len(functionName) > functionNameLimit {
		functionName = functionName[0:functionNameLimit]
	}

	jobName := fmt.Sprintf("%s%s.%s.%s", nuclioPrefix, r.builderConfiguration.JobPrefix, functionName, randomSuffix)

	// Fallback
	if !r.jobNameRegex.MatchString(jobName) {
		r.logger.DebugWithCtx(ctx,
			"Job name does not match k8s regex. Won't use function name",
			"jobName", jobName)
		jobName = fmt.Sprintf("%s.%s", r.builderConfiguration.JobPrefix, randomSuffix)
	}

	return jobName
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
