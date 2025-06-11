/*
Copyright 2025 The Nuclio Authors.

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

package eventprocessor

import (
	"time"

	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/processor/cloudevent"
	"github.com/nuclio/nuclio/pkg/processor/controlcommunication"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/statistics"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

var ErrNoAvailableObjects = errors.New("No available objects")
var ErrAllObjectsAreTerminated = errors.New("All objects are terminated")

type Allocator interface {
	// Allocate allocates event processor instance
	Allocate(timeout time.Duration) (EventProcessor, error)

	// Release releases event processor instance
	Release(processor EventProcessor, processedSuccessfully bool)

	// GetObjects gets direct access to all event processors for things like management / housekeeping
	GetObjects() []EventProcessor

	// SetObjects allows to set or replace event processors for the allocator
	SetObjects([]EventProcessor) error

	// GetNumObjectsAvailable gets number of event processors available in the allocator
	GetNumObjectsAvailable() int

	// GetStatistics returns allocator statistics
	GetStatistics() *statistics.AllocatorStatistics

	// SignalDraining signals all event processors to drain events
	SignalDraining() error

	// SignalContinue signals all event processors to continue event processing
	SignalContinue() error

	// SignalTermination signals all event processors to terminate
	SignalTermination() error

	// Stop stops all event processors and allocator
	Stop() error

	// IsTerminated returns true if all event processors are terminated
	IsTerminated() bool
}

type EventProcessor interface {
	// ProcessEvent processes a single event
	ProcessEvent(event nuclio.Event, functionLogger logger.Logger) (nuclio.ProcessingResult, error)

	// ProcessEventBatch processes batch of events
	ProcessEventBatch(batch []nuclio.Event, functionLogger logger.Logger) ([]*runtime.ResponseWithErrors, error)

	// Terminate terminates event processor
	Terminate() error

	// Drain drains event processor
	Drain() error

	// Continue signals event processor to continue processing
	Continue() error

	// GetIndex returns unique index of this event processor
	GetIndex() int

	// GetRuntime returns runtime of this event processor
	GetRuntime() runtime.Runtime

	// GetStatus returns event processor status
	GetStatus() status.Status

	// SetStatus sets event processor status
	SetStatus(status.Status)

	// Stop stops event processor
	Stop() error

	// GetStatistics returns event processing statistics, such as counts and latencies
	GetStatistics() *statistics.EventProcessingStatistics

	// GetAllocationStatistics returns allocation statistics, such as allocation time and number of allocations
	GetAllocationStatistics() *statistics.AllocatorStatistics

	// GetStructuredCloudEvent retrieves the last processed structured CloudEvent
	GetStructuredCloudEvent() *cloudevent.Structured

	// GetBinaryCloudEvent retrieves the last processed binary CloudEvent
	GetBinaryCloudEvent() *cloudevent.Binary

	// RestartRequired returns whether the event processor requires a restart
	RestartRequired() bool

	// Restart restarts the event processor, if supported
	Restart() error

	// SupportsRestart checks if the event processor supports restarting
	SupportsRestart() bool

	// Subscribe registers a channel to receive control messages of a specific kind
	Subscribe(kind controlcommunication.ControlMessageKind, channel chan *controlcommunication.ControlMessage) error

	// Unsubscribe removes a previously registered channel for control messages
	Unsubscribe(kind controlcommunication.ControlMessageKind, channel chan *controlcommunication.ControlMessage) error

	// WaitForStart blocks until the event processor has fully started
	WaitForStart(timeout time.Duration) (err error)

	// RunHandler starts the event processing loop, handling incoming events
	RunHandler()

	// IsAsync checks if the event processor is asynchronous
	IsAsync() bool

	// IsBusy checks if the event processor is busy processing events
	IsBusy() bool

	// IncrementEventProcessingMetric increments an event processing metric
	IncrementEventProcessingMetric(success bool)
}
