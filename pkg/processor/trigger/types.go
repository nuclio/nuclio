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

package trigger

import (
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/errors"
)

type DurationConfigField struct {
	Name    string
	Value   string
	Field   *time.Duration
	Default time.Duration
}

type AnnotationConfigField struct {
	Key             string
	ValueString     *string
	ValueListString []string
	ValueInt        *int
	ValueUInt64     *uint64
	ValueBool       *bool
}

type Configuration struct {
	functionconfig.Trigger

	// the runtime configuration, for reference
	RuntimeConfiguration *runtime.Configuration

	// a unique trigger ID
	ID string
}

func NewConfiguration(id string,
	triggerConfiguration *functionconfig.Trigger,
	runtimeConfiguration *runtime.Configuration) *Configuration {

	configuration := &Configuration{
		Trigger:              *triggerConfiguration,
		RuntimeConfiguration: runtimeConfiguration,
		ID:                   id,
	}

	// set defaults
	if configuration.MaxWorkers == 0 {
		configuration.MaxWorkers = 1
	}

	return configuration
}

// PopulateConfigurationFromAnnotations allows setting configuration via annotations, for experimental settings
func (c *Configuration) PopulateConfigurationFromAnnotations(annotationConfigFields []AnnotationConfigField) error {
	var err error

	for _, annotationConfigField := range annotationConfigFields {
		annotationValue, annotationKeyExists := c.RuntimeConfiguration.Config.Meta.Annotations[annotationConfigField.Key]
		if !annotationKeyExists {
			continue
		}

		switch {
		case annotationConfigField.ValueString != nil:
			*annotationConfigField.ValueString = annotationValue
		case annotationConfigField.ValueInt != nil:
			*annotationConfigField.ValueInt, err = strconv.Atoi(annotationValue)
			if err != nil {
				return errors.Wrapf(err, "Annotation %s must be numeric", annotationConfigField.Key)
			}
		case annotationConfigField.ValueBool != nil:
			*annotationConfigField.ValueBool, err = strconv.ParseBool(annotationValue)
			if err != nil {
				return errors.Wrapf(err, "Annotation %s must represent boolean", annotationConfigField.Key)
			}
		case annotationConfigField.ValueListString != nil:
			annotationConfigField.ValueListString = strings.Split(annotationValue, ",")
		case annotationConfigField.ValueUInt64 != nil:
			*annotationConfigField.ValueUInt64, err = strconv.ParseUint(annotationValue, 10, 64)
			if err != nil {
				return errors.Wrapf(err, "Annotation %s must be positive numeric (uint64)", annotationConfigField.Key)
			}
		}
	}

	return nil
}

// ParseDurationOrDefault parses a duration string into a time.duration field. if empty, sets the field to the default
func (c *Configuration) ParseDurationOrDefault(durationConfigField *DurationConfigField) error {
	if durationConfigField.Value == "" {
		*durationConfigField.Field = durationConfigField.Default
		return nil
	}

	parsedDurationValue, err := time.ParseDuration(durationConfigField.Value)
	if err != nil {
		return errors.Wrapf(err, "Failed to parse %s", durationConfigField.Name)
	}

	*durationConfigField.Field = parsedDurationValue

	return nil
}

type Statistics struct {
	EventsHandledSuccessTotal uint64
	EventsHandledFailureTotal uint64
	WorkerAllocatorStatistics worker.AllocatorStatistics
}

func (s *Statistics) DiffFrom(prev *Statistics) Statistics {
	workerAllocatorStatisticsDiff := s.WorkerAllocatorStatistics.DiffFrom(&prev.WorkerAllocatorStatistics)

	// atomically load the counters
	currEventsHandledSuccessTotal := atomic.LoadUint64(&s.EventsHandledSuccessTotal)
	currEventsHandledFailureTotal := atomic.LoadUint64(&s.EventsHandledFailureTotal)

	prevEventsHandledSuccessTotal := atomic.LoadUint64(&prev.EventsHandledSuccessTotal)
	prevEventsHandledFailureTotal := atomic.LoadUint64(&prev.EventsHandledFailureTotal)

	return Statistics{
		EventsHandledSuccessTotal: currEventsHandledSuccessTotal - prevEventsHandledSuccessTotal,
		EventsHandledFailureTotal: currEventsHandledFailureTotal - prevEventsHandledFailureTotal,
		WorkerAllocatorStatistics: workerAllocatorStatisticsDiff,
	}
}

type Secret struct {
	Contents string
}
