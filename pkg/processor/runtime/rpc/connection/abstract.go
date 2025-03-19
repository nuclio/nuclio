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
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/processor/cloudevent"
	"github.com/nuclio/nuclio/pkg/processor/controlcommunication"
	"github.com/nuclio/nuclio/pkg/processor/eventprocessor"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/runtime/rpc/controlmessagebroker"
	"github.com/nuclio/nuclio/pkg/processor/runtime/rpc/encoder"
	"github.com/nuclio/nuclio/pkg/processor/runtime/rpc/result"

	"github.com/mitchellh/mapstructure"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/rs/xid"
)

type AbstractConnectionManager struct {
	Logger logger.Logger

	MinConnectionsNum int
	MaxConnectionsNum int

	RuntimeConfiguration runtime.Configuration
	Configuration        *ManagerConfigration

	controlMessageSocket *ControlMessageSocket
	allocator            eventprocessor.Allocator

	status *status.SafeStatus
}

func NewAbstractConnectionManager(parentLogger logger.Logger, runtimeConfiguration runtime.Configuration, configuration *ManagerConfigration) (*AbstractConnectionManager, error) {
	abstractConnectionManager := &AbstractConnectionManager{
		Logger:               parentLogger.GetChild("connection-manager"),
		MinConnectionsNum:    1,
		MaxConnectionsNum:    1,
		RuntimeConfiguration: runtimeConfiguration,
		Configuration:        configuration,

		// by default status is ready
		status: status.NewSafeStatus(status.Ready),
	}
	var err error
	if runtimeConfiguration.AsyncConfig != nil {
		abstractConnectionManager.MinConnectionsNum = runtimeConfiguration.AsyncConfig.MinConnectionsNumber
		abstractConnectionManager.MaxConnectionsNum = runtimeConfiguration.AsyncConfig.MaxConnectionsNumber
	}
	if abstractConnectionManager.MinConnectionsNum == 1 && abstractConnectionManager.MaxConnectionsNum == 1 {
		// TODO: add support sync singleton
		abstractConnectionManager.allocator = eventprocessor.NewAsyncSingletonAllocator(
			abstractConnectionManager.Logger,
			nil)
	} else if abstractConnectionManager.allocator, err = eventprocessor.NewSyncPoolAllocator(
		abstractConnectionManager.Logger,
		nil); err != nil {
		return nil, err
	}

	return abstractConnectionManager, nil
}

func (bc *AbstractConnectionManager) UpdateStatistics(durationSec float64) {
	bc.Configuration.Statistics.DurationMilliSecondsCount++
	bc.Configuration.Statistics.DurationMilliSecondsSum += uint64(durationSec * 1000)
}

func (bc *AbstractConnectionManager) GetConfig() ManagerConfigration {
	return *bc.Configuration
}

func (bc *AbstractConnectionManager) SetStatus(newStatus status.Status) {
	bc.status.SetStatus(newStatus)
}

func (bc *AbstractConnectionManager) GetStatus() status.Status {
	return bc.status.GetStatus()
}

// PrepareControlMessageSocket prepares control message socket for processing
// If SupportControlCommunication is enabled, a control communication socket is created,
// wrapped in a ControlMessageSocket, and integrated with the ControlMessageBroker for runtime operations.
func (bc *AbstractConnectionManager) prepareControlMessageSocket() error {
	if bc.Configuration.SupportControlCommunication {
		controlConnection, err := bc.createSocketConnection()
		if err != nil {
			return errors.Wrap(err, "Failed to create control socket connection")
		}
		bc.controlMessageSocket = NewControlMessageSocket(
			bc.Logger,
			controlConnection,
			bc.RuntimeConfiguration.ControlMessageBroker)
	}
	return nil
}

func (bc *AbstractConnectionManager) startControlMessageSocket() error {
	if bc.Configuration.SupportControlCommunication {
		var err error
		bc.controlMessageSocket.Conn, err = bc.controlMessageSocket.listener.Accept()
		if err != nil {
			return errors.Wrap(err, "Failed to get control connection from wrapper")
		}
		bc.controlMessageSocket.SetEncoder(bc.Configuration.GetEventEncoderFunc(bc.controlMessageSocket.Conn))

		// initialize control message broker
		bc.controlMessageSocket.SetBroker(bc.RuntimeConfiguration.ControlMessageBroker)
		go bc.controlMessageSocket.RunHandler()
		bc.Logger.Debug("Successfully established connection for control socket")
	}
	return nil
}

func (bc *AbstractConnectionManager) stopControlMessageSocket() {
	if bc.controlMessageSocket != nil {
		go func() {
			bc.controlMessageSocket.Stop()
		}()
	}
}

// Create a listener on unix domain docker, return listener, path to socket and error
func (bc *AbstractConnectionManager) createSocketConnection() (*socketConnection, error) {
	connection := &socketConnection{}
	var err error
	if bc.Configuration.SocketType == UnixSocket {
		connection.listener, connection.address, err = bc.createUnixListener()
	} else {
		connection.listener, connection.address, err = bc.createTCPListener()
	}
	if err != nil {
		return nil, errors.Wrap(err, "Can't create listener")
	}

	return connection, nil
}

// Create a listener on unix domain docker, return listener, path to socket and error
func (bc *AbstractConnectionManager) createUnixListener() (net.Listener, string, error) {
	socketPath := fmt.Sprintf(socketPathTemplate, xid.New().String())

	if common.FileExists(socketPath) {
		if err := os.Remove(socketPath); err != nil {
			return nil, "", errors.Wrapf(err, "Can't remove socket at %q", socketPath)
		}
	}

	bc.Logger.DebugWith("Creating listener socket", "path", socketPath)

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
func (bc *AbstractConnectionManager) createTCPListener() (net.Listener, string, error) {
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

type AbstractConnection struct {
	Logger     logger.Logger
	encoder    encoder.EventEncoder
	cancelChan chan struct{}

	Conn    net.Conn
	Address string

	// TODO: implement status attribute logic when support multiple conn
	status         *status.SafeStatus
	functionLogger logger.Logger
}

func (b *AbstractConnection) Stop() {
	b.cancelChan <- struct{}{}
}

func (b *AbstractConnection) SetEncoder(encoderInstance encoder.EventEncoder) {
	b.encoder = encoderInstance
}

func (b *AbstractConnection) handleResponseLog(logRecord interface{}) {
	var rpcLogRecord result.RpcLogRecord

	switch logRecord := logRecord.(type) {
	case map[string]interface{}:
		if err := mapstructure.Decode(logRecord, &rpcLogRecord); err != nil {
			b.Logger.ErrorWith("Failed to decode log",
				"error", err.Error(),
				"record", logRecord)
			return
		}
	case []byte:
		if err := json.Unmarshal(logRecord, &rpcLogRecord); err != nil {
			b.Logger.ErrorWith("Can't decode log", "error", err)
			return
		}
	}

	loggerInstance := b.resolveFunctionLogger()
	logFunc := loggerInstance.DebugWith
	switch rpcLogRecord.Level {
	case "error", "critical", "fatal":
		logFunc = loggerInstance.ErrorWith
	case "warning":
		logFunc = loggerInstance.WarnWith
	case "info":
		logFunc = loggerInstance.InfoWith
	}

	vars := common.MapToSlice(rpcLogRecord.With)
	logFunc(rpcLogRecord.Message, vars...)
}

func (b *AbstractConnection) resolveFunctionLogger() logger.Logger {
	if b.functionLogger == nil {
		return b.Logger
	}
	return b.functionLogger
}

type AbstractEventConnection struct {
	*AbstractConnection
	resultChan chan *result.BatchedResults
	startChan  chan struct{}

	connectionManager ConnectionManager
}

func NewAbstractEventConnection(parentLogger logger.Logger, connectionManager ConnectionManager) *AbstractEventConnection {
	abstractConnection := &AbstractConnection{
		Logger:     parentLogger.GetChild("event"),
		cancelChan: make(chan struct{}, 1),
		status:     status.NewSafeStatus(status.Initializing),
	}
	return &AbstractEventConnection{
		AbstractConnection: abstractConnection,
		resultChan:         make(chan *result.BatchedResults),
		startChan:          make(chan struct{}, 1),
		connectionManager:  connectionManager,
	}
}
func (be *AbstractEventConnection) WaitForStart() {
	<-be.startChan
}

func (be *AbstractEventConnection) ProcessEvent(event nuclio.Event, functionLogger logger.Logger) (interface{}, error) {
	processingResult, err := be.processItem(event, functionLogger)
	if err != nil {
		return nil, err
	}
	// this is a single event processing flow, so we only take the first item from the result
	return nuclio.Response{
		Body:        processingResult.Results[0].DecodedBody,
		ContentType: processingResult.Results[0].ContentType,
		Headers:     processingResult.Results[0].Headers,
		StatusCode:  processingResult.Results[0].StatusCode,
	}, processingResult.Results[0].Err
}

func (be *AbstractEventConnection) ProcessEventBatch(batch []nuclio.Event, functionLogger logger.Logger) ([]*runtime.ResponseWithErrors, error) {
	processingResults, err := be.processItem(batch, functionLogger)
	responsesWithErrors := make([]*runtime.ResponseWithErrors, len(processingResults.Results))

	for index, processingResult := range processingResults.Results {
		if processingResult.EventId == "" {
			functionLogger.WarnWith("Received response with empty event_id, response won't be returned")
			continue
		}
		responsesWithErrors[index] = &runtime.ResponseWithErrors{
			Response: nuclio.Response{
				Body:        processingResult.DecodedBody,
				ContentType: processingResult.ContentType,
				Headers:     processingResult.Headers,
				StatusCode:  processingResult.StatusCode,
			},
			EventId:      processingResult.EventId,
			ProcessError: processingResult.Err,
		}
	}
	return responsesWithErrors, err
}

func (be *AbstractEventConnection) processItem(item interface{}, functionLogger logger.Logger) (*result.BatchedResults, error) {
	be.functionLogger = functionLogger
	if err := be.encoder.Encode(item); err != nil {
		be.functionLogger = nil
		return nil, errors.Wrapf(err, "Can't encode item: %+v", item)
	}

	// if eventTimeout is 0, we wait for response without timeout
	// if it is set, we wait either for response or the timeout
	if be.connectionManager.GetConfig().eventTimeout == 0 {
		processingResults, ok := <-be.resultChan
		// We don't use defer to reset be.functionLogger since it decreases performance
		be.functionLogger = nil

		if !ok {
			msg := "Client disconnected"
			be.Logger.Error(msg)

			// TODO: support status for socket separately when implementing multiple socket support
			be.connectionManager.SetStatus(status.RestartRequired)
			return nil, errors.New(msg)
		}
		// if processingResults.err is not nil, it means that whole batch processing was failed
		if processingResults.Err != nil {
			return nil, processingResults.Err
		}
		return processingResults, nil
	} else {
		return be.waitForResponseWithTimeout()
	}
}

func (be *AbstractEventConnection) waitForResponseWithTimeout() (*result.BatchedResults, error) {
	ticker := time.NewTicker(be.connectionManager.GetConfig().eventTimeout)
	defer ticker.Stop()

	select {
	case processingResults, ok := <-be.resultChan:
		// We don't use defer to reset be.functionLogger since it decreases performance
		be.functionLogger = nil

		if !ok {
			msg := "Client disconnected"
			be.Logger.Error(msg)

			// TODO: support status for socket separately when implementing multiple socket support
			be.connectionManager.SetStatus(status.RestartRequired)
			return nil, errors.New(msg)
		}
		// if processingResults.err is not nil, it means that whole batch processing was failed
		if processingResults.Err != nil {
			return nil, processingResults.Err
		}
		return processingResults, nil
	case <-ticker.C:
		be.Logger.WarnWith("Event processing timeout, connection should be restarted")
		be.status.SetStatus(status.RestartRequired)
		return &result.BatchedResults{
			Results: []*result.Result{{
				StatusCode: http.StatusRequestTimeout,
				Err:        errors.New("Connection closed"),
			}},
		}, nil
	}
}

func (be *AbstractEventConnection) RunHandler() {
	defer close(be.cancelChan)

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

// GetStatistics returns a pointer to the statistics object. This must not be modified by the reader
func (be *AbstractEventConnection) GetStatistics() *eventprocessor.Statistics {
	return nil
}

// GetIndex returns the index of the worker, as specified during creation
func (be *AbstractEventConnection) GetIndex() int {
	return -1
}

// GetRuntime returns the runtime of the worker, as specified during creation
func (be *AbstractEventConnection) GetRuntime() runtime.Runtime {
	return nil
}

// GetStatus returns the status of the worker, as updated by the runtime
func (be *AbstractEventConnection) GetStatus() status.Status {
	return be.status.GetStatus()
}

// Stop stops the connection
func (be *AbstractEventConnection) Stop() error {
	be.AbstractConnection.Stop()
	return be.Conn.Close()
}

// GetStructuredCloudEvent return a structued clould event
func (be *AbstractEventConnection) GetStructuredCloudEvent() *cloudevent.Structured {
	return nil
}

// GetBinaryCloudEvent return a binary cloud event
func (be *AbstractEventConnection) GetBinaryCloudEvent() *cloudevent.Binary {
	return nil
}

// GetEventTime return current event time, nil if we're not handling event
func (be *AbstractEventConnection) GetEventTime() *time.Time {
	return nil
}

// ResetEventTime resets the event time
func (be *AbstractEventConnection) ResetEventTime() {
}

func (be *AbstractEventConnection) RestartRequired(*time.Duration) bool {
	return false
}

// Restart restarts the worker
func (be *AbstractEventConnection) Restart() error {
	return nuclio.ErrNotImplemented
}

// SupportsRestart returns true if the underlying runtime supports restart
func (be *AbstractEventConnection) SupportsRestart() bool {
	return false
}

func (be *AbstractEventConnection) Terminate() error {
	return nuclio.ErrNotImplemented
}

func (be *AbstractEventConnection) Drain() error {
	return nuclio.ErrNotImplemented
}

func (be *AbstractEventConnection) Continue() error {
	return nuclio.ErrNotImplemented
}

// Subscribe subscribes to a control message kind
func (be *AbstractEventConnection) Subscribe(kind controlcommunication.ControlMessageKind, channel chan *controlcommunication.ControlMessage) error {
	return nuclio.ErrNotImplemented
}

// Unsubscribe unsubscribes from a control message kind
func (be *AbstractEventConnection) Unsubscribe(kind controlcommunication.ControlMessageKind, channel chan *controlcommunication.ControlMessage) error {
	return nuclio.ErrNotImplemented
}

func (be *AbstractEventConnection) handleResponseMetric(response []byte) {
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

func (be *AbstractEventConnection) handleStart() {
	be.startChan <- struct{}{}
}

type AbstractControlMessageConnection struct {
	*AbstractConnection

	broker controlcommunication.ControlMessageBroker
}

func NewAbstractControlMessageConnection(parentLogger logger.Logger, broker controlcommunication.ControlMessageBroker) *AbstractControlMessageConnection {

	abstractConnection := &AbstractConnection{
		Logger:     parentLogger.GetChild("control"),
		cancelChan: make(chan struct{}, 1),
	}
	return &AbstractControlMessageConnection{
		AbstractConnection: abstractConnection,
		broker:             broker,
	}
}

func (bc *AbstractControlMessageConnection) SetBroker(abstractBroker *controlcommunication.AbstractControlMessageBroker) {
	bc.broker = controlmessagebroker.NewRpcControlMessageBroker(
		bc.encoder,
		bc.Logger,
		abstractBroker)
}

func (bc *AbstractControlMessageConnection) GetBroker() controlcommunication.ControlMessageBroker {
	return bc.broker
}

func (bc *AbstractControlMessageConnection) RunHandler() {

	// recover from panic in case of error
	defer common.CatchAndLogPanicWithOptions(context.Background(), // nolint: errcheck
		bc.Logger,
		"control wrapper output handler (Restart called?)",
		&common.CatchAndLogPanicOptions{
			Args:          nil,
			CustomHandler: nil,
		})
	defer close(bc.cancelChan)

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

			// if the message is of `log` kind, then we just want to log it without sending to consumer
			if controlMessage.Kind == controlcommunication.LogMessageKind {
				bc.handleResponseLog(controlMessage.Attributes)
				continue
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
