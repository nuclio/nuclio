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
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	kclConfig := NewKinesisClientLibConfig("appName", "StreamName", "us-west-2", "workerId").
		WithFailoverTimeMillis(500).
		WithMaxRecords(100).
		WithInitialPositionInStream(TRIM_HORIZON).
		WithIdleTimeBetweenReadsInMillis(20).
		WithCallProcessRecordsEvenForEmptyRecordList(true).
		WithTaskBackoffTimeMillis(10).
		WithMetricsBufferTimeMillis(500).
		WithMetricsMaxQueueSize(200)

	assert.Equal(t, "appName", kclConfig.ApplicationName)
	assert.Equal(t, 500, kclConfig.FailoverTimeMillis)
}
