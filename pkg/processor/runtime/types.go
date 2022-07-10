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

package runtime

import (
	"sync/atomic"
	"time"

	"github.com/nuclio/nuclio/pkg/processor"

	"github.com/nuclio/logger"
)

type Statistics struct {
	DurationMilliSecondsSum   uint64
	DurationMilliSecondsCount uint64
}

func (s *Statistics) DiffFrom(prev *Statistics) Statistics {

	// atomically load the counters
	currDurationMilliSecondsSum := atomic.LoadUint64(&s.DurationMilliSecondsSum)
	currDurationMilliSecondsCount := atomic.LoadUint64(&s.DurationMilliSecondsCount)

	prevDurationMilliSecondsSum := atomic.LoadUint64(&prev.DurationMilliSecondsSum)
	prevDurationMilliSecondsCount := atomic.LoadUint64(&prev.DurationMilliSecondsCount)

	return Statistics{
		DurationMilliSecondsSum:   currDurationMilliSecondsSum - prevDurationMilliSecondsSum,
		DurationMilliSecondsCount: currDurationMilliSecondsCount - prevDurationMilliSecondsCount,
	}
}

type Configuration struct {
	*processor.Configuration
	FunctionLogger           logger.Logger
	WorkerID                 int
	TriggerName              string
	TriggerKind              string
	ExplicitAckEnabled       bool
	WorkerTerminationTimeout time.Duration
	ControlChannels          processor.ControlChannel
}
