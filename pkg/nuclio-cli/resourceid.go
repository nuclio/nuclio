package nucliocli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

func ParseResourceIdentifier(resourceIdentifier string) (resourceName string,
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
	if err = validateVersion(list[1]); err != nil {
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

func validateVersion(resourceVersion string) error {

	// can be either "latest" or numeric
	if resourceVersion != "latest" {
		_, err := strconv.Atoi(resourceVersion)
		if err != nil {
			return errors.Wrap(err, `Version must be either "latest" or numeric`)
		}
	}

	return nil
}
