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

	"github.com/nuclio/nuclio/pkg/common"
)

// MergeSubscriptions returns a single Subscription whose channel delivers
// messages from every provided subscription. Used by triggers to fan-in
// per-worker subscriptions into one stream.
//
// Closing the returned Subscription closes every underlying subscription and
// then closes the merged channel exactly once. Close is idempotent.
//
// If subs is empty, a non-nil already-closed Subscription is returned so that
// callers ranging over C() exit immediately instead of blocking forever.
func MergeSubscriptions(subs []Subscription) Subscription {
	switch len(subs) {
	case 0:
		m := newMergedSubscription(nil)
		m.Close()
		return m
	case 1:
		return subs[0]
	default:
		return newMergedSubscription(subs)
	}
}

type mergedSubscription struct {
	subs      []Subscription
	merged    chan *ControlMessage
	done      chan struct{}
	forwarder sync.WaitGroup
	closeOnce sync.Once
}

func newMergedSubscription(subs []Subscription) *mergedSubscription {
	merged := &mergedSubscription{
		subs:   subs,
		merged: make(chan *ControlMessage),
		done:   make(chan struct{}),
	}

	merged.forwarder.Add(len(subs))
	for _, sub := range subs {
		go merged.forward(sub)
	}

	return merged
}

func (m *mergedSubscription) C() <-chan *ControlMessage {
	return m.merged
}

func (m *mergedSubscription) Close() {
	m.closeOnce.Do(func() {

		// Close the underlying subscriptions concurrently. Each drains its in-flight
		// deliveries to our forwarders, which flush them to the merged channel for
		// the holder. Closing them concurrently (and waiting on the forwarders below,
		// not here) overlaps the two drains so the whole close stays bounded by a
		// single drainOnCloseTimeout.
		var subClosers sync.WaitGroup
		for _, sub := range m.subs {
			subClosers.Go(func() {
				sub.Close()
			})
		}

		// If the holder has stopped reading, the drain times out and we signal `done`
		// so forwarders parked on `merged <- msg` drop instead of hanging Close;
		// otherwise they exit as the underlying channels close. Either way every
		// forwarder must exit before we close `merged` - on the timeout path `done` is
		// already closed, so this returns promptly rather than waiting again.
		if !common.WaitGroupWithTimeout(&m.forwarder, drainOnCloseTimeout) {
			close(m.done)
		}
		m.forwarder.Wait()
		subClosers.Wait()

		close(m.merged)
	})
}

func (m *mergedSubscription) forward(sub Subscription) {
	defer m.forwarder.Done()

	for {
		select {
		case msg, ok := <-sub.C():
			if !ok {
				return
			}
			select {
			case m.merged <- msg:
			case <-m.done:
				return
			}
		case <-m.done:
			return
		}
	}
}
