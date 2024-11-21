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
	"io"

	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/runtime/rpc/encoder"
	"github.com/nuclio/nuclio/pkg/processor/runtime/rpc/result"

	"github.com/nuclio/logger"
)

type ConnectionManager interface {

	// Prepare initializes resources or configurations necessary for the ConnectionManager
	Prepare() error

	// Start begins the operations required for the ConnectionManager to accept and manage connections
	Start() error

	// Stop halts the operations of the ConnectionManager
	Stop() error

	// Allocate provides an instance of EventConnection for handling event
	Allocate() (EventConnection, error)

	// GetAddressesForWrapperStart returns a list of addresses as required for starting a wrapper
	GetAddressesForWrapperStart() ([]string, string)

	// UpdateStatistics records performance or usage statistics based on the
	// duration of an event or process, specified in seconds
	UpdateStatistics(durationSec float64)

	// SetStatus updates the operational status of the ConnectionManager
	SetStatus(status.Status)
}

type EventConnection interface {
	// Start initializes and starts the event connection, preparing it for processing events
	Start()

	// Stop stops the event connection and performs any necessary cleanup tasks
	Stop()

	// ProcessEvent processes a single event item, using the provided functionLogger for any logging
	ProcessEvent(item interface{}, functionLogger logger.Logger) (*result.BatchedResults, error)

	// RunHandler starts the main event handler loop, managing incoming responses until the connection is stopped
	RunHandler()
}

type ManagerConfigration struct {
	Kind                        ManagerKind
	SupportControlCommunication bool
	WaitForStart                bool
	SocketType                  SocketType
	GetEventEncoderFunc         func(writer io.Writer) encoder.EventEncoder
	Statistics                  runtime.Statistics
}

type ManagerKind string

const SocketAllocatorManagerKind ManagerKind = "socketAllocator"
