package kube

import (
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/errors"
)

func ValidateUpstreamSpec(apiGatewaySpec *platform.APIGatewaySpec) error {
	upstreams := apiGatewaySpec.Upstreams

	if len(upstreams) > 2 {
		return errors.New("Received more than 2 upstreams. Currently not supported")
	} else if len(upstreams) == 0 {
		return errors.New("One or more upstreams must be provided in spec")
	} else if apiGatewaySpec.Host == "" {
		return errors.New("Host must be provided in spec")
	}

	// TODO: update this when adding more upstream kinds. for now allow only `nucliofunction` upstreams
	kind := upstreams[0].Kind
	if kind != platform.APIGatewayUpstreamKindNuclioFunction {
		return errors.Errorf("Unsupported upstream kind: %s. (Currently supporting only nucliofunction)", kind)
	}

	if apiGatewaySpec.Name == "" {
		return errors.New("Api gateway name must be provided in spec")
	}

	// make sure all upstreams have the same kind
	for _, upstream := range upstreams {
		if upstream.Kind != kind {
			return errors.New("All upstreams must be of the same kind")
		}
	}

	return nil
}
