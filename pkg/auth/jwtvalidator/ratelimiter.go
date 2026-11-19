/*
Copyright 2026 The Nuclio Authors.

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

package jwtvalidator

import (
	"context"
	"sync"
	"time"

	"github.com/nuclio/errors"
	"golang.org/x/sync/semaphore"
)

// downloadRateLimiter decides when the key set may be downloaded, and limits how many requests are held for it.
type downloadRateLimiter struct {
	sync.Mutex
	gap            time.Duration
	backoff        time.Duration
	lastDownloadAt time.Time
	backoffUntil   time.Time
	pending        *semaphore.Weighted
}

// newDownloadRateLimiter creates a rate limiter
func newDownloadRateLimiter(gap, backoff time.Duration, maxPending int) *downloadRateLimiter {
	return &downloadRateLimiter{
		gap:     gap,
		backoff: backoff,
		pending: semaphore.NewWeighted(int64(maxPending)),
	}
}

// acquirePendingSlot lets the request be held for a download. When every slot is taken, the request waits
// for a free one until ctx is done. It fails while downloads are backing off after a failure. A request that
// got a slot must release it.
func (limiter *downloadRateLimiter) acquirePendingSlot(ctx context.Context) error {
	if err := limiter.pending.Acquire(ctx, 1); err != nil {
		return errors.Wrap(err, "Failed to acquire a slot for a key set download")
	}

	limiter.Lock()
	backingOff := time.Now().Before(limiter.backoffUntil)
	limiter.Unlock()

	// a download that failed while this request was queued for a slot started the back-off
	if backingOff {
		limiter.pending.Release(1)
		return errors.New("Key set download is backing off")
	}
	return nil
}

// releasePendingSlot gives back a slot taken by acquirePendingSlot.
func (limiter *downloadRateLimiter) releasePendingSlot() {
	limiter.pending.Release(1)
}

// delayForDownloadGap sleeps until the gap since the last download has passed.
func (limiter *downloadRateLimiter) delayForDownloadGap() {
	limiter.Lock()
	delay := time.Until(limiter.lastDownloadAt.Add(limiter.gap))
	limiter.Unlock()

	if delay > 0 {
		time.Sleep(delay)
	}
}

// finishDownload records the end of a download, and starts the back-off when it failed.
func (limiter *downloadRateLimiter) finishDownload(downloadErr error) {
	limiter.Lock()
	defer limiter.Unlock()

	limiter.lastDownloadAt = time.Now()
	if downloadErr != nil {
		limiter.backoffUntil = limiter.lastDownloadAt.Add(limiter.backoff)
	}
}

// reset clears the gap and the back-off, so the next download may start at once.
func (limiter *downloadRateLimiter) reset() {
	limiter.Lock()
	defer limiter.Unlock()

	limiter.lastDownloadAt = time.Time{}
	limiter.backoffUntil = time.Time{}
}
