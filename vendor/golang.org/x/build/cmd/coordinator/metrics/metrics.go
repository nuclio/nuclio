// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package metrics enumerates the set of Stackdriver metrics
// used by the Go build system.
package metrics

import (
	"errors"
	"fmt"

	monapi "cloud.google.com/go/monitoring/apiv3"
	"google.golang.org/genproto/googleapis/api/label"
	metpb "google.golang.org/genproto/googleapis/api/metric"
	monpb "google.golang.org/genproto/googleapis/monitoring/v3"
)

// Metric defines a custom metric type used by Go build system.
type Metric struct {
	Name       string
	Descriptor *metpb.MetricDescriptor
}

// ReverseCount is the Stackdriver metric for monitoring
// the number of reverse buildlets up at any given moment.
var ReverseCount = &Metric{
	Name: "reverse/count",
	Descriptor: &metpb.MetricDescriptor{
		Type: "custom.googleapis.com/reverse/count",
		Labels: []*label.LabelDescriptor{
			{
				Key:       "hosttype",
				ValueType: label.LabelDescriptor_STRING,
			},
		},
		MetricKind: metpb.MetricDescriptor_GAUGE,
		ValueType:  metpb.MetricDescriptor_INT64,
	},
}

// Metrics is the set of all Stackdriver metrics being used
// to monitor the Go build system.
var Metrics = []*Metric{
	ReverseCount,
}

// DescriptorPath returns the unique path for this metric among all
// GCP resources in all projects.
// See cloud.google.com/monitoring/custom-metrics/creating-metrics
// for details.
func (m *Metric) DescriptorPath(project string) string {
	return monapi.MetricMetricDescriptorPath(project, m.Descriptor.Type)
}

// Labels populates the set of labels with the given label values.
// The labels should be passed in the same order as defined in the metric
// descriptor. All labels listed in the Descriptor must be assigned values.
func (m *Metric) Labels(labels ...string) (map[string]string, error) {
	if len(m.Descriptor.Labels) != len(labels) {
		return nil, errors.New("mismatch metric labels")
	}
	lm := make(map[string]string)
	for i, l := range m.Descriptor.Labels {
		lm[l.Key] = labels[i]
	}
	return lm, nil
}

// TypedValue returns the cooresponding *monpb.TypedValue based on
// the metric descriptor's value type.
func (m *Metric) TypedValue(v interface{}) (*monpb.TypedValue, error) {
	var tv monpb.TypedValue
	switch m.Descriptor.ValueType {
	case metpb.MetricDescriptor_BOOL:
		if vt, ok := v.(bool); ok {
			tv.Value = &monpb.TypedValue_BoolValue{
				BoolValue: vt,
			}
		} else {
			return nil, fmt.Errorf("wrong value type (%T) for BOOL", v)
		}
	case metpb.MetricDescriptor_INT64:
		if vt, ok := v.(int); ok {
			tv.Value = &monpb.TypedValue_Int64Value{
				Int64Value: int64(vt),
			}
		} else {
			return nil, fmt.Errorf("wrong value type (%T) for INT64", v)
		}
	case metpb.MetricDescriptor_DOUBLE:
		if vt, ok := v.(float64); ok {
			tv.Value = &monpb.TypedValue_DoubleValue{
				DoubleValue: float64(vt),
			}
		} else {
			return nil, fmt.Errorf("wrong value type (%T) for DOUBLE", v)
		}
	case metpb.MetricDescriptor_STRING:
		if vt, ok := v.(string); ok {
			tv.Value = &monpb.TypedValue_StringValue{
				StringValue: string(vt),
			}
		} else {
			return nil, fmt.Errorf("wrong value type (%T) for STRING", v)
		}
	case metpb.MetricDescriptor_DISTRIBUTION, metpb.MetricDescriptor_MONEY:
		return nil, errors.New("unused metric descriptor value type")
	}
	return &tv, nil
}
