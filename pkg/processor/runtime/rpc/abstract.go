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
	"encoding/base64"
	"encoding/json"
	"fmt"
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
)

// AbstractRuntime is a runtime that communicates via unix domain socket
type AbstractRuntime struct {
	runtime.AbstractRuntime
	configuration  *runtime.Configuration
	wrapperProcess *os.Process
	functionLogger logger.Logger
	runtime        Runtime
	stopChan       chan struct{}
	socketType     SocketType
	processWaiter  *processwaiter.ProcessWaiter

	socketAllocator *SocketAllocator
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
	processingResult, err := r.processItemAndWaitForResult(event, functionLogger)
	if err != nil {
		return nil, err
	}
	// this is a single event processing flow, so we only take the first item from the result
	return nuclio.Response{
		Body:        processingResult.results[0].DecodedBody,
		ContentType: processingResult.results[0].ContentType,
		Headers:     processingResult.results[0].Headers,
		StatusCode:  processingResult.results[0].StatusCode,
	}, processingResult.results[0].err
}

// ProcessBatch processes a batch of events
func (r *AbstractRuntime) ProcessBatch(batch []nuclio.Event, functionLogger logger.Logger) ([]*runtime.ResponseWithErrors, error) {
	processingResults, err := r.processItemAndWaitForResult(batch, functionLogger)
	if err != nil {
		return nil, err
	}
	responsesWithErrors := make([]*runtime.ResponseWithErrors, len(processingResults.results))

	for index, processingResult := range processingResults.results {
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
			ProcessError: processingResult.err,
		}
	}

	return responsesWithErrors, nil
}

// Stop stops the abstractRuntime
func (r *AbstractRuntime) Stop() error {
	r.Logger.WarnWith("Stopping",
		"status", r.GetStatus(),
		"wrapperProcess", r.wrapperProcess)

	// move to `stopped` state before actually stopping it
	// to avoid sending any events while stopping
	r.SetStatus(status.Stopped)

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
	r.Logger.Warn("Successfully stopped wrapper process")
	return nil
}

// Restart restarts the runtime
func (r *AbstractRuntime) Restart() error {
	if err := r.Stop(); err != nil {
		return err
	}

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

	// do not send a signal if the runtime isn't ready,
	// because the signal handler may not be initialized yet.
	// if the process receives a signal before the handler is set up,
	// the default behaviour will cause the Linux process to terminate.
	if r.GetStatus() != status.Ready {
		return nil
	}

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

func (r *AbstractRuntime) processItemAndWaitForResult(item interface{}, functionLogger logger.Logger) (*batchedResults, error) {

	if currentStatus := r.GetStatus(); currentStatus != status.Ready {
		return nil, errors.Errorf("Processor not ready (current status: %s)", currentStatus)
	}

	socket := r.socketAllocator.Allocate()

	r.functionLogger = functionLogger
	defer func() {
		r.functionLogger = nil
	}()

	return socket.processEvent(item)
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
	r.socketAllocator = NewSocketAllocator(r.Logger.GetChild("socketAllocator"), r)
	var err error
	if err = r.socketAllocator.createSockets(); err != nil {
		return errors.Wrap(err, "Failed to start sockets")
	}

	if r.processWaiter, err = processwaiter.NewProcessWaiter(); err != nil {
		return errors.Wrap(err, "Failed to create process waiter")
	}

	wrapperProcess, err := r.runtime.RunWrapper(r.socketAllocator.getSocketAddresses())
	if err != nil {
		return errors.Wrap(err, "Can't run wrapper")
	}

	r.wrapperProcess = wrapperProcess

	go r.watchWrapperProcess()

	if err := r.socketAllocator.start(); err != nil {
		return errors.Wrap(err, "Failed to start socket allocator")
	}

	return nil
}

// resolveFunctionLogger return either functionLogger if provided or root Logger if not
func (r *AbstractRuntime) resolveFunctionLogger() logger.Logger {
	if r.functionLogger == nil {
		return r.Logger
	}
	return r.functionLogger
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

	var errorMsg string
	if processWaitResult.Err != nil {
		errorMsg = processWaitResult.Err.Error()
	}

	r.Logger.ErrorWith(string(common.UnexpectedTerminationChildProcess),
		"error", errorMsg,
		"status", processWaitResult.ProcessState.String(),
		"exitCode", processWaitResult.ProcessState.ExitCode(),
		"pid", r.wrapperProcess.Pid,
	)

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

func unmarshalResponseData(logger logger.Logger, data []byte, unmarshalledResults *batchedResults) {
	var results []*result

	// define method to process a single result
	handleSingleUnmarshalledResult := func(unmarshalledResult *result) {
		switch unmarshalledResult.BodyEncoding {
		case "text":
			unmarshalledResult.DecodedBody = []byte(unmarshalledResult.Body)
		case "base64":
			unmarshalledResult.DecodedBody, unmarshalledResults.err = base64.StdEncoding.DecodeString(unmarshalledResult.Body)
		default:
			unmarshalledResult.err = fmt.Errorf("Unknown body encoding - %q", unmarshalledResult.BodyEncoding)
		}
	}

	if unmarshalledResults.err = json.Unmarshal(data, &results); unmarshalledResults.err != nil {
		// try to unmarshall data as a single result
		var singleResult *result
		if unmarshalledResults.err = json.Unmarshal(data, &singleResult); unmarshalledResults.err != nil {
			logger.DebugWith("Failed to unmarshal result",
				"err", unmarshalledResults.err.Error())
			return
		} else {
			handleSingleUnmarshalledResult(singleResult)
			unmarshalledResults.results = append(unmarshalledResults.results, singleResult)
			return
		}
	}

	unmarshalledResults.results = results
	for _, unmarshalledResult := range unmarshalledResults.results {
		handleSingleUnmarshalledResult(unmarshalledResult)
	}
}
