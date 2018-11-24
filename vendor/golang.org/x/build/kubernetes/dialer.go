// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kubernetes

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"
)

var dialRand = rand.New(rand.NewSource(time.Now().UnixNano()))

// DialService connects to the named service. The service must have only one
// port. For multi-port services, use DialServicePort.
func (c *Client) DialService(ctx context.Context, serviceName string) (net.Conn, error) {
	return c.DialServicePort(ctx, serviceName, "")
}

// DialServicePort connects to the named port on the named service.
// If portName is the empty string, the service must have exactly 1 port.
func (c *Client) DialServicePort(ctx context.Context, serviceName, portName string) (net.Conn, error) {
	// TODO: cache the result of GetServiceEndpoints, at least for
	// a few seconds, to rate-limit calls to the master?
	eps, err := c.GetServiceEndpoints(ctx, serviceName, portName)
	if err != nil {
		return nil, err
	}
	if len(eps) == 0 {
		return nil, fmt.Errorf("no endpoints found for service %q", serviceName)
	}
	if portName == "" {
		firstName := eps[0].PortName
		for _, p := range eps[1:] {
			if p.PortName != firstName {
				return nil, fmt.Errorf("unspecified port name for DialServicePort is ambiguous for service %q (mix of %q, %q, ...)", serviceName, firstName, p.PortName)
			}
		}
	}
	ep := eps[dialRand.Intn(len(eps))]
	var dialer net.Dialer
	return dialer.DialContext(ctx, strings.ToLower(ep.Protocol), net.JoinHostPort(ep.IP, strconv.Itoa(ep.Port)))
}

func (c *Client) DialPod(ctx context.Context, podName string, port int) (net.Conn, error) {
	status, err := c.PodStatus(ctx, podName)
	if err != nil {
		return nil, fmt.Errorf("PodStatus of %q: %v", podName, err)
	}
	if status.Phase != "Running" {
		return nil, fmt.Errorf("pod %q in state %q", podName, status.Phase)
	}
	var dialer net.Dialer
	return dialer.DialContext(ctx, "tcp", net.JoinHostPort(status.PodIP, strconv.Itoa(port)))
}
