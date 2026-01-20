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

package serviceaccounttoken

import "github.com/nuclio/nuclio/pkg/common/headers"

const (
	// DefaultTokenPath is the default path to the service account token file
	DefaultTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	// DefaultTokenExpirationSeconds is the default expiration time for the service account token in seconds
	DefaultTokenExpirationSeconds = 600
	// DefaultTokenRefreshRatio is the default ratio to refresh the token before it expires
	DefaultTokenRefreshRatio = 0.75
)

var ServiceAccountAuthenticationHeader = map[string]string{
	headers.IguazioAuthenticatorKind: "sa",
}

type ServiceAccountTokenClient interface {
	GetSAToken() (string, error)
	AuthHeaders() (map[string]string, error)
	EscalateAuthHeaders(headers map[string]string) error
}
