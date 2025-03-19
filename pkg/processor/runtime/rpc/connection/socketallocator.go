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

func (sa *SocketAllocator) Start() error {
	eventSockets := sa.allocator.GetObjects()
	if err := sa.startSockets(eventSockets); err != nil {
		return errors.Wrap(err, "Failed to start socket allocator")
	}

	// wait for start if required to
	if sa.Configuration.WaitForStart {
		sa.Logger.Debug("Waiting for start")
		for _, socket := range eventSockets {
			socket.WaitForStart()
		}
	}

	sa.Logger.Debug("Socker allocator started")
	return nil
}

func (sa *SocketAllocator) Stop() error {
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
	return nil
}

func (sa *SocketAllocator) Allocate(duration time.Duration) (eventprocessor.EventProcessor, error) {
	return sa.allocator.Allocate(duration)
}

// Release releases an instance of EventConnection
func (sa *SocketAllocator) Release(processor eventprocessor.EventProcessor) {
	sa.allocator.Release(processor)
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
