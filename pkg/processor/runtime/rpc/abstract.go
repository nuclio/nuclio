/*
Copyright 2023 The Nuclio Authors.

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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processwaiter"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/rs/xid"
)

// TODO: Find a better place (both on file system and configuration)
const (
	socketPathTemplate = "/tmp/nuclio-rpc-%s.sock"
	connectionTimeout  = 2 * time.Minute
)

type socketConnection struct {
	conn     net.Conn
	listener net.Listener
	address  string
}

type result struct {
	StatusCode   int                    `json:"status_code"`
	ContentType  string                 `json:"content_type"`
	Body         string                 `json:"body"`
	BodyEncoding string                 `json:"body_encoding"`
	Headers      map[string]interface{} `json:"headers"`

	DecodedBody []byte
	err         error
}

// AbstractRuntime is a runtime that communicates via unix domain socket
type AbstractRuntime struct {
	runtime.AbstractRuntime
	configuration     *runtime.Configuration
	eventEncoder      EventEncoder
	controlEncoder    EventEncoder
	wrapperProcess    *os.Process
	resultChan        chan *result
	functionLogger    logger.Logger
	runtime           Runtime
	startChan         chan struct{}
	stopChan          chan struct{}
	cancelHandlerChan chan struct{}
	socketType        SocketType
	processWaiter     *processwaiter.ProcessWaiter
}

type rpcLogRecord struct {
	DateTime string                 `json:"datetime"`
	Level    string                 `json:"level"`
	Message  string                 `json:"message"`
	With     map[string]interface{} `json:"with"`
}

// NewAbstractRuntime returns a new RPC runtime
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
		stopChan:        make(chan struct{}, 1),
		socketType:      UnixSocket,
	}

	return newRuntime, nil
}

func (r *AbstractRuntime) Start() error {
	if err := r.startWrapper(); err != nil {
		r.SetStatus(status.Error)
		return errors.Wrap(err, "Failed to run wrapper")
	}

	r.SetStatus(status.Ready)
	return nil
}

// ProcessEvent processes an event
func (r *AbstractRuntime) ProcessEvent(event nuclio.Event, functionLogger logger.Logger) (interface{}, error) {
	if currentStatus := r.GetStatus(); currentStatus != status.Ready {
		return nil, errors.Errorf("Processor not ready (current status: %s)", currentStatus)
	}

	r.functionLogger = functionLogger

	// We don't use defer to reset r.functionLogger since it decreases performance
	if err := r.eventEncoder.Encode(event); err != nil {
		r.functionLogger = nil
		return nil, errors.Wrapf(err, "Can't encode event: %+v", event)
	}

	result, ok := <-r.resultChan
	r.functionLogger = nil
	if !ok {
		msg := "Client disconnected"
		r.Logger.Error(msg)
		r.SetStatus(status.Error)
		r.functionLogger = nil
		return nil, errors.New(msg)
	}

	return nuclio.Response{
		Body:        result.DecodedBody,
		ContentType: result.ContentType,
		Headers:     result.Headers,
		StatusCode:  result.StatusCode,
	}, result.err
}

// Stop stops the runtime
func (r *AbstractRuntime) Stop() error {
	r.Logger.WarnWith("Stopping",
		"status", r.GetStatus(),
		"wrapperProcess", r.wrapperProcess)

	if r.wrapperProcess != nil {

		// stop waiting for process
		if err := r.processWaiter.Cancel(); err != nil {
			r.Logger.WarnWith("Failed to cancel process waiting")
		}

		r.Logger.WarnWith("Killing wrapper process", "wrapperProcessPid", r.wrapperProcess.Pid)
		if err := r.wrapperProcess.Kill(); err != nil {
			r.SetStatus(status.Error)
			return errors.Wrap(err, "Can't kill wrapper process")
		}
	}

	r.waitForProcessTermination(10 * time.Second)

	r.wrapperProcess = nil

	r.SetStatus(status.Stopped)
	r.Logger.Warn("Successfully stopped wrapper process")
	return nil
}

// Restart restarts the runtime
func (r *AbstractRuntime) Restart() error {
	if err := r.Stop(); err != nil {
		return err
	}

	// Send error for current event (non-blocking)
	select {
	case r.resultChan <- &result{
		StatusCode: http.StatusRequestTimeout,
		err:        errors.New("Runtime restarted"),
	}:

	default:
		r.Logger.Warn("Nothing waiting on result channel during restart. Continuing")
	}

	close(r.resultChan)
	if err := r.startWrapper(); err != nil {
		r.SetStatus(status.Error)
		return errors.Wrap(err, "Can't start wrapper process")
	}

	r.SetStatus(status.Ready)
	return nil
}

// GetSocketType returns the type of socket the runtime works with (unix/tcp)
func (r *AbstractRuntime) GetSocketType() SocketType {
	return r.socketType
}

// WaitForStart returns whether the runtime supports sending an indication that it started
func (r *AbstractRuntime) WaitForStart() bool {
	return false
}

// SupportsRestart returns true if the runtime supports restart
func (r *AbstractRuntime) SupportsRestart() bool {
	return true
}

// SupportsControlCommunication returns true if the runtime supports control communication
func (r *AbstractRuntime) SupportsControlCommunication() bool {
	return false
}

// Drain signals to the runtime to drain its accumulated events and waits for it to finish
func (r *AbstractRuntime) Drain() error {
	// we use SIGUSR2 to signal the wrapper process to drain events
	if err := r.signal(syscall.SIGUSR2); err != nil {
		return errors.Wrap(err, "Failed to signal wrapper process to drain")
	}

	// wait for process to finish event handling or timeout
	// TODO: replace the following function with one that waits for a control communication message or timeout
	r.waitForProcessTermination(r.configuration.WorkerTerminationTimeout)

	return nil
}

// Continue signals the runtime to continue event processing
func (r *AbstractRuntime) Continue() error {
	// we use SIGCONT to signal the wrapper process to continue event processing
	if err := r.signal(syscall.SIGCONT); err != nil {
		return errors.Wrap(err, "Failed to signal wrapper process to continue")
	}

	return nil
}

// Terminate signals to the runtime process that processor is about to stop working
func (r *AbstractRuntime) Terminate() error {

	// we use SIGUSR1 to signal the wrapper process to terminate
	if err := r.signal(syscall.SIGUSR1); err != nil {
		return errors.Wrap(err, "Failed to signal wrapper process to terminate")
	}

	// wait for process to finish event handling or timeout
	// TODO: replace the following function with one that waits for a control communication message or timeout
	r.waitForProcessTermination(r.configuration.WorkerTerminationTimeout)

	return nil
}

func (r *AbstractRuntime) signal(signal syscall.Signal) error {

	if r.wrapperProcess != nil {
		r.Logger.DebugWith("Signaling wrapper process",
			"pid", r.wrapperProcess.Pid,
			"signal", signal.String())

		if err := r.wrapperProcess.Signal(signal); err != nil {
			r.Logger.WarnWith("Failed to signal wrapper process",
				"pid", r.wrapperProcess.Pid,
				"signal", signal.String())
		}
	} else {
		r.Logger.DebugWith("No wrapper process exists, skipping signal")
	}

	return nil
}

func (r *AbstractRuntime) startWrapper() error {
	var (
		err                                error
		eventConnection, controlConnection socketConnection
	)

	// create socket connections
	if err := r.createSocketConnection(&eventConnection); err != nil {
		return errors.Wrap(err, "Failed to create socket connection")
	}

	if r.runtime.SupportsControlCommunication() {
		if err := r.createSocketConnection(&controlConnection); err != nil {
			return errors.Wrap(err, "Failed to create socket connection")
		}
	}

	r.processWaiter, err = processwaiter.NewProcessWaiter()
	if err != nil {
		return errors.Wrap(err, "Failed to create process waiter")
	}

	wrapperProcess, err := r.runtime.RunWrapper(eventConnection.address, controlConnection.address)
	if err != nil {
		return errors.Wrap(err, "Can't run wrapper")
	}

	r.wrapperProcess = wrapperProcess

	go r.watchWrapperProcess()

	// event connection
	eventConnection.conn, err = eventConnection.listener.Accept()
	if err != nil {
		return errors.Wrap(err, "Can't get connection from wrapper")
	}

	r.Logger.InfoWith("Wrapper connected",
		"wid", r.Context.WorkerID,
		"pid", r.wrapperProcess.Pid)

	r.eventEncoder = r.runtime.GetEventEncoder(eventConnection.conn)
	r.resultChan = make(chan *result)
	r.cancelHandlerChan = make(chan struct{})
	go r.eventWrapperOutputHandler(eventConnection.conn, r.resultChan)

	// control connection
	if r.runtime.SupportsControlCommunication() {

		r.Logger.DebugWith("Creating control connection",
			"wid", r.Context.WorkerID)
		controlConnection.conn, err = controlConnection.listener.Accept()
		if err != nil {
			return errors.Wrap(err, "Can't get control connection from wrapper")
		}

		r.controlEncoder = r.runtime.GetEventEncoder(controlConnection.conn)

		// initialize control message broker
		r.ControlMessageBroker = NewRpcControlMessageBroker(r.controlEncoder, r.Logger, r.configuration.ControlMessageBroker)

		go r.controlOutputHandler(controlConnection.conn)

		r.Logger.DebugWith("Control connection created",
			"wid", r.Context.WorkerID)
	}

	// wait for start if required to
	if r.runtime.WaitForStart() {
		r.Logger.Debug("Waiting for start")

		<-r.startChan
	}

	r.Logger.Debug("Started")

	return nil
}

// Create a listener on unix domain docker, return listener, path to socket and error
func (r *AbstractRuntime) createSocketConnection(connection *socketConnection) error {
	var err error
	if r.runtime.GetSocketType() == UnixSocket {
		connection.listener, connection.address, err = r.createUnixListener()
	} else {
		connection.listener, connection.address, err = r.createTCPListener()
	}

	if err != nil {
		return errors.Wrap(err, "Can't create listener")
	}

	return nil
}

// Create a listener on unix domain docker, return listener, path to socket and error
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

func (r *AbstractRuntime) eventWrapperOutputHandler(conn io.Reader, resultChan chan *result) {

	// Reset might close outChan, which will cause panic when sending
	defer common.CatchAndLogPanicWithOptions(context.Background(), // nolint: errcheck
		r.Logger,
		"handling event wrapper output (Restart called?)",
		&common.CatchAndLogPanicOptions{
			Args:          nil,
			CustomHandler: nil,
		})
	defer func() {
		r.cancelHandlerChan <- struct{}{}
	}()

	outReader := bufio.NewReader(conn)

	// Read logs & output
	for {
		select {

		// TODO: sync between event and control output handlers using a shared context
		case <-r.cancelHandlerChan:
			r.Logger.Warn("Event output handler was canceled (Restart called?)")
			return

		default:

			unmarshalledResult := &result{}
			var data []byte

			data, unmarshalledResult.err = outReader.ReadBytes('\n')

			if unmarshalledResult.err != nil {
				r.Logger.WarnWith(string(common.FailedReadFromEventConnection),
					"err", unmarshalledResult.err.Error())
				resultChan <- unmarshalledResult
				continue
			}

			switch data[0] {
			case 'r':

				// try to unmarshall the result
				if unmarshalledResult.err = json.Unmarshal(data[1:], unmarshalledResult); unmarshalledResult.err != nil {
					r.Logger.WarnWith("Failed to unmarshal result", "err", unmarshalledResult.err.Error())
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
				resultChan <- unmarshalledResult
			case 'm':
				r.handleResponseMetric(data[1:])
			case 'l':
				r.handleResponseLog(data[1:])
			case 's':
				r.handleStart()
			}
		}
	}
}

func (r *AbstractRuntime) controlOutputHandler(conn io.Reader) {

	// recover from panic in case of error
	defer common.CatchAndLogPanicWithOptions(context.Background(), // nolint: errcheck
		r.Logger,
		"control wrapper output handler (Restart called?)",
		&common.CatchAndLogPanicOptions{
			Args:          nil,
			CustomHandler: nil,
		})
	defer func() {
		r.cancelHandlerChan <- struct{}{}
	}()

	outReader := bufio.NewReader(conn)

	// keep a counter for log throttling
	errLogCounter := 0
	logCounterTime := time.Now()

	for {
		select {

		// TODO: sync between event and control output handlers using a shared context
		case <-r.cancelHandlerChan:
			r.Logger.Warn("Control output handler was canceled (Restart called?)")
			return

		default:

			// read control message
			controlMessage, err := r.ControlMessageBroker.ReadControlMessage(outReader)
			if err != nil {

				// if enough time has passed, log the error
				if time.Since(logCounterTime) > 500*time.Millisecond {
					logCounterTime = time.Now()
					errLogCounter = 0
				}
				if errLogCounter%5 == 0 {
					r.Logger.WarnWith(string(common.FailedReadControlMessage),
						"errRootCause", errors.RootCause(err).Error())
					errLogCounter++
				}

				// if error is EOF it means the connection was closed, so we should exit
				if errors.RootCause(err) == io.EOF {
					r.Logger.Debug("Control connection was closed")
					return
				}

				continue
			} else {
				errLogCounter = 0
			}

			r.Logger.DebugWith("Received control message", "messageKind", controlMessage.Kind)

			// send message to control consumers
			if err := r.GetControlMessageBroker().SendToConsumers(controlMessage); err != nil {
				r.Logger.WarnWith("Failed to send control message to consumers", "err", err.Error())
			}

			// TODO: validate and respond to wrapper process
		}
	}
}

func (r *AbstractRuntime) handleResponseLog(response []byte) {
	var logRecord rpcLogRecord

	if err := json.Unmarshal(response, &logRecord); err != nil {
		r.Logger.ErrorWith("Can't decode log", "error", err)
		return
	}

	loggerInstance := r.resolveFunctionLogger(r.functionLogger)
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

func (r *AbstractRuntime) handleResponseMetric(response []byte) {
	var metrics struct {
		DurationSec float64 `json:"duration"`
	}

	loggerInstance := r.resolveFunctionLogger(r.functionLogger)
	if err := json.Unmarshal(response, &metrics); err != nil {
		loggerInstance.ErrorWith("Can't decode metric", "error", err)
		return
	}

	if metrics.DurationSec == 0 {
		loggerInstance.ErrorWith("No duration in metrics", "metrics", metrics)
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

func (r *AbstractRuntime) watchWrapperProcess() {

	// whatever happens, clear wrapper process
	defer func() {
		r.stopChan <- struct{}{}
	}()

	// wait for the process
	processWaitResult := <-r.processWaiter.Wait(r.wrapperProcess, nil)

	// if we were simply canceled, do nothing
	if processWaitResult.Err == processwaiter.ErrCancelled {
		r.Logger.DebugWith("Process watch cancelled. Returning",
			"pid", r.wrapperProcess.Pid,
			"wid", r.Context.WorkerID)
		return
	}

	// if process exited gracefully (i.e. wasn't force killed), do nothing
	if processWaitResult.Err == nil && processWaitResult.ProcessState.Success() {
		r.Logger.DebugWith("Process watch done - process exited successfully")
		return
	}

	r.Logger.ErrorWith(string(common.UnexpectedTerminationChildProcess),
		"error", processWaitResult.Err,
		"status", processWaitResult.ProcessState.String())

	var panicMessage string
	if processWaitResult.Err != nil {
		panicMessage = processWaitResult.Err.Error()
	} else {
		panicMessage = processWaitResult.ProcessState.String()
	}

	panic(fmt.Sprintf("Wrapper process for worker %d exited unexpectedly with: %s", r.Context.WorkerID, panicMessage))
}

// waitForProcessTermination will best effort wait few seconds to stop channel, if timeout - assume closed
func (r *AbstractRuntime) waitForProcessTermination(timeout time.Duration) {
	r.Logger.DebugWith("Waiting for process termination",
		"wid", r.Context.WorkerID,
		"process", r.wrapperProcess,
		"timeout", timeout.String())

	for {
		select {
		case <-r.stopChan:
			r.Logger.DebugWith("Process terminated",
				"wid", r.Context.WorkerID,
				"process", r.wrapperProcess)
			return
		case <-time.After(timeout):
			r.Logger.DebugWith("Timeout waiting for process termination, assuming closed",
				"wid", r.Context.WorkerID,
				"process", r.wrapperProcess)
			return
		}
	}
}
