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
	ctx "context"
	"fmt"
	"time"

	"github.com/nuclio/nuclio/pkg/errgroup"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

// Processor is minimal processor interface
type Processor interface {

	// GetTriggers returns triggers
	GetTriggers() []trigger.Trigger

	// Stop stops the triggers
	Stop()
}

// EventTimeoutWatcher checks for event timesout
type EventTimeoutWatcher struct {
	timeout      time.Duration
	logger       logger.Logger
	processor    Processor
	shuttingDown bool
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
	for !w.shuttingDown {
		time.Sleep(w.timeout)
		now := time.Now()

		// create error group
		triggerErrGroup, triggerErrGroupCtx := errgroup.WithContext(ctx.Background(), w.logger)

		// TODO: Run in parallel
		for triggerName, triggerInstance := range w.processor.GetTriggers() {
			triggerName, triggerInstance := triggerName, triggerInstance

			triggerErrGroup.Go("Watch trigger event timeout", func() error {

				// create error group
				workerErrGroup, workerErrGroupCtx := errgroup.WithContext(triggerErrGroupCtx, w.logger)

				// iterate over worker
				for _, workerInstance := range triggerInstance.GetWorkers() {
					workerInstance := workerInstance

					workerErrGroup.Go("Watch Event Timeout", func() error {
						eventTime := workerInstance.GetEventTime()
						if eventTime == nil {
							return nil
						}

						elapsedTime := now.Sub(*eventTime)
						if elapsedTime <= w.timeout {
							return nil
						}

						with := []interface{}{
							"trigger", triggerName,
							"worker", workerInstance.GetIndex(),
							"elapsed", elapsedTime,
						}

						// if the worker can be restarted, restart it. otherwise time it out
						if workerInstance.SupportsRestart() {
							w.logger.InfoWithCtx(workerErrGroupCtx, "Restarting worker due to timeout", with...)
							if err := workerInstance.Restart(); err != nil {
								with = append(with, "error", err)
								w.logger.ErrorWithCtx(workerErrGroupCtx, "Can't restart worker", with...)
							}
						} else {
							if err := triggerInstance.TimeoutWorker(workerInstance); err != nil {
								w.logger.WarnWithCtx(workerErrGroupCtx, "Error timing out a worker", "worker", workerInstance.GetIndex(), "trigger", triggerName)
							}
							w.gracefulShutdown(workerInstance)
						}

						return nil
					})

					return workerErrGroup.Wait()
				}

				return nil
			})
		}

		if err := triggerErrGroup.Wait(); err != nil {
			w.logger.WarnWithCtx(triggerErrGroupCtx, "Failed to wait for triggers", "err", errors.GetErrorStackString(err, 10))
		}
	}
}

func (w EventTimeoutWatcher) gracefulShutdown(timedoutWorker *worker.Worker) {
	w.logger.WarnWith("Staring graceful shutdown")

	w.shuttingDown = true

	runningWorkers := w.stopTriggers(timedoutWorker)
	w.waitForWorkers(runningWorkers)
	w.processor.Stop()
}

func (w EventTimeoutWatcher) stopTriggers(timedoutWorker *worker.Worker) map[string]*worker.Worker {
	runningWorkers := make(map[string]*worker.Worker)

	// create error group
	triggerErrGroup, triggerErrGroupCtx := errgroup.WithContext(ctx.Background(), w.logger)

	for triggerIdx, triggerInstance := range w.processor.GetTriggers() {
		triggerIdx, triggerInstance := triggerIdx, triggerInstance

		triggerErrGroup.Go("Stop trigger", func() error {

			if checkpoint, err := triggerInstance.Stop(false); err != nil {
				w.logger.ErrorWithCtx(triggerErrGroupCtx, "Can't stop trigger", "triggerIdx", triggerIdx, "error", err)
			} else {
				checkpointValue := ""
				if checkpoint != nil {
					checkpointValue = *checkpoint
				}
				w.logger.InfoWithCtx(triggerErrGroupCtx, "Trigger stopped", "triggerIdx", triggerIdx, "checkpoint", checkpointValue)
			}

			for _, workerInstance := range triggerInstance.GetWorkers() {
				if workerInstance == timedoutWorker {
					continue
				}

				if workerInstance.GetEventTime() == nil {
					continue
				}

				key := fmt.Sprintf("%d:%d", triggerIdx, workerInstance.GetIndex())
				runningWorkers[key] = workerInstance
			}

			return nil
		})
	}

	if err := triggerErrGroup.Wait(); err != nil {
		w.logger.WarnWithCtx(triggerErrGroupCtx, "Failed to wait for triggers", "err", errors.GetErrorStackString(err, 10))
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
