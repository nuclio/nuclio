// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package spanlog provides span and event logger interfaces that are used
// by the build coordinator infrastructure.
package spanlog

// SpanLogger is something that has the CreateSpan method, which
// creates a event spanning some duration which will eventually be
// logged and visualized.
type Logger interface {
	// CreateSpan logs the start of an event.
	// optText is 0 or 1 strings.
	CreateSpan(event string, optText ...string) Span
}

// Span is a handle that can eventually be closed.
// Typical usage:
//
//   sp := sl.CreateSpan("slow_operation")
//   result, err := doSlowOperation()
//   sp.Done(err)
//   // do something with result, err
type Span interface {
	// Done marks a span as done.
	// The err is returned unmodified for convenience at callsites.
	Done(err error) error
}

// TODO(quentin): Move loggerFunc and createSpan from coordinator.go to here.
