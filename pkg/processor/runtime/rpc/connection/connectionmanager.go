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
	runtime "github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/runtime/rpc/encoder"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

type ConnectionManager interface {
	Prepare() error

	Start() error

	Stop() error

	Allocate() (AbstractEventConnection, error)

	GetAddressesForWrapperStart() ([]string, string)

	UpdateStatistics(durationSec float64)

	SetStatus(status.Status)
}

// NewConnectionManager is a Factory function that returns a ConnectionManager based on the configuration
func NewConnectionManager(logger logger.Logger, runtimeConfiguration runtime.Configuration, configuration *ManagerConfigration) ConnectionManager {
	baseConnectionManager := NewBaseConnectionManager(logger.GetChild("connection manager"), runtimeConfiguration, configuration)
	// TODO: when support ConnectionPool, add option to return ConnectionPool object
	return NewSocketAllocator(baseConnectionManager)
}

type BaseConnectionManager struct {
	Logger logger.Logger

	MinSocketsNum int
	MaxSocketsNum int

	RuntimeConfiguration runtime.Configuration
	Configuration        *ManagerConfigration
}

func NewBaseConnectionManager(logger logger.Logger, runtimeConfiguration runtime.Configuration, configuration *ManagerConfigration) *BaseConnectionManager {
	// TODO: make MinSocketsNum and maxSocketsNum when support multiple sockets
	return &BaseConnectionManager{
		Logger:               logger,
		MinSocketsNum:        1,
		MaxSocketsNum:        1,
		RuntimeConfiguration: runtimeConfiguration,
		Configuration:        configuration,
	}
}

func (bc *BaseConnectionManager) Prepare() error {
	// Common or base implementation
	return nil
}

func (bc *BaseConnectionManager) Start() error {
	return nuclio.ErrNotImplemented
}

func (bc *BaseConnectionManager) Stop() error {
	return nuclio.ErrNotImplemented
}

func (bc *BaseConnectionManager) Allocate() (AbstractEventConnection, error) {
	return nil, nuclio.ErrNotImplemented
}

func (bc *BaseConnectionManager) GetAddressesForWrapperStart() ([]string, string) {
	return nil, ""
}

func (bc *BaseConnectionManager) UpdateStatistics(durationSec float64) {
	bc.Configuration.Statistics.DurationMilliSecondsCount++
	bc.Configuration.Statistics.DurationMilliSecondsSum += uint64(durationSec * 1000)
}

func (bc *BaseConnectionManager) SetStatus(newStatus status.Status) {
	//bc.abstractRuntime.SetStatus(newStatus)
}

type ManagerConfigration struct {
	SupportControlCommunication bool
	WaitForStart                bool
	SocketType                  SocketType
	GetEventEncoderFunc         func(writer io.Writer) encoder.EventEncoder
	Statistics                  runtime.Statistics
}
