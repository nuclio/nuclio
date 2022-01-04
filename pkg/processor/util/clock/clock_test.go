//go:build test_unit

/*
Copyright 2017 The Nuclio Authors.

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
	resolution := 7 * time.Millisecond
	c := New(resolution)
	maxDiff := 2 * resolution
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
