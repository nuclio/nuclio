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
	"sync/atomic"
	"time"
)

var (
	// DefaultClock is the default clock
	DefaultClock *Clock
)

// Clock is a low resulution clock. It uses less resources and is faster than calling
// time.Now
type Clock struct {
	// Resolution is the clock resolution
	Resolution time.Duration

	now atomic.Value
}

// New returns a new clock with desired resolution
func New(resolution time.Duration) *Clock {
	clock := &Clock{
		Resolution: resolution,
	}
	clock.now.Store(time.Now())

	go clock.tick()
	return clock
}

// Now returns the curren time
func (c *Clock) Now() time.Time {
	return c.now.Load().(time.Time)
}

// Now returns the current time from the default clock
func Now() time.Time {
	return DefaultClock.Now()
}

// SetResolution sets the default clock resolution
func SetResolution(resolution time.Duration) {
	DefaultClock.Resolution = resolution
}

func (c *Clock) tick() {
	for {
		time.Sleep(c.Resolution)
		c.now.Store(time.Now())
	}
}

func init() {
	DefaultClock = New(10 * time.Second)
}
