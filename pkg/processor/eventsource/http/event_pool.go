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

package http

// FixedEventPool is a pool of fixed number of events
type FixedEventPool chan *Event

// NewFixedEventPool creates a new FixedEventPool that holds numEvents events
func NewFixedEventPool(numEvents int) FixedEventPool {
	eventPool := make(chan *Event, numEvents)
	for i := 0; i < numEvents; i++ {
		eventPool <- &Event{}
	}

	return eventPool
}

// Get return a free event from the pool
func (fep FixedEventPool) Get() *Event {
	return <-fep
}

// Put returns an event to the pool
func (fep FixedEventPool) Put(event *Event) {
	fep <- event
}
