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

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

var ErrNoAvailableWorkers = errors.New("No available workers")
var ErrAllWorkersAreTerminated = errors.New("All workers are terminated")

type Allocator interface {
	Allocate(timeout time.Duration) (EventProcessor, error)

	Release(processor EventProcessor)
	// Shareable returns true if the several go routines can share this allocator
	Shareable() bool

	// GetObjects gets direct access to all workers for things like management / housekeeping
	GetObjects() []EventProcessor

	SetObjects([]EventProcessor) error

	// GetNumWorkersAvailable gets number of workers available in the allocator
	GetNumWorkersAvailable() int

	// GetStatistics returns worker allocator statistics
	GetStatistics() *AllocatorStatistics

	// SignalDraining signals all workers to drain events
	SignalDraining() error

	// SignalContinue signals all workers to continue event processing
	SignalContinue() error

	// SignalTermination signals all workers to terminate
	SignalTermination() error

	// IsTerminated returns true if all workers are terminated
	IsTerminated() bool
}

type EventProcessor interface {
	ProcessEvent(event nuclio.Event, functionLogger logger.Logger) (interface{}, error)

	ProcessEventBatch(batch []nuclio.Event, functionLogger logger.Logger) ([]*runtime.ResponseWithErrors, error)

	Terminate() error

	Drain() error

	Continue() error

	GetIndex() int

	GetRuntime() runtime.Runtime

	GetStatus() status.Status

	Stop() error

	GetStatistics() *Statistics

	GetStructuredCloudEvent() *cloudevent.Structured

	GetBinaryCloudEvent() *cloudevent.Binary

	GetEventTime() *time.Time

	ResetEventTime()

	Restart() error

	SupportsRestart() bool

	Subscribe(kind controlcommunication.ControlMessageKind, channel chan *controlcommunication.ControlMessage) error

	Unsubscribe(kind controlcommunication.ControlMessageKind, channel chan *controlcommunication.ControlMessage) error

	WaitForStart()

	RunHandler()
}
