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

package timeout

import (
	"time"

	"github.com/nuclio/nuclio/pkg/processor/trigger"

	"github.com/nuclio/logger"
)

// Processor is minimal processor interface
type Processor interface {
	GetTriggers() []trigger.Trigger
}

// EventTimeoutWatcher checks for event timesout
type EventTimeoutWatcher struct {
	timeout   time.Duration
	logger    logger.Logger
	processor Processor
}

// NewEventTimeoutWatcher returns a new watcher
func NewEventTimeoutWatcher(parentLogger logger.Logger, timeout time.Duration, processor Processor) (*EventTimeoutWatcher, error) {
	watcher := &EventTimeoutWatcher{
		logger:    parentLogger.GetChild("timeout"),
		timeout:   timeout,
		processor: processor,
	}

	go watcher.watch()

	return watcher, nil
}

func (w EventTimeoutWatcher) watch() {
	for {
		time.Sleep(w.timeout)
		now := time.Now()
		for triggerIndex, trigger := range w.processor.GetTriggers() {
			for _, worker := range trigger.GetWorkers() {
				eventTime := worker.GetEventTime()
				if eventTime == nil {
					continue
				}

				elapsedTime := now.Sub(*eventTime)
				if elapsedTime <= w.timeout {
					continue
				}

				with := []interface{}{
					"trigger", triggerIndex,
					"worker", worker.GetIndex(),
					"elapsed", elapsedTime,
				}

				// TODO: Convert processor.Triggers to map[string]Trigger so we'll get name
				w.logger.InfoWith("Restarting worker due to timeout", with...)
				if err := worker.Restart(); err != nil {
					with = append(with, "error", err)
					w.logger.ErrorWith("Can't restart worker", with...)
				}
			}
		}
	}
}
