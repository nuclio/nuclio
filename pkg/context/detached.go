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

package context

import (
	"context"
	"time"
)

// Detached enables creating a copy of a context with all of its values,
// but cancelling the parent context will not cancel the detached.
// Used when forwarding a request context to a goroutine, and when the request
// context is cancelled the goroutine can continue its operation.
type Detached struct {
	ctx context.Context
}

// NewDetached returns a context that is not affected by its parent.
func NewDetached(ctx context.Context) *Detached {
	return &Detached{ctx: ctx}
}

func (d *Detached) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

func (d *Detached) Done() <-chan struct{} {
	return nil
}

func (d *Detached) Err() error {
	return nil
}

func (d *Detached) Value(key interface{}) interface{} {
	return d.ctx.Value(key)
}
