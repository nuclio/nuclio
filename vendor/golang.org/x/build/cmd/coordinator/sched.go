// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"sort"
	"sync"
	"time"

	"golang.org/x/build/buildlet"
	"golang.org/x/build/internal/buildgo"
)

// useScheduler controls whether we actually use the scheduler. This
// is temporarily false during development. Once we're happy with it
// we'll delete this const.
//
// If false, any GetBuildlet call to the schedule delegates directly
// to the BuildletPool's GetBuildlet and we make a bunch of callers
// fight over a mutex and a random one wins, like we used to do it.
const useScheduler = false

// The Scheduler prioritizes access to buidlets. It accepts requests
// for buildlets, starts the creation of buildlets from BuildletPools,
// and prioritizes which callers gets them first when they're ready.
type Scheduler struct {
	mu      sync.Mutex
	paused  bool
	waiting []*SchedWaiter // index 0 is highest priority

	readyc chan ReadyBuildlet

	launching map[*SchedWaiter]bool
}

// A ReadyBuildlet is a buildlet that was just created and is up and
// is ready to be assigned to a caller based on priority.
type ReadyBuildlet struct {
	Pool     BuildletPool
	HostType string
	Client   *buildlet.Client
}

// NewScheduler returns a new scheduler.
func NewScheduler() *Scheduler {
	s := &Scheduler{
		readyc: make(chan ReadyBuildlet, 8),
	}
	if useScheduler {
		go s.assignLoop()
	}
	return s
}

// assignLoop waits for the successful creation of buildlets and
// assigns them the highest priority waiter.
//
// TODO: probably also need to deal with buildlet creation failures to
// at least re-nudge the scheduler to kick off new buildlet creations
// if still necessary.
func (s *Scheduler) assignLoop() {
	for {
		rb := <-s.readyc
		bestWaiter, ok := s.matchWaiter(rb)
		if !ok {
			go rb.Client.Close()
			continue
		}
		select {
		case bestWaiter.Res <- rb.Client:
			// Normal happy case. Something gets its buildlet.
		default:
			// Wait went away. (context timeout?)
			go rb.Client.Close()
		}
	}
}

// pause pauses the scheduler.
func (s *Scheduler) pause(v bool) {
	if !useScheduler {
		return
	}
	s.mu.Lock()
	s.paused = true
	s.mu.Unlock()
}

// unpause unpauses the scheduler and runs schedule.
func (s *Scheduler) unpause() {
	if !useScheduler {
		return
	}
	s.mu.Lock()
	s.paused = false
	s.mu.Unlock()
	s.schedule()
}

// schedule starts creating buildlets if there's demand.
//
// It acquires s.mu so should run as quickly as possible.
func (s *Scheduler) schedule() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.paused {
		return
	}
	poolExhausted := map[BuildletPool]bool{}
	for _, sw := range s.waiting {
		si := sw.si
		if poolExhausted[si.Pool] || !si.Pool.HasCapacity(si.HostType) {
			poolExhausted[si.Pool] = true
			continue
		}
		// ... TODO kick things off, using a goroutine per
		// slow buildlet creation call. If the creation fails,
		// the goroutine can call back into the scheduler to
		// inform it of that.
	}
}

// matchWaiter returns (and removes from the waiting queue) the highest priority SchedWaiter
// that matches the provided ReadyBuildlet.
func (s *Scheduler) matchWaiter(rb ReadyBuildlet) (sw *SchedWaiter, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, sw := range s.waiting {
		si := sw.si
		if si.Pool == rb.Pool && si.HostType == rb.HostType {
			copy(s.waiting[i:], s.waiting[i+1:])
			s.waiting[len(s.waiting)-1] = nil
			s.waiting = s.waiting[:len(s.waiting)-1]
			return sw, true
		}
	}
	return nil, false
}

func (s *Scheduler) removeWaiter(remove *SchedWaiter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	newWaiting := s.waiting[:0]
	for _, sw := range s.waiting {
		if sw != remove {
			newWaiting = append(newWaiting, sw)
		}
	}
	s.waiting = newWaiting
}

func (s *Scheduler) enqueueWaiter(si *SchedItem) *SchedWaiter {
	defer s.schedule()

	w := &SchedWaiter{
		s:   s,
		si:  si,
		Res: make(chan interface{}), // NOT buffered
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.waiting = append(s.waiting, w)
	sort.Slice(s.waiting, func(i, j int) bool {
		ia, ib := s.waiting[i].si, s.waiting[j].si
		return schedLess(ia, ib)
	})
	return w
}

// schedLess reports whether scheduled item ia is "less" (more
// important) than scheduled item ib.
func schedLess(ia, ib *SchedItem) bool {
	// TryBots are more important.
	if ia.IsTry != ib.IsTry {
		return ia.IsTry
	}
	return ia.commitTime.Before(ib.commitTime)
}

type SchedItem struct {
	buildgo.BuilderRev
	Pool     BuildletPool
	HostType string
	IsTry    bool

	// We set in GetBuildlet:
	commitTime time.Time
	tryFor     string // which user. (user with 1 trybot >> user with 50 trybots)
}

type SchedWaiter struct {
	s  *Scheduler
	si *SchedItem

	// Res is the result channel, containing either a
	// *buildlet.Client or an error. It is read by GetBuildlet and
	// written by assignBuildlet.
	Res chan interface{}
}

func (sw *SchedWaiter) cancel() {
	sw.s.removeWaiter(sw)
}

// GetBuildlet requests a buildlet with the parameters described in si.
func (s *Scheduler) GetBuildlet(ctx context.Context, lg logger, si *SchedItem) (*buildlet.Client, error) {
	if !useScheduler {
		return si.Pool.GetBuildlet(ctx, si.HostType, lg)
	}

	// TODO: once we remove the useScheduler const, we can
	// probably remove the "lg" logger parameter. We don't need to
	// log anything during the buildlet creation process anymore
	// because we don't which build it'll be for. So all we can
	// say in the logs for is "Asking for a buildlet" and "Got
	// one", which the caller already does. I think. Verify that.

	// TODO: populate si unexported fields

	sw := s.enqueueWaiter(si)
	select {
	case v := <-sw.Res:
		if bc, ok := v.(*buildlet.Client); ok {
			return bc, nil
		}
		return nil, v.(error)
	case <-ctx.Done():
		sw.cancel()
		return nil, ctx.Err()
	}
}
