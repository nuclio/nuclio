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

package abstract

import (
	"encoding/json"
	"time"

	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
)

type LogStream struct {
	bufferLogger *nucliozap.BufferLogger
	muxLogger    *nucliozap.MuxLogger
}

// NewLogStream returns a new log stream
func NewLogStream(name string, level nucliozap.Level, loggers ...logger.Logger) (*LogStream, error) {
	var err error

	newLogStream := LogStream{}

	// create a buffer logger
	newLogStream.bufferLogger, err = nucliozap.NewBufferLogger(name, "json", level)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create mux logger")
	}

	loggers = append(loggers, newLogStream.bufferLogger.Logger)

	// wrap a mux logger
	newLogStream.muxLogger, err = nucliozap.NewMuxLogger(loggers...)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create mux logger")
	}

	return &newLogStream, nil
}

// GetLogger returns the underlying logger
func (ls *LogStream) GetLogger() logger.Logger {
	return ls.muxLogger
}

func (ls *LogStream) ReadLogs(timeout *time.Duration, logs *[]map[string]interface{}) {
	deadline := time.Now()
	if timeout != nil {
		deadline = deadline.Add(*timeout)
	}

	// since the logs stream in, we can never know if they make for valid JSON. we can try until it works or unti
	// the deadline passes. if timeout is nil, we only try once
	for retryIndex := 0; true; retryIndex++ {

		// remove the last comma from the string
		marshalledLogs := ls.bufferLogger.Buffer.String()

		// if something went wrong and there are no logs, do nothing
		if len(marshalledLogs) != 0 {

			marshalledLogs = "[" + marshalledLogs[:len(marshalledLogs)-1] + "]"

			// try to unmarshal the json
			err := json.Unmarshal([]byte(marshalledLogs), logs)

			// if we got valid json we're done
			if err == nil {
				return
			}
		}

		// if we we're passed the deadline, we're done
		if time.Now().After(deadline) {
			return
		}

		// wait a bit and retry
		time.Sleep(time.Duration(25*retryIndex) * time.Millisecond)
	}
}
