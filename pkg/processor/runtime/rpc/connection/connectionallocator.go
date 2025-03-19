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
	"sync"
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
	if err := ca.prepareControlMessageSocket(); err != nil {
		return errors.Wrap(err, "Failed to prepare control message socket")
	}
	return nil
}

func (ca *ConnectionAllocator) Start() error {

	ca.Logger.DebugWith("Starting connection allocator")
	// starts control message socket
	if err := ca.startControlMessageSocket(); err != nil {
		return errors.Wrap(err, "Failed to start control message socket")
	}

	// create event connections
	eventProcessors, err := ca.createConnections(ca.MinConnectionsNum)
	if err != nil {
		return errors.Wrap(err, "Failed to create connections")
	}

	// set objects in allocator, which is the only object that holds connections
	if err := ca.allocator.SetObjects(eventProcessors); err != nil {
		return errors.Wrap(err, "Failed to set objects in allocator")
	}

	ca.Logger.Debug("Connection allocator started")
	return nil
}

func (ca *ConnectionAllocator) Stop() error {
	var wg sync.WaitGroup

	for _, eventConnection := range ca.allocator.GetObjects() {
		connection := eventConnection.(Connection)
		wg.Add(1)

		go func() {
			defer wg.Done()
			if err := connection.Stop(); err != nil {
				ca.Logger.WarnWith("Failed to close connection", "error", err)
			}
		}()
	}

	wg.Wait()
	ca.stopControlMessageSocket()
	return nil
}

func (ca *ConnectionAllocator) Allocate(duration time.Duration) (eventprocessor.EventProcessor, error) {
	return ca.allocator.Allocate(duration)
}

func (ca *ConnectionAllocator) Release(processor eventprocessor.EventProcessor) {
	// if when releasing processor requires restart, recreate the connection
	if processor.GetStatus() == status.RestartRequired {
		var newProcessor []eventprocessor.EventProcessor
		var err error

		// Retry connection creation up to 3 times
		for i := 0; i < 3; i++ {
			newProcessor, err = ca.createConnections(1)
			if err == nil {
				break
			}
			ca.Logger.WarnWith("Failed to recreate connection, retrying", "attempt", i+1, "error", err)
		}

		// If still failing after retries, log the error and set status to not ready (it will signal to wrapper that restart is needed)
		if err != nil {
			ca.Logger.WarnWith("Failed to recreate connection after retries", "error", err)

			// TODO: add a background check which checks connection manager status and restarts wrapper if needed
			// it's only added to timeout.go which checks for event timeouts
			// for now it's fine, however if we want to add more cases when restart is needed, then it should be done separately
			ca.SetStatus(status.RestartRequired)
		} else {
			processor = newProcessor[0]
		}
	}
	ca.allocator.Release(processor)
}

func (ca *ConnectionAllocator) GetAddressesForWrapperStart() ([]string, string) {
	controlAddress := ""
	if ca.controlMessageSocket != nil {
		controlAddress = ca.controlMessageSocket.Address
	}
	return []string{ca.serverAddress}, controlAddress
}

// ConnectionMonitor checks that all connections in allocator are healthy
// if some are not, it will re-establish them
func (ca *ConnectionAllocator) ConnectionMonitor() {
	// by default, wait for 5 seconds before the next check
	timeout := 5 * time.Second
	if ca.GetConfig().eventTimeout != 0 {
		// if event timeout is set, then use it (because we want to find out about termination of a socket as soon as possible)
		timeout = ca.GetConfig().eventTimeout
	}
	for {
		ca.Logger.DebugWith("ConnectionsMonitor", "numObjectsAvailable", ca.allocator.GetNumObjectsAvailable())
		time.Sleep(timeout)

	}
}

func (ca *ConnectionAllocator) createConnections(connectionsNumber int) ([]eventprocessor.EventProcessor, error) {
	eventConnections := make([]*Connection, 0)
	for i := 0; i < connectionsNumber; i++ {
		conn, err := retryableDial(ca.serverAddress, 30, 1*time.Second, 1*time.Minute)
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
		ca.Logger.Debug("Waiting for start")
		for _, eventConnection := range eventConnections {
			eventConnection.WaitForStart()
		}
	}
	for _, eventConnection := range eventConnections {
		eventConnection.status.SetStatus(status.Ready)
	}

	eventProcessors := make([]eventprocessor.EventProcessor, len(eventConnections))
	for i, eventConnection := range eventConnections {
		eventProcessors[i] = eventConnection
	}
	return eventProcessors, nil
}

func retryableDial(address string, maxRetries int, retryInterval, dialTimeout time.Duration) (net.Conn, error) {
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
