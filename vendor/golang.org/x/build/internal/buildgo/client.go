// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package buildgo

import (
	"context"
	"net/http"
	"sync"

	"golang.org/x/build/buildenv"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v1"
)

// Client is an authenticated client to the Go build system.
type Client struct {
	Env    *buildenv.Environment // generally Production or Staging
	Creds  *google.Credentials
	Client *http.Client // OAuth2 client

	mu             sync.Mutex
	computeService *compute.Service // lazily initialized
}

// NewClient returns a new client for using the Go build system in the provided environment.
// The authentication information is discovered using env.Credentials.
func NewClient(ctx context.Context, env *buildenv.Environment) (*Client, error) {
	creds, err := env.Credentials(ctx)
	if err != nil {
		return nil, err
	}
	c := &Client{Env: env, Creds: creds}
	c.Client = oauth2.NewClient(ctx, creds.TokenSource)
	return c, nil
}

// Compute returns the GCE compute service.
func (c *Client) Compute() *compute.Service {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.computeService == nil {
		c.computeService, _ = compute.New(c.Client)
	}
	return c.computeService
}
