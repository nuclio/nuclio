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

package metricsink

import "github.com/nuclio/logger"

type MetricSink interface {

	// Start starts processing metrics
	Start() error

	// Stop stops processing metrics, returns a channel that is closed when the sink actually stops
	Stop() chan struct{}

	// GetKind returns the kind of metric sink
	GetKind() string

	// GetName returns the name of metric sink
	GetName() string
}

// AbstractMetricSink is the base struct for all metric sinks
type AbstractMetricSink struct {
	Logger         logger.Logger
	Name           string
	Kind           string
	MetricProvider MetricProvider
	StopChannel    chan struct{}
	StoppedChannel chan struct{}
}

// NewAbstractMetricSink creates a new abstract metric sink
func NewAbstractMetricSink(logger logger.Logger,
	kind string,
	name string,
	metricProvider MetricProvider) (*AbstractMetricSink, error) {
	return &AbstractMetricSink{
		Logger:         logger,
		Kind:           kind,
		Name:           name,
		MetricProvider: metricProvider,
		StopChannel:    make(chan struct{}),
		StoppedChannel: make(chan struct{}),
	}, nil
}

// GetKind returns the kind of metric sink
func (at *AbstractMetricSink) GetKind() string {
	return at.Kind
}

// GetName returns the name of metric sink
func (at *AbstractMetricSink) GetName() string {
	return at.Name
}

// Start starts processing metrics
func (at *AbstractMetricSink) Start() error {
	return nil
}

// Stop stops processing metrics
func (at *AbstractMetricSink) Stop() chan struct{} {

	// closing the channel will break the loop
	close(at.StopChannel)

	// return the channel that indicates when we stopped
	return at.StoppedChannel
}
