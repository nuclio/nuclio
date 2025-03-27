/*
Copyright 2023 The Nuclio Authors.

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
	"context"
	"time"

	"github.com/nuclio/nuclio/pkg/errgroup"
	"github.com/nuclio/nuclio/pkg/processor/eventprocessor"
	"github.com/nuclio/nuclio/pkg/processor/trigger"

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

		// create error group
		triggerErrGroup, triggerErrGroupCtx := errgroup.WithContext(context.Background(), w.logger)

		for triggerName, triggerInstance := range w.processor.GetTriggers() {
			triggerName, triggerInstance := triggerName, triggerInstance

			triggerErrGroup.Go("Watch trigger event timeout", func() error {

				// create error group
				workerErrGroup, workerErrGroupCtx := errgroup.WithContext(triggerErrGroupCtx, w.logger)

				// iterate over worker
				for _, workerInstance := range triggerInstance.GetWorkers() {
					workerInstance := workerInstance

					workerErrGroup.Go("Watch Event Timeout", func() error {
						restartRequired := workerInstance.RestartRequired()
						if !restartRequired {
							return nil
						}

						with := []interface{}{
							"trigger", triggerName,
							"worker", workerInstance.GetIndex(),
						}

						// if the worker can be restarted, restart it. otherwise shut it completely
						if workerInstance.SupportsRestart() {
							w.logger.InfoWithCtx(workerErrGroupCtx, "Restarting worker due to timeout", with...)
							if err := workerInstance.Restart(); err != nil {
								with = append(with, "error", err)
								w.logger.ErrorWithCtx(workerErrGroupCtx, "Can't restart worker", with...)
							}
						} else {
							w.gracefulShutdown(triggerErrGroupCtx, workerInstance)
						}

						return nil
					})

				}
				return workerErrGroup.Wait()
			})
		}

		if err := triggerErrGroup.Wait(); err != nil {
			w.logger.WarnWithCtx(triggerErrGroupCtx, "Failed to wait for triggers", "err", errors.GetErrorStackString(err, 10))
		}
	}
}

func (w EventTimeoutWatcher) gracefulShutdown(ctx context.Context, timedoutWorker eventprocessor.EventProcessor) {
	w.logger.WarnWithCtx(ctx, "Staring graceful shutdown")

	w.shuttingDown = true

	w.logger.WarnWithCtx(ctx, "Stopping triggers")
	busyTriggers := w.stopTriggers(ctx, timedoutWorker)

	w.logger.WarnWithCtx(ctx, "Waiting for workers termination")
	w.waitForTriggers(ctx, busyTriggers)

	w.logger.WarnWithCtx(ctx, "Stopping processor")
	w.processor.Stop()

	w.logger.WarnWithCtx(ctx, "Graceful shutdown completed")
}

func (w EventTimeoutWatcher) stopTriggers(ctx context.Context, timedoutWorker eventprocessor.EventProcessor) map[int]trigger.Trigger {
	busyTriggers := make(map[int]trigger.Trigger)

	// create error group
	triggerErrGroup, triggerErrGroupCtx := errgroup.WithContext(ctx, w.logger)

	for triggerIdx, triggerInstance := range w.processor.GetTriggers() {
		triggerIdx, triggerInstance := triggerIdx, triggerInstance

		triggerErrGroup.Go("Stop trigger", func() error {

			if checkpoint, err := triggerInstance.Stop(false); err != nil {
				w.logger.ErrorWithCtx(triggerErrGroupCtx,
					"Can't stop trigger",
					"triggerIdx", triggerIdx,
					"error", err)
			} else {
				checkpointValue := ""
				if checkpoint != nil {
					checkpointValue = *checkpoint
				}
				w.logger.InfoWithCtx(triggerErrGroupCtx,
					"Trigger stopped",
					"triggerIdx", triggerIdx,
					"checkpoint", checkpointValue)
			}
			if triggerInstance.IsBusy() {
				busyTriggers[triggerIdx] = triggerInstance
			}
			return nil
		})
	}

	if err := triggerErrGroup.Wait(); err != nil {
		w.logger.WarnWithCtx(triggerErrGroupCtx,
			"Failed to wait for triggers",
			"err", errors.GetErrorStackString(err, 10))
	}

	return busyTriggers
}

func (w EventTimeoutWatcher) waitForTriggers(ctx context.Context, busyTriggers map[int]trigger.Trigger) {
	// TODO: Find a better deadline
	shutdownDuration := 10 * w.timeout
	deadline := time.Now().Add(shutdownDuration)

	for {

		// exit when there are no busy triggers
		if len(busyTriggers) == 0 {
			return
		}

		now := time.Now()
		if now.After(deadline) {
			w.logger.WarnWithCtx(ctx,
				"Graceful shutdown deadline reached",
				"duration", shutdownDuration)
			return
		}

		// go though the busy triggers and check if they are still busy
		// if they are not, remove them from the busy triggers
		for triggerIdx, triggerInstance := range busyTriggers {
			if !triggerInstance.IsBusy() {
				delete(busyTriggers, triggerIdx)
				continue
			}
		}

		time.Sleep(100 * time.Millisecond)
	}
}
