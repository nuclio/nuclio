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

package common

import (
	"context"

	"github.com/nuclio/logger"
)

type CatchAndLogPanicOptions struct {
	Args          []interface{}
	CustomHandler func(error)
}

type ExportFunctionOptions struct {
	NoScrub     bool
	CleanupSpec bool
	PrevState   string
}

// ChannelWithRecover wraps a context.Context and a Channel, providing a safe way to write
// data to the Channel with panic recovery
type ChannelWithRecover struct {
	context.Context
	Channel chan interface{}
}

// Write writes the specified object to the Channel, handling any panics that may occur
// during the operation and logging them using the provided logger.
// If the associated context is canceled (via `Done()`), the write operation is aborted.
// Otherwise, the objectToWrite is sent to the Channel.
// This method is designed to recover from panics during the write operation
func (c *ChannelWithRecover) Write(logger logger.Logger, objectToWrite interface{}) {

	// recover from any panic that may occur during the write operation
	defer func() {
		if r := recover(); r != nil {
			// handle the panic: log the error and return without crashing
			logger.WarnWith("Panic occurred during write operation to the Channel", "error", r)
		}
	}()
	// attempt to write to the Channel or return if the context is canceled
	select {
	case <-c.Done():
		return
	case c.Channel <- objectToWrite:
		return
	}
}
