package worker

import (
	"time"
)

type Statistics struct {
	Iterations uint64
	Items      uint64
	Succeeded  uint64
	Failed     uint64
	Retry      uint64
	Duration   uint64
	Queued     uint64
	StartTime  time.Time
}
