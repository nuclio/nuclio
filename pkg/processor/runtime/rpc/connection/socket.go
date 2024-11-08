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
	"net"

	"github.com/nuclio/nuclio/pkg/processor/controlcommunication"

	"github.com/nuclio/logger"
)

// SocketType SocketType is type of socket to use
type SocketType int

// RPC socket types
const (
	UnixSocket SocketType = iota
	TCPSocket
)

type socketConnection struct {
	conn     net.Conn
	listener net.Listener
	address  string
}

type ControlMessageSocket struct {
	*BaseControlMessageConnection

	// socket-specific entity
	listener net.Listener
}

func NewControlMessageSocket(parentLogger logger.Logger, socketConn *socketConnection, broker controlcommunication.ControlMessageBroker) *ControlMessageSocket {

	baseControlMessageConnection := NewBaseControlMessageConnection(parentLogger, broker)
	baseControlMessageConnection.Conn = socketConn.conn
	baseControlMessageConnection.Address = socketConn.address

	return &ControlMessageSocket{
		BaseControlMessageConnection: baseControlMessageConnection,
		listener:                     socketConn.listener,
	}
}

type EventSocket struct {
	*BaseEventConnection

	// socket-specific entity
	listener net.Listener
}

func NewEventSocket(parentLogger logger.Logger, socketConn *socketConnection, connectionManager ConnectionManager) *EventSocket {

	baseEventConnection := NewBaseEventConnection(parentLogger, connectionManager)
	baseEventConnection.Conn = socketConn.conn
	baseEventConnection.Address = socketConn.address

	return &EventSocket{
		BaseEventConnection: baseEventConnection,
		listener:            socketConn.listener,
	}
}
