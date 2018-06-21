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
	"fmt"
	"time"

	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/logger"
)

// Processor is minimal processor interface
type Processor interface {
	GetTriggers() map[string]trigger.Trigger
	Stop()
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
		// TODO: Run in parallel
		for triggerName, trigger := range w.processor.GetTriggers() {
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
					"trigger", triggerName,
					"worker", worker.GetIndex(),
					"elapsed", elapsedTime,
				}

				if worker.SupportsRestart() {
					// TODO: Convert processor.Triggers to map[string]Trigger so we'll get name
					w.logger.InfoWith("Restarting worker due to timeout", with...)
					if err := worker.Restart(); err != nil {
						with = append(with, "error", err)
						w.logger.ErrorWith("Can't restart worker", with...)
					}
				} else {
					trigger.TimeoutWorker(worker)
					w.gracefulShutdown(worker)
					return
				}
			}
		}
	}
}

func (w EventTimeoutWatcher) gracefulShutdown(timedoutWorker *worker.Worker) {
	w.logger.WarnWith("Staring graceful shutdown")

	runningWorkers := w.stopTriggers(timedoutWorker)
	w.waitForWorkers(runningWorkers)
	w.processor.Stop()
}

func (w EventTimeoutWatcher) stopTriggers(timedoutWorker *worker.Worker) map[string]*worker.Worker {
	runningWorkers := make(map[string]*worker.Worker)

	for triggerName, trigger := range w.processor.GetTriggers() {
		if checkpoint, err := trigger.Stop(false); err != nil {
			w.logger.ErrorWith("Can't stop trigger", "trigger", triggerName, "error", err)
		} else {
			checkpointValue := ""
			if checkpoint != nil {
				checkpointValue = *checkpoint
			}
			w.logger.InfoWith("Trigger stopped", "trigger", triggerName, "checkpoint", checkpointValue)
		}

		for _, workerInstance := range trigger.GetWorkers() {
			if workerInstance == timedoutWorker {
				continue
			}

			if workerInstance.GetEventTime() == nil {
				continue
			}

			key := fmt.Sprintf("%s:%d", triggerName, workerInstance.GetIndex())
			runningWorkers[key] = workerInstance
		}
	}

	return runningWorkers
}

func (w EventTimeoutWatcher) waitForWorkers(runningWorkers map[string]*worker.Worker) {
	// TODO: Find a better deadline
	shutdownDuration := 10 * w.timeout
	deadline := time.Now().Add(shutdownDuration)

	for {
		if len(runningWorkers) == 0 {
			return
		}

		now := time.Now()
		if now.After(deadline) {
			w.logger.WarnWith("Graceful shutdown deadline reached", "duration", shutdownDuration)
			return
		}

		for key, workerInstance := range runningWorkers {
			eventTime := workerInstance.GetEventTime()
			if eventTime == nil {
				delete(runningWorkers, key)
				continue
			}

			if now.Sub(*eventTime) > w.timeout {
				w.logger.WarnWith("Worker timed out", "worker", key)
				delete(runningWorkers, key)
				continue
			}
		}

		time.Sleep(100 * time.Millisecond)
	}
}
