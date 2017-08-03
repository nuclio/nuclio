package getter

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/nuclio/nuclio-sdk"
	"github.com/pkg/errors"
)

type FunctionGetter struct {
	logger  nuclio.Logger
	options *Options
}

func NewFunctionGetter(parentLogger nuclio.Logger) *FunctionGetter {
	return &FunctionGetter{
		logger: parentLogger.GetChild("get").(nuclio.Logger),
	}
}

func (fg *FunctionGetter) Execute(options *Options) error {



	return nil
}

func (fg *FunctionGetter) parseResourceIdentifier(resourceIdentifier string) (resourceName string,
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
	if err = fg.validateVersion(list[1]); err != nil {
		return
	}

	// set the resource version
	*resourceVersion = list[1]

	// if the resource is numeric
	if *resourceVersion != "latest" {
		resourceName = fmt.Sprintf("%s-%s", resourceName, *resourceVersion)
	}

	return
}

func (fg *FunctionGetter) validateVersion(resourceVersion string) error {

	// can be either "latest" or numeric
	if resourceVersion != "latest" {
		_, err := strconv.Atoi(resourceVersion)
		if err != nil {
			return errors.Wrap(err, `Version must be either "latest" or numeric`)
		}
	}

	return nil
}
