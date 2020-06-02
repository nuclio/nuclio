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

package kube

import (
	"fmt"

	"github.com/nuclio/nuclio/pkg/platform/kube/ingress"
)

type DeployOptions struct {
}

func DeploymentNameFromFunctionName(functionName string) string {
	return fmt.Sprintf("nuclio-%s", functionName)
}

func PodNameFromFunctionName(functionName string) string {
	return fmt.Sprintf("nuclio-%s", functionName)
}

func ConfigMapNameFromFunctionName(functionName string) string {
	return fmt.Sprintf("nuclio-%s", functionName)
}

func HPANameFromFunctionName(functionName string) string {
	return fmt.Sprintf("nuclio-%s", functionName)
}

func IngressNameFromFunctionName(functionName string) string {
	return fmt.Sprintf("nuclio-%s", functionName)
}

func ServiceNameFromFunctionName(functionName string) string {
	return fmt.Sprintf("nuclio-%s", functionName)
}


//
// APIGateway
//

const DefaultAPIGatewayName string = "default"

type APIGatewayMeta struct {
	Name        string            `json:"name,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type APIGatewayAuthenticationMode string

const (
	APIGatewayAuthenticationModeNone      APIGatewayAuthenticationMode = "none"
	APIGatewayAuthenticationModeBasicAuth APIGatewayAuthenticationMode = "basicAuth"
	APIGatewayAuthenticationModeDex       APIGatewayAuthenticationMode = "dex"
	APIGatewayAuthenticationAccessKey     APIGatewayAuthenticationMode = "accessKey"
)

type APIGatewayAuthenticationSpec struct {
	BasicAuth *ingress.BasicAuth `json:"basic_auth,omitempty"`
	DexAuth   *ingress.DexAuth   `json:"dex_auth,omitempty"`
}

type APIGatewayUpstreamKind string

const (
	APIGatewayUpstreamKindNuclioFunction APIGatewayUpstreamKind = "nucliofunction"
)

type NuclioFunctionAPIGatewaySpec struct {
	Name string `json:"name,omitempty"`
}

type APIGatewayUpstreamSpec struct {
	Kind             APIGatewayUpstreamKind        `json:"kind,omitempty"`
	Nucliofunction   *NuclioFunctionAPIGatewaySpec `json:"nucliofunction,omitempty"`
	Percentage       int                           `json:"percentage,omitempty"`
	RewriteTarget    string                        `json:"rewrite_target,omitempty"`
	ExtraAnnotations map[string]string             `json:"extra_annotations,omitempty"`
}

type APIGatewaySpec struct {
	Host               string                        `json:"host,omitempty"`
	Name               string                        `json:"name,omitempty"`
	Description        string                        `json:"description,omitempty"`
	Path               string                        `json:"path,omitempty"`
	AuthenticationMode APIGatewayAuthenticationMode  `json:"authentication_mode,omitempty"`
	Authentication     *APIGatewayAuthenticationSpec `json:"authentication,omitempty"`
	Upstreams          []APIGatewayUpstreamSpec      `json:"upstreams,omitempty"`
}

type APIGatewayConfig struct {
	Meta   APIGatewayMeta
	Spec   APIGatewaySpec
	Status APIGatewayStatus
}

// APIGatewayState is state of api-gateway
type APIGatewayState string

// Possible api-gateway states
const (
	APIGatewayStateNone                   APIGatewayState = ""
	APIGatewayStateReady                  APIGatewayState = "ready"
	APIGatewayStateError                  APIGatewayState = "error"
	APIGatewayStateWaitingForProvisioning APIGatewayState = "waitingForProvisioning"
)

type APIGatewayStatus struct {
	Name        string          `json:"name,omitempty"`
	LastError   string          `json:"last_error,omitempty"`
	Description string          `json:"description,omitempty"`
	State       APIGatewayState `json:"state,omitempty"`
}

type CreateAPIGatewayOptions struct {
	APIGatewayConfig APIGatewayConfig
}

type UpdateAPIGatewayOptions struct {
	APIGatewayConfig APIGatewayConfig
}

type DeleteAPIGatewayOptions struct {
	Meta APIGatewayMeta
}

type GetAPIGatewaysOptions struct {
	Meta APIGatewayMeta
}

// to appease k8s
func (s *APIGatewaySpec) DeepCopyInto(out *APIGatewaySpec) {

	// TODO: proper deep copy
	*out = *s
}
