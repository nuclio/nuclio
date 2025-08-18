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
	"os"
	"sync"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/cloudevent"
	"github.com/nuclio/nuclio/pkg/processor/controlcommunication"
	"github.com/nuclio/nuclio/pkg/processor/eventprocessor"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/runtime/rpc/controlmessagebroker"
	"github.com/nuclio/nuclio/pkg/processor/runtime/rpc/encoder"
	"github.com/nuclio/nuclio/pkg/processor/runtime/rpc/result"
	"github.com/nuclio/nuclio/pkg/processor/statistics"

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

	ConnectionAvailabilityTimeoutDuration time.Duration

	RuntimeConfiguration runtime.Configuration
	Configuration        *ManagerConfigration

	controlMessageSocket *ControlMessageSocket
	allocator            eventprocessor.Allocator

	status *status.SafeStatus
	// wrapper PID, used to identify connection allocator
	pid int
}

func NewAbstractConnectionManager(parentLogger logger.Logger, runtimeConfiguration runtime.Configuration, configuration *ManagerConfigration) (*AbstractConnectionManager, error) {
	abstractConnectionManager := &AbstractConnectionManager{
		Logger:               parentLogger.GetChild("connection-manager"),
		MinConnectionsNum:    1,
		MaxConnectionsNum:    1,
		RuntimeConfiguration: runtimeConfiguration,
		Configuration:        configuration,
		status:               status.NewSafeStatus(status.Initializing),
	}
	var err error
	if runtimeConfiguration.AsyncConfig != nil {
		abstractConnectionManager.MinConnectionsNum = runtimeConfiguration.AsyncConfig.MinConnectionsNumber
		abstractConnectionManager.MaxConnectionsNum = runtimeConfiguration.AsyncConfig.MaxConnectionsNumber
		if abstractConnectionManager.ConnectionAvailabilityTimeoutDuration, err = runtimeConfiguration.AsyncConfig.GetConnectionAvailabilityTimeoutDuration(); err != nil {
			return nil, errors.Wrap(err, "Failed to get connection availability timeout duration")
		}
	}

	if err = abstractConnectionManager.createAllocator(); err != nil {
		return nil, errors.Wrap(err, "Failed to create allocator")
	}

	return abstractConnectionManager, nil
}

func (bc *AbstractConnectionManager) UpdateStatistics(durationSec float64) {
	bc.Configuration.Statistics.DurationMilliSecondsCount++
	bc.Configuration.Statistics.DurationMilliSecondsSum += uint64(durationSec * 1000)
}

func (bc *AbstractConnectionManager) GetAllocationStatistics() *statistics.AllocatorStatistics {
	return bc.allocator.GetStatistics()
}

func (bc *AbstractConnectionManager) GetConfig() ManagerConfigration {
	return *bc.Configuration
}

func (bc *AbstractConnectionManager) IsAsync() bool {
	return bc.RuntimeConfiguration.Mode == functionconfig.AsyncTriggerWorkMode
}

func (bc *AbstractConnectionManager) IsBusy() bool {
	return len(bc.allocator.GetObjects()) != bc.allocator.GetNumObjectsAvailable()
}

func (bc *AbstractConnectionManager) SetStatus(newStatus status.Status) {
	bc.status.SetStatus(newStatus)
}

func (bc *AbstractConnectionManager) createAllocator() error {
	var err error
	if bc.MinConnectionsNum == 1 && bc.MaxConnectionsNum == 1 {
		// TODO: add support sync singleton
		bc.allocator = eventprocessor.NewNonBlockingSingletonAllocator(
			bc.Logger,
			nil)
	} else if bc.allocator, err = eventprocessor.NewBlockingPoolAllocator(
		bc.Logger,
		nil); err != nil {
		return err
	}
	return nil
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
		return nil, "", errors.Errorf("Can't get underlying Unix listener")
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

func (bc *AbstractConnectionManager) stopAllocator() error {
	// stop allocator
	if err := bc.allocator.Stop(); err != nil {
		bc.Logger.WarnWith("Failed to stop allocator",
			"error", err.Error())
		return err
	}
	return nil
}

func (bc *AbstractConnectionManager) stopControlMessageSocket() {
	if bc.controlMessageSocket != nil {
		bc.controlMessageSocket.Stop()
	}
	bc.controlMessageSocket = nil
}

type AbstractConnection struct {
	Logger     logger.Logger
	encoder    encoder.EventEncoder
	cancelChan chan struct{}
	// ensure stopping only once
	stopOnce sync.Once

	Conn    net.Conn
	Address string

	status         *status.SafeStatus
	functionLogger logger.Logger
}

func (b *AbstractConnection) Stop() {
	select {
	// try to send a signal to stop the connection (send only if the channel is not full)
	case b.cancelChan <- struct{}{}:
	default:
	}
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
	resultChan chan result.Result
	startChan  chan struct{}

	connectionManager ConnectionManager
}

func NewAbstractEventConnection(parentLogger logger.Logger, connectionManager ConnectionManager) *AbstractEventConnection {
	abstractConnection := &AbstractConnection{
		Logger:     parentLogger.GetChild("event"),
		cancelChan: make(chan struct{}, 1),
		status:     status.NewSafeStatus(status.Initializing),
		stopOnce:   sync.Once{},
	}
	return &AbstractEventConnection{
		AbstractConnection: abstractConnection,
		resultChan:         make(chan result.Result),
		startChan:          make(chan struct{}, 1),
		connectionManager:  connectionManager,
	}
}

func (be *AbstractEventConnection) WaitForStart(timeout time.Duration) error {
	var ok bool
	if timeout <= 0 {
		_, ok = <-be.startChan
	} else {
		ticker := time.NewTicker(timeout)
		defer ticker.Stop()

		// wait for start signal or timeout
		select {
		case _, ok = <-be.startChan:
		case <-ticker.C:
			return errors.Errorf("Timeout waiting for start signal, timeout: %s", timeout.String())
		}
	}

	// for the case when wrapper is restarting when connection was just created
	if !ok {
		return errors.New("Failed to get start signal, channel is closed")
	}
	return nil
}

func (be *AbstractEventConnection) ProcessEvent(event nuclio.Event, functionLogger logger.Logger) (result.ResultWithProcessingResult, error) {
	processingResult, err := be.processItem(event, functionLogger)
	if processingResult == nil {
		return nil, err
	}

	normalizedResult, normalisationErr := result.NormalizeToResultWithProcessingResult(processingResult)
	if normalisationErr != nil {
		return nil, errors.Wrap(normalisationErr, "Failed to normalize result")
	}

	return normalizedResult, err
}

func (be *AbstractEventConnection) ProcessEventBatch(batch []nuclio.Event, functionLogger logger.Logger) (*result.BatchedResults, error) {
	processingResult, err := be.processItem(batch, functionLogger)
	if processingResult == nil {
		return nil, err
	}

	normalizedResult, normalisationErr := result.NormalizeToBatchedResults(processingResult)
	if normalisationErr != nil {
		return nil, errors.Wrap(normalisationErr, "Failed to normalize result")
	}

	return normalizedResult, err
}

func (be *AbstractEventConnection) ProcessStream(stream *result.StreamStart) (err error) {
	// always close stream when processing is done
	defer be.postProcessStreaming(stream.ResponseStream)

	// first defer: set status code from error if any
	defer func() {
		if err != nil {
			stream.SetStatusCodeFromError(err)

			// mark connection for restart to clean up resources
			be.SetStatus(status.RestartRequired)
		}
	}()

	// start with writing a first chunk
	// this is blocking operation, so it will wait until the reader is ready to receive data
	if err = stream.WriteFirstChunk(); err != nil {
		return errors.Wrap(err, "Failed to write first chunk to stream")
	}

	var chunk result.Result
	// wait for next messages arrival
	for {
		if chunk, err = be.waitForNextResponseChunk(); err != nil {
			// if there was an error, return it
			return errors.Wrap(err, "Failed to read next chunk from stream")
		}

		switch typedChunk := chunk.(type) {
		case *result.StreamEnd:
			return nil
		case *result.BodyOnly:
			if _, err = stream.SendChunk(typedChunk.Body); err != nil {
				// if there was an error sending the chunk, mark connection for restart and return error
				return errors.Wrap(err, "Failed to send chunk to stream")
			}
			continue
		default:
			return errors.Errorf("Got unsupported message of type %T during stream processing", typedChunk)
		}
	}
}

// RunHandler runs the event connection handler.
// This is the main loop that reads from the connection
// and sends the results to the result channel.
// It also handles the cancel signal.
// However, since the operation of waiting for data in the channel is blocking, there might be a chance
// that it gets stuck either on reading from a connection or writing to the channel, and isn't fast enough to handle
// the cancel signal. In that case, the Stop() flow will close the channel and the goroutine will panic.
// This is handled by the recover() function in the defer statement.
func (be *AbstractEventConnection) RunHandler() {
	defer func() {
		if r := recover(); r != nil {
			// if channel is closed
			// it means resultChan was closed in stop - it is expected flow, no need to log anything here
			if err, ok := r.(error); ok && err.Error() == "send on closed channel" {
				return
			}
			be.Logger.WarnWith("Received unexpected panic in RunHandler",
				"error", r)
		}
	}()

	outReader := bufio.NewReader(be.Conn)

	defer close(be.cancelChan)

	// Read logs & output
	for {
		select {

		// TODO: sync between event and control output handlers using a shared context
		case <-be.cancelChan:
			//be.Logger.Warn("Event output handler was canceled (Restart called?)")
			return

		default:

			var data []byte
			var err error
			data, err = outReader.ReadBytes('\n')

			if err != nil {
				// if matches one of the errors and status is not ready, no need to log it, expected during restart/stop
				// if status is ready, we should always log the error
				if be.GetStatus() == status.Ready || !common.StringSliceContainsStringSuffix([]string{
					"EOF",
					"connection reset by peer",
					"use of closed network connection",
				}, errors.RootCause(err).Error()) {
					be.Logger.WarnWith(string(common.FailedReadFromEventConnection),
						"err", errors.RootCause(err).Error())
				}

				batchedResultsWithError := result.NewSingleResultsWithError(err)
				select {
				// if no receiver is waiting for the result, we should not send it
				// otherwise it may get stuck here and block select
				case be.resultChan <- batchedResultsWithError:
				default:
					// Explicitly clear batchedResultsWithError to release memory and indicate it's no longer needed.
					// Although the variable isn't used afterward, we assign nil here intentionally to help the
					// garbage collector reclaim memory early, especially if it held a large data structure.
					batchedResultsWithError = nil
					_ = batchedResultsWithError // suppress 'ineffassign' linter warning about unused assignment

				}
				continue
			}

			switch result.PacketType(data[0]) {
			case result.PacketTypeSingleResponse,
				result.PacketTypeBodyChunk,
				result.PacketTypeEndOfStream,
				result.PacketTypeStreamStart:
				be.resultChan <- result.NewResultFromData(data)
			case result.PacketTypeMetrics:
				be.handleResponseMetric(data[1:])
			case result.PacketTypeLog:
				be.handleResponseLog(data[1:])
			case result.PacketTypeStart:
				be.handleStart()
			}
		}
	}
}

// GetStatistics returns a pointer to the statistics object. This must not be modified by the reader
func (be *AbstractEventConnection) GetStatistics() *statistics.EventProcessingStatistics {
	return nil
}

func (be *AbstractEventConnection) StreamProcessedSuccessfully() {
}

// GetAllocationStatistics returns the statistics of the allocator if there is any in the runtime
// AbstractEventConnection runtime doesn't have any allocator in it, so return nil
func (be *AbstractEventConnection) GetAllocationStatistics() *statistics.AllocatorStatistics {
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

// SetStatus sets event connection status
func (be *AbstractEventConnection) SetStatus(newStatus status.Status) {
	be.status.SetStatus(newStatus)
}

// Stop stops the connection
// This is the only place where the connection object should be properly stopped
// meaning closing all channels and connection
func (be *AbstractEventConnection) Stop() (err error) {
	be.stopOnce.Do(func() {
		err = be.stop()
	})
	return err
}

// GetStructuredCloudEvent return a structued clould event
func (be *AbstractEventConnection) GetStructuredCloudEvent() *cloudevent.Structured {
	return nil
}

// GetBinaryCloudEvent return a binary cloud event
func (be *AbstractEventConnection) GetBinaryCloudEvent() *cloudevent.Binary {
	return nil
}

func (be *AbstractEventConnection) RestartRequired() bool {
	return false
}

func (be *AbstractEventConnection) Replace(newConnection *AbstractEventConnection) {
	// stop current connection
	if err := be.Stop(); err != nil {
		be.Logger.WarnWith("Failed to stop connection",
			"error", err)
	}
	// reset stopOnce to allow stop() to be called again
	be.stopOnce = sync.Once{}

	// replace all entities with new connection
	be.AbstractConnection = newConnection.AbstractConnection
	be.resultChan = newConnection.resultChan
	be.startChan = newConnection.startChan
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
	// same as stop
	return be.Stop()
}

func (be *AbstractEventConnection) Drain() error {
	return nuclio.ErrNotImplemented
}

func (be *AbstractEventConnection) IsAsync() bool {
	return be.connectionManager.IsAsync()
}

func (be *AbstractEventConnection) IsBusy() bool {
	// aligns eventProcessor interfaces
	// should not be used
	return false
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

func (be *AbstractEventConnection) processItem(item interface{}, functionLogger logger.Logger) (result.Result, error) {
	be.functionLogger = functionLogger
	if err := be.encoder.Encode(item); err != nil {
		be.functionLogger = nil
		return nil, errors.Wrapf(err, "Can't encode item: %+v", item)
	}
	return be.waitForResponseWithOptionalTimeout(
		be.connectionManager.GetConfig().eventTimeout,
		be.postProcessEventRegularFlow,
		be.postProcessEventOnTimeout)
}

func (be *AbstractEventConnection) waitForNextResponseChunk() (result.Result, error) {
	return be.waitForResponseWithOptionalTimeout(be.connectionManager.GetConfig().chunkTimeout,
		be.postProcessResponseCheckForFailure,
		be.postProcessEventOnTimeout)
}

func (be *AbstractEventConnection) stop() error {
	be.SetStatus(status.Stopping)
	be.AbstractConnection.Stop()

	var err error
	// close start chan
	// other two channels (result and cancel chan) are closed in the run handler
	close(be.startChan)

	// if the channel is closed while waiting for response in it, this will be handled in processItem() with no issues
	close(be.resultChan)

	if be.Conn != nil {
		err = be.Conn.Close()
	}

	be.SetStatus(status.Stopped)
	return err
}

// waitForResponseWithOptionalTimeout waits for a response on be.resultChan,
// with optional timeout logic controlled by the provided duration.
//
// Behavior:
//   - If timeout == 0: waits indefinitely for a response and calls postProcessRegularFlow.
//   - If timeout > 0: waits for either a response or a timeout.
//   - On response: calls postProcessRegularFlow.
//   - On timeout: calls postProcessOnTimeout.
//
// Parameters:
//   - timeout: duration to wait before triggering timeout flow. 0 means wait indefinitely.
//   - postProcessRegularFlow: handler to call on receiving a result.
//   - postProcessOnTimeout: handler to call if timeout elapses before a result arrives.
//
// Returns:
//   - result.Result: the processed result or nil on timeout.
//   - error: error from post-processing callbacks.
func (be *AbstractEventConnection) waitForResponseWithOptionalTimeout(
	timeout time.Duration,
	postProcessRegularFlow func(processingResults result.Result, isClientDisconnected bool) (result.Result, error),
	postProcessOnTimeout func() error,
) (result.Result, error) {

	// Wait indefinitely if no timeout set
	if timeout == 0 {
		processingResults, ok := <-be.resultChan
		return postProcessRegularFlow(processingResults, !ok)
	}

	// Otherwise wait for response or timeout
	ticker := time.NewTicker(timeout)
	defer ticker.Stop()

	select {
	case processingResults, ok := <-be.resultChan:
		return postProcessRegularFlow(processingResults, !ok)
	case <-ticker.C:
		return nil, postProcessOnTimeout()
	}
}

func (be *AbstractEventConnection) postProcessEventRegularFlow(processingResults result.Result, isClientDisconnected bool) (result.Result, error) {
	// We don't use defer to reset be.functionLogger since it decreases performance
	if !processingResults.IsStream() || isClientDisconnected {
		// Instead, we reset it immediately after execution **only if**:
		// - the result is not part of a stream (i.e., IsStream() is false), **or**
		// - the client has disconnected.
		// This ensures we clean up the logger in non-streaming scenarios or when streaming is no longer relevant.
		be.resetLogger()
	}

	return be.postProcessResponseCheckForFailure(processingResults, isClientDisconnected)
}

func (be *AbstractEventConnection) postProcessStreaming(stream *nuclio.ResponseStream) {
	stream.StopStreaming()
	be.resetLogger()
}

func (be *AbstractEventConnection) resetLogger() {
	be.functionLogger = nil
}

func (be *AbstractEventConnection) postProcessResponseCheckForFailure(processingResults result.Result, isClientDisconnected bool) (result.Result, error) {

	if isClientDisconnected {
		return nil, be.postProcessClientDisconnected()
	}
	// if processingResults.err is not nil, it means that whole batch processing has failed
	// or that connection was closed (EOF)
	if processingResults.Error() != nil {
		// if a client disconnected, we should restart the connection
		if errors.RootCause(processingResults.Error()) == io.EOF {
			return nil, be.postProcessClientDisconnected()
		}
		return nil, processingResults.Error()
	}
	return processingResults, nil
}

func (be *AbstractEventConnection) postProcessClientDisconnected() error {
	msg := "Client disconnected"
	be.Logger.Error(msg)
	be.SetStatus(status.RestartRequired)
	return errors.New(msg)
}

func (be *AbstractEventConnection) postProcessEventOnTimeout() error {
	be.resetLogger()
	be.Logger.WarnWith("Event processing timed out, connection should be restarted",
		"localAddress", be.Conn.LocalAddr().String())
	be.SetStatus(status.RestartRequired)
	return nuclio.NewErrRequestTimeout("Execution timed out")
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

func (bc *AbstractControlMessageConnection) Stop() {
	bc.stopOnce.Do(func() {
		bc.AbstractConnection.Stop()
	})
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
