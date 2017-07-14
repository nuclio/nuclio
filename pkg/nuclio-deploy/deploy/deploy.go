package deploy

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/nuclio/nuclio/pkg/util/cmd"
	"github.com/nuclio/nuclio-sdk/logger"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/kubernetes"
	"github.com/nuclio/nuclio/pkg/functioncr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"strings"
)

type Options struct {
	Verbose        bool
	KubeconfigPath string
	RegistryURL    string
	HTTPPort       int
	ImageName      string
}

type Deployer struct {
	logger  logger.Logger
	options *Options
}

func NewDeployer(parentLogger logger.Logger, options *Options) *Deployer {
	return &Deployer{
		logger:  parentLogger.GetChild("deployer").(logger.Logger),
		options: options,
	}
}

func (d *Deployer) Deploy() error {
	d.logger.DebugWith("Deploying", "options", d.options)

	// push the image to the registry
	taggedImage, err := d.pushImageToRegistry()
	if err != nil {
		return errors.Wrap(err, "Failed to push image to registry")
	}

	// create function custom resource
	err = d.createFunctionCR(taggedImage)
	if err != nil {
		return errors.Wrap(err, "Failed to created function custom resource")
	}

	return nil
}

func (d *Deployer) createFunctionCR(taggedImage string) error {
	restConfig, err := clientcmd.BuildConfigFromFlags("", d.options.KubeconfigPath)
	if err != nil {
		return errors.Wrap(err, "Failed to create REST config")
	}

	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return errors.Wrap(err, "Failed to create client set")
	}

	// create a client for function custom resources
	functioncrClient, err := functioncr.NewClient(d.logger,
		restConfig,
		clientSet)

	if err != nil {
		return errors.Wrap(err, "Failed to create function custom resource client")
	}

	// get the function name from the image
	functionName := d.getFunctionName()

	var function functioncr.Function
	function.TypeMeta.APIVersion = "nuclio.io/v1"
	function.TypeMeta.Kind = "Function"
	function.ObjectMeta.Name = functionName
	function.ObjectMeta.Namespace = "default"
	function.Spec.Image = taggedImage
	function.Spec.Replicas = 1
	function.Spec.HTTPPort = int32(d.options.HTTPPort)

	// first, try to delete function
	err = functioncrClient.Delete("default", functionName, nil)
	if err != nil {

		// if the error is that it's not found, don't stop
		if !apierrors.IsNotFound(err) {
			return errors.Wrap(err, "Failed to delete function")
		}

		d.logger.DebugWith("Function does not exist - didn't delete", "function", functionName)
	} else {
		d.logger.DebugWith("Function existed - deleted (waiting a bit before creating)",
			"function", functionName)

		// workaround controller bug - fast delete/create doesn't work
		time.Sleep(5 * time.Second)

	}

	createdFunction, err := functioncrClient.Create(&function)
	if err != nil {
		return errors.Wrap(err, "Failed to create function custom resource")
	}

	d.logger.InfoWith("Created function", "function", createdFunction)

	return nil
}

func (d *Deployer) pushImageToRegistry() (string, error) {
	taggedImage := fmt.Sprintf("%s/%s", d.options.RegistryURL, d.options.ImageName)

	err := cmdutil.RunCommand(d.logger, nil, "docker tag %s %s", d.options.ImageName, taggedImage)
	if err != nil {
		return "", errors.Wrap(err, "Unable to tag image")
	}

	// untag at the end, ignore errors
	defer cmdutil.RunCommand(d.logger, nil, "docker rmi %s", taggedImage)

	err = cmdutil.RunCommand(d.logger, nil, "docker push %s", taggedImage)
	if err != nil {
		return "", errors.Wrap(err, "Unable to push image")
	}

	return taggedImage, nil
}

func (d *Deployer) getFunctionName() string {

	// currently assumes <name>:<label> or <name>
	return strings.Split(d.options.ImageName, ":")[0]
}
