// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package buildlet contains client tools for working with a buildlet
// server.
package buildlet // import "golang.org/x/build/buildlet"

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"context"
	"golang.org/x/oauth2"
)

// NewClient returns a *Client that will manipulate ipPort,
// authenticated using the provided keypair.
//
// This constructor returns immediately without testing the host or auth.
func NewClient(ipPort string, kp KeyPair) *Client {
	tr := &http.Transport{
		Dial:            defaultDialer(),
		DialTLS:         kp.tlsDialer(),
		IdleConnTimeout: time.Minute,
	}
	c := &Client{
		ipPort:     ipPort,
		tls:        kp,
		password:   kp.Password(),
		httpClient: &http.Client{Transport: tr},
		closeFuncs: []func(){tr.CloseIdleConnections},
	}
	c.setCommon()
	return c
}

func (c *Client) setCommon() {
	c.peerDead = make(chan struct{})
	c.ctx, c.ctxCancel = context.WithCancel(context.Background())
}

// SetOnHeartbeatFailure sets a function to be called when heartbeats
// against this builder fail, or when the client is destroyed with
// Close. The function fn is never called more than once.
// SetOnHeartbeatFailure must be set before any use of the buildlet.
func (c *Client) SetOnHeartbeatFailure(fn func()) {
	c.heartbeatFailure = fn
}

var ErrClosed = errors.New("buildlet: Client closed")

// Closes destroys and closes down the buildlet, destroying all state
// immediately.
func (c *Client) Close() error {
	// TODO(bradfitz): have a Client-wide Done channel we set on
	// all outbound HTTP Requests and close it in the once here?
	// Then if something was in-flight and somebody else Closes,
	// the local http.Transport notices, rather than noticing via
	// the remote machine dying or timing out.
	c.closeOnce.Do(func() {
		// Send a best-effort notification to the server to destroy itself.
		// Don't want too long (since it's likely in a broken state anyway).
		// Ignore the return value, since we're about to forcefully destroy
		// it anyway.
		req, err := http.NewRequest("POST", c.URL()+"/halt", nil)
		if err != nil {
			// ignore.
		} else {
			_, err = c.doHeaderTimeout(req, 2*time.Second)
		}
		if err == nil {
			err = ErrClosed
		}
		for _, fn := range c.closeFuncs {
			fn()
		}
		c.setPeerDead(err) // which will also cause c.heartbeatFailure to run
	})
	return nil
}

func (c *Client) setPeerDead(err error) {
	c.setPeerDeadOnce.Do(func() {
		c.MarkBroken()
		if err == nil {
			err = errors.New("peer dead (no specific error)")
		}
		c.deadErr = err
		close(c.peerDead)
	})
}

// SetDescription sets a short description of where the buildlet
// connection came from.  This is used by the build coordinator status
// page, mostly for debugging.
func (c *Client) SetDescription(v string) {
	c.desc = v
}

// SetHTTPClient replaces the underlying HTTP client.
// It should only be called before the Client is used.
func (c *Client) SetHTTPClient(httpClient *http.Client) {
	c.httpClient = httpClient
}

// SetDialer sets the function that creates a new connection to the buildlet.
// By default, net.Dial is used.
func (c *Client) SetDialer(dialer func() (net.Conn, error)) {
	c.dialer = dialer
}

// defaultDialer returns the net/http package's default Dial function.
// Notably, this sets TCP keep-alive values, so when we kill VMs
// (whose TCP stacks stop replying, forever), we don't leak file
// descriptors for otherwise forever-stalled TCP connections.
func defaultDialer() func(network, addr string) (net.Conn, error) {
	if fn := http.DefaultTransport.(*http.Transport).Dial; fn != nil {
		return fn
	}
	return net.Dial
}

// A Client interacts with a single buildlet.
type Client struct {
	ipPort         string // required, unless remoteBuildlet+baseURL is set
	tls            KeyPair
	httpClient     *http.Client
	dialer         func() (net.Conn, error) // nil means to use net.Dial
	baseURL        string                   // optional baseURL (used by remote buildlets)
	authUser       string                   // defaults to "gomote", if password is non-empty
	password       string                   // basic auth password or empty for none
	remoteBuildlet string                   // non-empty if for remote buildlets

	closeFuncs  []func() // optional extra code to run on close
	releaseMode bool

	ctx              context.Context
	ctxCancel        context.CancelFunc
	heartbeatFailure func() // optional
	desc             string

	closeOnce         sync.Once
	initHeartbeatOnce sync.Once
	setPeerDeadOnce   sync.Once
	peerDead          chan struct{} // closed on peer death
	deadErr           error         // guarded by peerDead's close

	mu     sync.Mutex
	broken bool // client is broken in some way
}

// SetReleaseMode sets whether this client is being used in "release
// mode", to prepare the final binaries to be shipped to users. All
// this mode does for now is disable pargzip multi-stream gzip
// files. See golang.org/issue/19052.
//
// SetReleaseMode must be set before using the client.
func (c *Client) SetReleaseMode(v bool) {
	c.releaseMode = v
}

func (c *Client) String() string {
	if c == nil {
		return "(nil *buildlet.Client)"
	}
	return strings.TrimSpace(c.URL() + " " + c.desc)
}

// RemoteName returns the name of this client's buildlet on the
// coordinator. If this buildlet isn't a remote buildlet created via
// a buildlet, this returns the empty string.
func (c *Client) RemoteName() string {
	return c.remoteBuildlet
}

// URL returns the buildlet's URL prefix, without a trailing slash.
func (c *Client) URL() string {
	if c.baseURL != "" {
		return strings.TrimRight(c.baseURL, "/")
	}
	if !c.tls.IsZero() {
		return "https://" + strings.TrimSuffix(c.ipPort, ":443")
	}
	return "http://" + strings.TrimSuffix(c.ipPort, ":80")
}

func (c *Client) IPPort() string { return c.ipPort }

func (c *Client) Name() string {
	if c.ipPort != "" {
		return c.ipPort
	}
	return "(unnamed-buildlet)"
}

// MarkBroken marks this client as broken in some way.
func (c *Client) MarkBroken() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.broken = true
	c.ctxCancel()
}

// IsBroken reports whether this client is broken in some way.
func (c *Client) IsBroken() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.broken
}

func (c *Client) authUsername() string {
	if c.authUser != "" {
		return c.authUser
	}
	return "gomote"
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	if req.Cancel == nil {
		req.Cancel = c.ctx.Done()
	}
	c.initHeartbeatOnce.Do(c.initHeartbeats)
	if c.password != "" {
		req.SetBasicAuth(c.authUsername(), c.password)
	}
	if c.remoteBuildlet != "" {
		req.Header.Set("X-Buildlet-Proxy", c.remoteBuildlet)
	}
	return c.httpClient.Do(req)
}

// ProxyRoundTripper returns a RoundTripper that sends HTTP requests directly
// through to the underlying buildlet, adding auth and X-Buildlet-Proxy headers
// as necessary. This is really only intended for use by the coordinator.
func (c *Client) ProxyRoundTripper() http.RoundTripper {
	return proxyRoundTripper{c}
}

type proxyRoundTripper struct {
	c *Client
}

func (p proxyRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return p.c.do(req)
}

func (c *Client) initHeartbeats() {
	go c.heartbeatLoop()
}

func (c *Client) heartbeatLoop() {
	failInARow := 0
	for {
		select {
		case <-c.peerDead:
			// Dead for whatever reason (heartbeat, remote
			// side closed, caller Closed
			// normally). Regardless, we call the
			// heartbeatFailure func if set.
			if c.heartbeatFailure != nil {
				c.heartbeatFailure()
			}
			return
		case <-time.After(10 * time.Second):
			t0 := time.Now()
			if _, err := c.Status(); err != nil {
				failInARow++
				if failInARow == 3 {
					log.Printf("Buildlet %v failed three heartbeats; final error: %v", c, err)
					c.setPeerDead(fmt.Errorf("Buildlet %v failed heartbeat after %v; marking dead; err=%v", c, time.Since(t0), err))
				}
			} else {
				failInARow = 0
			}
		}
	}
}

var errHeaderTimeout = errors.New("timeout waiting for headers")

// doHeaderTimeout calls c.do(req) and returns its results, or
// errHeaderTimeout if max elapses first.
func (c *Client) doHeaderTimeout(req *http.Request, max time.Duration) (res *http.Response, err error) {
	type resErr struct {
		res *http.Response
		err error
	}

	if req.Cancel != nil {
		panic("use of Request.Cancel inside the buildlet package is reserved for doHeaderTimeout")
	}
	cancelc := make(chan struct{}) // closed to abort
	req.Cancel = cancelc

	resErrc := make(chan resErr, 1)
	go func() {
		res, err := c.do(req)
		resErrc <- resErr{res, err}
	}()

	timer := time.NewTimer(max)
	defer timer.Stop()

	cleanup := func() {
		close(cancelc)
		if re := <-resErrc; re.res != nil {
			re.res.Body.Close()
		}
	}

	select {
	case re := <-resErrc:
		return re.res, re.err
	case <-c.peerDead:
		go cleanup()
		return nil, c.deadErr
	case <-timer.C:
		go cleanup()
		return nil, errHeaderTimeout
	}
}

// doOK sends the request and expects a 200 OK response.
func (c *Client) doOK(req *http.Request) error {
	res, err := c.do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		slurp, _ := ioutil.ReadAll(io.LimitReader(res.Body, 4<<10))
		return fmt.Errorf("%v; body: %s", res.Status, slurp)
	}
	return nil
}

// PutTar writes files to the remote buildlet, rooted at the relative
// directory dir.
// If dir is empty, they're placed at the root of the buildlet's work directory.
// The dir is created if necessary.
// The Reader must be of a tar.gz file.
func (c *Client) PutTar(r io.Reader, dir string) error {
	req, err := http.NewRequest("PUT", c.URL()+"/writetgz?dir="+url.QueryEscape(dir), r)
	if err != nil {
		return err
	}
	return c.doOK(req)
}

// PutTarFromURL tells the buildlet to download the tar.gz file from tarURL
// and write it to dir, a relative directory from the workdir.
// If dir is empty, they're placed at the root of the buildlet's work directory.
// The dir is created if necessary.
// The url must be of a tar.gz file.
func (c *Client) PutTarFromURL(tarURL, dir string) error {
	form := url.Values{
		"url": {tarURL},
	}
	req, err := http.NewRequest("POST", c.URL()+"/writetgz?dir="+url.QueryEscape(dir), strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.doOK(req)
}

// Put writes the provided file to path (relative to workdir) and sets mode.
func (c *Client) Put(r io.Reader, path string, mode os.FileMode) error {
	param := url.Values{
		"path": {path},
		"mode": {fmt.Sprint(int64(mode))},
	}
	req, err := http.NewRequest("PUT", c.URL()+"/write?"+param.Encode(), r)
	if err != nil {
		return err
	}
	return c.doOK(req)
}

// GetTar returns a .tar.gz stream of the given directory, relative to the buildlet's work dir.
// The provided dir may be empty to get everything.
func (c *Client) GetTar(ctx context.Context, dir string) (io.ReadCloser, error) {
	var args string
	if c.releaseMode {
		args = "&pargzip=0"
	}
	req, err := http.NewRequest("GET", c.URL()+"/tgz?dir="+url.QueryEscape(dir)+args, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		slurp, _ := ioutil.ReadAll(io.LimitReader(res.Body, 4<<10))
		res.Body.Close()
		return nil, fmt.Errorf("%v; body: %s", res.Status, slurp)
	}
	return res.Body, nil
}

// ExecOpts are options for a remote command invocation.
type ExecOpts struct {
	// Output is the output of stdout and stderr.
	// If nil, the output is discarded.
	Output io.Writer

	// Dir is the directory from which to execute the command.
	// It is optional. If not specified, it defaults to the directory of
	// the command, or the work directory if SystemLevel is set.
	Dir string

	// Args are the arguments to pass to the cmd given to Client.Exec.
	Args []string

	// ExtraEnv are KEY=VALUE pairs to append to the buildlet
	// process's environment.
	ExtraEnv []string

	// Path, if non-nil, specifies the PATH variable of the executed
	// process's environment. A non-nil empty list clears the path.
	// The following expansions apply:
	//   - the string "$PATH" expands to any existing PATH element(s)
	//   - the substring "$WORKDIR" expands to buildlet's temp workdir
	// After expansions, the list is joined with an OS-specific list
	// separator and supplied to the executed process as its PATH
	// environment variable.
	Path []string

	// SystemLevel controls whether the command is run outside of
	// the buildlet's environment.
	SystemLevel bool

	// Debug, if true, instructs to the buildlet to print extra debug
	// info to the output before the command begins executing.
	Debug bool

	// OnStartExec is an optional hook that runs after the 200 OK
	// response from the buildlet, but before the output begins
	// writing to Output.
	OnStartExec func()

	// Timeout is an optional duration before ErrTimeout is returned.
	Timeout time.Duration
}

var ErrTimeout = errors.New("buildlet: timeout waiting for command to complete")

// Exec runs cmd on the buildlet.
//
// Two errors are returned: one is whether the command succeeded
// remotely (remoteErr), and the second (execErr) is whether there
// were system errors preventing the command from being started or
// seen to completition. If execErr is non-nil, the remoteErr is
// meaningless.
func (c *Client) Exec(cmd string, opts ExecOpts) (remoteErr, execErr error) {
	var mode string
	if opts.SystemLevel {
		mode = "sys"
	}
	path := opts.Path
	if len(path) == 0 && path != nil {
		// url.Values doesn't distinguish between a nil slice and
		// a non-nil zero-length slice, so use this sentinel value.
		path = []string{"$EMPTY"}
	}
	form := url.Values{
		"cmd":    {cmd},
		"mode":   {mode},
		"dir":    {opts.Dir},
		"cmdArg": opts.Args,
		"env":    opts.ExtraEnv,
		"path":   path,
		"debug":  {fmt.Sprint(opts.Debug)},
	}
	req, err := http.NewRequest("POST", c.URL()+"/exec", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// The first thing the buildlet's exec handler does is flush the headers, so
	// 10 seconds should be plenty of time, regardless of where on the planet
	// (Atlanta, Paris, etc) the reverse buildlet is:
	res, err := c.doHeaderTimeout(req, 10*time.Second)
	if err == errHeaderTimeout {
		c.MarkBroken()
		return nil, errors.New("buildlet: timeout waiting for exec header response")
	}
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		slurp, _ := ioutil.ReadAll(io.LimitReader(res.Body, 4<<10))
		return nil, fmt.Errorf("buildlet: HTTP status %v: %s", res.Status, slurp)
	}
	condRun(opts.OnStartExec)

	type errs struct {
		remoteErr, execErr error
	}
	resc := make(chan errs, 1)
	go func() {
		// Stream the output:
		out := opts.Output
		if out == nil {
			out = ioutil.Discard
		}
		if _, err := io.Copy(out, res.Body); err != nil {
			resc <- errs{execErr: fmt.Errorf("error copying response: %v", err)}
			return
		}

		// Don't record to the dashboard unless we heard the trailer from
		// the buildlet, otherwise it was probably some unrelated error
		// (like the VM being killed, or the buildlet crashing due to
		// e.g. https://golang.org/issue/9309, since we require a tip
		// build of the buildlet to get Trailers support)
		state := res.Trailer.Get("Process-State")
		if state == "" {
			resc <- errs{execErr: errors.New("missing Process-State trailer from HTTP response; buildlet built with old (<= 1.4) Go?")}
			return
		}
		if state != "ok" {
			resc <- errs{remoteErr: errors.New(state)}
		} else {
			resc <- errs{} // success
		}
	}()
	var timer <-chan time.Time
	if opts.Timeout > 0 {
		t := time.NewTimer(opts.Timeout)
		defer t.Stop()
		timer = t.C
	}
	select {
	case <-timer:
		c.MarkBroken()
		return nil, ErrTimeout
	case res := <-resc:
		return res.remoteErr, res.execErr
	case <-c.peerDead:
		return nil, c.deadErr
	}
}

// RemoveAll deletes the provided paths, relative to the work directory.
func (c *Client) RemoveAll(paths ...string) error {
	if len(paths) == 0 {
		return nil
	}
	form := url.Values{"path": paths}
	req, err := http.NewRequest("POST", c.URL()+"/removeall", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.doOK(req)
}

// DestroyVM shuts down the buildlet and destroys the VM instance.
func (c *Client) DestroyVM(ts oauth2.TokenSource, proj, zone, instance string) error {
	// TODO(bradfitz): move GCE stuff out of this package?
	gceErrc := make(chan error, 1)
	buildletErrc := make(chan error, 1)
	go func() {
		gceErrc <- DestroyVM(ts, proj, zone, instance)
	}()
	go func() {
		buildletErrc <- c.Close()
	}()
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()

	var retErr error
	var gceDone, buildletDone bool
	for !gceDone || !buildletDone {
		select {
		case err := <-gceErrc:
			if err != nil {
				retErr = err
			}
			gceDone = true
		case err := <-buildletErrc:
			if err != nil {
				retErr = err
			}
			buildletDone = true
		case <-timeout.C:
			e := ""
			if !buildletDone {
				e = "timeout asking buildlet to shut down"
			}
			if !gceDone {
				if e != "" {
					e += " and "
				}
				e += "timeout asking GCE to delete builder VM"
			}
			return errors.New(e)
		}
	}
	return retErr
}

// Status provides status information about the buildlet.
//
// A coordinator can use the provided information to decide what, if anything,
// to do with a buildlet.
type Status struct {
	Version int // buildlet version, coordinator rejects any value less than 1.
}

// Status returns an Status value describing this buildlet.
func (c *Client) Status() (Status, error) {
	select {
	case <-c.peerDead:
		return Status{}, c.deadErr
	default:
		// Continue below.
	}
	req, err := http.NewRequest("GET", c.URL()+"/status", nil)
	if err != nil {
		return Status{}, err
	}
	resp, err := c.doHeaderTimeout(req, 10*time.Second) // plenty of time
	if err != nil {
		return Status{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return Status{}, errors.New(resp.Status)
	}
	b, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return Status{}, err
	}
	var status Status
	if err := json.Unmarshal(b, &status); err != nil {
		return Status{}, err
	}
	return status, nil
}

// WorkDir returns the absolute path to the buildlet work directory.
func (c *Client) WorkDir() (string, error) {
	req, err := http.NewRequest("GET", c.URL()+"/workdir", nil)
	if err != nil {
		return "", err
	}
	resp, err := c.doHeaderTimeout(req, 10*time.Second) // plenty of time
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", errors.New(resp.Status)
	}
	b, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// DirEntry is the information about a file on a buildlet.
type DirEntry struct {
	// line is of the form "drw-rw-rw\t<name>" and then if a regular file,
	// also "\t<size>\t<modtime>". in either case, without trailing newline.
	// TODO: break into parsed fields?
	line string
}

func (de DirEntry) String() string {
	return de.line
}

func (de DirEntry) Name() string {
	f := strings.Split(de.line, "\t")
	if len(f) < 2 {
		return ""
	}
	return f[1]
}

func (de DirEntry) Digest() string {
	f := strings.Split(de.line, "\t")
	if len(f) < 5 {
		return ""
	}
	return f[4]
}

// ListDirOpts are options for Client.ListDir.
type ListDirOpts struct {
	// Recursive controls whether the directory is listed
	// recursively.
	Recursive bool

	// Skip are the directories to skip, relative to the directory
	// passed to ListDir. Each item should contain only forward
	// slashes and not start or end in slashes.
	Skip []string

	// Digest controls whether the SHA-1 digests of regular files
	// are returned.
	Digest bool
}

// ListDir lists the contents of a directory.
// The fn callback is run for each entry.
func (c *Client) ListDir(dir string, opts ListDirOpts, fn func(DirEntry)) error {
	param := url.Values{
		"dir":       {dir},
		"recursive": {fmt.Sprint(opts.Recursive)},
		"skip":      opts.Skip,
		"digest":    {fmt.Sprint(opts.Digest)},
	}
	req, err := http.NewRequest("GET", c.URL()+"/ls?"+param.Encode(), nil)
	if err != nil {
		return err
	}
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		slurp, _ := ioutil.ReadAll(io.LimitReader(resp.Body, 1<<10))
		return fmt.Errorf("%s: %s", resp.Status, slurp)
	}
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		fn(DirEntry{line: line})
	}
	return sc.Err()
}

func (c *Client) getDialer() func() (net.Conn, error) {
	if c.dialer != nil {
		return c.dialer
	}
	return c.dialWithNetDial
}

func (c *Client) dialWithNetDial() (net.Conn, error) {
	// TODO: contexts? the tedious part will be adding it to
	// revdial.Dial. For now just do a 5 second timeout. Probably
	// fine. This is currently only used for ssh connections.
	d := net.Dialer{Timeout: 5 * time.Second}
	return d.Dial("tcp", c.ipPort)
}

// ConnectSSH opens an SSH connection to the buildlet for the given username.
// The authorizedPubKey must be a line from an ~/.ssh/authorized_keys file
// and correspond to the private key to be used to communicate over the net.Conn.
func (c *Client) ConnectSSH(user, authorizedPubKey string) (net.Conn, error) {
	conn, err := c.getDialer()()
	if err != nil {
		return nil, fmt.Errorf("error dialing HTTP connection before SSH upgrade: %v", err)
	}
	req, err := http.NewRequest("POST", "/connect-ssh", nil)
	if err != nil {
		conn.Close()
		return nil, err
	}
	req.Header.Add("X-Go-Ssh-User", user)
	req.Header.Add("X-Go-Authorized-Key", authorizedPubKey)
	if err := req.Write(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("writing /connect-ssh HTTP request failed: %v", err)
	}
	bufr := bufio.NewReader(conn)
	res, err := http.ReadResponse(bufr, req)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("reading /connect-ssh response: %v", err)
	}
	if res.StatusCode != http.StatusSwitchingProtocols {
		slurp, _ := ioutil.ReadAll(res.Body)
		return nil, fmt.Errorf("unexpected /connect-ssh response: %v, %s", res.Status, slurp)
	}
	return conn, nil
}

func condRun(fn func()) {
	if fn != nil {
		fn()
	}
}
