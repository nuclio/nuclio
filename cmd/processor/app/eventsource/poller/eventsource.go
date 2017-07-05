package poller

import (
	"time"

	"github.com/nuclio/nuclio/cmd/processor/app/event"
	"github.com/nuclio/nuclio/cmd/processor/app/eventsource"
	"github.com/nuclio/nuclio/cmd/processor/app/worker"
	"github.com/nuclio/nuclio/pkg/logger"

	"github.com/pkg/errors"
)

type AbstractPoller struct {
	eventsource.AbstractEventSource
	configuration *Configuration
	poller        Poller
}

func NewAbstractPoller(logger logger.Logger,
	workerAllocator worker.WorkerAllocator,
	configuration *Configuration) *AbstractPoller {

	return &AbstractPoller{
		AbstractEventSource: eventsource.AbstractEventSource{
			Logger:          logger,
			WorkerAllocator: workerAllocator,
			Class:           "batch",
			Kind:            "poller",
		},
		configuration: configuration,
	}
}

// to allow parent to call functions implemented in child
func (ap *AbstractPoller) SetPoller(poller Poller) {
	ap.poller = poller
}

func (ap *AbstractPoller) Start(checkpoint eventsource.Checkpoint) error {

	// process one cycle at a time (don't getNewEvents again while processing)
	go ap.getEventsSingleCycle()

	return nil
}

func (ap *AbstractPoller) Stop(force bool) (eventsource.Checkpoint, error) {

	// TODO
	return nil, nil
}

// in this strategy, we trigger getNewEvents once, process all the events it creates (while getNewEvents is producing
// and only then re-trigger getNewEvents. in the future we'll probably have getNewEvents producing in the background
func (ap *AbstractPoller) getEventsSingleCycle() {
	var eventBatch []event.Event
	var err error

	eventsChan := make(chan event.Event)

	for {
		eventCycleCompleted := false

		// trigger a single poll for events. do this in a go routine so that we can start processing
		// event batches while the poll is happening. getNewEvents will add a "nil" entry into the channel
		// when it's done
		go ap.poller.GetNewEvents(eventsChan)

		// while getNewEvents is still producing events
		for !eventCycleCompleted {

			// create a batch from the events we poll
			eventBatch, eventCycleCompleted, err = ap.waitForEventBatch(eventsChan,
				ap.configuration.MaxBatchSize,
				time.Duration(ap.configuration.MaxBatchWaitMs)*time.Millisecond)

			if err != nil {
				errors.Wrap(err, "Failed to gather event batch")
				continue

				// TODO
			}

			ap.Logger.DebugWith("Got events", "num", len(eventBatch))

			// send the batch to the worker
			// eventResponses, submitError, eventErrors := ap.SubmitEventsToWorker(eventBatch, 10 * time.Second)
			eventResponses, submitError, eventErrors := ap.SubmitEventsToWorker(eventBatch, 10*time.Second)

			if submitError != nil {
				errors.Wrap(err, "Failed to submit events to worker")
				continue

				// TODO
			}

			// post process the events
			ap.poller.PostProcessEvents(eventBatch, eventResponses, eventErrors)
		}

		// wait the interva
		time.Sleep(time.Duration(ap.configuration.IntervalMs) * time.Millisecond)
	}
}

// gets a batch of events from the channel. will return when either the max number of events per batch is read, if a
// timeout expires or if we get a nil event from the channel indicating the reader completed a cycle
func (ap *AbstractPoller) waitForEventBatch(eventsChan chan event.Event,
	maxBatchSize int,
	maxBatchDuration time.Duration) ([]event.Event, bool, error) {

	done := false
	eventCycleCompleted := false
	events := make([]event.Event, 0, maxBatchSize)

	// calculate the deadline
	deadline := time.Now().Add(maxBatchDuration)

	for !done {
		timeLeft := time.Until(deadline)

		select {
		case receivedEvent := <-eventsChan:

			// if nil, the cycle is complete, can stop
			if receivedEvent == nil {
				eventCycleCompleted = true
				done = true
			} else {

				// add to events
				events = append(events, receivedEvent)

				// check if we reached max size. if so we're done
				if len(events) > maxBatchSize {
					done = true
				}
			}
		case <-time.After(timeLeft):
			done = true
		}
	}

	return events, eventCycleCompleted, nil
}

func (ap *AbstractPoller) onV3ioLog(formattedRecord string) {
	ap.Logger.Debug(formattedRecord)
}
