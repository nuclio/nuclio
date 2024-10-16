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

package rpc

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/nuclio/nuclio/pkg/common"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/rs/xid"
)

const (
	socketPathTemplate = "/tmp/nuclio-rpc-%s.sock"
	connectionTimeout  = 2 * time.Minute
)

type SocketAllocator struct {
	abstractRuntime *AbstractRuntime
	logger          logger.Logger

	minSocketsNum        int
	maxSocketsNum        int
	eventSockets         []*EventSocket
	controlMessageSocket *ControlMessageSocket
}

func NewSocketAllocator(logger logger.Logger, runtime *AbstractRuntime) *SocketAllocator {
	// TODO: make minSocketsNum and maxSocketsNum when support multiple sockets
	return &SocketAllocator{
		logger:          logger,
		minSocketsNum:   1,
		maxSocketsNum:   1,
		eventSockets:    make([]*EventSocket, 0),
		abstractRuntime: runtime,
	}
}

func (sa *SocketAllocator) createSockets() error {
	if sa.abstractRuntime.runtime.SupportsControlCommunication() {
		controlConnection, err := sa.createSocketConnection()
		if err != nil {
			return errors.Wrap(err, "Failed to create socket connection")
		}
		sa.controlMessageSocket = NewControlMessageSocket(
			sa.logger.GetChild("ControlMessageSocket"),
			controlConnection,
			sa.abstractRuntime)
	}

	for i := 0; i < sa.minSocketsNum; i++ {
		eventConnection, err := sa.createSocketConnection()
		if err != nil {
			return errors.Wrap(err, "Failed to create socket connection")
		}
		sa.eventSockets = append(sa.eventSockets, NewEventSocket(sa.logger.GetChild("EventSocket"),
			eventConnection,
			sa.abstractRuntime))
	}
	return nil
}

func (sa *SocketAllocator) start() error {
	var err error
	for _, socket := range sa.eventSockets {
		if socket.conn, err = socket.listener.Accept(); err != nil {
			return errors.Wrap(err, "Can't get connection from wrapper")
		}
		socket.encoder = sa.abstractRuntime.runtime.GetEventEncoder(socket.conn)
		socket.resultChan = make(chan *batchedResults)
		socket.cancelChan = make(chan struct{})
		go socket.runHandler()
	}
	sa.logger.Debug("Successfully established connection for event sockets")

	if sa.abstractRuntime.runtime.SupportsControlCommunication() {
		sa.logger.DebugWith("Creating control connection",
			"wid", sa.abstractRuntime.Context.WorkerID)
		sa.controlMessageSocket.conn, err = sa.controlMessageSocket.listener.Accept()
		if err != nil {
			return errors.Wrap(err, "Can't get control connection from wrapper")
		}
		sa.controlMessageSocket.encoder = sa.abstractRuntime.runtime.GetEventEncoder(sa.controlMessageSocket.conn)

		// initialize control message broker
		sa.abstractRuntime.ControlMessageBroker = NewRpcControlMessageBroker(
			sa.controlMessageSocket.encoder,
			sa.logger,
			sa.abstractRuntime.configuration.ControlMessageBroker)

		go sa.controlMessageSocket.runHandler()

		sa.logger.DebugWith("Control connection created",
			"wid", sa.abstractRuntime.Context.WorkerID)
	}

	// wait for start if required to
	if sa.abstractRuntime.runtime.WaitForStart() {
		sa.logger.Debug("Waiting for start")
		for _, socket := range sa.eventSockets {
			<-socket.startChan
		}
	}

	sa.logger.Debug("Socker allocator started")
	return nil
}

func (sa *SocketAllocator) Allocate() *EventSocket {
	// TODO: implement allocation logic when support multiple sockets
	return sa.eventSockets[0]
}

func (sa *SocketAllocator) getSocketAddresses() ([]string, string) {
	eventAddresses := make([]string, 0)

	for _, socket := range sa.eventSockets {
		eventAddresses = append(eventAddresses, socket.address)
	}

	if sa.controlMessageSocket == nil {
		sa.logger.DebugWith("Get socket addresses",
			"eventAddresses", eventAddresses,
			"controlAddress", "")
		return eventAddresses, ""
	}
	sa.logger.DebugWith("Get socket addresses",
		"eventAddresses", eventAddresses,
		"controlAddress", sa.controlMessageSocket.address)
	return eventAddresses, sa.controlMessageSocket.address
}

// Create a listener on unix domain docker, return listener, path to socket and error
func (sa *SocketAllocator) createSocketConnection() (*socketConnection, error) {
	connection := &socketConnection{}
	var err error
	if sa.abstractRuntime.runtime.GetSocketType() == UnixSocket {
		connection.listener, connection.address, err = sa.createUnixListener()
	} else {
		connection.listener, connection.address, err = sa.createTCPListener()
	}
	if err != nil {
		return nil, errors.Wrap(err, "Can't create listener")
	}

	return connection, nil
}

// Create a listener on unix domain docker, return listener, path to socket and error
func (sa *SocketAllocator) createUnixListener() (net.Listener, string, error) {
	socketPath := fmt.Sprintf(socketPathTemplate, xid.New().String())

	if common.FileExists(socketPath) {
		if err := os.Remove(socketPath); err != nil {
			return nil, "", errors.Wrapf(err, "Can't remove socket at %q", socketPath)
		}
	}

	sa.logger.DebugWith("Creating listener socket", "path", socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, "", errors.Wrapf(err, "Can't listen on %s", socketPath)
	}

	unixListener, ok := listener.(*net.UnixListener)
	if !ok {
		return nil, "", fmt.Errorf("Can't get underlying Unix listener")
	}

	if err = unixListener.SetDeadline(time.Now().Add(connectionTimeout)); err != nil {
		return nil, "", errors.Wrap(err, "Can't set deadline")
	}

	return listener, socketPath, nil
}

// Create a listener on TCP docker, return listener, port and error
func (sa *SocketAllocator) createTCPListener() (net.Listener, string, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, "", errors.Wrap(err, "Can't find free port")
	}

	tcpListener, ok := listener.(*net.TCPListener)
	if !ok {
		return nil, "", errors.Wrap(err, "Can't get underlying TCP listener")
	}
	if err = tcpListener.SetDeadline(time.Now().Add(connectionTimeout)); err != nil {
		return nil, "", errors.Wrap(err, "Can't set deadline")
	}

	port := listener.Addr().(*net.TCPAddr).Port

	return listener, fmt.Sprintf("%d", port), nil
}
