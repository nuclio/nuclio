/*
Copyright 2024 The Nuclio Authors.

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
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/status"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type result struct {
	StatusCode   int                    `json:"status_code"`
	ContentType  string                 `json:"content_type"`
	Body         string                 `json:"body"`
	BodyEncoding string                 `json:"body_encoding"`
	Headers      map[string]interface{} `json:"headers"`
	EventId      string                 `json:"event_id"`

	DecodedBody []byte
	err         error
}

type batchedResults struct {
	results []*result
	err     error
}

func newBatchedResults() *batchedResults {
	return &batchedResults{results: make([]*result, 0)}
}

type socketConnection struct {
	conn     net.Conn
	listener net.Listener
	address  string
}

type AbstractSocket struct {
	*socketConnection
	Logger     logger.Logger
	runtime    *AbstractRuntime
	encoder    EventEncoder
	cancelChan chan struct{}
	//status     status.Status
}

func (s *AbstractSocket) stop() {
	s.cancelChan <- struct{}{}
}

type ControlMessageSocket struct {
	*AbstractSocket
}

func NewControlMessageSocket(parentLogger logger.Logger, socketConnection *socketConnection, runtime *AbstractRuntime) *ControlMessageSocket {
	abstractSocket := &AbstractSocket{
		socketConnection: socketConnection,
		Logger:           parentLogger.GetChild("control message socket"),
		runtime:          runtime,
		cancelChan:       make(chan struct{}, 1),
	}
	return &ControlMessageSocket{AbstractSocket: abstractSocket}
}

func (cm *ControlMessageSocket) runHandler() {

	// recover from panic in case of error
	defer common.CatchAndLogPanicWithOptions(context.Background(), // nolint: errcheck
		cm.Logger,
		"control wrapper output handler (Restart called?)",
		&common.CatchAndLogPanicOptions{
			Args:          nil,
			CustomHandler: nil,
		})
	defer func() {
		cm.cancelChan <- struct{}{}
	}()

	outReader := bufio.NewReader(cm.conn)

	// keep a counter for log throttling
	errLogCounter := 0
	logCounterTime := time.Now()

	for {
		select {

		// TODO: sync between event and control output handlers using a shared context
		case <-cm.cancelChan:
			cm.Logger.Warn("Control output handler was canceled (Restart called?)")
			return

		default:

			// read control message
			controlMessage, err := cm.runtime.ControlMessageBroker.ReadControlMessage(outReader)
			if err != nil {

				// if enough time has passed, log the error
				if time.Since(logCounterTime) > 500*time.Millisecond {
					logCounterTime = time.Now()
					errLogCounter = 0
				}
				if errLogCounter%5 == 0 {
					cm.Logger.WarnWith(string(common.FailedReadControlMessage),
						"errRootCause", errors.RootCause(err).Error())
					errLogCounter++
				}

				// if error is EOF it means the connection was closed, so we should exit
				if errors.RootCause(err) == io.EOF {
					cm.Logger.Debug("Control connection was closed")
					return
				}

				continue
			} else {
				errLogCounter = 0
			}

			cm.Logger.DebugWith("Received control message", "messageKind", controlMessage.Kind)

			// send message to control consumers
			if err := cm.runtime.GetControlMessageBroker().SendToConsumers(controlMessage); err != nil {
				cm.Logger.WarnWith("Failed to send control message to consumers", "err", err.Error())
			}

			// TODO: validate and respond to wrapper process
		}
	}
}

type EventSocket struct {
	*AbstractSocket
	resultChan chan *batchedResults
	startChan  chan struct{}
}

func NewEventSocket(parentLogger logger.Logger, socketConnection *socketConnection, runtime *AbstractRuntime) *EventSocket {

	abstractSocket := &AbstractSocket{
		socketConnection: socketConnection,
		Logger:           parentLogger.GetChild("event socket"),
		runtime:          runtime,
		cancelChan:       make(chan struct{}, 1),
	}
	return &EventSocket{
		AbstractSocket: abstractSocket,
		resultChan:     make(chan *batchedResults),
		startChan:      make(chan struct{}, 1),
	}
}

func (s *EventSocket) processEvent(item interface{}) (*batchedResults, error) {
	if err := s.encoder.Encode(item); err != nil {
		s.runtime = nil
		return nil, errors.Wrapf(err, "Can't encode item: %+v", item)
	}
	processingResults, ok := <-s.resultChan
	if !ok {
		msg := "Client disconnected"
		s.Logger.Error(msg)

		// TODO: support status for socket separately when implementing multiple socket support
		s.runtime.SetStatus(status.Error)
		return nil, errors.New(msg)
	}
	// if processingResults.err is not nil, it means that whole batch processing was failed
	if processingResults.err != nil {
		return nil, processingResults.err
	}
	return processingResults, nil
}

func (s *EventSocket) runHandler() {

	// Reset might close outChan, which will cause panic when sending
	defer common.CatchAndLogPanicWithOptions(context.Background(), // nolint: errcheck
		s.Logger,
		"handling event wrapper output (Restart called?)",
		&common.CatchAndLogPanicOptions{
			Args:          nil,
			CustomHandler: nil,
		})
	defer func() {
		select {
		case s.resultChan <- &batchedResults{
			results: []*result{{
				StatusCode: http.StatusRequestTimeout,
				err:        errors.New("Runtime restarted"),
			}},
		}:

		default:
			s.Logger.Warn("Nothing waiting on result channel during restart. Continuing")
		}

	}()

	outReader := bufio.NewReader(s.conn)

	// Read logs & output
	for {
		select {

		// TODO: sync between event and control output handlers using a shared context
		case <-s.cancelChan:
			s.Logger.Warn("Event output handler was canceled (Restart called?)")
			return

		default:

			unmarshalledResults := newBatchedResults()
			var data []byte
			data, unmarshalledResults.err = outReader.ReadBytes('\n')

			if unmarshalledResults.err != nil {
				s.Logger.WarnWith(string(common.FailedReadFromEventConnection),
					"err", unmarshalledResults.err.Error())
				s.resultChan <- unmarshalledResults
				continue
			}

			switch data[0] {
			case 'r':
				unmarshalResponseData(s.Logger, data[1:], unmarshalledResults)

				// write back to result channel
				s.resultChan <- unmarshalledResults
			case 'm':
				s.handleResponseMetric(data[1:])
			case 'l':
				s.handleResponseLog(data[1:])
			case 's':
				s.handleStart()
			}
		}
	}
}

func (s *EventSocket) handleResponseMetric(response []byte) {
	var metrics struct {
		DurationSec float64 `json:"duration"`
	}

	loggerInstance := s.runtime.resolveFunctionLogger()
	if err := json.Unmarshal(response, &metrics); err != nil {
		loggerInstance.ErrorWith("Can't decode metric", "error", err)
		return
	}

	if metrics.DurationSec == 0 {
		loggerInstance.ErrorWith("No duration in metrics", "metrics", metrics)
		return
	}

	s.runtime.Statistics.DurationMilliSecondsCount++
	s.runtime.Statistics.DurationMilliSecondsSum += uint64(metrics.DurationSec * 1000)
}

func (s *EventSocket) handleResponseLog(response []byte) {
	var logRecord rpcLogRecord

	if err := json.Unmarshal(response, &logRecord); err != nil {
		s.Logger.ErrorWith("Can't decode log", "error", err)
		return
	}

	loggerInstance := s.runtime.resolveFunctionLogger()
	logFunc := loggerInstance.DebugWith

	switch logRecord.Level {
	case "error", "critical", "fatal":
		logFunc = loggerInstance.ErrorWith
	case "warning":
		logFunc = loggerInstance.WarnWith
	case "info":
		logFunc = loggerInstance.InfoWith
	}

	vars := common.MapToSlice(logRecord.With)
	logFunc(logRecord.Message, vars...)
}

func (s *EventSocket) handleStart() {
	s.startChan <- struct{}{}
}
