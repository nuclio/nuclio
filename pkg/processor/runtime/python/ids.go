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
package python

import (
	"fmt"
	"os"
	"sync/atomic"
)

// IDGenerator generate incresting IDs in goroutine safe manner
type IDGenerator struct {
	pid int
	id  uint32
}

// NewIDGenerator returns next ID
func NewIDGenerator() *IDGenerator {
	return &IDGenerator{
		pid: os.Getpid(),
	}
}

// NextID return the next ID
func (idg *IDGenerator) NextID() string {
	id := atomic.AddUint32(&idg.id, 1)
	return fmt.Sprintf("%d-%d", idg.pid, id)
}
