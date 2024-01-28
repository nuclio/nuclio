package utils

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/nuclio/nuclio/pkg/common/headers"
	"github.com/nuclio/nuclio/pkg/nexus/common/models"
)

// GetEnvironmentHost returns the host of the environment
//
// Currently for linux and mac os it is host.docker.internal
// We set in the docker-compose an external host to it for ensuring that the host.docker.internal will be resolved
// More info: https://docs.docker.com/docker-for-mac/networking/#use-cases-and-workarounds
// Docker compose: profaastinate/deployment/docker/docker-compose.yml
func GetEnvironmentHost() (host string) {
	return "host.docker.internal"
}

// TransformRequestToClientRequest transforms the async request send to the nexus from outside the cluster to a request
// that can be sent to the function inside the cluster
func TransformRequestToClientRequest(nexusItemRequest *http.Request) (newRequest *http.Request) {
	if nexusItemRequest.Header.Get(headers.ProcessDeadline) != "" {
		nexusItemRequest.Header.Del(headers.ProcessDeadline)
	}

	var requestUrl url.URL
	requestUrl.Scheme = "http"
	requestUrl.Path = nexusItemRequest.URL.Path
	// Needs to be set to the port of the env the default port is 8080
	requestUrl.Host = fmt.Sprintf("%s:%s", GetEnvironmentHost(), models.PORT)

	newRequest, _ = http.NewRequest(nexusItemRequest.Method, requestUrl.String(), nexusItemRequest.Body)
	newRequest.Header = nexusItemRequest.Header
	return
}
