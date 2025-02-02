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
	"net"
	"time"

	"github.com/nuclio/errors"
)

type ConnectionAllocator struct {
	*AbstractConnectionManager

	serverAddress string

	// should be a buffered chan when support multiple
	eventConnections []*Connection
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
	if err := ca.prepareControlMessageSocket(); err != nil {
		return errors.Wrap(err, "Failed to prepare control message socket")
	}
	return nil
}

func (ca *ConnectionAllocator) Start() error {
	if ca.MinConnectionsNum != 0 {
		for i := 0; i < ca.MinConnectionsNum; i++ {
			var conn net.Conn
			var err error

			// Use retryable dial for the first connection
			if i == 0 {
				conn, err = retryableDial(ca.serverAddress, 30, 1*time.Second)
			} else {
				conn, err = net.Dial("tcp", ca.serverAddress)
			}

			if err != nil {
				return errors.Wrap(err, "Failed to establish connection")
			}
			ca.eventConnections = append(ca.eventConnections, NewConnection(ca.Logger, conn, ca))
		}
	}
	// start event processing
	for _, eventConnection := range ca.eventConnections {
		eventConnection.SetEncoder(ca.Configuration.GetEventEncoderFunc(eventConnection.Conn))
		go eventConnection.AbstractEventConnection.RunHandler()
	}

	if err := ca.startControlMessageSocket(); err != nil {
		return errors.Wrap(err, "Failed to start control message socket")
	}

	// wait for start if required to
	if ca.Configuration.WaitForStart {
		ca.Logger.Debug("Waiting for start")
		for _, eventConnection := range ca.eventConnections {
			eventConnection.WaitForStart()
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
	ca.stopControlMessageSocket()
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

func retryableDial(address string, maxRetries int, retryInterval time.Duration) (net.Conn, error) {
	var conn net.Conn
	var err error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		conn, err = net.Dial("tcp", address)
		if err == nil {
			return conn, nil
		}

		if attempt < maxRetries {
			time.Sleep(retryInterval)
		}
	}

	return nil, errors.Wrap(err, "Failed to establish connection after retries")
}
