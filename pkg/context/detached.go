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
