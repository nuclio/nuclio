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
	"time"

	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/eventprocessor"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/runtime/rpc/encoder"
	"github.com/nuclio/nuclio/pkg/processor/runtime/rpc/result"
	"github.com/nuclio/nuclio/pkg/processor/statistics"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type ConnectionManager interface {

	// Prepare initializes resources or configurations necessary for the ConnectionManager
	Prepare() error

	// Start begins the operations required for the ConnectionManager to accept and manage connections
	Start(pid int) error

	// Stop halts the operations of the ConnectionManager
	Stop() error

	// Allocate provides an instance of EventConnection for handling event
	Allocate(duration time.Duration) (eventprocessor.EventProcessor, error)

	// Release releases an instance of EventConnection
	Release(eventprocessor.EventProcessor)

	// GetAddressesForWrapperStart returns a list of addresses as required for starting a wrapper
	GetAddressesForWrapperStart() ([]string, string)

	// UpdateStatistics records performance or usage statistics based on the
	// duration of an event or process, specified in seconds
	UpdateStatistics(durationSec float64)

	// GetAllocationStatistics retrieves the current statistics of the ConnectionManager allocator
	GetAllocationStatistics() *statistics.AllocatorStatistics

	// SetStatus updates the operational status of the ConnectionManager
	SetStatus(status.Status)

	// GetStatus returns the operational status of the ConnectionManager
	GetStatus() status.Status

	// GetConfig returns the configuration of the ConnectionManager
	GetConfig() ManagerConfigration

	// IsAsync returns true if the ConnectionManager is in async mode
	IsAsync() bool

	// IsBusy return true if any of the connections in manager is in use
	IsBusy() bool
}

type EventConnection interface {
	// WaitForStart waits for connection and handler to be ready for event processing
	WaitForStart()

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
	Statistics                  *runtime.Statistics

	// EstablishConnectionTimeout is the total budget for connection establishment and wrapper readiness
	// signalling in async mode. It is set by EnrichAndValidate to 3× ReadinessTimeoutSeconds.
	EstablishConnectionTimeout time.Duration

	eventTimeout time.Duration
	chunkTimeout time.Duration

	host     string
	port     int
	workerId int
}

func NewManagerConfigration(
	supportControlCommunication bool,
	waitForStart bool,
	socketType SocketType,
	getEventEncoderFunc func(writer io.Writer) encoder.EventEncoder,
	statistics *runtime.Statistics,
	workerId int,
	mode functionconfig.TriggerWorkMode,
	eventTimeout time.Duration,
	chunkTimeout time.Duration) *ManagerConfigration {
	manager := &ManagerConfigration{
		SupportControlCommunication: supportControlCommunication,
		WaitForStart:                waitForStart,
		SocketType:                  socketType,
		GetEventEncoderFunc:         getEventEncoderFunc,
		Statistics:                  statistics,
		workerId:                    workerId,
		eventTimeout:                eventTimeout,
		chunkTimeout:                chunkTimeout,
	}
	switch mode {
	case functionconfig.AsyncTriggerWorkMode:
		manager.Kind = ConnectionAllocatorManagerKind
	default:
		manager.Kind = SocketAllocatorManagerKind
	}
	if manager.Kind == ConnectionAllocatorManagerKind {
		manager.host = "127.0.0.1"
		manager.port = portRangeBeginning + workerId
	}
	return manager
}

// EnrichAndValidate resolves EstablishConnectionTimeout for the connection manager.
// Primary path: parses AsyncConfig.EstablishConnectionTimeout populated by the platform's
// EnrichFunctionConfig flow. Fallback (for configs that bypassed deploy-time enrichment, e.g.
// older configs or unit tests): applies the same DefaultEstablishConnectionTimeoutMultiplier ×
// ReadinessTimeoutSeconds policy here. Must be called once after NewManagerConfigration and
// before the manager is used.
func (mc *ManagerConfigration) EnrichAndValidate(runtimeConfiguration runtime.Configuration) error {
	if runtimeConfiguration.AsyncConfig == nil {
		return nil
	}
	duration, err := runtimeConfiguration.AsyncConfig.GetEstablishConnectionTimeoutDuration()
	if err != nil {
		return errors.Wrap(err, "Failed to parse async config establish connection timeout")
	}
	if duration == 0 {
		readinessTimeout := time.Duration(runtimeConfiguration.Spec.ReadinessTimeoutSeconds) * time.Second
		if readinessTimeout <= 0 && runtimeConfiguration.PlatformConfig != nil {
			readinessTimeout = runtimeConfiguration.PlatformConfig.GetDefaultFunctionReadinessTimeout()
		}
		duration = functionconfig.DefaultEstablishConnectionTimeoutMultiplier * readinessTimeout
	}
	mc.EstablishConnectionTimeout = duration
	return nil
}

type ManagerKind string

const SocketAllocatorManagerKind ManagerKind = "socketAllocator"
const ConnectionAllocatorManagerKind ManagerKind = "connectionAllocator"

const portRangeBeginning = 1337
