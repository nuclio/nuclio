// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"log"
	"time"

	"golang.org/x/build/cmd/coordinator/metrics"

	"github.com/golang/protobuf/ptypes"
	metpb "google.golang.org/genproto/googleapis/api/metric"
	monpb "google.golang.org/genproto/googleapis/monitoring/v3"
)

// reportMetrics gathers and reports buildlet metrics to Stackdriver.
// It currently only reports count of running reverse buildlets per type.
func reportMetrics(ctx context.Context) {
	for {
		err := reportReverseCountMetrics(ctx)
		if err != nil {
			log.Printf("error reporting %q metrics: %v\n",
				metrics.ReverseCount.Name, err)
		}

		time.Sleep(5 * time.Minute)
	}

}

func reportReverseCountMetrics(ctx context.Context) error {
	m := metrics.ReverseCount
	// 1. Gather # buildlets up per reverse builder type
	totals := reversePool.hostTypeCount()
	// 2. Write counts to Stackdriver
	ts := []*monpb.TimeSeries{}
	now := ptypes.TimestampNow()
	for hostType, n := range totals {
		labels, err := m.Labels(hostType)
		if err != nil {
			return err
		}
		tv, err := m.TypedValue(n)
		if err != nil {
			return err
		}
		ts = append(ts, &monpb.TimeSeries{
			Metric: &metpb.Metric{
				Type:   m.Descriptor.Type,
				Labels: labels,
			},
			Points: []*monpb.Point{
				{
					Interval: &monpb.TimeInterval{
						EndTime: now,
					},
					Value: tv,
				},
			},
		})
	}

	return metricsClient.CreateTimeSeries(ctx, &monpb.CreateTimeSeriesRequest{
		Name:       m.DescriptorPath(buildEnv.ProjectName),
		TimeSeries: ts,
	})
}
