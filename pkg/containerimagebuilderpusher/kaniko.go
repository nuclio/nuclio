package containerimagebuilderpusher

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"time"

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
}

func NewKaniko(logger logger.Logger, kubeClientSet kubernetes.Interface,
	builderConfiguration *ContainerBuilderConfiguration) (*Kaniko, error) {

	if builderConfiguration == nil {
		return nil, errors.New("Missing kaniko builder configuration")
	}

	// Valid job name is composed from a DNS-1123 subdomains which in turn must contain only lower case
	// alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com')
	jobNameRegex, err := regexp.Compile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)
	if err != nil {
		return nil, errors.New("Failed to compile job name regex")
	}

	kanikoBuilder := &Kaniko{
		logger:               logger,
		kubeClientSet:        kubeClientSet,
		builderConfiguration: builderConfiguration,
		jobNameRegex:         jobNameRegex,
	}

	return kanikoBuilder, nil
}

func (k *Kaniko) GetKind() string {
	return "kaniko"
}

func (k *Kaniko) BuildAndPushContainerImage(buildOptions *BuildOptions, namespace string) error {
	bundleFilename, assetPath, err := k.createContainerBuildBundle(buildOptions.Image, buildOptions.ContextDir, buildOptions.TempDir)
	if err != nil {
		return errors.Wrap(err, "Failed to create container build bundle")
	}

	// Remove bundle file from NGINX assets once we are done
	defer os.Remove(assetPath) // nolint: errcheck

	// Generate kaniko job spec
	kanikoJobSpec := k.compileKanikoJobSpec(namespace, buildOptions, bundleFilename)

	k.logger.DebugWith("Create kaniko job", "namespace", namespace, "jobSpec", kanikoJobSpec)
	kanikoJob, err := k.kubeClientSet.BatchV1().Jobs(namespace).Create(kanikoJobSpec)
	if err != nil {
		return errors.Wrap(err, "Failed to publish kaniko job")
	}

	// Cleanup
	defer k.deleteJob(namespace, kanikoJob.Name) // nolint: errcheck

	// Wait for kaniko to finish
	return k.waitForKanikoJobCompletion(namespace, kanikoJob.Name, buildOptions.BuildTimeoutSeconds)
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
	var err error

	// Create temp directory to store compressed container build bundle
	buildContainerBundleDir := path.Join(tempDir, "tar")
	err = os.Mkdir(buildContainerBundleDir, 0744)
	if err != nil {
		return "", "", errors.Wrapf(err, "Failed to create tar dir: %s", buildContainerBundleDir)
	}

	k.logger.DebugWith("Created tar dir", "dir", buildContainerBundleDir)

	tarFilename := fmt.Sprintf("%s.tar.gz", strings.Replace(image, "/", "_", -1))
	tarFilename = strings.Replace(tarFilename, ":", "_", -1)
	tarFilePath := path.Join(buildContainerBundleDir, tarFilename)

	k.logger.DebugWith("Compressing build bundle", "tarFilePath", tarFilePath)

	// Just in case file with the same filename already exist - delete it
	_ = os.Remove(tarFilePath)

	_, err = exec.Command("tar", "-zcvf", tarFilePath, contextDir).Output()
	if err != nil {
		return "", "", errors.Wrapf(err, "Failed to compress build bundle")
	}
	k.logger.Debug("Build bundle was successfully compressed")

	// Create symlink to bundle tar file in nginx serving directory
	assetPath := path.Join("/etc/nginx/static/assets", tarFilename)
	err = os.Link(tarFilePath, assetPath)
	if err != nil {
		return "", "", errors.Wrapf(err, "Failed to create symlink to build bundle")
	}

	return tarFilename, assetPath, nil
}

func (k *Kaniko) compileKanikoJobSpec(namespace string,
	buildOptions *BuildOptions, bundleFilename string) *batchv1.Job {
	completions := int32(1)
	backoffLimit := int32(0)
	buildArgs := []string{
		fmt.Sprintf("--dockerfile=%s", buildOptions.DockerfileInfo.DockerfilePath),
		fmt.Sprintf("--context=%s", buildOptions.ContextDir),
		fmt.Sprintf("--destination=%s/%s", buildOptions.RegistryURL, buildOptions.Image),
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
								"wget",
								fmt.Sprintf("http://%s:8070/assets/%s", os.Getenv("NUCLIO_DASHBOARD_DEPLOYMENT_NAME"), bundleFilename),
								"-P",
								"/tmp",
							},
							VolumeMounts: []v1.VolumeMount{tmpFolderVolumeMount},
						},
						{
							Name:  "extract-bundle",
							Image: k.builderConfiguration.BusyBoxImage,
							Command: []string{
								"tar",
								"-xvf",
								fmt.Sprintf("/tmp/%s", bundleFilename),
								"-C",
								"/",
							},
							VolumeMounts: []v1.VolumeMount{tmpFolderVolumeMount},
						},
					},
					Volumes: []v1.Volume{
						{
							Name: "tmp",
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

	functionName := strings.Replace(image, "/", "", -1)
	functionName = strings.Replace(functionName, ":", "", -1)
	functionName = strings.Replace(functionName, "-", "", -1)
	timestamp := fmt.Sprintf("%d", time.Now().Unix())

	// Truncate function name so the job name won't exceed k8s limit of 63
	functionNameLimit := 63 - (len(k.builderConfiguration.JobPrefix) + len(timestamp) + 2)
	if len(functionName) > functionNameLimit {
		functionName = functionName[0:functionNameLimit]
	}

	jobName := fmt.Sprintf("%s.%s.%s", k.builderConfiguration.JobPrefix, functionName, timestamp)

	// Fallback
	if !k.jobNameRegex.MatchString(jobName) {
		k.logger.DebugWith("Kaniko job name does not match k8s regex. Won't use function name", "jobName", jobName)
		jobName = fmt.Sprintf("%s.%s", k.builderConfiguration.JobPrefix, timestamp)
	}

	return jobName
}

func (k *Kaniko) waitForKanikoJobCompletion(namespace string, jobName string, buildTimeoutSeconds int64) error {
	k.logger.DebugWith("Waiting for kaniko to finish", "buildTimeoutSeconds", buildTimeoutSeconds)
	timeout := time.Now().Add(time.Duration(buildTimeoutSeconds) * time.Second)
	for time.Now().Before(timeout) {
		runningJob, err := k.kubeClientSet.BatchV1().Jobs(namespace).Get(jobName, metav1.GetOptions{})

		if err != nil {
			return errors.Wrap(err, "Failed to poll kaniko job status")
		}

		if runningJob.Status.Succeeded > 0 {
			jobLogs, err := k.getJobLogs(namespace, jobName)
			if err != nil {
				k.logger.Debug("Kaniko job was completed successfully but failed to retrieve job logs")
				return nil
			}

			k.logger.DebugWith("Kaniko job was completed successfully", "jobLogs", jobLogs)
			return nil
		}
		if runningJob.Status.Failed > 0 {
			jobLogs, err := k.getJobLogs(namespace, jobName)
			if err != nil {
				return errors.Wrap(err, "Failed to retrieve kaniko job logs")
			}
			return fmt.Errorf("Kaniko job failed. Job logs:\n%s", jobLogs)
		}

		time.Sleep(10 * time.Second)
	}
	jobLogs, err := k.getJobLogs(namespace, jobName)
	if err != nil {
		return errors.Wrap(err, "Kaniko job failed and was unable to retrieve job logs")
	}
	return fmt.Errorf("Kaniko job has timed out. Job logs:\n%s", jobLogs)
}

func (k *Kaniko) getJobLogs(namespace string, jobName string) (string, error) {
	k.logger.DebugWith("Fetching kaniko job logs", "namespace", namespace, "job", jobName)

	// list pods
	jobPods, err := k.kubeClientSet.CoreV1().Pods(namespace).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})

	if err != nil {
		return "", errors.Wrapf(err, "Failed to list job's pods")
	}
	if len(jobPods.Items) == 0 {
		return "", errors.New("No pods found for job")
	}
	if len(jobPods.Items) > 1 {
		return "", errors.New("Got too many job pods")
	}

	// find job pod
	restClientRequest := k.kubeClientSet.
		CoreV1().
		Pods(namespace).
		GetLogs(jobPods.Items[0].Name, &v1.PodLogOptions{})

	restReadCloser, err := restClientRequest.Stream()
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
	k.logger.DebugWith("Deleting kaniko job", "namespace", namespace, "job", jobName)

	propagationPolicy := metav1.DeletePropagationBackground
	if err := k.kubeClientSet.BatchV1().Jobs(namespace).Delete(jobName, &metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	}); err != nil {
		return errors.Wrap(err, "Failed to delete kaniko job")
	}
	return nil
}
