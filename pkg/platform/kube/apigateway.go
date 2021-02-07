package kube

import (
	"fmt"

	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/nuclio-sdk-go"
)

func ValidateAPIGatewaySpec(apiGatewaySpec *platform.APIGatewaySpec) error {
	upstreams := apiGatewaySpec.Upstreams

	switch upstreamsLength := len(upstreams); {
	case upstreamsLength == 0:
		return nuclio.NewErrBadRequest("One or more upstreams must be provided in spec")
	case upstreamsLength > 2:
		return nuclio.NewErrBadRequest("Received more than 2 upstreams. Currently not supported")
	}

	if apiGatewaySpec.Host == "" {
		return nuclio.NewErrBadRequest("Host must be provided in spec")
	}

	// TODO: update this when adding more upstream kinds. for now allow only `nucliofunction` upstreams
	kind := upstreams[0].Kind
	if !isSupportedAPIGatewayUpstreamKind(kind) {
		return nuclio.NewErrBadRequest(fmt.Sprintf("Unsupported upstream kind: '%s'. (Currently supporting only nucliofunction)", kind))
	}

	// make sure all upstreams have the same kind
	for _, upstream := range upstreams {
		if upstream.Kind != kind {
			return nuclio.NewErrBadRequest("All upstreams must be of the same kind")
		}
	}

	return nil
}

func getAPIGatewayUpstreamKinds() []platform.APIGatewayUpstreamKind {
	return []platform.APIGatewayUpstreamKind{
		platform.APIGatewayUpstreamKindNuclioFunction,
	}
}

func isSupportedAPIGatewayUpstreamKind(upstreamKind platform.APIGatewayUpstreamKind) bool {
	for _, supportedUpstreamKind := range getAPIGatewayUpstreamKinds() {
		if upstreamKind == supportedUpstreamKind {
			return true
		}
	}

	return false
}
