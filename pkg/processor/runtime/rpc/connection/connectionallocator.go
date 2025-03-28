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
	"context"
	"fmt"
	"net"
	"time"

	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/processor/eventprocessor"

	"github.com/nuclio/errors"
)

// ConnectionAllocator implements AbstractConnectionManager and is responsible for managing connections
// between the processor and a runtime wrapper.
//
// The connection allocation flow is as follows:
//   - Prepare(): Prepares everything needed before the runtime starts.
//   - After the runtime process has started, Start() should be called to establish all connections
//     between the processor and the runtime.
//   - Only after Start() has completed, Allocate() can be called.
//   - At the end of the flow, before stopping the runtime process, Stop() should be called
//     to close all connections.
type ConnectionAllocator struct {
	*AbstractConnectionManager

	serverAddress string
}

func NewConnectionAllocator(abstractConnectionManager *AbstractConnectionManager) *ConnectionAllocator {
	return &ConnectionAllocator{
		AbstractConnectionManager: abstractConnectionManager,
		serverAddress: fmt.Sprintf("%s:%d",
			abstractConnectionManager.Configuration.host,
			abstractConnectionManager.Configuration.port),
	}
}

func (ca *ConnectionAllocator) Prepare() error {
	ca.SetStatus(status.Initializing)

	if err := ca.prepareControlMessageSocket(); err != nil {
		return errors.Wrap(err, "Failed to prepare control message socket")
	}
	return nil
}

func (ca *ConnectionAllocator) Start(pid int) error {
	ca.pid = pid
	ca.Logger.DebugWith("Starting connection allocator",
		"wrapperPID", ca.pid)
	// starts control message socket
	if err := ca.startControlMessageSocket(); err != nil {
		return errors.Wrap(err, "Failed to start control message socket")
	}
	if err := ca.startEventConnections(); err != nil {
		return errors.Wrap(err, "Failed to start event connections")
	}

	ca.SetStatus(status.Ready)
	return nil
}

func (ca *ConnectionAllocator) Stop() error {
	ca.SetStatus(status.Stopping)

	err := ca.stopAllocator()
	ca.stopControlMessageSocket()
	ca.Logger.DebugWith("Stopped connection allocator",
		"wrapperPID", ca.pid)

	ca.SetStatus(status.Stopped)
	return err
}

func (ca *ConnectionAllocator) Allocate(duration time.Duration) (eventprocessor.EventProcessor, error) {
	return ca.allocator.Allocate(duration)
}

func (ca *ConnectionAllocator) Release(connection eventprocessor.EventProcessor) {
	// if the processor requires restart, recreate the connection
	if connection.GetStatus() == status.RestartRequired {
		var err error
		var newConnection []*Connection
		// Retry connection creation up to 3 times
	loop:
		for i := 0; i < 3; i++ {
			// only if the connection is in restartRequired status, try to recreate it
			// yes, that's a double check, but it's required to cover cases where connection allocator is stopping and
			// it will close all the connections
			currentStatus := connection.GetStatus()
			switch currentStatus {
			case status.RestartRequired:
				newConnection, err = ca.createConnections(1)
				if err == nil {
					break loop
				}
			case status.Stopped, status.Stopping:
				// if stopped or stopping, do not release the connection
				// it means that allocator is stopping
				return
			default:
				// currently this is not really possible that status will become ready without this flow
				// just for future
				ca.allocator.Release(connection)
				return
			}
		}

		// If still failing after retries, log the error and set status to not ready (it will signal to wrapper that restart is needed)
		if err != nil || len(newConnection) == 0 {
			// TODO: add a background check which checks connection manager status and restarts wrapper if needed
			// it's only added to timeout.go which checks for event timeouts
			// for now it's fine, however if we want to add more cases when restart is needed, then it should be done separately
			// only run if status is ready, otherwise ca is already stopping or marked as restart required
			if ca.GetStatus() == status.Ready {
				ca.SetStatus(status.RestartRequired)

				// different log depending on the error
				if err != nil {
					ca.Logger.WarnWith("Failed to recreate connection after retries, wrapper restart required",
						"rootCause", errors.RootCause(err).Error())
				} else {
					ca.Logger.WarnWith("Failed to recreate connection after retries, wrapper restart required",
						"reason", "no connection created, empty slice")
				}
			}
			// no need to release the connection as it is dead, whole allocator will be restarted
			return
		} else {
			switch typedConnection := connection.(type) {
			case *Connection:
				typedConnection.Replace(newConnection[0].AbstractEventConnection)
			default:
				ca.Logger.WarnWith("Failed to cast connection to *Connection",
					"connection", connection)
				ca.SetStatus(status.RestartRequired)
			}
		}
	}
	ca.allocator.Release(connection)
}

func (ca *ConnectionAllocator) GetAddressesForWrapperStart() ([]string, string) {
	controlAddress := ""
	if ca.controlMessageSocket != nil {
		controlAddress = ca.controlMessageSocket.Address
	}
	return []string{ca.serverAddress}, controlAddress
}

func (ca *ConnectionAllocator) startEventConnections() error {
	// create event connections
	eventConnections, err := ca.createConnections(ca.MinConnectionsNum)
	if err != nil {
		return errors.Wrap(err, "Failed to create connections")
	}

	eventProcessors := connectionsToEventProcessors(eventConnections)

	// set objects in allocator, which is the only object that holds connections
	if err := ca.allocator.SetObjects(eventProcessors); err != nil {
		return errors.Wrap(err, "Failed to set objects in allocator")
	}

	ca.Logger.Debug("Connection allocator started")
	return nil
}

func (ca *ConnectionAllocator) createConnections(connectionsNumber int) ([]*Connection, error) {
	eventConnections := make([]*Connection, 0)
	timeout := 30 * time.Second

	for i := 0; i < connectionsNumber; i++ {
		conn, err := retryableDial(ca.serverAddress, 30, 1*time.Second, timeout)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to establish connection")
		}
		eventConnections = append(eventConnections, NewConnection(ca.Logger, conn, ca))
	}

	// start event processing
	for _, eventConnection := range eventConnections {
		eventConnection.SetEncoder(ca.Configuration.GetEventEncoderFunc(eventConnection.Conn))
		go eventConnection.AbstractEventConnection.RunHandler()
	}

	// wait for start if required to
	if ca.Configuration.WaitForStart {
		ca.Logger.DebugWith("Waiting for start",
			"connectionsNumber", connectionsNumber,
			"timeout", timeout.String())
		for _, eventConnection := range eventConnections {
			if err := eventConnection.WaitForStart(timeout); err != nil {
				return nil, errors.Wrap(err,
					fmt.Sprintf("Failed to wait for connection start, remoteAddr: %s, localAddr: %s",
						eventConnection.Conn.RemoteAddr().String(), eventConnection.Conn.LocalAddr().String()))
			}
		}
		ca.Logger.DebugWith("Started successfully",
			"connectionsNumber", connectionsNumber)
	}
	for _, eventConnection := range eventConnections {
		eventConnection.status.SetStatus(status.Ready)
	}

	return eventConnections, nil
}

func retryableDial(address string,
	maxRetries int,
	retryInterval,
	dialTimeout time.Duration) (net.Conn, error) {
	var conn net.Conn
	var err error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Create a context with a timeout for each dial attempt
		ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)

		dialer := net.Dialer{}
		conn, err = dialer.DialContext(ctx, "tcp", address)
		if err == nil {
			cancel()
			return conn, nil
		}

		// If max retries are not reached, wait before retrying
		if attempt < maxRetries {
			time.Sleep(retryInterval)
		}
		cancel()
	}

	return nil, errors.Wrap(err, "Failed to establish connection after retries")
}

func connectionsToEventProcessors(eventConnections []*Connection) []eventprocessor.EventProcessor {
	eventProcessors := make([]eventprocessor.EventProcessor, len(eventConnections))
	for i, eventConnection := range eventConnections {
		eventProcessors[i] = eventConnection
	}
	return eventProcessors
}
