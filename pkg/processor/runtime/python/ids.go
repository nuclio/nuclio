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
