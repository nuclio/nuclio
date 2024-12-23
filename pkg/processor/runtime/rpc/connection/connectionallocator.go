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

type ConnectionAllocator struct {
	*AbstractConnectionManager

	host string
	port int

	eventConnections     []*Connection
	controlMessageSocket *ControlMessageSocket
}

func NewConnectionAllocator(abstractConnectionManager *AbstractConnectionManager, host string, port int) *ConnectionAllocator {
	return &ConnectionAllocator{
		AbstractConnectionManager: abstractConnectionManager,
		host:                      host,
		port:                      port,
		eventConnections:          make([]*Connection, 0),
	}
}

func (ca *ConnectionAllocator) Prepare() error {
	return nil
}

func (ca *ConnectionAllocator) Start() error {
	return nil
}

func (ca *ConnectionAllocator) Stop() error {
	return nil
}

func (ca *ConnectionAllocator) Allocate() (EventConnection, error) {
	return nil, nil
}

func (ca *ConnectionAllocator) GetAddressesForWrapperStart() ([]string, string) {
	controlAddress := ""
	if ca.controlMessageSocket != nil {
		controlAddress = ca.controlMessageSocket.Address
	}
	return []string{}, controlAddress
}
