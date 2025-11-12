/*
Copyright 2025 The Nuclio Authors.

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

package annotations

import (
	"github.com/nuclio/nuclio/pkg/common/headers"
)

// nginx annotations
const (
	NginxAuthResponseHeaders    = "nginx.ingress.kubernetes.io/auth-response-headers"
	NginxAuthSignIn             = "nginx.ingress.kubernetes.io/auth-signin"
	NginxAuthURL                = "nginx.ingress.kubernetes.io/auth-url"
	NginxProxyBodySize          = "nginx.ingress.kubernetes.io/proxy-body-size"
	NginxProxyBufferSize        = "nginx.ingress.kubernetes.io/proxy-buffer-size"
	NginxServiceUpstream        = "nginx.ingress.kubernetes.io/service-upstream"
	NginxSSLRedirect            = "nginx.ingress.kubernetes.io/ssl-redirect"
	NginxDefaultProxyBodySize   = "0"
	NginxDefaultProxyBufferSize = "16k"
	NginxDefaultServiceUpstream = "true"
	NginxDefaultSSLRedirect     = "true"
)

// GetIguazioAuthenticationModeAnnotations returns a map of nginx ingress annotations for iguazio authentication mode
func GetIguazioAuthenticationModeAnnotations() map[string]string {
	return map[string]string{
		NginxAuthResponseHeaders: headers.AuthorizationHeader,
		NginxProxyBodySize:       NginxDefaultProxyBodySize,
		NginxProxyBufferSize:     NginxDefaultProxyBufferSize,
		NginxServiceUpstream:     NginxDefaultServiceUpstream,
		NginxSSLRedirect:         NginxDefaultSSLRedirect,
		NginxAuthURL:             "", // should be set during runtime
		NginxAuthSignIn:          "", // should be set during runtime
	}
}
