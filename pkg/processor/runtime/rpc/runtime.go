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

package rpc

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/nuclio/nuclio-sdk"
	"github.com/rs/xid"
)

// TODO: Find a better place (both on file system and configuration)
const (
	socketPathTemplate = "/tmp/nuclio-rpc-%s.sock"
	connectionTimeout  = 10 * time.Second
	eventTimeout       = 5 * time.Minute
)

type result struct {
	StatusCode   int                    `json:"status_code"`
	ContentType  string                 `json:"content_type"`
	Body         string                 `json:"body"`
	BodyEncoding string                 `json:"body_encoding"`
	Headers      map[string]interface{} `json:"headers"`

	DecodedBody []byte
	err         error
}

// Runtime is a runtime that communicates via unix domain socket
type Runtime struct {
	runtime.AbstractRuntime
	configuration *runtime.Configuration
	eventEncoder  *EventJSONEncoder
	outReader     *bufio.Reader
	socketPath    string
}

type rpcLogRecord struct {
	DateTime string                 `json:"datetime"`
	Level    string                 `json:"level"`
	Message  string                 `json:"message"`
	With     map[string]interface{} `json:"with"`
}

// NewRPCRuntime returns a new RPC runtime
func NewRPCRuntime(logger nuclio.Logger, configuration *runtime.Configuration, runWrapper func(string) error) (*Runtime, error) {
	var err error

	abstractRuntime, err := runtime.NewAbstractRuntime(logger, configuration)
	if err != nil {
		return nil, errors.Wrap(err, "Can't create AbstractRuntime")
	}

	newRuntime := &Runtime{
		AbstractRuntime: *abstractRuntime,
		configuration:   configuration,
	}

	// create socket path
	newRuntime.socketPath = newRuntime.createSocketPath()

	listener, err := newRuntime.createListener()
	if err != nil {
		return nil, errors.Wrapf(err, "Can't listen on %q", newRuntime.socketPath)
	}

	if err = runWrapper(newRuntime.socketPath); err != nil {
		return nil, errors.Wrap(err, "Can't run wrapper")
	}

	unixListener, ok := listener.(*net.UnixListener)
	if !ok {
		return nil, errors.Wrap(err, "Can't get underlying Unix listener")
	}
	if err = unixListener.SetDeadline(time.Now().Add(connectionTimeout)); err != nil {
		return nil, errors.Wrap(err, "Can't set deadline")
	}
	conn, err := listener.Accept()
	if err != nil {
		return nil, errors.Wrap(err, "Can't get connection from wrapper")
	}

	newRuntime.eventEncoder = NewEventJSONEncoder(newRuntime.Logger, conn)
	newRuntime.outReader = bufio.NewReader(conn)

	return newRuntime, nil
}

func (r *Runtime) ProcessEvent(event nuclio.Event, functionLogger nuclio.Logger) (interface{}, error) {
	r.Logger.DebugWith("Processing event",
		"name", r.configuration.Meta.Name,
		"version", r.configuration.Spec.Version,
		"eventID", event.GetID())

	resultChan := make(chan *result)
	go r.handleEvent(functionLogger, event, resultChan)

	select {
	case result := <-resultChan:
		r.Logger.DebugWith("Event executed",
			"name", r.configuration.Meta.Name,
			"status", result.StatusCode,
			"eventID", event.GetID())
		return nuclio.Response{
			Body:        result.DecodedBody,
			ContentType: result.ContentType,
			Headers:     result.Headers,
			StatusCode:  result.StatusCode,
		}, nil
	case <-time.After(eventTimeout):
		return nil, fmt.Errorf("handler timeout after %s", eventTimeout)
	}
}

func (r *Runtime) createListener() (net.Listener, error) {
	if common.FileExists(r.socketPath) {
		if err := os.Remove(r.socketPath); err != nil {
			return nil, errors.Wrapf(err, "Can't remove socket at %q", r.socketPath)
		}
	}

	r.Logger.DebugWith("Creating listener socket", "path", r.socketPath)

	return net.Listen("unix", r.socketPath)
}

func (r *Runtime) createSocketPath() string {
	return fmt.Sprintf(socketPathTemplate, xid.New().String())
}

func (r *Runtime) handleEvent(functionLogger nuclio.Logger, event nuclio.Event, resultChan chan *result) {
	unmarshalledResult := &result{}

	// Send event
	if unmarshalledResult.err = r.eventEncoder.Encode(event); unmarshalledResult.err != nil {
		resultChan <- unmarshalledResult
		return
	}

	var data []byte

	// Read logs & output
	for {
		data, unmarshalledResult.err = r.outReader.ReadBytes('\n')

		if unmarshalledResult.err != nil {
			r.Logger.WarnWith("Failed to read from connection", "err", unmarshalledResult.err)

			resultChan <- unmarshalledResult
			return
		}

		switch data[0] {
		case 'r':

			// try to unmarshall the result
			if unmarshalledResult.err = json.Unmarshal(data[1:], unmarshalledResult); unmarshalledResult.err != nil {
				resultChan <- unmarshalledResult
				return
			}

			switch unmarshalledResult.BodyEncoding {
			case "text":
				unmarshalledResult.DecodedBody = []byte(unmarshalledResult.Body)
			case "base64":
				unmarshalledResult.DecodedBody, unmarshalledResult.err = base64.StdEncoding.DecodeString(unmarshalledResult.Body)
			default:
				unmarshalledResult.err = fmt.Errorf("Unknown body encoding - %q", unmarshalledResult.BodyEncoding)
			}

			// write back to result channel
			resultChan <- unmarshalledResult

			return // reply is the last message the wrapper sends

		case 'm':
			r.handleReponseMetric(functionLogger, data[1:])

		case 'l':
			r.handleResponseLog(functionLogger, data[1:])
		}
	}
}

func (r *Runtime) handleResponseLog(functionLogger nuclio.Logger, response []byte) {
	var logRecord rpcLogRecord

	if err := json.Unmarshal(response, &logRecord); err != nil {
		r.Logger.ErrorWith("Can't decode log", "error", err)
		return
	}

	logger := r.resolveFunctionLogger(functionLogger)
	logFunc := logger.DebugWith

	switch logRecord.Level {
	case "error", "critical", "fatal":
		logFunc = logger.ErrorWith
	case "warning":
		logFunc = logger.WarnWith
	case "info":
		logFunc = logger.InfoWith
	}

	vars := common.MapToSlice(logRecord.With)
	logFunc(logRecord.Message, vars...)
}

func (r *Runtime) handleReponseMetric(functionLogger nuclio.Logger, response []byte) {
	var metrics struct {
		DurationSec float64 `json:"duration"`
	}

	logger := r.resolveFunctionLogger(functionLogger)
	if err := json.Unmarshal(response, &metrics); err != nil {
		logger.ErrorWith("Can't decode metric", "error", err)
		return
	}

	if metrics.DurationSec == 0 {
		logger.ErrorWith("No duration in metrics", "metrics", metrics)
		return
	}

	r.Statistics.DurationMilliSecondsCount++
	r.Statistics.DurationMilliSecondsSum += uint64(metrics.DurationSec * 1000)
}

// resolveFunctionLogger return either functionLogger if provided or root logger if not
func (r *Runtime) resolveFunctionLogger(functionLogger nuclio.Logger) nuclio.Logger {
	if functionLogger == nil {
		return r.Logger
	}
	return functionLogger
}
