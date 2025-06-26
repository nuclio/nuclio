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
	"time"

	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/processor/eventprocessor"

	"github.com/nuclio/errors"
)

const (
	socketPathTemplate = "/tmp/nuclio-rpc-%s.sock"
	connectionTimeout  = 2 * time.Minute
)

type SocketAllocator struct {
	*AbstractConnectionManager
}

func NewSocketAllocator(abstractConnectionManager *AbstractConnectionManager) *SocketAllocator {
	return &SocketAllocator{
		AbstractConnectionManager: abstractConnectionManager,
	}
}

// Prepare initializes the SocketAllocator by setting up control and event sockets
// according to the configuration.
//
// If SupportControlCommunication is enabled, a control communication socket is created,
// wrapped in a ControlMessageSocket, and integrated with the ControlMessageBroker for runtime operations.
//
// Creates a minimum number of event sockets (MinConnectionsNum).
func (sa *SocketAllocator) Prepare() error {
	sa.SetStatus(status.Initializing)
	if err := sa.prepareControlMessageSocket(); err != nil {
		return errors.Wrap(err, "Failed to prepare control message socket")
	}
	eventSockets := make([]*EventSocket, 0)
	for i := 0; i < sa.MinConnectionsNum; i++ {
		eventConnection, err := sa.createSocketConnection()
		if err != nil {
			return errors.Wrap(err, "Failed to create event socket connection")
		}
		eventSockets = append(eventSockets,
			NewEventSocket(sa.Logger, eventConnection, sa))
	}
	// set objects in allocator
	eventProcessors := make([]eventprocessor.EventProcessor, len(eventSockets))
	for i, eventConnection := range eventSockets {
		eventProcessors[i] = eventConnection
	}
	if err := sa.allocator.SetObjects(eventProcessors); err != nil {
		return errors.Wrap(err, "Failed to set objects in allocator")
	}
	return nil
}

func (sa *SocketAllocator) Start(pid int) error {
	sa.pid = pid
	eventSockets := sa.allocator.GetObjects()
	if err := sa.startSockets(eventSockets); err != nil {
		return errors.Wrap(err, "Failed to start socket allocator")
	}

	// wait for start if required to
	if sa.Configuration.WaitForStart {
		sa.Logger.Debug("Waiting for start")
		for _, socket := range eventSockets {
			if err := socket.WaitForStart(0); err != nil {
				return errors.Wrap(err, "Failed to wait for socket start")
			}
			socket.SetStatus(status.Ready)
		}
	}

	sa.SetStatus(status.Ready)
	sa.Logger.Debug("Socker allocator started")
	return nil
}

func (sa *SocketAllocator) Stop() error {
	sa.SetStatus(status.Stopping)
	eventSockets := sa.allocator.GetObjects()
	for _, eventSocket := range eventSockets {
		socket := eventSocket
		go func() {
			err := socket.Stop()
			if err != nil {
				sa.Logger.WarnWith("Failed to close socket",
					"error", err.Error())
			}
		}()
	}
	sa.stopControlMessageSocket()
	sa.SetStatus(status.Stopped)
	return nil
}

func (sa *SocketAllocator) Allocate(duration time.Duration) (eventprocessor.EventProcessor, error) {
	socket, err := sa.allocator.Allocate(duration)
	if err == nil && socket != nil {
		// required for the case when non-blocking singleton allocator is used
		// and the connection was set to restart during stream processing and writer was closed;
		// closing a writer unblocks processing from trigger side and releases the worker
		// so we might have a delay between setting the allocator for restart during release
		if socket.GetStatus() != status.Ready {
			sa.Logger.DebugWith("Connection not ready", "status", socket.GetStatus().String())
			if len(sa.allocator.GetObjects()) == 1 {

				// set status to restart required if we have only one connection
				sa.SetStatus(status.RestartRequired)
			}
			return nil, errors.Errorf("Connection not ready. Status: %s", socket.GetStatus().String())
		}
	}
	return socket, err
}

// Release releases an instance of EventConnection
func (sa *SocketAllocator) Release(connection eventprocessor.EventProcessor) {
	if connection.GetStatus() == status.RestartRequired {
		sa.SetStatus(status.RestartRequired)
	}
	sa.allocator.Release(connection)
}

func (sa *SocketAllocator) GetAddressesForWrapperStart() ([]string, string) {
	eventAddresses := make([]string, 0)
	eventSockets := sa.allocator.GetObjects()
	for _, socket := range eventSockets {
		eventAddresses = append(eventAddresses, socket.(*EventSocket).Address)
	}

	controlAddress := ""
	if sa.controlMessageSocket != nil {
		controlAddress = sa.controlMessageSocket.Address
	}
	sa.Logger.DebugWith("Got socket addresses",
		"eventAddresses", eventAddresses,
		"controlAddress", controlAddress)
	return eventAddresses, controlAddress
}

func (sa *SocketAllocator) startSockets(eventSockets []eventprocessor.EventProcessor) error {
	var err error
	for _, socket := range eventSockets {
		eventSocketInstance := socket.(*EventSocket)
		// TODO: when having multiple sockets supported, we might want to reconsider failing here
		if eventSocketInstance.Conn, err = eventSocketInstance.listener.Accept(); err != nil {
			return errors.Wrap(err, "Can't get connection from wrapper")
		}
		eventSocketInstance.SetEncoder(sa.Configuration.GetEventEncoderFunc(eventSocketInstance.Conn))
		go eventSocketInstance.AbstractEventConnection.RunHandler()
	}
	sa.Logger.Debug("Successfully established connection for event sockets")

	if err := sa.startControlMessageSocket(); err != nil {
		return errors.Wrap(err, "Failed to start control message socket")
	}
	return nil
}
