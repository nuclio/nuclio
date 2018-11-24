// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metrics_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"golang.org/x/build/cmd/coordinator/metrics"
	"google.golang.org/genproto/googleapis/api/label"
	metpb "google.golang.org/genproto/googleapis/api/metric"
)

func TestMetricDescriptorPath(t *testing.T) {
	project := "my-project"
	m := &metrics.Metric{
		Name: "parent/child",
		Descriptor: &metpb.MetricDescriptor{
			Type: "custom.googleapis.com/parent/child",
		},
	}

	got := m.DescriptorPath(project)
	wantType := "parent/child"
	wantProject := project

	if !strings.HasSuffix(got, wantType) {
		t.Fatalf("expected type to be url-encoded.	got: %s,	want: %s", got, wantType)
	}
	if !strings.Contains(got, wantProject) {
		t.Fatalf("expected project encoding not to change.	got: %s	want: %s", got, wantProject)
	}
}

func TestMetricLabels(t *testing.T) {
	m := &metrics.Metric{
		Name: "parent/child",
		Descriptor: &metpb.MetricDescriptor{
			Labels: []*label.LabelDescriptor{
				{
					Key:       "label1",
					ValueType: label.LabelDescriptor_STRING,
				},
			},
		},
	}

	if _, err := m.Labels("v1"); err != nil {
		t.Errorf("unexpected error got %v want nil", err)
	}

	if lm, err := m.Labels("v1", "v2"); err == nil {
		t.Errorf("expected error got nil, labels populated into %v", lm)
	}
}

func TestMetricTypedValue(t *testing.T) {
	cases := []struct {
		name string
		m    *metrics.Metric
		v    interface{}
		e    error
	}{
		{
			name: "Wrong BOOL value",
			m: &metrics.Metric{
				Descriptor: &metpb.MetricDescriptor{
					ValueType: metpb.MetricDescriptor_BOOL,
				},
			},
			v: 1,
			e: fmt.Errorf("wrong value type (int) for BOOL"),
		},
		{
			name: "Correct BOOL value",
			m: &metrics.Metric{
				Descriptor: &metpb.MetricDescriptor{
					ValueType: metpb.MetricDescriptor_BOOL,
				},
			},
			v: true,
			e: nil,
		},
		{
			name: "Wrong INT64 value",
			m: &metrics.Metric{
				Descriptor: &metpb.MetricDescriptor{
					ValueType: metpb.MetricDescriptor_INT64,
				},
			},
			v: int32(1),
			e: fmt.Errorf("wrong value type (int32) for INT64"),
		},
		{
			name: "Correct INT64 value",
			m: &metrics.Metric{
				Descriptor: &metpb.MetricDescriptor{
					ValueType: metpb.MetricDescriptor_INT64,
				},
			},
			v: 1,
			e: nil,
		},
		{
			name: "Wrong DOUBLE value",
			m: &metrics.Metric{
				Descriptor: &metpb.MetricDescriptor{
					ValueType: metpb.MetricDescriptor_DOUBLE,
				},
			},
			v: float32(3.14),
			e: fmt.Errorf("wrong value type (float32) for DOUBLE"),
		},
		{
			name: "Correct DOUBLE value",
			m: &metrics.Metric{
				Descriptor: &metpb.MetricDescriptor{
					ValueType: metpb.MetricDescriptor_DOUBLE,
				},
			},
			v: 3.14,
			e: nil,
		},
		{
			name: "Wrong STRING value",
			m: &metrics.Metric{
				Descriptor: &metpb.MetricDescriptor{
					ValueType: metpb.MetricDescriptor_STRING,
				},
			},
			v: false,
			e: fmt.Errorf("wrong value type (bool) for STRING"),
		},
		{
			name: "Correct STRING value",
			m: &metrics.Metric{
				Descriptor: &metpb.MetricDescriptor{
					ValueType: metpb.MetricDescriptor_STRING,
				},
			},
			v: "false",
			e: nil,
		},
		{
			name: "unused type",
			m: &metrics.Metric{
				Descriptor: &metpb.MetricDescriptor{
					ValueType: metpb.MetricDescriptor_DISTRIBUTION,
				},
			},
			v: false,
			e: errors.New("unused metric descriptor value type"),
		},
	}

	for _, tc := range cases {
		_, err := tc.m.TypedValue(tc.v)
		if tc.e == nil && err == nil {
			continue
		}
		if tc.e == nil && err != nil {
			t.Errorf("Test case %s: unexpected error got %v, want nil", tc.name, err)
		} else if tc.e != nil && err == nil {
			t.Errorf("Test case %s: expected error got nil, want %v", tc.name, tc.e)
		} else if err.Error() != tc.e.Error() {
			t.Errorf("Test case %s: wrong error got %v, want error %v", tc.name, err, tc.e)
		}
	}
}
