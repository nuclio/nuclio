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

package jwtvalidator

import (
	"context"
	"testing"
	"time"

	"github.com/nuclio/errors"
	"github.com/stretchr/testify/suite"
)

type DownloadRateLimiterTestSuite struct {
	suite.Suite
}

func (suite *DownloadRateLimiterTestSuite) TestNewDownloadRateLimiter() {
	limiter := newDownloadRateLimiter(time.Second, 30*time.Second, 2)

	suite.Require().Equal(time.Second, limiter.gap)
	suite.Require().Equal(30*time.Second, limiter.backoff)
	suite.Require().True(limiter.pending.TryAcquire(2))
	suite.Require().False(limiter.pending.TryAcquire(1))
	suite.Require().True(limiter.lastDownloadAt.IsZero())
	suite.Require().True(limiter.backoffUntil.IsZero())
}

func (suite *DownloadRateLimiterTestSuite) TestAcquirePendingSlot() {
	for _, testCase := range []struct {
		name          string
		backoffUntil  time.Time
		expectedError string
	}{
		{name: "free slot"},
		{name: "backing off", backoffUntil: time.Now().Add(time.Hour), expectedError: "Key set download is backing off"},
		{name: "back-off is over", backoffUntil: time.Now().Add(-time.Second)},
	} {
		suite.Run(testCase.name, func() {
			limiter := newDownloadRateLimiter(time.Second, time.Minute, 1)
			limiter.backoffUntil = testCase.backoffUntil

			err := limiter.acquirePendingSlot(context.Background())

			if testCase.expectedError == "" {
				suite.Require().NoError(err)
				return
			}
			suite.Require().Error(err)
			suite.Require().Contains(errors.GetErrorStackString(err, -1), testCase.expectedError)
		})
	}
}

func (suite *DownloadRateLimiterTestSuite) TestBackoffDoesNotKeepASlot() {
	limiter := newDownloadRateLimiter(time.Second, time.Minute, 1)
	limiter.backoffUntil = time.Now().Add(time.Hour)

	suite.Require().Error(limiter.acquirePendingSlot(context.Background()))

	suite.Require().True(limiter.pending.TryAcquire(1))
}

func (suite *DownloadRateLimiterTestSuite) TestAcquirePendingSlotQueuesUntilASlotIsFree() {
	limiter := newDownloadRateLimiter(time.Second, time.Minute, 1)
	suite.Require().NoError(limiter.acquirePendingSlot(context.Background()))

	// the request gets its slot once the first one is released, whether it started before or after the release
	acquired := make(chan error, 1)
	go func() {
		acquired <- limiter.acquirePendingSlot(context.Background())
	}()
	limiter.releasePendingSlot()

	suite.Require().NoError(<-acquired)
}

func (suite *DownloadRateLimiterTestSuite) TestAcquirePendingSlotStopsWhenContextIsDone() {
	limiter := newDownloadRateLimiter(time.Second, time.Minute, 1)
	suite.Require().NoError(limiter.acquirePendingSlot(context.Background()))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := limiter.acquirePendingSlot(ctx)

	suite.Require().ErrorIs(err, context.Canceled)
}

func (suite *DownloadRateLimiterTestSuite) TestAcquirePendingSlotRejectsRequestQueuedDuringFailedDownload() {
	limiter := newDownloadRateLimiter(time.Second, time.Hour, 1)
	suite.Require().NoError(limiter.acquirePendingSlot(context.Background()))

	acquired := make(chan error, 1)
	go func() {
		acquired <- limiter.acquirePendingSlot(context.Background())
	}()

	// the holder's download fails, which starts the back-off before its slot is released
	limiter.finishDownload(errors.New("Failed to download"))
	limiter.releasePendingSlot()

	suite.Require().Error(<-acquired)
	suite.Require().True(limiter.pending.TryAcquire(1))
}

func (suite *DownloadRateLimiterTestSuite) TestReleasePendingSlot() {
	limiter := newDownloadRateLimiter(time.Second, time.Minute, 1)
	suite.Require().NoError(limiter.acquirePendingSlot(context.Background()))
	suite.Require().False(limiter.pending.TryAcquire(1))

	limiter.releasePendingSlot()

	suite.Require().NoError(limiter.acquirePendingSlot(context.Background()))
}

func (suite *DownloadRateLimiterTestSuite) TestDelayForDownloadGapSleepsTheRestOfTheGap() {
	gap := 100 * time.Millisecond
	limiter := newDownloadRateLimiter(gap, time.Minute, 1)
	downloadedAt := time.Now()
	limiter.lastDownloadAt = downloadedAt

	limiter.delayForDownloadGap()

	// a sleep never ends early, so this holds whatever the machine's speed
	suite.Require().GreaterOrEqual(time.Since(downloadedAt), gap)
}

func (suite *DownloadRateLimiterTestSuite) TestDelayForDownloadGapDoesNotSleepWhenGapPassed() {
	for _, testCase := range []struct {
		name           string
		lastDownloadAt time.Time
	}{
		{name: "no download yet"},
		{name: "last download long ago", lastDownloadAt: time.Now().Add(-2 * time.Hour)},
	} {
		suite.Run(testCase.name, func() {
			limiter := newDownloadRateLimiter(time.Hour, time.Minute, 1)
			limiter.lastDownloadAt = testCase.lastDownloadAt

			// a sleep of the gap would hold the test for an hour, so only the deadline below ends it
			returned := make(chan struct{})
			go func() {
				limiter.delayForDownloadGap()
				close(returned)
			}()

			select {
			case <-returned:
			case <-time.After(10 * time.Second):
				suite.Fail("The limiter slept although the gap had passed")
			}
		})
	}
}

func (suite *DownloadRateLimiterTestSuite) TestFinishDownload() {
	for _, testCase := range []struct {
		name            string
		downloadErr     error
		expectedBackoff bool
	}{
		{name: "success"},
		{name: "failure", downloadErr: errors.New("Failed to download"), expectedBackoff: true},
	} {
		suite.Run(testCase.name, func() {
			backoff := time.Minute
			limiter := newDownloadRateLimiter(time.Second, backoff, 1)
			startedAt := time.Now()

			limiter.finishDownload(testCase.downloadErr)

			suite.Require().False(limiter.lastDownloadAt.Before(startedAt))
			if !testCase.expectedBackoff {
				suite.Require().True(limiter.backoffUntil.IsZero())
				return
			}
			suite.Require().Equal(limiter.lastDownloadAt.Add(backoff), limiter.backoffUntil)
		})
	}
}

func (suite *DownloadRateLimiterTestSuite) TestFinishDownloadStartsBackoffThatRejectsRequests() {
	limiter := newDownloadRateLimiter(time.Second, time.Hour, 1)

	limiter.finishDownload(errors.New("Failed to download"))

	suite.Require().Error(limiter.acquirePendingSlot(context.Background()))
}

func (suite *DownloadRateLimiterTestSuite) TestReset() {
	limiter := newDownloadRateLimiter(time.Second, time.Hour, 1)
	limiter.finishDownload(errors.New("Failed to download"))
	suite.Require().Error(limiter.acquirePendingSlot(context.Background()))

	limiter.reset()

	suite.Require().True(limiter.lastDownloadAt.IsZero())
	suite.Require().True(limiter.backoffUntil.IsZero())
	suite.Require().NoError(limiter.acquirePendingSlot(context.Background()))
}

func TestDownloadRateLimiterTestSuite(t *testing.T) {
	suite.Run(t, new(DownloadRateLimiterTestSuite))
}
