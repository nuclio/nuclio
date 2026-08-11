//go:build test_unit

/*
Copyright 2026 The Nuclio Authors.

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

package trigger

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/processor/controlcommunication"
	"github.com/nuclio/nuclio/pkg/processor/eventprocessor"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/statistics"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

// brokerOnlyRuntime satisfies runtime.Runtime by embedding the interface (nil, so any other
// method panics rather than silently no-op'ing) and implementing only GetControlMessageBroker.
type brokerOnlyRuntime struct {
	runtime.Runtime
	broker *controlcommunication.ControlMessageBrokerBase
}

func (r *brokerOnlyRuntime) GetControlMessageBroker() controlcommunication.ControlMessageBroker {
	return r.broker
}

// drainTestWorker is a minimal EventProcessor whose Subscribe is backed by a real control-message
// broker, so the test delivers drain acknowledgements exactly as the RPC reader would in
// production. It embeds MockEventProcessor only to satisfy the rest of the interface; Drain's code
// path touches only GetIndex and Subscribe.
//
// ackTimes controls how many drain-complete messages the worker emits on SignalDraining: 0 models
// a wrapper stuck in its drain callback, 1 is the normal case, and >1 models duplicate
// acknowledgements (the processor may re-signal; the wrapper re-acks when already drained).
type drainTestWorker struct {
	*eventprocessor.MockEventProcessor
	index    int
	broker   *controlcommunication.ControlMessageBrokerBase
	ackTimes int
}

func (w *drainTestWorker) GetIndex() int {
	return w.index
}

func (w *drainTestWorker) Subscribe(kind controlcommunication.ControlMessageKind) (controlcommunication.Subscription, error) {
	return w.broker.Subscribe(kind)
}

// GetRuntime exposes this worker's broker for SubscribeToControlMessageKind's dedup - the
// embedded mock's GetRuntime is unstubbed and would panic.
func (w *drainTestWorker) GetRuntime() runtime.Runtime {
	return &brokerOnlyRuntime{broker: w.broker}
}

// acknowledgeDrain simulates the Python wrapper sending its drain-complete control message.
func (w *drainTestWorker) acknowledgeDrain() error {
	return w.broker.SendToConsumers(&controlcommunication.ControlMessage{
		Kind: controlcommunication.DrainMessageKind,
		Attributes: map[string]interface{}{
			"workerId": strconv.Itoa(w.index),
		},
	})
}

// drainTestAllocator is a fake Allocator that, on SignalDraining, delivers each worker's
// acknowledgements. SignalDraining runs synchronously inside Drain after the subscription is in
// place, so deliveries are never lost to a race.
type drainTestAllocator struct {
	workers []*drainTestWorker
}

func (a *drainTestAllocator) SignalDraining() error {
	for _, worker := range a.workers {
		for i := 0; i < worker.ackTimes; i++ {
			if err := worker.acknowledgeDrain(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *drainTestAllocator) GetObjects() []eventprocessor.EventProcessor {
	objects := make([]eventprocessor.EventProcessor, 0, len(a.workers))
	for _, worker := range a.workers {
		objects = append(objects, worker)
	}
	return objects
}

func (a *drainTestAllocator) Allocate(timeout time.Duration) (eventprocessor.EventProcessor, error) {
	return nil, nil
}
func (a *drainTestAllocator) Release(eventprocessor.EventProcessor)            {}
func (a *drainTestAllocator) SetObjects([]eventprocessor.EventProcessor) error { return nil }
func (a *drainTestAllocator) GetNumObjectsAvailable() int                      { return len(a.workers) }
func (a *drainTestAllocator) GetStatistics() *statistics.AllocatorStatistics   { return nil }
func (a *drainTestAllocator) SignalContinue() error                            { return nil }
func (a *drainTestAllocator) SignalTermination() error                         { return nil }
func (a *drainTestAllocator) Stop() error                                      { return nil }
func (a *drainTestAllocator) IsTerminated() bool                               { return false }

type DrainTestSuite struct {
	suite.Suite
	logger logger.Logger
}

func (suite *DrainTestSuite) SetupTest() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
}

// newTrigger builds an AbstractTrigger whose workers each emit ackTimes[i] drain acknowledgements.
func (suite *DrainTestSuite) newTrigger(ackTimes []int) *AbstractTrigger {
	allocator := &drainTestAllocator{}
	for i, times := range ackTimes {
		allocator.workers = append(allocator.workers, &drainTestWorker{
			MockEventProcessor: &eventprocessor.MockEventProcessor{},
			index:              i,
			broker:             controlcommunication.NewControlMessageBrokerBase(),
			ackTimes:           times,
		})
	}
	return &AbstractTrigger{Logger: suite.logger, WorkerAllocator: allocator}
}

// TestDrainReturnsWhenAllWorkersAcknowledge asserts the happy path: once every worker sends its
// drain-complete message, Drain returns the full set of worker IDs with no error and without
// waiting for any timeout. This is the acknowledged-drain behaviour NUC-756 restores.
func (suite *DrainTestSuite) TestDrainReturnsWhenAllWorkersAcknowledge() {
	abstractTrigger := suite.newTrigger([]int{1, 1, 1})

	// generous deadline: the test must pass by acknowledgement, never by timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	drained, err := abstractTrigger.Drain(ctx)

	suite.Require().NoError(err)
	suite.Require().Len(drained, 3)
	for i := 0; i < 3; i++ {
		suite.Require().Contains(drained, strconv.Itoa(i))
	}
}

// TestDrainReturnsPartialSetOnTimeout asserts that when a worker never acknowledges (its drain
// callback is stuck), Drain does not hang: it returns once the context expires, reporting only the
// workers that drained. The caller (kafka Cleanup) uses exactly this set to decide which workers to
// restart - the recovery that fixes NUC-778 / NUC-766 on the waitForHandler=false path.
func (suite *DrainTestSuite) TestDrainReturnsPartialSetOnTimeout() {
	// worker 0 acknowledges, worker 1 stays silent
	abstractTrigger := suite.newTrigger([]int{1, 0})

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	start := time.Now()
	drained, err := abstractTrigger.Drain(ctx)
	elapsed := time.Since(start)

	suite.Require().Error(err)
	suite.Require().Len(drained, 1)
	suite.Require().Contains(drained, "0")
	suite.Require().NotContains(drained, "1")

	// returned because the context expired, not because it hung indefinitely
	suite.Require().Less(elapsed, 3*time.Second)
}

// TestDrainDeduplicatesRepeatedAcknowledgements asserts that duplicate drain-complete messages from
// the same worker are collapsed by worker ID: worker 0 acks twice and worker 1 once, and Drain
// returns exactly {"0","1"} - the duplicate neither inflates the count nor causes an early return
// before the other worker is accounted for.
func (suite *DrainTestSuite) TestDrainDeduplicatesRepeatedAcknowledgements() {
	abstractTrigger := suite.newTrigger([]int{2, 1})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	drained, err := abstractTrigger.Drain(ctx)

	suite.Require().NoError(err)
	suite.Require().Len(drained, 2)
	suite.Require().Contains(drained, "0")
	suite.Require().Contains(drained, "1")
}

// TestDrainWithNoWorkersReturnsEmpty asserts Drain does not block or panic when there are no
// workers to drain: MergeSubscriptions returns an already-closed subscription, and Drain must
// return an empty set with no error rather than nil-dereferencing a closed-channel read.
func (suite *DrainTestSuite) TestDrainWithNoWorkersReturnsEmpty() {
	abstractTrigger := suite.newTrigger([]int{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	drained, err := abstractTrigger.Drain(ctx)

	suite.Require().NoError(err)
	suite.Require().Empty(drained)
	suite.Require().Less(time.Since(start), 3*time.Second)
}

func TestDrainTestSuite(t *testing.T) {
	suite.Run(t, new(DrainTestSuite))
}
