package processwaiter

import (
	"errors"
	"os"
	"time"
)

var ErrCancelled = errors.New("Wait cancelled")
var ErrTimeout = errors.New("Timed out waiting for process to exit")

type ProcessWaiter struct {
	cancelChan chan struct{}
	resultChan chan WaitResult
}

type WaitResult struct {
	ProcessState *os.ProcessState
	Err          error
}

func NewProcessWaiter() (*ProcessWaiter, error) {
	return &ProcessWaiter{
		resultChan: make(chan WaitResult, 1),
		cancelChan: make(chan struct{}, 1),
	}, nil
}

func (pw *ProcessWaiter) Wait(process *os.Process, timeout *time.Duration) <-chan WaitResult {
	var timeoutChan <-chan time.Time

	if timeout != nil {
		timeoutChan = time.After(*timeout)
	}

	processExitedChan := make(chan WaitResult)

	go func() {

		// run a go process to block on process. terminates only when the process terminates
		go pw.waitForProcess(process, processExitedChan)

		select {
		case <-timeoutChan:
			pw.resultChan <- WaitResult{nil, ErrTimeout}
		case waitResult := <-processExitedChan:

			// check if cancelled (could be that both cancelled and process exited at the same time)
			// and prefer that over a process termination
			select {
			case <-pw.cancelChan:
				pw.resultChan <- WaitResult{nil, ErrCancelled}
			default:
				pw.resultChan <- waitResult
			}
		case <-pw.cancelChan:
			pw.resultChan <- WaitResult{nil, ErrCancelled}
		}
	}()

	return pw.resultChan
}

func (pw *ProcessWaiter) Cancel() error {
	select {
	case pw.cancelChan <- struct{}{}:
	default:
		// already cancelled
	}

	return nil
}

func (pw *ProcessWaiter) waitForProcess(process *os.Process, processExitedChan chan WaitResult) {
	processState, err := process.Wait()

	// shove the error into the channel when we're done
	processExitedChan <- WaitResult{processState, err}

	close(processExitedChan)
}
