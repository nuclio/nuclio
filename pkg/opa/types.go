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
	"github.com/nuclio/nuclio/pkg/auth"

	"github.com/nuclio/opa-client"
)

const (
	DefaultRequestTimeout       = 10
	DefaultPermissionQueryPath  = "/v1/data/iguazio/authz/allow"
	DefaultPermissionFilterPath = "/v1/data/iguazio/authz/filter_allowed"

	IguazioV4ResourcePrefix   = "/resources"
	IguazioV4ManagementPrefix = "/mgmt"
)

type Config struct {
	*opaclient.Config
	AuthKind auth.Kind `json:"authKind,omitempty"`
}
