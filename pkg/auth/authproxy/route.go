/*
Copyright 2026 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package authproxy

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/nuclio/errors"
)

// Route describes one auth-proxy listener: the port it binds inside the pod and the pod-local upstream it
// forwards to.
type Route struct {
	ListenPort  int
	UpstreamURL string // empty in authOnly mode, which does no forwarding
}

// LoopbackRoute returns a route forwarding listenPort to a pod-local upstream port.
func LoopbackRoute(listenPort int, upstreamPort int) Route {
	return Route{
		ListenPort:  listenPort,
		UpstreamURL: fmt.Sprintf("http://127.0.0.1:%d", upstreamPort),
	}
}

// FormatRoutes renders routes into the --routes argument value.
func FormatRoutes(routes []Route) string {
	formattedRoutes := make([]string, len(routes))
	for routeIndex, route := range routes {
		if route.UpstreamURL == "" {
			formattedRoutes[routeIndex] = strconv.Itoa(route.ListenPort)
			continue
		}
		formattedRoutes[routeIndex] = fmt.Sprintf("%d=%s", route.ListenPort, route.UpstreamURL)
	}
	return strings.Join(formattedRoutes, ",")
}

// ParseRoutes parses the --routes argument value: a comma-separated list of "listenPort=upstreamURL" pairs.
// A bare "listenPort" yields an empty upstream, which is what authOnly mode expects since it does no
// forwarding.
func ParseRoutes(routes string) ([]Route, error) {
	routeSpecs := strings.Split(routes, ",")
	var parsedRoutes []Route
	for _, routeSpec := range routeSpecs {
		routeSpec = strings.TrimSpace(routeSpec)
		if routeSpec == "" {
			continue
		}

		listenPortSpec, upstreamURL, hasUpstream := strings.Cut(routeSpec, "=")
		listenPort, err := strconv.Atoi(strings.TrimSpace(listenPortSpec))
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to parse listen port of route: %s", routeSpec)
		}

		route := Route{ListenPort: listenPort}
		if hasUpstream {
			route.UpstreamURL = strings.TrimSpace(upstreamURL)
		}
		parsedRoutes = append(parsedRoutes, route)
	}

	return parsedRoutes, nil
}
