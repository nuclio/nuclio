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
func NewEventTimeoutWatcher(parentLogger logger.Logger, timeout time.Duration, processor Processor) *EventTimeoutWatcher {
	watcher := &EventTimeoutWatcher{
		logger:    parentLogger.GetChild("timeout"),
		timeout:   timeout,
		processor: processor,
	}

	go watcher.watch()

	return watcher
}

func (w EventTimeoutWatcher) watch() {
	ticker := time.NewTicker(w.timeout)
	for now := range ticker.C {
		w.logger.DebugWith("Checking event timeouts", "time", now)
		for triggerIndex, trigger := range w.processor.GetTriggers() {
			with := []interface{}{"index", triggerIndex}
			// TODO: Place triggers in map[string]*Trigger so we'll have the name
			w.logger.DebugWith("Checking trigger", with...)
			for _, worker := range trigger.GetWorkers() {
				with = append(with, "worker", worker.GetIndex())
				w.logger.DebugWith("Checking worker", with...)
				eventTime := worker.GetEventTime()
				if eventTime == nil {
					w.logger.DebugWith("No current event", with...)
					continue
				}

				elapsedTime := now.Sub(*eventTime)
				with = append(with, "elapsed", elapsedTime)
				if elapsedTime <= w.timeout {
					w.logger.DebugWith("Timeout OK", with...)
					continue
				}

				w.logger.InfoWith("Restarting worker due to timeout", with...)
				if err := worker.Restart(); err != nil {
					with = append(with, "error", err)
					w.logger.ErrorWith("Can't restart worker", with...)
				}
				with = with[:len(with)-4] // remove worker & elapsed
			}
		}
	}
}
