// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

/*
This file implements reverse buildlets. These are buildlets that are not
started by the coordinator. They dial the coordinator and then accept
instructions. This feature is used for machines that cannot be started by
an API, for example real OS X machines with iOS and Android devices attached.

You can test this setup locally. In one terminal start a coordinator.
It will default to dev mode, using a dummy TLS cert and not talking to GCE.

	$ coordinator

In another terminal, start a reverse buildlet:

	$ buildlet -reverse "darwin-amd64"

It will dial and register itself with the coordinator. To confirm the
coordinator can see the buildlet, check the logs output or visit its
diagnostics page: https://localhost:8119. To send the buildlet some
work, go to:

	https://localhost:8119/dosomework
*/

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/build/buildlet"
	"golang.org/x/build/dashboard"
	"golang.org/x/build/revdial"
	"golang.org/x/build/types"
)

const minBuildletVersion = 1

var reversePool = new(reverseBuildletPool)

type token struct{}

type reverseBuildletPool struct {
	mu sync.Mutex // guards all fields, including fields of *reverseBuildlet
	// TODO: switch to a map[hostType][]buildlets or map of set.
	buildlets []*reverseBuildlet
	wakeChan  map[string]chan token // hostType => best-effort wake-up chan when buildlet free
	waiters   map[string]int        // hostType => number waiters blocked in GetBuildlet
}

func (p *reverseBuildletPool) ServeReverseStatusJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	status := p.buildReverseStatusJSON()
	j, _ := json.MarshalIndent(status, "", "\t")
	w.Write(j)
}

func (p *reverseBuildletPool) buildReverseStatusJSON() *types.ReverseBuilderStatus {
	status := &types.ReverseBuilderStatus{}

	p.mu.Lock()
	defer p.mu.Unlock()
	for _, b := range p.buildlets {
		hs := status.Host(b.hostType)
		if hs.Machines == nil {
			hs.Machines = make(map[string]*types.ReverseBuilder)
		}
		hs.Connected++
		bs := &types.ReverseBuilder{
			Name:         b.hostname,
			HostType:     b.hostType,
			ConnectedSec: time.Since(b.regTime).Seconds(),
			Version:      b.version,
		}
		if b.inUse && !b.inHealthCheck {
			hs.Busy++
			bs.Busy = true
			bs.BusySec = time.Since(b.inUseTime).Seconds()
		} else {
			hs.Idle++
			bs.IdleSec = time.Since(b.inUseTime).Seconds()
		}

		hs.Machines[b.hostname] = bs
	}
	for hostType, waiters := range p.waiters {
		status.Host(hostType).Waiters = waiters
	}
	for hostType, hc := range dashboard.Hosts {
		if hc.ExpectNum > 0 {
			status.Host(hostType).Expect = hc.ExpectNum
		}
	}
	return status
}

// tryToGrab returns non-nil bc on success if a buildlet is free.
//
// Otherwise it returns how many were busy, which might be 0 if none
// were (yet?) registered. The busy valid is only valid if bc == nil.
func (p *reverseBuildletPool) tryToGrab(hostType string) (bc *buildlet.Client, busy int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, b := range p.buildlets {
		if b.hostType != hostType {
			continue
		}
		if b.inUse {
			busy++
			continue
		}
		// Found an unused match.
		b.inUse = true
		b.inUseTime = time.Now()
		return b.client, 0
	}
	return nil, busy
}

func (p *reverseBuildletPool) getWakeChan(hostType string) chan token {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.wakeChan == nil {
		p.wakeChan = make(map[string]chan token)
	}
	c, ok := p.wakeChan[hostType]
	if !ok {
		c = make(chan token)
		p.wakeChan[hostType] = c
	}
	return c
}

func (p *reverseBuildletPool) noteBuildletAvailable(hostType string) {
	wake := p.getWakeChan(hostType)
	select {
	case wake <- token{}:
	default:
	}
}

// nukeBuildlet wipes out victim as a buildlet we'll ever return again,
// and closes its TCP connection in hopes that it will fix itself
// later.
func (p *reverseBuildletPool) nukeBuildlet(victim *buildlet.Client) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, rb := range p.buildlets {
		if rb.client == victim {
			defer rb.conn.Close()
			p.buildlets = append(p.buildlets[:i], p.buildlets[i+1:]...)
			return
		}
	}
}

// healthCheckBuildletLoop periodically requests the status from b.
// If the buildlet fails to respond promptly, it is removed from the pool.
func (p *reverseBuildletPool) healthCheckBuildletLoop(b *reverseBuildlet) {
	for {
		time.Sleep(time.Duration(10+rand.Intn(5)) * time.Second)
		if !p.healthCheckBuildlet(b) {
			return
		}
	}
}

func (p *reverseBuildletPool) healthCheckBuildlet(b *reverseBuildlet) bool {
	if b.client.IsBroken() {
		return false
	}
	p.mu.Lock()
	if b.inHealthCheck { // sanity check
		panic("previous health check still running")
	}
	if b.inUse {
		p.mu.Unlock()
		return true // skip busy buildlets
	}
	b.inUse = true
	b.inHealthCheck = true
	b.inUseTime = time.Now()
	res := make(chan error, 1)
	go func() {
		_, err := b.client.Status()
		res <- err
	}()
	p.mu.Unlock()

	t := time.NewTimer(5 * time.Second) // give buildlets time to respond
	var err error
	select {
	case err = <-res:
		t.Stop()
	case <-t.C:
		err = errors.New("health check timeout")
	}

	if err != nil {
		// remove bad buildlet
		log.Printf("Health check fail; removing reverse buildlet %v (type %v): %v", b.hostname, b.hostType, err)
		go b.client.Close()
		go p.nukeBuildlet(b.client)
		return false
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if !b.inHealthCheck {
		// buildlet was grabbed while lock was released; harmless.
		return true
	}
	b.inUse = false
	b.inHealthCheck = false
	b.inUseTime = time.Now()
	go p.noteBuildletAvailable(b.hostType)
	return true
}

var (
	highPriorityBuildletMu sync.Mutex
	highPriorityBuildlet   = make(map[string]chan *buildlet.Client)
)

func highPriChan(hostType string) chan *buildlet.Client {
	highPriorityBuildletMu.Lock()
	defer highPriorityBuildletMu.Unlock()
	if c, ok := highPriorityBuildlet[hostType]; ok {
		return c
	}
	c := make(chan *buildlet.Client)
	highPriorityBuildlet[hostType] = c
	return c
}

func (p *reverseBuildletPool) updateWaiterCounter(hostType string, delta int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.waiters == nil {
		p.waiters = make(map[string]int)
	}
	p.waiters[hostType] += delta
}

func (p *reverseBuildletPool) HasCapacity(hostType string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, b := range p.buildlets {
		if b.hostType != hostType {
			continue
		}
		if b.inUse {
			continue
		}
		return true
	}
	return false
}

func (p *reverseBuildletPool) GetBuildlet(ctx context.Context, hostType string, lg logger) (*buildlet.Client, error) {
	p.updateWaiterCounter(hostType, 1)
	defer p.updateWaiterCounter(hostType, -1)
	seenErrInUse := false
	isHighPriority, _ := ctx.Value(highPriorityOpt{}).(bool)
	sp := lg.CreateSpan("wait_static_builder", hostType)
	for {
		bc, busy := p.tryToGrab(hostType)
		if bc != nil {
			select {
			case highPriChan(hostType) <- bc:
				// Somebody else was more important.
			default:
				sp.Done(nil)
				return p.cleanedBuildlet(bc, lg)
			}
		}
		if busy > 0 && !seenErrInUse {
			lg.LogEventTime("waiting_machine_in_use")
			seenErrInUse = true
		}
		var highPri chan *buildlet.Client
		if isHighPriority {
			highPri = highPriChan(hostType)
		}
		select {
		case <-ctx.Done():
			return nil, sp.Done(ctx.Err())
		case bc := <-highPri:
			sp.Done(nil)
			return p.cleanedBuildlet(bc, lg)

		case <-time.After(10 * time.Second):
			// As multiple goroutines can be listening for
			// the available signal, it must be treated as
			// a best effort signal. So periodically try
			// to grab a buildlet again.
		case <-p.getWakeChan(hostType):
		}
	}
}

func (p *reverseBuildletPool) cleanedBuildlet(b *buildlet.Client, lg logger) (*buildlet.Client, error) {
	// Clean up any files from previous builds.
	sp := lg.CreateSpan("clean_buildlet", b.String())
	err := b.RemoveAll(".")
	sp.Done(err)
	if err != nil {
		b.Close()
		return nil, err
	}
	return b, nil
}

func (p *reverseBuildletPool) WriteHTMLStatus(w io.Writer) {
	// total maps from a host type to the number of machines which are
	// capable of that role.
	total := make(map[string]int)
	for typ, host := range dashboard.Hosts {
		if host.ExpectNum > 0 {
			total[typ] = 0
		}
	}
	// inUse track the number of non-idle host types.
	inUse := make(map[string]int)

	var buf bytes.Buffer
	p.mu.Lock()
	buildlets := append([]*reverseBuildlet(nil), p.buildlets...)
	sort.Sort(byTypeThenHostname(buildlets))
	for _, b := range buildlets {
		machStatus := "<i>idle</i>"
		if b.inUse {
			machStatus = "working"
		}
		fmt.Fprintf(&buf, "<li>%s (%s) version %s, %s: connected %s, %s for %s</li>\n",
			b.hostname,
			b.conn.RemoteAddr(),
			b.version,
			b.hostType,
			friendlyDuration(time.Since(b.regTime)),
			machStatus,
			friendlyDuration(time.Since(b.inUseTime)))
		total[b.hostType]++
		if b.inUse && !b.inHealthCheck {
			inUse[b.hostType]++
		}
	}
	p.mu.Unlock()

	var typs []string
	for typ := range total {
		typs = append(typs, typ)
	}
	sort.Strings(typs)

	io.WriteString(w, "<b>Reverse pool summary</b> (in use / total)<ul>")
	if len(typs) == 0 {
		io.WriteString(w, "<li>no connections</li>")
	}
	for _, typ := range typs {
		if dashboard.Hosts[typ] != nil && total[typ] < dashboard.Hosts[typ].ExpectNum {
			fmt.Fprintf(w, "<li>%s: %d/%d (%d missing)</li>",
				typ, inUse[typ], total[typ], dashboard.Hosts[typ].ExpectNum-total[typ])
		} else {
			fmt.Fprintf(w, "<li>%s: %d/%d</li>", typ, inUse[typ], total[typ])
		}
	}
	io.WriteString(w, "</ul>")

	fmt.Fprintf(w, "<b>Reverse pool machine detail</b><ul>%s</ul>", buf.Bytes())
}

// hostTypeCount iterates through the running reverse buildlets, and
// constructs a count of running buildlets per hostType.
func (p *reverseBuildletPool) hostTypeCount() map[string]int {
	total := map[string]int{}
	p.mu.Lock()
	for _, b := range p.buildlets {
		total[b.hostType]++
	}
	p.mu.Unlock()
	return total
}

func (p *reverseBuildletPool) String() string {
	// This doesn't currently show up anywhere, so ignore it for now.
	return "TODO: some reverse buildlet summary"
}

// HostTypes returns the a deduplicated list of buildlet types curently supported
// by the pool.
func (p *reverseBuildletPool) HostTypes() (types []string) {
	s := make(map[string]bool)
	p.mu.Lock()
	for _, b := range p.buildlets {
		s[b.hostType] = true
	}
	p.mu.Unlock()

	for t := range s {
		types = append(types, t)
	}
	sort.Strings(types)
	return types
}

// CanBuild reports whether the pool has a machine capable of building mode,
// even if said machine isn't currently idle.
func (p *reverseBuildletPool) CanBuild(hostType string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, b := range p.buildlets {
		if b.hostType == hostType {
			return true
		}
	}
	return false
}

func (p *reverseBuildletPool) addBuildlet(b *reverseBuildlet) {
	p.mu.Lock()
	defer p.noteBuildletAvailable(b.hostType)
	defer p.mu.Unlock()
	p.buildlets = append(p.buildlets, b)
	go p.healthCheckBuildletLoop(b)
}

// reverseBuildlet is a registered reverse buildlet.
// Its immediate fields are guarded by the reverseBuildletPool mutex.
type reverseBuildlet struct {
	// hostname is the name of the buildlet host.
	// It doesn't have to be a complete DNS name.
	hostname string
	// version is the reverse buildlet's version.
	version string

	// sessRand is the unique random number for every unique buildlet session.
	sessRand string

	client  *buildlet.Client
	conn    net.Conn
	regTime time.Time // when it was first connected

	// hostType is the configuration of this machine.
	// It is the key into the dashboard.Hosts map.
	hostType string

	// inUseAs signifies that the buildlet is in use.
	// inUseTime is when it entered that state.
	// inHealthCheck is whether it's inUse due to a health check.
	// All three are guarded by the mutex on reverseBuildletPool.
	inUse         bool
	inUseTime     time.Time
	inHealthCheck bool
}

func handleReverse(w http.ResponseWriter, r *http.Request) {
	if r.TLS == nil {
		http.Error(w, "buildlet registration requires SSL", http.StatusInternalServerError)
		return
	}
	// Check build keys.

	// modes can be either 1 buildlet type (new way) or builder mode(s) (the old way)
	hostType := r.Header.Get("X-Go-Host-Type")
	modes := r.Header["X-Go-Builder-Type"] // old way
	gobuildkeys := r.Header["X-Go-Builder-Key"]

	// Convert the new argument style (X-Go-Host-Type) into the
	// old way, to minimize changes in the rest of this code.
	if hostType != "" {
		if len(modes) > 0 {
			http.Error(w, "invalid mix of X-Go-Host-Type and X-Go-Builder-Type", http.StatusBadRequest)
			return
		}
		modes = []string{hostType}
	}
	if len(modes) == 0 || len(modes) != len(gobuildkeys) {
		http.Error(w, fmt.Sprintf("need at least one mode and matching key, got %d/%d", len(modes), len(gobuildkeys)), http.StatusPreconditionFailed)
		return
	}
	hostname := r.Header.Get("X-Go-Builder-Hostname")

	for i, m := range modes {
		if gobuildkeys[i] != builderKey(m) {
			http.Error(w, fmt.Sprintf("bad key for mode %q", m), http.StatusPreconditionFailed)
			return
		}
	}

	// Silently pretend that "gomacmini-*.local" doesn't want to do darwin-amd64-10_10 and
	// darwin-386-10_10 anymore.
	// TODO(bradfitz): remove this hack after we reconfigure those machines.
	if strings.HasPrefix(hostname, "gomacmini-") && strings.HasSuffix(hostname, ".local") {
		var filtered []string
		for _, m := range modes {
			if m == "darwin-amd64-10_10" || m == "darwin-386-10_10" {
				continue
			}
			filtered = append(filtered, m)
		}
		modes = filtered
	}

	// For older builders using the buildlet's -reverse flag only,
	// collapse their builder modes down into a singular hostType.
	legacyNote := ""
	if hostType == "" {
		hostType = mapBuilderToHostType(modes)
		legacyNote = fmt.Sprintf(" (mapped from legacy modes %q)", modes)
	}

	conn, bufrw, err := w.(http.Hijacker).Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	revDialer := revdial.NewDialer(bufrw, conn)
	log.Printf("Registering reverse buildlet %q (%s) for host type %v%s",
		hostname, r.RemoteAddr, hostType, legacyNote)

	(&http.Response{StatusCode: http.StatusSwitchingProtocols, Proto: "HTTP/1.1"}).Write(conn)

	client := buildlet.NewClient(hostname, buildlet.NoKeyPair)
	client.SetHTTPClient(&http.Client{
		Transport: &http.Transport{
			Dial: func(network, addr string) (net.Conn, error) {
				return revDialer.Dial()
			},
		},
	})
	client.SetDialer(revDialer.Dial)
	client.SetDescription(fmt.Sprintf("reverse peer %s/%s for host type %v", hostname, r.RemoteAddr, hostType))

	var isDead struct {
		sync.Mutex
		v bool
	}
	client.SetOnHeartbeatFailure(func() {
		isDead.Lock()
		isDead.v = true
		isDead.Unlock()
		conn.Close()
		reversePool.nukeBuildlet(client)
	})

	// If the reverse dialer (which is always reading from the
	// conn) detects that the remote went away, close the buildlet
	// client proactively show
	go func() {
		<-revDialer.Done()
		isDead.Lock()
		defer isDead.Unlock()
		if !isDead.v {
			client.Close()
		}
	}()
	tstatus := time.Now()
	status, err := client.Status()
	if err != nil {
		log.Printf("Reverse connection %s/%s for modes %v did not answer status after %v: %v",
			hostname, r.RemoteAddr, modes, time.Since(tstatus), err)
		conn.Close()
		return
	}
	if status.Version < minBuildletVersion {
		log.Printf("Buildlet too old: %s, %+v", r.RemoteAddr, status)
		conn.Close()
		return
	}
	log.Printf("Buildlet %s/%s: %+v for %s", hostname, r.RemoteAddr, status, modes)

	now := time.Now()
	b := &reverseBuildlet{
		hostname:  hostname,
		version:   r.Header.Get("X-Go-Builder-Version"),
		hostType:  hostType,
		client:    client,
		conn:      conn,
		inUseTime: now,
		regTime:   now,
	}
	reversePool.addBuildlet(b)
	registerBuildlet(modes) // testing only
}

var registerBuildlet = func(modes []string) {} // test hook

type byTypeThenHostname []*reverseBuildlet

func (s byTypeThenHostname) Len() int      { return len(s) }
func (s byTypeThenHostname) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s byTypeThenHostname) Less(i, j int) bool {
	bi, bj := s[i], s[j]
	ti, tj := bi.hostType, bj.hostType
	if ti == tj {
		return bi.hostname < bj.hostname
	}
	return ti < tj
}

// mapBuilderToHostType maps from the user's Request.Header["X-Go-Builder-Type"]
// mode list down into a single host type, or the empty string if unknown.
func mapBuilderToHostType(modes []string) string {
	// First, see if any of the provided modes are a host type.
	// If so, this is an updated client.
	for _, v := range modes {
		if _, ok := dashboard.Hosts[v]; ok {
			return v
		}
	}

	// Else, it's an old client, still speaking in terms of
	// builder names.  See if any are registered aliases. First
	// one wins. (There are no ambiguities in the wild.)
	for hostType, hconf := range dashboard.Hosts {
		for _, alias := range hconf.ReverseAliases {
			for _, v := range modes {
				if v == alias {
					return hostType
				}
			}
		}
	}
	return ""
}
