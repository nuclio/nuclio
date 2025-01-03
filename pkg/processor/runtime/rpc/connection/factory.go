/*
Copyright 2024 The Nuclio Authors.

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

package connection

import (
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/nuclio/logger"
	nuclio "github.com/nuclio/nuclio-sdk-go"
)

// NewConnectionManager is a Factory function that returns a ConnectionManager based on the configuration
func NewConnectionManager(parentLogger logger.Logger, runtimeConfiguration runtime.Configuration, configuration *ManagerConfigration) (ConnectionManager, error) {
	abstractConnectionManager := NewAbstractConnectionManager(parentLogger, runtimeConfiguration, configuration)

	switch configuration.Kind {
	case SocketAllocatorManagerKind:
		return NewSocketAllocator(abstractConnectionManager), nil
	default:
		// TODO: when support ConnectionPool, add option to return ConnectionPool object
		return nil, nuclio.ErrNotImplemented
	}
}
