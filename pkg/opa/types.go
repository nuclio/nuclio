/*
Copyright 2017 The Nuclio Authors.

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

type ClientKind string

const (
	ClientKindHTTP ClientKind = "http"
	ClientKindNop  ClientKind = "nop"
	ClientKindMock ClientKind = "mock"

	DefaultClientKind          = ClientKindNop
	DefaultRequestTimeOut      = 10
	DefaultPermissionQueryPath = "/v1/data/iguazio/authz/allow"
)

type Config struct {

	// OPA server address
	Address string `json:"address,omitempty"`

	// client kind to use (nop | http | mock)
	ClientKind ClientKind `json:"clientKind,omitempty"`

	// timeout period when querying opa server
	RequestTimeout int `json:"requestTimeout,omitempty"`

	// the path used when querying opa server (e.g.: /v1/data/somewhere/authz/allow)
	PermissionQueryPath string `json:"permissionQueryPath,omitempty"`

	// for extra verbosity on top of nuclio logger
	LogLevel int `json:"logLevel,omitempty"`

	// the header value for bypassing OPA if needed
	OverrideHeaderValue string `json:"overrideHeaderValue,omitempty"`
}

type PermissionOptions struct {
	MemberIds           []string
	RaiseForbidden      bool
	OverrideHeaderValue string
}

type PermissionRequestInput struct {
	Resource string   `json:"resource,omitempty"`
	Action   string   `json:"action,omitempty"`
	Ids      []string `json:"ids,omitempty"`
}

type PermissionRequest struct {
	Input PermissionRequestInput `json:"input,omitempty"`
}

type PermissionResponse struct {
	Result bool `json:"result,omitempty"`
}

const (
	UserIDHeader       string = "x-user-id"
	UserGroupIdsHeader string = "x-user-group-ids"
	OverrideHeader     string = "x-projects-role"
)

type Action string

const (
	ActionRead   Action = "read"
	ActionCreate Action = "create"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
)
