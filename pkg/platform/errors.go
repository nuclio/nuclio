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

package platform

import (
	"github.com/nuclio/nuclio-sdk-go"
)

// projects
var ErrSuccessfulCreateProjectLeader = nuclio.NewErrAccepted("Successfully requested from the leader to create the project")
var ErrSuccessfulUpdateProjectLeader = nuclio.NewErrAccepted("Successfully requested from the leader to update the project")
var ErrSuccessfulDeleteProjectLeader = nuclio.NewErrAccepted("Successfully requested from the leader to delete the project")

// A project containing resources(functions/api gateways), cannot be deleted
var ErrProjectContainsFunctions = nuclio.NewErrPreconditionFailed("Project contains functions")
var ErrProjectContainsAPIGateways = nuclio.NewErrPreconditionFailed("Project contains api gateways")

var ErrFunctionIsUsedByAPIGateways = nuclio.NewErrPreconditionFailed("Function is used by api gateways")

var ErrIngressHostPathInUse = nuclio.NewErrPreconditionFailed("Ingress host and path are already in use")

var ErrUnsupportedMethod = nuclio.NewErrNotImplemented("Unsupported method")
