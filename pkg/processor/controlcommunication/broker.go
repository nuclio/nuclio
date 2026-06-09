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
	"bufio"
	"sync"
)

// ControlMessageBrokerBase is the default ControlMessageBroker implementation.
// Embed it in a struct to inherit fan-out and subscription management;
// override WriteControlMessage and ReadControlMessage for transport-specific behaviour.
type ControlMessageBrokerBase struct {
	subscriptions []*subscription
	lock          sync.Mutex
}

// NewControlMessageBrokerBase creates a new control message broker base
func NewControlMessageBrokerBase() *ControlMessageBrokerBase {
	return &ControlMessageBrokerBase{}
}

func (b *ControlMessageBrokerBase) WriteControlMessage(message *ControlMessage) error {
	return nil
}

func (b *ControlMessageBrokerBase) ReadControlMessage(reader *bufio.Reader) (*ControlMessage, error) {
	return nil, nil
}

func (b *ControlMessageBrokerBase) SendToConsumers(message *ControlMessage) error {

	// snapshot matching subscriptions under the lock so we can release it before
	// any blocking channel write. wg.Add must happen under the same lock that
	// guards removal, otherwise a concurrent Close() could miss in-flight sends.
	b.lock.Lock()
	var targets []*subscription
	for _, sub := range b.subscriptions {
		if sub.kind == message.Kind {
			sub.inFlight.Add(1)
			targets = append(targets, sub)
		}
	}
	b.lock.Unlock()

	// deliver to each subscription concurrently — a slow subscriber must not
	// block delivery to fast ones, and a Close()d subscription must not block
	// the broker at all
	for _, sub := range targets {
		go sub.deliver(message)
	}

	return nil
}

func (b *ControlMessageBrokerBase) Subscribe(kind ControlMessageKind) (Subscription, error) {
	sub := newSubscription(kind, b)

	b.lock.Lock()
	b.subscriptions = append(b.subscriptions, sub)
	b.lock.Unlock()

	return sub, nil
}

// removeSubscription removes sub from the broker's subscription list. Called by
// subscription.Close() — never call directly from outside the package.
func (b *ControlMessageBrokerBase) removeSubscription(target *subscription) {
	b.lock.Lock()
	defer b.lock.Unlock()
	for i, sub := range b.subscriptions {
		if sub == target {
			b.subscriptions = append(b.subscriptions[:i], b.subscriptions[i+1:]...)
			return
		}
	}
}
