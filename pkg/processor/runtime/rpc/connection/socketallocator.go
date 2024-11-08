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
	"fmt"
	"net"
	"os"
	"time"

	"github.com/nuclio/nuclio/pkg/common"

	"github.com/nuclio/errors"
	"github.com/rs/xid"
)

const (
	socketPathTemplate = "/tmp/nuclio-rpc-%s.sock"
	connectionTimeout  = 2 * time.Minute
)

type SocketAllocator struct {
	*BaseConnectionManager

	eventSockets         []*EventSocket
	controlMessageSocket *ControlMessageSocket
}

func NewSocketAllocator(baseConnectionManager *BaseConnectionManager) *SocketAllocator {
	return &SocketAllocator{
		BaseConnectionManager: baseConnectionManager,
		eventSockets:          make([]*EventSocket, 0),
	}
}

func (sa *SocketAllocator) Prepare() error {
	if sa.Configuration.SupportControlCommunication {
		controlConnection, err := sa.createSocketConnection()
		if err != nil {
			return errors.Wrap(err, "Failed to create socket connection")
		}
		sa.controlMessageSocket = NewControlMessageSocket(
			sa.Logger.GetChild("ControlMessageSocket"),
			controlConnection,
			sa.RuntimeConfiguration.ControlMessageBroker)
	}

	for i := 0; i < sa.MinSocketsNum; i++ {
		eventConnection, err := sa.createSocketConnection()
		if err != nil {
			return errors.Wrap(err, "Failed to create socket connection")
		}
		sa.eventSockets = append(sa.eventSockets, NewEventSocket(sa.Logger.GetChild("EventSocket"),
			eventConnection, sa))
	}
	return nil
}

func (sa *SocketAllocator) Start() error {
	if err := sa.startSockets(); err != nil {
		return errors.Wrap(err, "Failed to start socket allocator")
	}

	// wait for start if required to
	if sa.Configuration.WaitForStart {
		sa.Logger.Debug("Waiting for start")
		for _, socket := range sa.eventSockets {
			socket.Start()
		}
	}

	sa.Logger.Debug("Socker allocator started")
	return nil
}

func (sa *SocketAllocator) Stop() error {
	for _, eventSocket := range sa.eventSockets {
		socket := eventSocket
		go func() {
			socket.Stop()
		}()
	}
	if sa.controlMessageSocket != nil {
		go func() {
			sa.controlMessageSocket.Stop()
		}()
	}
	return nil
}

func (sa *SocketAllocator) Allocate() (EventConnection, error) {
	// TODO: implement allocation logic when support multiple sockets
	return sa.eventSockets[0], nil
}

func (sa *SocketAllocator) GetAddressesForWrapperStart() ([]string, string) {
	eventAddresses := make([]string, 0)

	for _, socket := range sa.eventSockets {
		eventAddresses = append(eventAddresses, socket.Address)
	}

	if sa.controlMessageSocket == nil {
		sa.Logger.DebugWith("Get socket addresses",
			"eventAddresses", eventAddresses,
			"controlAddress", "")
		return eventAddresses, ""
	}
	sa.Logger.DebugWith("Get socket addresses",
		"eventAddresses", eventAddresses,
		"controlAddress", sa.controlMessageSocket.Address)
	return eventAddresses, sa.controlMessageSocket.Address
}

func (sa *SocketAllocator) startSockets() error {
	var err error
	for _, socket := range sa.eventSockets {
		if socket.Conn, err = socket.listener.Accept(); err != nil {
			return errors.Wrap(err, "Can't get connection from wrapper")
		}
		socket.SetEncoder(sa.Configuration.GetEventEncoderFunc(socket.Conn))
		go socket.BaseEventConnection.RunHandler()
	}
	sa.Logger.Debug("Successfully established connection for event sockets")

	if sa.Configuration.SupportControlCommunication {
		sa.controlMessageSocket.Conn, err = sa.controlMessageSocket.listener.Accept()
		if err != nil {
			return errors.Wrap(err, "Can't get control connection from wrapper")
		}
		sa.controlMessageSocket.SetEncoder(sa.Configuration.GetEventEncoderFunc(sa.controlMessageSocket.Conn))

		// initialize control message broker
		sa.controlMessageSocket.SetBroker(sa.RuntimeConfiguration.ControlMessageBroker)
		go sa.controlMessageSocket.RunHandler()

	}
	return nil
}

// Create a listener on unix domain docker, return listener, path to socket and error
func (sa *SocketAllocator) createSocketConnection() (*socketConnection, error) {
	connection := &socketConnection{}
	var err error
	if sa.Configuration.SocketType == UnixSocket {
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

	sa.Logger.DebugWith("Creating listener socket", "path", socketPath)

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
