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

package functioncr

import (
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type FunctionState string

const (
	FunctionStateCreated   FunctionState = ""
	FunctionStateProcessed FunctionState = "processed"
	FunctionStateError     FunctionState = "error"
	FunctionStateDisabled  FunctionState = "disabled"
	FunctionStateTerminate FunctionState = "terminate"
)

type BuildState string

const (
	BuildStateUnknown BuildState = ""
	BuildStatePending BuildState = "pending"
	BuildStateError   BuildState = "error"
	BuildStateBuild   BuildState = "build"
	BuildStateReady   BuildState = "ready"
)

type FunctionStatus struct {
	State       FunctionState `json:"state,omitempty"`
	Message     string        `json:"message,omitempty"`
	BuildState  BuildState    `json:"build,omitempty"`
	ObservedGen string        `json:"observedVer,omitempty"`
}

type FunctionList struct {
	meta_v1.TypeMeta `json:",inline"`
	meta_v1.ListMeta `json:"metadata"`
	Items            []Function `json:"items"`
}

type FunctionCode struct {
	Path   string `json:"path,omitempty"`
	Key    string `json:"key,omitempty"`
	Code   string `json:"code,omitempty"`
	Watch  bool   `json:"watch,omitempty"`
	Inline string `json:"inline,omitempty"`
}
