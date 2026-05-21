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

package controlcommunication

import (
	"sync"
)

// subscription is the broker-owned Subscription implementation.
//
// Lifecycle invariants:
//   - The channel `messages` is closed exactly once, inside Close(), after all
//     in-flight delivery goroutines have observed the close signal.
//   - SendToConsumers increments `inFlight` while holding the broker lock and
//     before starting a delivery goroutine; Close() removes the subscription
//     from the broker (so no new increments can happen) and then waits on
//     `inFlight`. This pairs Add/Wait correctly without races.
//   - Delivery goroutines never panic on a closed channel because they exit via
//     the `done` arm before close() runs.
type subscription struct {
	kind     ControlMessageKind
	broker   *AbstractControlMessageBroker
	messages chan *ControlMessage
	done     chan struct{}

	// inFlight counts delivery goroutines currently between SendToConsumers
	// (which increments under the broker lock) and goroutine exit. Close()
	// waits on this before closing `messages`.
	inFlight sync.WaitGroup

	// closeOnce guards Close() so it is idempotent and goroutine-safe.
	closeOnce sync.Once
}

func newSubscription(kind ControlMessageKind, broker *AbstractControlMessageBroker) *subscription {
	return &subscription{
		kind:     kind,
		broker:   broker,
		messages: make(chan *ControlMessage),
		done:     make(chan struct{}),
	}
}

func (s *subscription) C() <-chan *ControlMessage {
	return s.messages
}

func (s *subscription) Close() {
	s.closeOnce.Do(func() {

		// stop the broker from scheduling new deliveries; after this point any
		// SendToConsumers call won't even see this subscription
		s.broker.removeSubscription(s)

		// unblock any delivery goroutines that are stuck on `messages <- msg`
		// (no reader on the other end after Close)
		close(s.done)

		// wait for every in-flight delivery to observe `done` and exit before
		// we close `messages` — otherwise the goroutine could race and write
		// to a closed channel
		s.inFlight.Wait()

		close(s.messages)
	})
}

// deliver attempts to send message to the subscription's reader. It exits when
// the reader picks up the message or when Close() has signalled `done`,
// whichever happens first. The caller must have called inFlight.Add(1) before
// invoking deliver (this is done in SendToConsumers under the broker lock).
func (s *subscription) deliver(message *ControlMessage) {
	defer s.inFlight.Done()

	select {
	case s.messages <- message:
	case <-s.done:
	}
}
