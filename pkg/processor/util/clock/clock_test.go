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

package clock

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type ClockSuite struct {
	suite.Suite
}

func (suite *ClockSuite) TestClock() {
	// Now() lags real time by up to one tick interval (resolution) plus however late the
	// scheduler wakes the tick goroutine. resolution is set well above that scheduler
	// jitter - which is large under the race detector (NUC-728) - so the deterministic
	// one-tick lag dominates the bound. maxDiff is that one-tick lag plus generous headroom
	// for jitter: it does not tightly bound accuracy, but it tolerates a healthy clock under
	// load while still failing if the tick goroutine stalls (the lag then grows past maxDiff
	// within a couple of iterations).
	resolution := 50 * time.Millisecond
	jitterHeadroom := 2 * resolution
	maxDiff := resolution + jitterHeadroom

	c := New(resolution)
	for i := 0; i < 10; i++ {
		diff := time.Since(*c.Now())
		if diff < 0 {
			diff = -diff
		}
		suite.Truef(diff <= maxDiff, "Time difference too big: %v > %v", diff, maxDiff)
		time.Sleep(3 * resolution)
	}
}

func TestClock(t *testing.T) {
	suite.Run(t, &ClockSuite{})
}
