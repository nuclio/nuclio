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
	"github.com/nuclio/nuclio/pkg/processor/status"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/rs/xid"
)

// TODO: Find a better place (both on file system and configuration)
const (
	socketPathTemplate = "/tmp/nuclio-rpc-%s.sock"
	connectionTimeout  = 2 * time.Minute
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
type AbstractRuntime struct {
	runtime.AbstractRuntime
	configuration  *runtime.Configuration
	eventEncoder   *EventJSONEncoder
	outReader      *bufio.Reader
	wrapperProcess *os.Process
	resultChan     chan *result
	functionLogger logger.Logger
	runtime        Runtime
	startChan      chan struct{}
}

type rpcLogRecord struct {
	DateTime string                 `json:"datetime"`
	Level    string                 `json:"level"`
	Message  string                 `json:"message"`
	With     map[string]interface{} `json:"with"`
}

// NewRPCRuntime returns a new RPC runtime
func NewAbstractRuntime(logger logger.Logger,
	configuration *runtime.Configuration,
	runtimeInstance Runtime) (*AbstractRuntime, error) {
	var err error

	abstractRuntime, err := runtime.NewAbstractRuntime(logger, configuration)
	if err != nil {
		return nil, errors.Wrap(err, "Can't create AbstractRuntime")
	}

	newRuntime := &AbstractRuntime{
		AbstractRuntime: *abstractRuntime,
		configuration:   configuration,
		runtime:         runtimeInstance,
		startChan:       make(chan struct{}, 1),
	}

	if err := newRuntime.runWrapper(); err != nil {
		newRuntime.SetStatus(status.Error)
		return nil, errors.Wrap(err, "Failed to run wrapper")
	}

	newRuntime.SetStatus(status.Ready)

	return newRuntime, nil
}

// ProcessEvent processes an event
func (r *AbstractRuntime) ProcessEvent(event nuclio.Event, functionLogger logger.Logger) (interface{}, error) {
	// TODO: Check that status is Ready?
	r.Logger.DebugWith("Processing event",
		"name", r.configuration.Meta.Name,
		"version", r.configuration.Spec.Version,
		"eventID", event.GetID())

	if currentStatus := r.GetStatus(); currentStatus != status.Ready {
		return nil, errors.Errorf("Processor not ready (current status: %s)", currentStatus)
	}

	r.functionLogger = functionLogger
	// We don't use defer to reset r.functionLogger since it decreases performance
	if err := r.eventEncoder.Encode(event); err != nil {
		r.functionLogger = nil
		return nil, errors.Wrapf(err, "Can't encode event: %+v", event)
	}

	select {
	case result, ok := <-r.resultChan:
		if !ok {
			msg := "Client disconnected"
			r.Logger.Error(msg)
			r.SetStatus(status.Error)
			r.functionLogger = nil
			return nil, errors.New(msg)
		}

		r.Logger.DebugWith("Event executed",
			"name", r.configuration.Meta.Name,
			"status", result.StatusCode,
			"eventID", event.GetID())

		r.functionLogger = nil
		return nuclio.Response{
			Body:        result.DecodedBody,
			ContentType: result.ContentType,
			Headers:     result.Headers,
			StatusCode:  result.StatusCode,
		}, nil
	case <-time.After(eventTimeout):
		r.functionLogger = nil
		return nil, fmt.Errorf("handler timeout after %s", eventTimeout)
	}
}

// GetSocketType returns the type of socket the runtime works with (unix/tcp)
func (r *AbstractRuntime) GetSocketType() SocketType {
	return UnixSocket
}

// WaitForStart returns whether the runtime supports sending an indication that it started
func (r *AbstractRuntime) WaitForStart() bool {
	return false
}

func (r *AbstractRuntime) runWrapper() error {
	var err error

	var listener net.Listener
	var address string

	if r.runtime.GetSocketType() == UnixSocket {
		listener, address, err = r.createUnixListener()
	} else {
		listener, address, err = r.createTCPListener()
	}

	if err != nil {
		return errors.Wrap(err, "Can't create listener")
	}

	wrapperProcess, err := r.runtime.RunWrapper(address)
	if err != nil {
		return errors.Wrap(err, "Can't run wrapper")
	}
	r.wrapperProcess = wrapperProcess

	conn, err := listener.Accept()
	if err != nil {
		return errors.Wrap(err, "Can't get connection from wrapper")
	}

	r.Logger.Info("Wrapper connected")

	r.eventEncoder = NewEventJSONEncoder(r.Logger, conn)
	r.outReader = bufio.NewReader(conn)
	r.resultChan = make(chan *result)
	go r.wrapperOutputHandler()

	// wait for start if required to
	if r.runtime.WaitForStart() {
		r.Logger.Debug("Waiting for start")

		<-r.startChan
	}

	r.Logger.Debug("Started")

	return nil
}

// Create a listener on unix domian docker, return listener, path to socket and error
func (r *AbstractRuntime) createUnixListener() (net.Listener, string, error) {
	socketPath := fmt.Sprintf(socketPathTemplate, xid.New().String())

	if common.FileExists(socketPath) {
		if err := os.Remove(socketPath); err != nil {
			return nil, "", errors.Wrapf(err, "Can't remove socket at %q", socketPath)
		}
	}

	r.Logger.DebugWith("Creating listener socket", "path", socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, "", errors.Wrapf(err, "Can't listen on %s", socketPath)
	}

	unixListener, ok := listener.(*net.UnixListener)
	if !ok {
		return nil, "", fmt.Errorf("Can't get underlying Unix listener")
	}

	if err = unixListener.SetDeadline(time.Now().Add(connectionTimeout)); err != nil {
		return nil, "", errors.Wrap(err, "Can't set deadline")
	}

	return listener, socketPath, nil
}

// Create a listener on TCP docker, return listener, port and error
func (r *AbstractRuntime) createTCPListener() (net.Listener, string, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, "", errors.Wrap(err, "Can't find free port")
	}

	tcpListener, ok := listener.(*net.TCPListener)
	if !ok {
		return nil, "", errors.Wrap(err, "Can't get underlying TCP listener")
	}
	if err = tcpListener.SetDeadline(time.Now().Add(connectionTimeout)); err != nil {
		return nil, "", errors.Wrap(err, "Can't set deadline")
	}

	port := listener.Addr().(*net.TCPAddr).Port

	return listener, fmt.Sprintf("%d", port), nil
}

func (r *AbstractRuntime) wrapperOutputHandler() {
	// Read logs & output
	for {
		unmarshalledResult := &result{}
		var data []byte

		data, unmarshalledResult.err = r.outReader.ReadBytes('\n')

		if unmarshalledResult.err != nil {
			r.Logger.WarnWith("Failed to read from connection", "err", unmarshalledResult.err)
			r.resultChan <- unmarshalledResult
			// TODO: close(r.resultChan) ?
			continue
		}

		switch data[0] {
		case 'r':

			// try to unmarshall the result
			if unmarshalledResult.err = json.Unmarshal(data[1:], unmarshalledResult); unmarshalledResult.err != nil {
				r.resultChan <- unmarshalledResult
				continue
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
			r.resultChan <- unmarshalledResult
		case 'm':
			r.handleReponseMetric(data[1:])
		case 'l':
			r.handleResponseLog(data[1:])
		case 's':
			r.handleStart()
		}
	}
}

func (r *AbstractRuntime) handleResponseLog(response []byte) {
	var logRecord rpcLogRecord

	if err := json.Unmarshal(response, &logRecord); err != nil {
		r.Logger.ErrorWith("Can't decode log", "error", err)
		return
	}

	logger := r.resolveFunctionLogger(r.functionLogger)
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

func (r *AbstractRuntime) handleReponseMetric(response []byte) {
	var metrics struct {
		DurationSec float64 `json:"duration"`
	}

	logger := r.resolveFunctionLogger(r.functionLogger)
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

func (r *AbstractRuntime) handleStart() {
	r.startChan <- struct{}{}
}

// resolveFunctionLogger return either functionLogger if provided or root logger if not
func (r *AbstractRuntime) resolveFunctionLogger(functionLogger logger.Logger) logger.Logger {
	if functionLogger == nil {
		return r.Logger
	}
	return functionLogger
}

// Stop stops the runtime
func (r *AbstractRuntime) Stop() error {
	err := r.wrapperProcess.Kill()
	if err != nil {
		r.SetStatus(status.Error)
	} else {
		r.SetStatus(status.Stopped)
	}

	return err
}
