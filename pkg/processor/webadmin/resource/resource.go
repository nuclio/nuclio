package resource

import (
	"github.com/nuclio/nuclio/cmd/processor/app"
	"github.com/nuclio/nuclio/pkg/processor/webadmin"
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

func (r *resource) getProcessor() *app.Processor {
	return r.GetServer().(*webadmin.Server).Processor.(*app.Processor)
}
