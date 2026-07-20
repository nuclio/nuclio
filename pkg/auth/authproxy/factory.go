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
	authpkg "github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/common"
	kubeclient "github.com/nuclio/nuclio/pkg/platform/kube/clients/kube"
	nuclioioclient "github.com/nuclio/nuclio/pkg/platform/kube/clients/nuclio/clientset/versioned"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

// NewAuthenticator creates the topology-appropriate Authenticator. For reverseProxy mode,
// staticAuthConfig is used and the kube parameters are ignored. For authOnly mode,
// nuclioClientSet, kubeClientSet, and namespace are required.
func NewAuthenticator(
	parentLogger logger.Logger,
	mode authpkg.ProxyMode,
	authURL string,
	signinURL string,
	authKind authpkg.Kind,
	staticAuthConfig FunctionAuthConfig,
	kubeconfigPath string,
	namespace string,
) (Authenticator, error) {
	switch mode {
	case authpkg.ProxyModeReverseProxy:
		return NewReverseProxyAuthenticator(parentLogger, authURL, signinURL, authKind, staticAuthConfig)
	case authpkg.ProxyModeAuthOnly:
		clientConfig, err := common.GetClientConfig(kubeconfigPath)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get client configuration")
		}
		nuclioClientSet, err := nuclioioclient.NewForConfig(clientConfig)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create nuclio client set")
		}
		kubeClientSet, err := kubeclient.NewClientWithRetryFromConfig(clientConfig)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create kube client set")
		}
		return NewAuthOnlyAuthenticator(parentLogger, authURL, signinURL, authKind, nuclioClientSet, kubeClientSet, namespace)
	default:
		return nil, errors.Errorf("Unknown auth-proxy mode: %s", mode)
	}
}
