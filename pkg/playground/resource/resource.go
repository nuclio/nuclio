package resource

import (
	"github.com/nuclio/nuclio/pkg/restful"
)

type resource struct {
	*restful.AbstractResource
}

func newResource(name string, resourceMethods []restful.ResourceMethod) *resource {
	return &resource{
		AbstractResource: restful.NewAbstractResource(name, resourceMethods),
	}
}
