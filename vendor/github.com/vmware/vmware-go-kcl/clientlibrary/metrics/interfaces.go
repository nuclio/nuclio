/*
 * Copyright (c) 2018 VMware, Inc.
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy of this software and
 * associated documentation files (the "Software"), to deal in the Software without restriction, including
 * without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is furnished to do
 * so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all copies or substantial
 * portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT
 * NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
 * IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY,
 * WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE
 * SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */
// The implementation is derived from https://github.com/patrobinson/gokini
//
// Copyright 2018 Patrick robinson
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
package metrics

import (
	"fmt"
)

// MonitoringConfiguration allows you to configure how record processing metrics are exposed
type MonitoringConfiguration struct {
	MonitoringService string // Type of monitoring to expose. Supported types are "prometheus"
	Region            string
	Prometheus        PrometheusMonitoringService
	CloudWatch        CloudWatchMonitoringService
	service           MonitoringService
}

type MonitoringService interface {
	Init() error
	Start() error
	IncrRecordsProcessed(string, int)
	IncrBytesProcessed(string, int64)
	MillisBehindLatest(string, float64)
	LeaseGained(string)
	LeaseLost(string)
	LeaseRenewed(string)
	RecordGetRecordsTime(string, float64)
	RecordProcessRecordsTime(string, float64)
	Shutdown()
}

func (m *MonitoringConfiguration) Init(nameSpace, streamName string, workerID string) error {
	if m.MonitoringService == "" {
		m.service = &noopMonitoringService{}
		return nil
	}

	switch m.MonitoringService {
	case "prometheus":
		m.Prometheus.Namespace = nameSpace
		m.Prometheus.KinesisStream = streamName
		m.Prometheus.WorkerID = workerID
		m.Prometheus.Region = m.Region
		m.service = &m.Prometheus
	case "cloudwatch":
		m.CloudWatch.Namespace = nameSpace
		m.CloudWatch.KinesisStream = streamName
		m.CloudWatch.WorkerID = workerID
		m.CloudWatch.Region = m.Region
		m.service = &m.CloudWatch
	default:
		return fmt.Errorf("Invalid monitoring service type %s", m.MonitoringService)
	}
	return m.service.Init()
}

func (m *MonitoringConfiguration) GetMonitoringService() MonitoringService {
	return m.service
}

type noopMonitoringService struct{}

func (n *noopMonitoringService) Init() error  { return nil }
func (n *noopMonitoringService) Start() error { return nil }
func (n *noopMonitoringService) Shutdown()    {}

func (n *noopMonitoringService) IncrRecordsProcessed(shard string, count int)         {}
func (n *noopMonitoringService) IncrBytesProcessed(shard string, count int64)         {}
func (n *noopMonitoringService) MillisBehindLatest(shard string, millSeconds float64) {}
func (n *noopMonitoringService) LeaseGained(shard string)                             {}
func (n *noopMonitoringService) LeaseLost(shard string)                               {}
func (n *noopMonitoringService) LeaseRenewed(shard string)                            {}
func (n *noopMonitoringService) RecordGetRecordsTime(shard string, time float64)      {}
func (n *noopMonitoringService) RecordProcessRecordsTime(shard string, time float64)  {}
