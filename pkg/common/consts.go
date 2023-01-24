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

package common

type ReusedMessage string

const (
	UnexpectedTerminationChildProcess ReusedMessage = "Unexpected termination of child process"
	WorkDirectoryDoesNotExist         ReusedMessage = "Work directory does not exist"
	WorkDirectoryExpectedBeString     ReusedMessage = "Work directory is expected to be string"
	FailedReadFromEventConnection     ReusedMessage = "Failed to read from event connection"
	FailedReadControlMessage          ReusedMessage = "Failed to read control message"
)

type FunctionStateMessage string

const (
	FunctionStateMessageUnhealthy FunctionStateMessage = "Function is not healthy"
)

type NuclioResourceLabelKey string

const NuclioResourceLabelKeyProjectName = "nuclio.io/project-name"
const NuclioResourceLabelKeyFunctionName = "nuclio.io/function-name"
const NuclioResourceLabelKeyApiGatewayName = "nuclio.io/apigateway-name"
const NuclioResourceLabelKeyVolumeName = "nuclio.io/volume-name"

// KubernetesDomainLevelMaxLength DNS domain level limitation is 63 chars
// https://en.wikipedia.org/wiki/Subdomain#Overview
const KubernetesDomainLevelMaxLength = 63

const (
	AutoPlatformName  = "auto"
	KubePlatformName  = "kube"
	LocalPlatformName = "local"
)
