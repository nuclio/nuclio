//go:build test_unit

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

package controlcommunication

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// regressionTimeout is the upper bound for any operation that we expect to
// finish ~immediately under correct behaviour. If we hit this timeout, the
// test fails — picking it long enough that load on CI doesn't false-positive
// but short enough to keep the suite snappy.
const regressionTimeout = 2 * time.Second

type BrokerTestSuite struct {
	suite.Suite
	broker *ControlMessageBrokerBase
}

func (s *BrokerTestSuite) SetupTest() {
	s.broker = NewControlMessageBrokerBase()
}

// TestSubscribeReceive — the happy path. A subscriber receives every message
// sent for its kind, in order, and other kinds are not delivered to it.
func (s *BrokerTestSuite) TestSubscribeReceive() {
	sub, err := s.broker.Subscribe(StreamMessageAckKind)
	s.Require().NoError(err)
	defer sub.Close()

	want := &ControlMessage{Kind: StreamMessageAckKind, Attributes: map[string]interface{}{"i": 1}}
	other := &ControlMessage{Kind: LogMessageKind, Attributes: map[string]interface{}{"i": 2}}

	s.Require().NoError(s.broker.SendToConsumers(want))
	s.Require().NoError(s.broker.SendToConsumers(other)) // must not be delivered

	got := s.recvWithTimeout(sub)
	s.Require().Equal(want, got)

	// no second delivery — verify by checking the channel is empty after a short wait
	select {
	case msg := <-sub.C():
		s.Failf("received unexpected message", "%v", msg)
	case <-time.After(50 * time.Millisecond):
	}
}

// TestMultipleSubscribersSameKind — both subscribers must receive every
// matching message (broadcast).
func (s *BrokerTestSuite) TestMultipleSubscribersSameKind() {
	subA, err := s.broker.Subscribe(StreamMessageAckKind)
	s.Require().NoError(err)
	defer subA.Close()

	subB, err := s.broker.Subscribe(StreamMessageAckKind)
	s.Require().NoError(err)
	defer subB.Close()

	msg := &ControlMessage{Kind: StreamMessageAckKind}
	s.Require().NoError(s.broker.SendToConsumers(msg))

	s.Require().Equal(msg, s.recvWithTimeout(subA))
	s.Require().Equal(msg, s.recvWithTimeout(subB))
}

// TestCloseUnblocksSendWithoutReader — the core NUC-765 regression test.
//
// Before the fix, a SendToConsumers call would hold the broker mutex while
// writing to an unbuffered channel that had no reader, deadlocking every
// subsequent Subscribe/Unsubscribe/Send. After the fix, Close must unblock the
// in-flight send and the broker must remain usable.
func (s *BrokerTestSuite) TestCloseUnblocksSendWithoutReader() {
	sub, err := s.broker.Subscribe(StreamMessageAckKind)
	s.Require().NoError(err)

	// SendToConsumers schedules a delivery goroutine that will block on the
	// unbuffered channel because no one is reading.
	s.Require().NoError(s.broker.SendToConsumers(&ControlMessage{Kind: StreamMessageAckKind}))

	// Close must return promptly even though the delivery goroutine is parked
	// on `messages <-`.
	done := make(chan struct{})
	go func() {
		sub.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(regressionTimeout):
		s.Fail("Close() blocked — broker is still deadlocked (NUC-765 regression)")
	}

	// And the broker is still usable after Close — subscribe a fresh
	// subscription, send, receive.
	fresh, err := s.broker.Subscribe(StreamMessageAckKind)
	s.Require().NoError(err)
	defer fresh.Close()

	msg := &ControlMessage{Kind: StreamMessageAckKind, Attributes: map[string]interface{}{"post": "close"}}
	s.Require().NoError(s.broker.SendToConsumers(msg))
	s.Require().Equal(msg, s.recvWithTimeout(fresh))
}

// TestSendAfterCloseIsSafe — once a subscription is closed, the broker must
// not panic on a closed-channel write, and must not deliver any further
// messages to the closed channel.
func (s *BrokerTestSuite) TestSendAfterCloseIsSafe() {
	sub, err := s.broker.Subscribe(StreamMessageAckKind)
	s.Require().NoError(err)
	sub.Close()

	// post-close sends must succeed (no panic, no error) and must be no-ops
	// for this subscription
	for i := 0; i < 10; i++ {
		s.Require().NoError(s.broker.SendToConsumers(&ControlMessage{Kind: StreamMessageAckKind}))
	}

	// channel was closed by Close() — receiving on a closed channel returns
	// the zero value immediately with ok=false
	msg, ok := <-sub.C()
	s.Require().False(ok, "channel must be closed after Close()")
	s.Require().Nil(msg)
}

// TestCloseIdempotent — Close must be safe to call multiple times.
func (s *BrokerTestSuite) TestCloseIdempotent() {
	sub, err := s.broker.Subscribe(StreamMessageAckKind)
	s.Require().NoError(err)

	sub.Close()
	s.Require().NotPanics(func() { sub.Close() })
	s.Require().NotPanics(func() { sub.Close() })
}

// TestCloseRaceWithSend — high-concurrency stress: many goroutines hammer
// SendToConsumers while another closes the subscription. The race detector
// is expected to be clean. No panics, no deadlocks.
func (s *BrokerTestSuite) TestCloseRaceWithSend() {
	sub, err := s.broker.Subscribe(StreamMessageAckKind)
	s.Require().NoError(err)

	var senders sync.WaitGroup
	stop := make(chan struct{})

	for i := 0; i < 8; i++ {
		senders.Add(1)
		go func() {
			defer senders.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = s.broker.SendToConsumers(&ControlMessage{Kind: StreamMessageAckKind})
				}
			}
		}()
	}

	// drain a few messages so the readers don't all park immediately, then
	// close while sends are in flight
	go func() {
		for i := 0; i < 50; i++ {
			select {
			case <-sub.C():
			case <-time.After(10 * time.Millisecond):
				return
			}
		}
	}()

	time.Sleep(50 * time.Millisecond) // let some sends pile up
	closed := make(chan struct{})
	go func() {
		sub.Close()
		close(closed)
	}()

	select {
	case <-closed:
	case <-time.After(regressionTimeout):
		s.Fail("Close raced with concurrent senders and deadlocked")
	}

	close(stop)
	senders.Wait()
}

// TestSlowSubscriberDoesNotBlockOthers — if one subscription stops reading,
// the broker must still deliver to other subscriptions of the same kind.
// Before the fix, sub.Send held the broker lock and one slow channel froze
// the whole broker.
//
// Delivery is concurrent (one goroutine per send per subscription), so we
// assert the *set* of messages fast received, not the order.
func (s *BrokerTestSuite) TestSlowSubscriberDoesNotBlockOthers() {
	slow, err := s.broker.Subscribe(StreamMessageAckKind)
	s.Require().NoError(err)
	defer slow.Close()

	fast, err := s.broker.Subscribe(StreamMessageAckKind)
	s.Require().NoError(err)
	defer fast.Close()

	// slow has no reader. fast does.
	for i := 0; i < 3; i++ {
		s.Require().NoError(s.broker.SendToConsumers(&ControlMessage{
			Kind:       StreamMessageAckKind,
			Attributes: map[string]interface{}{"i": i},
		}))
	}

	seen := map[int]bool{}
	for i := 0; i < 3; i++ {
		got := s.recvWithTimeout(fast)
		seen[got.Attributes["i"].(int)] = true
	}
	s.Require().Equal(map[int]bool{0: true, 1: true, 2: true}, seen,
		"fast subscriber should receive every send despite slow being stalled")
}

// TestSubscribeWhileSendInFlight — Subscribe and SendToConsumers must not
// deadlock against each other (both acquire broker lock; lock holders must
// never block on channel I/O while holding the lock).
func (s *BrokerTestSuite) TestSubscribeWhileSendInFlight() {
	stalled, err := s.broker.Subscribe(StreamMessageAckKind)
	s.Require().NoError(err)
	defer stalled.Close()

	// stall a delivery on `stalled`
	s.Require().NoError(s.broker.SendToConsumers(&ControlMessage{Kind: StreamMessageAckKind}))

	done := make(chan struct{})
	go func() {
		// must not block on the stalled delivery
		_, err := s.broker.Subscribe(StreamMessageAckKind)
		s.Require().NoError(err)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(regressionTimeout):
		s.Fail("Subscribe blocked on broker lock held by stalled SendToConsumers — NUC-765 regression")
	}
}

// TestNoSubscribersIsNoop — SendToConsumers with no subscribers must succeed
// and be a no-op.
func (s *BrokerTestSuite) TestNoSubscribersIsNoop() {
	s.Require().NoError(s.broker.SendToConsumers(&ControlMessage{Kind: StreamMessageAckKind}))
}

// TestCrossKindIsolation — the original NUC-765 deadlock froze every
// Subscribe/Unsubscribe/SendToConsumers across *all* message kinds, because a
// single mutex was held while broadcasting. After the fix, a stalled subscriber
// on kind A must not block subscribe or send on kind B.
func (s *BrokerTestSuite) TestCrossKindIsolation() {

	// stuck reader on StreamMessageAckKind
	stalled, err := s.broker.Subscribe(StreamMessageAckKind)
	s.Require().NoError(err)
	defer stalled.Close()

	// fire a send that will park indefinitely waiting for a reader of `stalled`
	s.Require().NoError(s.broker.SendToConsumers(&ControlMessage{Kind: StreamMessageAckKind}))

	// while that send goroutine is parked, a different-kind subscribe must
	// succeed promptly
	done := make(chan struct {
		sub Subscription
		err error
	})
	go func() {
		sub, err := s.broker.Subscribe(LogMessageKind)
		done <- struct {
			sub Subscription
			err error
		}{sub, err}
	}()

	var logSub Subscription
	select {
	case result := <-done:
		s.Require().NoError(result.err)
		logSub = result.sub
	case <-time.After(regressionTimeout):
		s.Fail("Subscribe(LogMessageKind) blocked while a StreamMessageAck delivery was parked — broker mutex held across kinds (NUC-765 regression)")
	}
	defer logSub.Close()

	// and a send on the unrelated kind must reach its subscriber promptly
	logMsg := &ControlMessage{Kind: LogMessageKind, Attributes: map[string]interface{}{"x": 1}}
	s.Require().NoError(s.broker.SendToConsumers(logMsg))
	s.Require().Equal(logMsg, s.recvWithTimeout(logSub))
}

// recvWithTimeout receives one message from the subscription or fails the test
// after regressionTimeout.
func (s *BrokerTestSuite) recvWithTimeout(sub Subscription) *ControlMessage {
	s.T().Helper()
	select {
	case msg, ok := <-sub.C():
		s.Require().True(ok, "subscription closed unexpectedly")
		return msg
	case <-time.After(regressionTimeout):
		s.Fail("timed out waiting for message")
		return nil
	}
}

type MergedSubscriptionTestSuite struct {
	suite.Suite
}

// TestMergeAggregatesFromAllSubs — every message sent through any underlying
// subscription must arrive on the merged channel.
func (s *MergedSubscriptionTestSuite) TestMergeAggregatesFromAllSubs() {
	brokerA := NewControlMessageBrokerBase()
	brokerB := NewControlMessageBrokerBase()

	subA, err := brokerA.Subscribe(StreamMessageAckKind)
	s.Require().NoError(err)
	subB, err := brokerB.Subscribe(StreamMessageAckKind)
	s.Require().NoError(err)

	merged := MergeSubscriptions([]Subscription{subA, subB})
	defer merged.Close()

	msgA := &ControlMessage{Kind: StreamMessageAckKind, Attributes: map[string]interface{}{"from": "A"}}
	msgB := &ControlMessage{Kind: StreamMessageAckKind, Attributes: map[string]interface{}{"from": "B"}}

	s.Require().NoError(brokerA.SendToConsumers(msgA))
	s.Require().NoError(brokerB.SendToConsumers(msgB))

	seen := map[string]bool{}
	for i := 0; i < 2; i++ {
		select {
		case got := <-merged.C():
			seen[got.Attributes["from"].(string)] = true
		case <-time.After(regressionTimeout):
			s.Fail("timed out aggregating messages")
		}
	}
	s.Require().True(seen["A"])
	s.Require().True(seen["B"])
}

// TestMergeEmpty — MergeSubscriptions([]) must return an already-closed
// Subscription so that range loops on C() exit immediately. The alternative —
// an open never-delivering channel — would deadlock callers like
// explicitAckHandler that do `for msg := range sub.C()`.
func (s *MergedSubscriptionTestSuite) TestMergeEmpty() {
	merged := MergeSubscriptions(nil)
	s.Require().NotNil(merged)

	select {
	case _, ok := <-merged.C():
		s.Require().False(ok, "empty merge must return a pre-closed channel")
	case <-time.After(regressionTimeout):
		s.Fail("empty merge returned a channel that never closes — callers would block forever")
	}

	// Close on an already-closed merge must be a no-op
	s.Require().NotPanics(func() { merged.Close() })
}

// TestSingleSubPassthrough — MergeSubscriptions([sub]) should just return the
// sub itself to avoid an unnecessary forwarder goroutine.
func (s *MergedSubscriptionTestSuite) TestSingleSubPassthrough() {
	broker := NewControlMessageBrokerBase()
	sub, err := broker.Subscribe(StreamMessageAckKind)
	s.Require().NoError(err)

	merged := MergeSubscriptions([]Subscription{sub})
	s.Require().Same(sub, merged, "single-sub merge should be a passthrough")
	merged.Close()
}

// TestCloseClosesAllUnderlying — closing the merged subscription must close
// every underlying subscription too (no leaked channels or goroutines).
func (s *MergedSubscriptionTestSuite) TestCloseClosesAllUnderlying() {
	broker := NewControlMessageBrokerBase()

	const n = 4
	subs := make([]Subscription, 0, n)
	for i := 0; i < n; i++ {
		sub, err := broker.Subscribe(StreamMessageAckKind)
		s.Require().NoError(err)
		subs = append(subs, sub)
	}

	merged := MergeSubscriptions(subs)
	merged.Close()

	// every underlying sub's channel is closed
	for i, sub := range subs {
		select {
		case _, ok := <-sub.C():
			s.Require().Falsef(ok, "underlying sub %d channel should be closed", i)
		case <-time.After(regressionTimeout):
			s.Failf("timed out", "underlying sub %d channel did not close", i)
		}
	}

	// merged channel is closed too
	select {
	case _, ok := <-merged.C():
		s.Require().False(ok)
	case <-time.After(regressionTimeout):
		s.Fail("merged channel did not close")
	}
}

// TestCloseUnblocksAggregatedSendWithoutReader — the NUC-765 race at the
// trigger layer: messages arrive at the underlying sub while no one is
// reading from the merged channel (e.g. ctx.Done in Drain() just fired).
// Close must drain the in-flight messages and not deadlock.
func (s *MergedSubscriptionTestSuite) TestCloseUnblocksAggregatedSendWithoutReader() {
	broker := NewControlMessageBrokerBase()
	sub, err := broker.Subscribe(StreamMessageAckKind)
	s.Require().NoError(err)

	merged := MergeSubscriptions([]Subscription{sub, mustSubscribe(s.T(), broker)})

	// fire several messages while no one reads merged.C()
	for i := 0; i < 5; i++ {
		s.Require().NoError(broker.SendToConsumers(&ControlMessage{Kind: StreamMessageAckKind}))
	}

	closed := make(chan struct{})
	go func() {
		merged.Close()
		close(closed)
	}()

	select {
	case <-closed:
	case <-time.After(regressionTimeout):
		s.Fail("merged Close() deadlocked when reader had stopped")
	}
}

// TestConcurrentCloseAndSends — race detector regression. Many concurrent
// sends and a Close in the middle must not panic, deadlock, or trip -race.
func (s *MergedSubscriptionTestSuite) TestConcurrentCloseAndSends() {
	broker := NewControlMessageBrokerBase()

	const n = 4
	subs := make([]Subscription, 0, n)
	for i := 0; i < n; i++ {
		subs = append(subs, mustSubscribe(s.T(), broker))
	}
	merged := MergeSubscriptions(subs)

	var senders sync.WaitGroup
	stop := make(chan struct{})
	var sent int64

	for i := 0; i < 8; i++ {
		senders.Add(1)
		go func() {
			defer senders.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = broker.SendToConsumers(&ControlMessage{Kind: StreamMessageAckKind})
					atomic.AddInt64(&sent, 1)
				}
			}
		}()
	}

	// reader that may stop at any point
	go func() {
		for {
			select {
			case _, ok := <-merged.C():
				if !ok {
					return
				}
			case <-time.After(100 * time.Millisecond):
				return
			}
		}
	}()

	time.Sleep(50 * time.Millisecond)

	closed := make(chan struct{})
	go func() {
		merged.Close()
		close(closed)
	}()

	select {
	case <-closed:
	case <-time.After(regressionTimeout):
		s.Fail("merged Close() deadlocked under concurrent senders")
	}

	close(stop)
	senders.Wait()
}

func mustSubscribe(t *testing.T, broker *ControlMessageBrokerBase) Subscription {
	t.Helper()
	sub, err := broker.Subscribe(StreamMessageAckKind)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	return sub
}

func TestBrokerSuite(t *testing.T) {
	suite.Run(t, new(BrokerTestSuite))
}

func TestMergedSubscriptionSuite(t *testing.T) {
	suite.Run(t, new(MergedSubscriptionTestSuite))
}
