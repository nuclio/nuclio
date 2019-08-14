package containerimagebuilder

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/errors"
	batch_v1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Kaniko struct {
	kubeClientSet        kubernetes.Interface
	logger               logger.Logger
	builderConfiguration *ContainerBuilderConfiguration
}

func NewKaniko(logger logger.Logger, kubeClientSet kubernetes.Interface,
	builderConfiguration *ContainerBuilderConfiguration) (*Kaniko, error) {

	if builderConfiguration == nil {
		return nil, errors.New("Missing kaniko builder configuration")
	}

	kanikoBuilder := &Kaniko{
		logger:               logger,
		kubeClientSet:        kubeClientSet,
		builderConfiguration: builderConfiguration,
	}

	return kanikoBuilder, nil
}

func (k *Kaniko) BuildAndPushContainerImage(buildOptions *BuildOptions, namespace string) error {
	bundleFilename, err := k.createDockerBuildBundle(buildOptions.Image, buildOptions.ContextDir, buildOptions.TempDir)
	if err != nil {
		return errors.Wrap(err, "Failed to create docker build bundle")
	}

	// Generate kaniko job spec
	kanikoJobSpec := k.getKanikoJobSpec(buildOptions, bundleFilename)

	k.logger.DebugWith("About to publish kaniko job", "jobSpec", kanikoJobSpec)
	kanikoJob, err := k.kubeClientSet.BatchV1().Jobs(namespace).Create(kanikoJobSpec)
	if err != nil {
		return errors.Wrap(err, "Failed to publish kaniko job")
	}

	k.logger.Debug("Waiting for kaniko to finish")
	timeout := time.Now().Add(10 * time.Minute)
	for time.Now().Before(timeout) {
		runningJob, err := k.kubeClientSet.BatchV1().Jobs(namespace).
			Get(kanikoJob.Name, meta_v1.GetOptions{IncludeUninitialized: true})

		if err != nil {
			return errors.Wrap(err, "Failed to poll kaniko job status")
		}

		if runningJob.Status.Succeeded > 0 {
			k.logger.Debug("Kaniko job was completed successfully")

			// TODO: Enable cleanup
			//err = p.consumer.kubeClientSet.BatchV1().Jobs(p.ResolveDefaultNamespace("")).Delete(kanikoJob.Name, nil)
			//if err != nil {
			//	p.Logger.Error("Failed to delete Kaniko job after successful completion")
			//}
			return nil
		}
		if runningJob.Status.Failed > 0 {
			k.logger.DebugWith("Kaniko job has failed", "status", runningJob.Status)
			return errors.New("Kaniko job has failed")
		}
	}
	return errors.New("Kaniko job has timed out")
}

func (k *Kaniko) createDockerBuildBundle(image string, contextDir string, tempDir string) (string, error) {
	var err error

	// Create temp directory to store compressed docker build bundle
	buildDockerBundleDir := path.Join(tempDir, "tar")
	err = os.Mkdir(buildDockerBundleDir, 0744)
	if err != nil {
		return "", errors.Wrapf(err, "Failed to create tar dir: %s", buildDockerBundleDir)
	}

	k.logger.DebugWith("Created tar dir", "dir", buildDockerBundleDir)

	tarFilename := fmt.Sprintf("%s.tar.gz", strings.Replace(image, "/", "_", -1))
	tarFilename = strings.Replace(tarFilename, ":", "_", -1)
	tarFilePath := path.Join(buildDockerBundleDir, tarFilename)

	k.logger.DebugWith("Compressing build bundle", "tarFilePath", tarFilePath)
	_, err = exec.Command("tar", "-zcvf", tarFilePath, contextDir).Output()
	if err != nil {
		return "", errors.Wrapf(err, "Failed to compress build bundle")
	}
	k.logger.Debug("Build bundle was successfully compressed")

	// Create symlink to bundle tar file in nginx serving directory
	err = os.Link(tarFilePath, path.Join("/etc/nginx/static/assets", tarFilename))
	if err != nil {
		return "", errors.Wrapf(err, "Failed to create symlink to build bundle")
	}

	return tarFilename, nil
}

func (k *Kaniko) getKanikoJobSpec(buildOptions *BuildOptions, bundleFilename string) *batch_v1.Job {

	completions := int32(1)
	buildArgs := []string{
		fmt.Sprintf("--dockerfile=%s", buildOptions.DockerfilePath),
		fmt.Sprintf("--context=%s", buildOptions.ContextDir),
		"--no-push",
	}

	// TODO: Enable once push to docker registry is sorted out
	//if !buildOptions.NoCache {
	//	buildArgs = append(buildArgs, "--cache=true")
	//}

	// Add build options args
	for k, v := range buildOptions.BuildArgs {
		buildArgs = append(buildArgs, fmt.Sprintf("--build-arg=%s=%s", k, v))
	}

	volumeMount := v1.VolumeMount{
		Name:      "tmp",
		MountPath: "/tmp",
	}

	functionName := strings.Replace(buildOptions.Image, "/", "-", -1)
	functionName = strings.Replace(functionName, ":", "-", -1)
	jobName := fmt.Sprintf("%s.%s", k.builderConfiguration.JobPrefix, functionName)

	kanikoJobSpec := &batch_v1.Job{
		ObjectMeta: meta_v1.ObjectMeta{
			Name: jobName,
		},
		Spec: batch_v1.JobSpec{
			Completions: &completions,
			Template: v1.PodTemplateSpec{
				ObjectMeta: meta_v1.ObjectMeta{
					Name: jobName,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:         "kaniko-executor",
							Image:        k.builderConfiguration.KanikoImage,
							Args:         buildArgs,
							VolumeMounts: []v1.VolumeMount{volumeMount},
						},
					},
					InitContainers: []v1.Container{
						{
							Name:  "fetch-bundle",
							Image: k.builderConfiguration.BusyBoxImage,
							Command: []string{
								"wget",
								fmt.Sprintf("http://%s:8070/assets/%s", os.Getenv("DASHBOARD_DEPLOYMENT_NAME"), bundleFilename),
								"-P",
								"/tmp",
							},
							VolumeMounts: []v1.VolumeMount{volumeMount},
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
							VolumeMounts: []v1.VolumeMount{volumeMount},
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
					RestartPolicy: v1.RestartPolicyOnFailure,
				},
			},
		},
	}

	return kanikoJobSpec
}
