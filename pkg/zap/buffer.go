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

package nucliozap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
)

var ErrBufferPoolAllocationTimeout = errors.New("Timed out waiting for buffer logger")

// a logger who outputs the records to a buffer
type BufferLogger struct {
	encoding string
	Logger   *NuclioZap
	Buffer   *bytes.Buffer
}

func NewBufferLogger(name string, encoding string, level Level) (*BufferLogger, error) {
	writer := &bytes.Buffer{}

	// create a logger that is able to capture the output into a buffer. if a request arrives
	// and the user wishes to capture the log, this will be used as the logger instead of the default
	// logger
	newLogger, err := NewNuclioZap(name,
		encoding,
		writer,
		writer,
		level)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create buffer logger")
	}

	return &BufferLogger{
		Logger:   newLogger,
		Buffer:   writer,
		encoding: encoding,
	}, nil
}

func (bl *BufferLogger) GetJSONString() (string, error) {
	if bl.encoding != "json" {
		return "", fmt.Errorf("Can only return JSON when encoding is JSON, not %s", bl.encoding)
	}

	jsonBody := bl.Buffer.Bytes()
	if len(jsonBody) != 0 {

		// remove last comma
		jsonBody = jsonBody[:len(jsonBody)-1]
	}

	return "[" + string(jsonBody) + "]", nil
}

func (bl *BufferLogger) GetLogEntries() ([]map[string]interface{}, error) {
	jsonBody, err := bl.GetJSONString()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get JSON string")
	}

	unmarshalledJSONBody := []map[string]interface{}{}

	err = json.Unmarshal([]byte(jsonBody), &unmarshalledJSONBody)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal JSON body")
	}

	return unmarshalledJSONBody, nil
}

// a pool for buffer loggers
type BufferLoggerPool struct {
	bufferLoggerChan       chan *BufferLogger
	defaultAllocateTimeout time.Duration
}

// a pool of buffer loggers
func NewBufferLoggerPool(numBufferLoggers int,
	name string,
	encoding string,
	level Level) (*BufferLoggerPool, error) {

	// create a channel for the buffer loggers
	bufferLoggersChan := make(chan *BufferLogger, numBufferLoggers)

	// create buffer loggers
	for bufferLoggerIdx := 0; bufferLoggerIdx < numBufferLoggers; bufferLoggerIdx++ {
		newBufferLogger, err := NewBufferLogger(name, encoding, level)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create buffer logger")
		}

		// shove to channel
		bufferLoggersChan <- newBufferLogger
	}

	return &BufferLoggerPool{
		bufferLoggerChan:       bufferLoggersChan,
		defaultAllocateTimeout: 10 * time.Second,
	}, nil
}

func (blp *BufferLoggerPool) Allocate(timeout *time.Duration) (*BufferLogger, error) {
	if timeout == nil {
		timeout = &blp.defaultAllocateTimeout
	}

	select {
	case bufferLogger := <-blp.bufferLoggerChan:

		// clear the buffer
		bufferLogger.Buffer.Reset()

		return bufferLogger, nil
	case <-time.After(*timeout):
		return nil, ErrBufferPoolAllocationTimeout
	}
}

func (blp *BufferLoggerPool) Release(bufferLogger *BufferLogger) {
	blp.bufferLoggerChan <- bufferLogger
}
