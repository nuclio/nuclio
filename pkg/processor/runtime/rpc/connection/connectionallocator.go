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

package connection

import (
	"fmt"
	"github.com/nuclio/errors"
	"net"
)

type ConnectionAllocator struct {
	*AbstractConnectionManager

	serverAddress string

	// should be a buffered chan when support multiple
	eventConnections     []*Connection
	controlMessageSocket *ControlMessageSocket
}

func NewConnectionAllocator(abstractConnectionManager *AbstractConnectionManager) *ConnectionAllocator {
	return &ConnectionAllocator{
		AbstractConnectionManager: abstractConnectionManager,
		serverAddress: fmt.Sprintf("%s:%d",
			abstractConnectionManager.Configuration.host,
			abstractConnectionManager.Configuration.port),
		eventConnections: make([]*Connection, 0),
	}
}

func (ca *ConnectionAllocator) Prepare() error {
	if ca.MinConnectionsNum != 0 {
		for i := 0; i < ca.MinConnectionsNum; i++ {
			conn, err := net.Dial("tcp", ca.serverAddress)
			if err != nil {
				return errors.Wrap(err, "Failed to establish connection")
			}
			ca.eventConnections = append(ca.eventConnections, NewConnection(ca.Logger, conn, ca))
		}
	}
	return nil
}

func (ca *ConnectionAllocator) Start() error {
	if err := ca.startSockets(); err != nil {
		return errors.Wrap(err, "Failed to start socket allocator")
	}

	// wait for start if required to
	if sa.Configuration.WaitForStart {
		sa.Logger.Debug("Waiting for start")
		for _, socket := range sa.eventSockets {
			socket.WaitForStart()
		}
	}

	ca.Logger.Debug("Connection allocator started")
	return nil

}

func (ca *ConnectionAllocator) Stop() error {
	for _, eventConnection := range ca.eventConnections {
		connection := eventConnection
		go func() {
			if err := connection.Conn.Close(); err != nil {
				ca.Logger.WarnWith("Failed to close connection",
					"error", err)
			}
		}()
	}
	if ca.controlMessageSocket != nil {
		go func() {
			ca.controlMessageSocket.Stop()
		}()
	}
	return nil
}

func (ca *ConnectionAllocator) Allocate() (EventConnection, error) {
	// TODO: support multiple connections
	return ca.eventConnections[0], nil
}

func (ca *ConnectionAllocator) GetAddressesForWrapperStart() ([]string, string) {
	controlAddress := ""
	if ca.controlMessageSocket != nil {
		controlAddress = ca.controlMessageSocket.Address
	}
	return []string{ca.serverAddress}, controlAddress
}
