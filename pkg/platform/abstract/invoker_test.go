//go:build test_unit

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

package abstract

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/zap"
	"github.com/stretchr/testify/require"
)

// TestInvokeDoesNotReuseStaleConnection guards against a regression of NUC-902: a redeploy
// replaces the pod behind a function's stable service address between two invokes. The first
// invoke's connection must never be handed back out to the second invoke once that peer has
// gone quiet, or the second invoke hangs against a dead connection instead of dialing the pod
// that's actually listening.
func TestInvokeDoesNotReuseStaleConnection(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close() // nolint: errcheck

	stall := make(chan struct{})
	defer close(stall)

	var connCount atomic.Int32
	go acceptLoop(listener, &connCount, stall)

	testLogger, err := nucliozap.NewNuclioZapTest("test")
	require.NoError(t, err)

	inv, err := newInvoker(testLogger, nil)
	require.NoError(t, err)

	functionInstance, err := platform.NewAbstractFunction(testLogger, nil,
		&functionconfig.Config{
			Meta: functionconfig.Meta{Name: "test-func"},
		},
		&functionconfig.Status{
			InternalInvocationURLs: []string{listener.Addr().String()},
		},
		nil)
	require.NoError(t, err)

	invokeOptions := &platform.CreateFunctionInvocationOptions{
		Method:           "GET",
		Headers:          http.Header{},
		LogLevelName:     "none",
		Timeout:          2 * time.Second,
		URL:              listener.Addr().String(),
		FunctionInstance: functionInstance,
	}

	// first invoke: hits the "old pod" (connection #1), which answers once and then goes
	// silent on that same connection, simulating a pod that a redeploy is about to replace
	result, err := inv.invoke(context.Background(), invokeOptions)
	require.NoError(t, err)
	require.Equal(t, "old-pod", string(result.Body))

	// second invoke, same URL: must reach the "new pod" (a fresh connection), not hang against
	// connection #1's now-silent peer
	result, err = inv.invoke(context.Background(), invokeOptions)
	require.NoError(t, err)
	require.Equal(t, "new-pod", string(result.Body))
}

func acceptLoop(listener net.Listener, connCount *atomic.Int32, stall chan struct{}) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		go serveOneConnection(conn, connCount.Add(1) == 1, stall)
	}
}

// serveOneConnection answers every request on conn with a 200. If isFirstConnection, it goes
// silent after the first response instead of reading/answering further requests on this same
// connection - standing in for a pod that's gone but whose old connection is still open.
func serveOneConnection(conn net.Conn, isFirstConnection bool, stall chan struct{}) {
	defer conn.Close() // nolint: errcheck

	req, err := http.ReadRequest(bufio.NewReader(conn))
	if err != nil {
		return
	}
	io.Copy(io.Discard, req.Body) // nolint: errcheck

	body := "new-pod"
	if isFirstConnection {
		body = "old-pod"
	}
	response := &http.Response{
		StatusCode:    http.StatusOK,
		ProtoMajor:    1,
		ProtoMinor:    1,
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Header:        http.Header{},
		Request:       req,
	}
	if err := response.Write(conn); err != nil {
		return
	}

	if isFirstConnection {
		<-stall
	}
}
