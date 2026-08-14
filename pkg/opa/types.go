/*
Copyright 2023 The Nuclio Authors.

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

package opa

import (
	"encoding/json"

	"github.com/nuclio/nuclio/pkg/auth"

	"github.com/nuclio/opa-client"
)

const (
	DefaultRequestTimeout       = 10
	DefaultPermissionQueryPath  = "/v1/data/iguazio/authz/allow"
	DefaultPermissionFilterPath = "/v1/data/iguazio/authz/filter_allowed"

	// redactedSecretValue mirrors the "[redacted]" convention already used in
	// pkg/restful/middleware/middleware.go and pkg/cmdrunner/shellrunner.go.
	redactedSecretValue = "[redacted]"
)

type Config struct {
	*opaclient.Config
	AuthKind                auth.Kind              `json:"authKind,omitempty"`
	AuthorizationNamespaces AuthorizationNamespace `json:"authorizationNamespaces"`
}

// configAlias has Config's exact fields/tags but none of its methods, so marshaling it
// from within MarshalJSON below can't recurse.
type configAlias Config

// MarshalJSON redacts OverrideHeaderValue, the OPA-bypass shared secret, so it never
// reaches a log sink or other JSON output.
func (c Config) MarshalJSON() ([]byte, error) {
	if c.Config != nil && c.OverrideHeaderValue != "" {
		clonedClientConfig := *c.Config
		clonedClientConfig.OverrideHeaderValue = redactedSecretValue
		c.Config = &clonedClientConfig
	}
	return json.Marshal(configAlias(c))
}

// String redacts the same way as MarshalJSON, so fmt's %v/%+v/%s - which format via
// reflection and don't call json.Marshaler - can't be used to bypass the redaction.
func (c Config) String() string {
	encoded, err := c.MarshalJSON()
	if err != nil {
		return redactedSecretValue
	}
	return string(encoded)
}

type AuthorizationNamespace struct {
	Resources  string `json:"resources,omitempty"`
	Management string `json:"management,omitempty"`
}
