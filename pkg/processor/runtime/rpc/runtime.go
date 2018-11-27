/*
Copyright 2017 The Nuclio Authors.

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

import "os"

// SocketType is type of socket to use
type SocketType int

// RPC socket types
const (
	UnixSocket SocketType = iota
	TCPSocket
)

type Runtime interface {

	// RunWrapper runs the wrapper
	RunWrapper(string) (*os.Process, error)

	// GetSocketType returns the type of socket the runtime works with (unix/tcp)
	GetSocketType() SocketType

	// WaitForStart returns whether the runtime supports sending an indication that it started
	WaitForStart() bool
}
