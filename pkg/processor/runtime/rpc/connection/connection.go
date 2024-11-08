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

package connection

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
	"github.com/nuclio/nuclio/pkg/processor/controlcommunication"
	"github.com/nuclio/nuclio/pkg/processor/runtime/rpc/controlmessagebroker"
	"github.com/nuclio/nuclio/pkg/processor/runtime/rpc/encoder"
	"github.com/nuclio/nuclio/pkg/processor/runtime/rpc/result"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type EventConnection interface {
	Start()

	Stop()

	ProcessEvent(item interface{}, functionLogger logger.Logger) (*result.BatchedResults, error)

	RunHandler()
}

type BaseConnection struct {
	Logger     logger.Logger
	encoder    encoder.EventEncoder
	cancelChan chan struct{}

	Conn    net.Conn
	Address string

	// TODO: implement status attribute logic when support multiple conn
	//status     status.Status
}

func (b *BaseConnection) Stop() {
	b.cancelChan <- struct{}{}
}

func (b *BaseConnection) SetEncoder(encoderInstance encoder.EventEncoder) {
	b.encoder = encoderInstance
}

type BaseEventConnection struct {
	*BaseConnection
	resultChan chan *result.BatchedResults
	startChan  chan struct{}

	connectionManager ConnectionManager
	functionLogger    logger.Logger
}

func NewBaseEventConnection(parentLogger logger.Logger, connectionManager ConnectionManager) *BaseEventConnection {
	baseConnection := &BaseConnection{
		Logger:     parentLogger.GetChild("event connection"),
		cancelChan: make(chan struct{}, 1),
	}
	return &BaseEventConnection{
		BaseConnection:    baseConnection,
		resultChan:        make(chan *result.BatchedResults),
		startChan:         make(chan struct{}, 1),
		connectionManager: connectionManager}
}
func (be *BaseEventConnection) Start() {
	<-be.startChan
}

func (be *BaseEventConnection) ProcessEvent(item interface{}, functionLogger logger.Logger) (*result.BatchedResults, error) {
	be.functionLogger = functionLogger
	if err := be.encoder.Encode(item); err != nil {
		be.functionLogger = nil
		return nil, errors.Wrapf(err, "Can't encode item: %+v", item)
	}
	processingResults, ok := <-be.resultChan

	// We don't use defer to reset be.functionLogger since it decreases performance
	be.functionLogger = nil

	if !ok {
		msg := "Client disconnected"
		be.Logger.Error(msg)

		// TODO: support status for socket separately when implementing multiple socket support
		be.connectionManager.SetStatus(status.Error)
		return nil, errors.New(msg)
	}
	// if processingResults.err is not nil, it means that whole batch processing was failed
	if processingResults.Err != nil {
		return nil, processingResults.Err
	}
	return processingResults, nil
}

func (be *BaseEventConnection) resolveFunctionLogger() logger.Logger {
	if be.functionLogger == nil {
		return be.Logger
	}
	return be.functionLogger
}

func (be *BaseEventConnection) RunHandler() {

	// Reset might close outChan, which will cause panic when sending
	defer common.CatchAndLogPanicWithOptions(context.Background(), // nolint: errcheck
		be.Logger,
		"handling event wrapper output (Restart called?)",
		&common.CatchAndLogPanicOptions{
			Args:          nil,
			CustomHandler: nil,
		})
	defer func() {
		select {
		case be.resultChan <- &result.BatchedResults{
			Results: []*result.Result{{
				StatusCode: http.StatusRequestTimeout,
				Err:        errors.New("Runtime restarted"),
			}},
		}:

		default:
			be.Logger.Warn("Nothing waiting on result channel during restart. Continuing")
		}

	}()

	outReader := bufio.NewReader(be.Conn)

	// Read logs & output
	for {
		select {

		// TODO: sync between event and control output handlers using a shared context
		case <-be.cancelChan:
			be.Logger.Warn("Event output handler was canceled (Restart called?)")
			return

		default:

			unmarshalledResults := result.NewBatchedResults()
			var data []byte
			data, unmarshalledResults.Err = outReader.ReadBytes('\n')

			if unmarshalledResults.Err != nil {
				be.Logger.WarnWith(string(common.FailedReadFromEventConnection),
					"err", unmarshalledResults.Err.Error())
				be.resultChan <- unmarshalledResults
				continue
			}

			switch data[0] {
			case 'r':
				unmarshalledResults.UnmarshalResponseData(be.Logger, data[1:])

				// write back to result channel
				be.resultChan <- unmarshalledResults
			case 'm':
				be.handleResponseMetric(data[1:])
			case 'l':
				be.handleResponseLog(data[1:])
			case 's':
				be.handleStart()
			}
		}
	}
}

func (be *BaseEventConnection) handleResponseMetric(response []byte) {
	var metrics struct {
		DurationSec float64 `json:"duration"`
	}

	loggerInstance := be.resolveFunctionLogger()
	if err := json.Unmarshal(response, &metrics); err != nil {
		loggerInstance.ErrorWith("Can't decode metric", "error", err)
		return
	}

	if metrics.DurationSec == 0 {
		loggerInstance.ErrorWith("No duration in metrics", "metrics", metrics)
		return
	}
	be.connectionManager.UpdateStatistics(metrics.DurationSec)
}

func (be *BaseEventConnection) handleResponseLog(response []byte) {
	var logRecord result.RpcLogRecord

	if err := json.Unmarshal(response, &logRecord); err != nil {
		be.Logger.ErrorWith("Can't decode log", "error", err)
		return
	}

	loggerInstance := be.resolveFunctionLogger()
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

func (be *BaseEventConnection) handleStart() {
	be.startChan <- struct{}{}
}

type BaseControlMessageConnection struct {
	*BaseConnection

	broker controlcommunication.ControlMessageBroker
}

func NewBaseControlMessageConnection(parentLogger logger.Logger, broker controlcommunication.ControlMessageBroker) *BaseControlMessageConnection {

	baseConnection := &BaseConnection{
		Logger:     parentLogger.GetChild("event connection"),
		cancelChan: make(chan struct{}, 1),
	}
	return &BaseControlMessageConnection{
		BaseConnection: baseConnection,
		broker:         broker,
	}
}

func (bc *BaseControlMessageConnection) SetBroker(abstractBroker *controlcommunication.AbstractControlMessageBroker) {
	bc.broker = controlmessagebroker.NewRpcControlMessageBroker(
		bc.encoder,
		bc.Logger,
		abstractBroker)
}

func (bc *BaseControlMessageConnection) GetBroker() controlcommunication.ControlMessageBroker {
	return bc.broker
}

func (bc *BaseControlMessageConnection) RunHandler() {

	// recover from panic in case of error
	defer common.CatchAndLogPanicWithOptions(context.Background(), // nolint: errcheck
		bc.Logger,
		"control wrapper output handler (Restart called?)",
		&common.CatchAndLogPanicOptions{
			Args:          nil,
			CustomHandler: nil,
		})
	defer func() {
		bc.cancelChan <- struct{}{}
	}()

	outReader := bufio.NewReader(bc.Conn)

	// keep a counter for log throttling
	errLogCounter := 0
	logCounterTime := time.Now()

	for {
		select {

		// TODO: sync between event and control output handlers using a shared context
		case <-bc.cancelChan:
			bc.Logger.Warn("Control output handler was canceled (Restart called?)")
			return

		default:

			// read control message
			controlMessage, err := bc.broker.ReadControlMessage(outReader)
			if err != nil {

				// if enough time has passed, log the error
				if time.Since(logCounterTime) > 500*time.Millisecond {
					logCounterTime = time.Now()
					errLogCounter = 0
				}
				if errLogCounter%5 == 0 {
					bc.Logger.WarnWith(string(common.FailedReadControlMessage),
						"errRootCause", errors.RootCause(err).Error())
					errLogCounter++
				}

				// if error is EOF it means the connection was closed, so we should exit
				if errors.RootCause(err) == io.EOF {
					bc.Logger.Debug("Control connection was closed")
					return
				}

				continue
			} else {
				errLogCounter = 0
			}

			bc.Logger.DebugWith("Received control message", "messageKind", controlMessage.Kind)

			// send message to control consumers
			if err := bc.broker.SendToConsumers(controlMessage); err != nil {
				bc.Logger.WarnWith("Failed to send control message to consumers", "err", err.Error())
			}

			// TODO: validate and respond to wrapper process
		}
	}
}
