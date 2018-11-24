// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package status contains code for monitoring the golang.org infrastructure.
// It is not intended for use outside of the Go project.
package status

import (
	"os"
	"sync"
	"time"

	"cloud.google.com/go/compute/metadata"
)

var (
	statusTokenMu  sync.Mutex
	statusTokenVal string
	statusTokenErr error
	lastCheck      time.Time
)

// UpdateToken returns the value of "monitor-probe-token" from the GCE
// metadata server, or from the environment variable
// $DEV_STATUS_UPDATE_TOKEN. This is the the shared secret that must
// be sent to dev.golang.org in prober status update requests.
// Most probers will not use this directly.
func UpdateToken() (string, error) {
	statusTokenMu.Lock()
	defer statusTokenMu.Unlock()
	if statusTokenVal != "" {
		return statusTokenVal, nil
	}
	if lastCheck.After(time.Now().Add(-10 * time.Second)) {
		return "", statusTokenErr
	}
	if v := os.Getenv("DEV_STATUS_UPDATE_TOKEN"); v != "" {
		statusTokenVal = v
		return v, nil
	}
	v, err := metadata.Get("monitor-probe-token")
	if err == nil {
		statusTokenVal = v
		return v, nil
	}
	lastCheck = time.Now()
	statusTokenErr = err
	return "", err
}
