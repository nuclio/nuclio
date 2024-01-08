package utils

import (
	"fmt"
	"github.com/nuclio/nuclio/pkg/common/headers"
	"github.com/nuclio/nuclio/pkg/nexus/common/models"
	"net/http"
	"net/url"
)

func GetEnvironmentHost() (host string) {
	return "host.docker.internal"
}

func TransformRequestToClientRequest(nexusItemRequest *http.Request) (newRequest *http.Request) {
	if nexusItemRequest.Header.Get(headers.ProcessDeadline) != "" {
		nexusItemRequest.Header.Del(headers.ProcessDeadline)
	}

	var requestUrl url.URL
	requestUrl.Scheme = nexusItemRequest.URL.Scheme
	requestUrl.Path = nexusItemRequest.URL.Path
	// Needs to be set to the port of the environment the default port is 8080
	requestUrl.Host = fmt.Sprintf("%s:%s", GetEnvironmentHost(), models.PORT)

	newRequest, _ = http.NewRequest(nexusItemRequest.Method, requestUrl.String(), nexusItemRequest.Body)
	newRequest.Header = nexusItemRequest.Header
	return
}
