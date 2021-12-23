package containerimagebuilderpusher

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
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

	// Valid job name is composed from a DNS-1123 subdomains which in turn must contain only lower case
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

func (k *Kaniko) BuildAndPushContainerImage(buildOptions *BuildOptions, namespace string) error {
	bundleFilename, assetPath, err := k.createContainerBuildBundle(buildOptions.Image,
		buildOptions.ContextDir,
		buildOptions.TempDir)
	if err != nil {
		return errors.Wrap(err, "Failed to create container build bundle")
	}

	// Remove bundle file from NGINX assets once we are done
	defer os.Remove(assetPath) // nolint: errcheck

	// Generate job spec
	jobSpec := k.compileJobSpec(namespace, buildOptions, bundleFilename)

	// create job
	k.logger.DebugWith("Creating job", "namespace", namespace, "jobSpec", jobSpec)
	job, err := k.kubeClientSet.BatchV1().Jobs(namespace).Create(context.Background(), jobSpec, metav1.CreateOptions{})
	if err != nil {
		return errors.Wrap(err, "Failed to publish kaniko job")
	}

	// Cleanup after 30 minutes, allowing to dev to inspect job / pod information before getting deleted
	defer time.AfterFunc(k.builderConfiguration.JobDeletionTimeout, func() {
		if err := k.deleteJob(namespace, job.Name); err != nil {
			k.logger.WarnWith("Failed to delete job", "err", err.Error())
		}
	})

	// Wait for kaniko to finish
	return k.waitForJobCompletion(namespace, job.Name, buildOptions.BuildTimeoutSeconds)
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

func (k *Kaniko) GetOnbuildImageRegistry(registry string) string {
	return k.builderConfiguration.DefaultOnbuildRegistryURL
}

func (k *Kaniko) createContainerBuildBundle(image string, contextDir string, tempDir string) (string, string, error) {

	// Create temp directory to store compressed container build bundle
	buildContainerBundleDir := path.Join(tempDir, "tar")
	if err := os.Mkdir(buildContainerBundleDir, 0744); err != nil {
		return "", "", errors.Wrapf(err, "Failed to create tar dir: %s", buildContainerBundleDir)
	}
	k.logger.DebugWith("Created tar dir", "dir", buildContainerBundleDir)

	tarFilename := fmt.Sprintf("%s.tar.gz", strings.ReplaceAll(image, "/", "_"))
	tarFilename = strings.ReplaceAll(tarFilename, ":", "_")
	tarFile, err := ioutil.TempFile(buildContainerBundleDir, fmt.Sprintf("*-%s", tarFilename))
	if err != nil {
		return "", "", errors.Wrap(err, "Failed to create tar bundle")
	}

	// allow read on group
	tarFile.Chmod(0744) // nolint: errcheck

	// we dont use its fd
	tarFile.Close() // nolint: errcheck

	k.logger.DebugWith("Compressing build bundle", "tarFilePath", tarFile.Name())
	if _, err := k.cmdRunner.Run(&cmdrunner.RunOptions{
		WorkingDir: &buildContainerBundleDir,
	}, "tar -zcvf %s %s", path.Base(tarFile.Name()), contextDir); err != nil {
		return "", "", errors.Wrapf(err, "Failed to compress build bundle")
	}

	// Create symlink to bundle tar file in nginx serving directory
	assetPath := path.Join("/etc/nginx/static/assets", path.Base(tarFile.Name()))
	k.logger.DebugWith("Creating symlink to bundle tar",
		"tarFileName", tarFile.Name(),
		"assetPath", assetPath)
	if err := os.Link(tarFile.Name(), assetPath); err != nil {
		return "", "", errors.Wrapf(err, "Failed to create symlink to build bundle")
	}

	return path.Base(tarFile.Name()), assetPath, nil
}

func (k *Kaniko) compileJobSpec(namespace string,
	buildOptions *BuildOptions,
	bundleFilename string) *batchv1.Job {

	completions := int32(1)
	backoffLimit := int32(0)
	buildArgs := []string{
		fmt.Sprintf("--dockerfile=%s", buildOptions.DockerfileInfo.DockerfilePath),
		fmt.Sprintf("--context=%s", buildOptions.ContextDir),
		fmt.Sprintf("--destination=%s", common.CompileImageName(buildOptions.RegistryURL, buildOptions.Image)),
		fmt.Sprintf("--push-retry=%d", k.builderConfiguration.PushImagesRetries),
	}

	if !buildOptions.NoCache {
		buildArgs = append(buildArgs, "--cache=true")
	}

	if k.builderConfiguration.InsecurePushRegistry {
		buildArgs = append(buildArgs, "--insecure")
	}
	if k.builderConfiguration.InsecurePullRegistry {
		buildArgs = append(buildArgs, "--insecure-pull")
	}

	if k.builderConfiguration.CacheRepo != "" {
		buildArgs = append(buildArgs, fmt.Sprintf("--cache-repo=%s", k.builderConfiguration.CacheRepo))
	}

	// Add build options args
	for k, v := range buildOptions.BuildArgs {
		buildArgs = append(buildArgs, fmt.Sprintf("--build-arg=%s=%s", k, v))
	}

	tmpFolderVolumeMount := v1.VolumeMount{
		Name:      "tmp",
		MountPath: "/tmp",
	}

	jobName := k.compileJobName(buildOptions.Image)

	assetsURL := fmt.Sprintf("http://%s:8070/assets/%s", os.Getenv("NUCLIO_DASHBOARD_DEPLOYMENT_NAME"), bundleFilename)
	getAssetCommand := fmt.Sprintf("while true; do wget -T 5 -c %s -P %s && break; done", assetsURL, tmpFolderVolumeMount.MountPath)

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
							Name:  "fetch-bundle",
							Image: k.builderConfiguration.BusyBoxImage,
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
							Name:  "extract-bundle",
							Image: k.builderConfiguration.BusyBoxImage,
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
					RestartPolicy: v1.RestartPolicyNever,
				},
			},
		},
	}

	// if SecretName is defined - configure mount with docker credentials
	if len(buildOptions.SecretName) > 0 {

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

	return kanikoJobSpec
}

func (k *Kaniko) compileJobName(image string) string {

	functionName := strings.ReplaceAll(image, "/", "")
	functionName = strings.ReplaceAll(functionName, ":", "")
	functionName = strings.ReplaceAll(functionName, "-", "")
	timestamp := fmt.Sprintf("%d", time.Now().Unix())

	// Truncate function name so the job name won't exceed k8s limit of 63
	functionNameLimit := 63 - (len(k.builderConfiguration.JobPrefix) + len(timestamp) + 2)
	if len(functionName) > functionNameLimit {
		functionName = functionName[0:functionNameLimit]
	}

	jobName := fmt.Sprintf("%s.%s.%s", k.builderConfiguration.JobPrefix, functionName, timestamp)

	// Fallback
	if !k.jobNameRegex.MatchString(jobName) {
		k.logger.DebugWith("Job name does not match k8s regex. Won't use function name", "jobName", jobName)
		jobName = fmt.Sprintf("%s.%s", k.builderConfiguration.JobPrefix, timestamp)
	}

	return jobName
}

func (k *Kaniko) waitForJobCompletion(namespace string, jobName string, buildTimeoutSeconds int64) error {
	k.logger.DebugWith("Waiting for job completion", "buildTimeoutSeconds", buildTimeoutSeconds)
	timeout := time.Now().Add(time.Duration(buildTimeoutSeconds) * time.Second)
	for time.Now().Before(timeout) {
		runningJob, err := k.kubeClientSet.
			BatchV1().
			Jobs(namespace).
			Get(context.Background(), jobName, metav1.GetOptions{})
		if err != nil {
			return errors.Wrap(err, "Failed to poll kaniko job status")
		}

		if runningJob.Status.Succeeded > 0 {
			jobLogs, err := k.getJobPodLogs(namespace, jobName)
			if err != nil {
				k.logger.Debug("Job was completed successfully but failed to retrieve job logs",
					"err", err.Error())
				return nil
			}

			k.logger.DebugWith("Job was completed successfully", "jobLogs", jobLogs)
			return nil
		}
		if runningJob.Status.Failed > 0 {
			jobPod, err := k.getJobPod(jobName, namespace)
			if err != nil {
				return errors.Wrap(err, "Failed to get job pod")
			}
			k.logger.WarnWith("Build container image job has failed",
				"initContainerStatuses", jobPod.Status.InitContainerStatuses,
				"containerStatuses", jobPod.Status.ContainerStatuses,
				"conditions", jobPod.Status.Conditions,
				"reason", jobPod.Status.Reason,
				"message", jobPod.Status.Message,
				"phase", jobPod.Status.Phase,
				"jobName", jobName)

			jobLogs, err := k.getPodLogs(jobPod)
			if err != nil {
				k.logger.WarnWith("Failed to get job logs", "err", err.Error())
				return errors.Wrap(err, "Failed to retrieve kaniko job logs")
			}
			return fmt.Errorf("Job failed. Job logs:\n%s", jobLogs)
		}

		k.logger.DebugWith("Waiting for job completion",
			"ttl", time.Until(timeout),
			"jobName", jobName)
		time.Sleep(10 * time.Second)
	}

	jobPod, err := k.getJobPod(jobName, namespace)
	if err != nil {
		return errors.Wrap(err, "Job failed and was unable to get job pod")
	}

	k.logger.WarnWith("Build container image job has timed out",
		"initContainerStatuses", jobPod.Status.InitContainerStatuses,
		"containerStatuses", jobPod.Status.ContainerStatuses,
		"conditions", jobPod.Status.Conditions,
		"reason", jobPod.Status.Reason,
		"message", jobPod.Status.Message,
		"phase", jobPod.Status.Phase,
		"jobName", jobName)

	jobLogs, err := k.getPodLogs(jobPod)
	if err != nil {
		return errors.Wrap(err, "Job failed and was unable to retrieve job logs")
	}
	return fmt.Errorf("Job has timed out. Job logs:\n%s", jobLogs)
}

func (k *Kaniko) getJobPodLogs(jobName string, namespace string) (string, error) {
	jobPod, err := k.getJobPod(jobName, namespace)
	if err != nil {
		return "", errors.Wrap(err, "Failed to get job pod")
	}
	return k.getPodLogs(jobPod)
}

func (k *Kaniko) getPodLogs(jobPod *v1.Pod) (string, error) {
	k.logger.DebugWith("Fetching pod logs",
		"name", jobPod.Name,
		"namespace", jobPod.Namespace)

	// find job pod
	restClientRequest := k.kubeClientSet.
		CoreV1().
		Pods(jobPod.Namespace).
		GetLogs(jobPod.Name, &v1.PodLogOptions{})

	restReadCloser, err := restClientRequest.Stream(context.Background())
	if err != nil {
		return "", errors.Wrap(err, "Failed to get log read/closer")
	}

	defer restReadCloser.Close() // nolint: errcheck

	logContents, err := ioutil.ReadAll(restReadCloser)
	if err != nil {
		return "", errors.Wrap(err, "Failed to read logs")
	}

	formattedLogContents := k.prettifyLogContents(string(logContents))

	return formattedLogContents, nil
}

func (k *Kaniko) getJobPod(jobName, namespace string) (*v1.Pod, error) {
	k.logger.DebugWith("Getting job pods", "jobName", jobName)
	jobPods, err := k.kubeClientSet.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
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

	// remove ansi color characters generated automatically by kaniko - so the log will be human readable on the UI
	logLine = common.RemoveANSIColorsFromString(logLine)

	return logLine
}

func (k *Kaniko) deleteJob(namespace string, jobName string) error {
	k.logger.DebugWith("Deleting job", "namespace", namespace, "job", jobName)

	propagationPolicy := metav1.DeletePropagationBackground
	if err := k.kubeClientSet.BatchV1().Jobs(namespace).Delete(context.Background(), jobName, metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	}); err != nil {
		return errors.Wrap(err, "Failed to delete job")
	}
	k.logger.DebugWith("Successfully deleted job", "namespace", namespace, "job", jobName)
	return nil
}
