package resource

import (
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/playground"
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

func (r *resource) getPlatform() platform.Platform {
	return r.GetServer().(*playground.Server).Platform
}
