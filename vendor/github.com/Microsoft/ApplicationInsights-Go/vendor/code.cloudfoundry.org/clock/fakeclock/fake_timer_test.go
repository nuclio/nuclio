package fakeclock_test

import (
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("FakeTimer", func() {
	const Δ = 10 * time.Millisecond

	var (
		fakeClock   *fakeclock.FakeClock
		initialTime time.Time
	)

	BeforeEach(func() {
		initialTime = time.Date(2014, 1, 1, 3, 0, 30, 0, time.UTC)
		fakeClock = fakeclock.NewFakeClock(initialTime)
	})

	It("proivdes a channel that receives after the given interval has elapsed", func() {
		timer := fakeClock.NewTimer(10 * time.Second)
		timeChan := timer.C()
		Consistently(timeChan, Δ).ShouldNot(Receive())

		fakeClock.Increment(5 * time.Second)
		Consistently(timeChan, Δ).ShouldNot(Receive())

		fakeClock.Increment(4 * time.Second)
		Consistently(timeChan, Δ).ShouldNot(Receive())

		fakeClock.Increment(1 * time.Second)
		Eventually(timeChan).Should(Receive(Equal(initialTime.Add(10 * time.Second))))

		fakeClock.Increment(10 * time.Second)
		Consistently(timeChan, Δ).ShouldNot(Receive())
	})

	Describe("Stop", func() {
		It("is idempotent", func() {
			timer := fakeClock.NewTimer(time.Second)
			timer.Stop()
			timer.Stop()
			fakeClock.Increment(time.Second)
			Consistently(timer.C()).ShouldNot(Receive())
		})
	})

	Describe("WaitForWatcherAndIncrement", func() {
		It("consistently fires timers that start asynchronously", func() {
			received := make(chan time.Time)

			stop := make(chan struct{})
			defer close(stop)

			duration := 10 * time.Second

			go func() {
				for {
					timer := fakeClock.NewTimer(duration)

					select {
					case ticked := <-timer.C():
						received <- ticked
					case <-stop:
						return
					}
				}
			}()

			for i := 0; i < 100; i++ {
				fakeClock.WaitForWatcherAndIncrement(duration)
				Expect((<-received).Sub(initialTime)).To(Equal(duration * time.Duration(i+1)))
			}
		})

		It("consistently fires timers that reset asynchronously", func() {
			received := make(chan time.Time, 1)

			stop := make(chan struct{})
			defer close(stop)

			duration := 10 * time.Second

			timer := fakeClock.NewTimer(duration)

			go func() {
				for {
					select {
					case ticked := <-timer.C():
						received <- ticked
						timer.Reset(duration)
					case <-stop:
						return
					}
				}
			}()

			incrementClock := make(chan struct{})

			go func() {
				for {
					<-incrementClock
					fakeClock.WaitForWatcherAndIncrement(duration)
				}
			}()

			for i := 0; i < 100; i++ {
				Eventually(incrementClock).Should(BeSent(struct{}{}))
				var timestamp time.Time
				Eventually(received, 5*time.Second).Should(Receive(&timestamp))
			}
		})
	})
})
