package deleter

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/nuclio/nuclio/pkg/functioncr"
	"github.com/nuclio/nuclio/pkg/nuclio-cli"

	"github.com/nuclio/nuclio-sdk"
	"github.com/pkg/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type FunctionDeleter struct {
	nucliocli.KubeConsumer
	logger           nuclio.Logger
	options          *Options
	functioncrClient *functioncr.Client
	clientset        *kubernetes.Clientset
}

func NewFunctionDeleter(parentLogger nuclio.Logger, options *Options) (*FunctionDeleter, error) {
	var err error

	newFunctionDeleter := &FunctionDeleter{
		logger:  parentLogger.GetChild("deleter").(nuclio.Logger),
		options: options,
	}

	// get kube stuff
	_, newFunctionDeleter.clientset,
		newFunctionDeleter.functioncrClient,
		err = newFunctionDeleter.GetClients(newFunctionDeleter.logger, options.Common.KubeconfigPath)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to get clients")
	}

	return newFunctionDeleter, nil
}

func (fd *FunctionDeleter) Execute() error {
	var err error

	resourceName, _, err := fd.parseResourceIdentifier(fd.options.ResourceIdentifier)
	if err != nil {
		return errors.Wrap(err, "Failed to parse resource identifier")
	}

	// get specific function CR
	return fd.functioncrClient.Delete(fd.options.Common.Namespace, resourceName, &meta_v1.DeleteOptions{})
}

func (fd *FunctionDeleter) parseResourceIdentifier(resourceIdentifier string) (resourceName string,
	resourceVersion *string,
	err error) {

	// of the form: resourceName:resourceVersion or just resourceName
	list := strings.Split(resourceIdentifier, ":")

	// set the resource name
	resourceName = list[0]

	// only resource name provided
	if len(list) == 1 {
		return
	}

	// validate the resource version
	if err = fd.validateVersion(list[1]); err != nil {
		return
	}

	// set the resource version
	resourceVersion = &list[1]

	// if the resource is numeric
	if *resourceVersion != "latest" {
		resourceName = fmt.Sprintf("%s-%s", resourceName, *resourceVersion)
	}

	return
}

func (fd *FunctionDeleter) validateVersion(resourceVersion string) error {

	// can be either "latest" or numeric
	if resourceVersion != "latest" {
		_, err := strconv.Atoi(resourceVersion)
		if err != nil {
			return errors.Wrap(err, `Version must be either "latest" or numeric`)
		}
	}

	return nil
}
