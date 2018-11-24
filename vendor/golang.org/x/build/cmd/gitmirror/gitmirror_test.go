// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestHomepage(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handleRoot(w, req)
	if w.Code != 200 {
		t.Fatalf("GET /: want code 200, got %d", w.Code)
	}
	if hdr := w.Header().Get("Content-Type"); !strings.Contains(hdr, "text/html") {
		t.Fatalf("GET /: want html content-type, got %s", hdr)
	}
}

func TestDebugWatcher(t *testing.T) {
	r := &Repo{path: "build"}
	r.setStatus("waiting")
	req := httptest.NewRequest("GET", "/debug/watcher/build", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("GET /: want code 200, got %d", w.Code)
	}
	body := w.Body.String()
	if substr := `watcher status for repo: "build"`; !strings.Contains(body, substr) {
		t.Fatalf("GET /debug/watcher/build: want %q in body, got %s", substr, body)
	}
	if substr := "waiting"; !strings.Contains(body, substr) {
		t.Fatalf("GET /debug/watcher/build: want %q in body, got %s", substr, body)
	}
}

// fakeCmd records the results of CommandContext and echoes any arguments to
// stdout.
type fakeCmd struct {
	Cmd       string
	Args      []string
	callCount int
}

func (f *fakeCmd) CommandContext(ctx context.Context, cmd string, args ...string) *exec.Cmd {
	f.callCount++
	f.Cmd = cmd
	f.Args = args
	return exec.CommandContext(ctx, "echo", append([]string{cmd}, args...)...)
}

func TestRev(t *testing.T) {
	f := &fakeCmd{}
	testHookArchiveCmd = f.CommandContext
	defer func() { testHookArchiveCmd = nil }()
	r := &Repo{path: "build"}
	r.setStatus("waiting")
	req := httptest.NewRequest("GET", "/build.tar.gz?rev=example-branch", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("GET /: want code 200, got %d", w.Code)
	}
	if f.Cmd != "git" {
		t.Fatalf("cmd: want 'git' for cmd, got %s", f.Cmd)
	}
	wantArgs := []string{"archive", "--format=tgz", "example-branch"}
	if !reflect.DeepEqual(f.Args, wantArgs) {
		t.Fatalf("cmd: want '%q' for args, got %q", wantArgs, f.Args)
	}
}

func TestRevNotFound(t *testing.T) {
	f := &fakeCmd{}
	f2 := &fakeCmd{}
	testHookArchiveCmd = f.CommandContext
	testHookFetchCmd = f2.CommandContext
	defer func() {
		testHookArchiveCmd = nil
		testHookFetchCmd = nil
	}()
	r := &Repo{path: "build"}
	r.setStatus("waiting")
	req := httptest.NewRequest("GET", "/build.tar.gz?rev=example-branch", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("GET /build.tar.gz: want code 200, got %d", w.Code)
	}
	if f2.callCount != 1 {
		t.Fatal("GET /build.tar.gz: want 'git fetch' to be called, wasn't called")
	}
	wantArgs := []string{"fetch", "origin", "example-branch"}
	if !reflect.DeepEqual(f2.Args, wantArgs) {
		t.Fatalf("cmd: want '%q' for args, got %q", wantArgs, f2.Args)
	}
}

// TestUpdate tests that we link new commits correctly in
// our linked list (parent <=> child) of commits, and update
// the Repo's map of commits correctly.
func TestUpdate(t *testing.T) {
	oldNetwork := *network
	defer func() {
		*network = oldNetwork
	}()

	*network = false

	hash := func(i int) string {
		return fmt.Sprintf("abc123%d", i)
	}

	commit := func(i int) *Commit {
		c := &Commit{
			Hash:   hash(i),
			Author: "Sarah Adams <shadams@google.com>",
			Date:   "Fri, 15 Sep 2017 13:56:53 -0700",
			Desc:   fmt.Sprintf("CONTRIBUTORS: add person %d.", i),
			Files:  "CONTRIBUTORS",
		}

		if i > 0 {
			c.Parent = hash(i - 1)
		}

		return c
	}

	gitLogFn = func(r *Repo, dir string, args ...string) ([]*Commit, error) {
		// We are testing new commits on a non-master branch.
		// So, for simplicity, return no new commits on master.
		for _, a := range args {
			if strings.Contains(a, "origin/master") {
				return nil, nil
			}
		}

		var cs []*Commit
		for i := 0; i < 5; i++ {
			cs = append(cs, commit(i))
		}
		return cs, nil
	}

	gitRemotesFn = func(r *Repo) ([]string, error) {
		return []string{"origin/master", "origin/other.branch"}, nil
	}

	repo := newTestRepo()

	// Add a known commit on master.
	// This commit is HEAD of origin/master when we forked to
	// create the 'origin/other.branch' branch.
	head := commit(0)
	head.Branch = "origin/master"
	repo.commits[hash(0)] = head

	master := &Branch{
		Name:     "origin/master",
		Head:     head,
		LastSeen: head,
	}
	repo.branches["origin/master"] = master

	err := repo.update(false)
	if err != nil {
		t.Fatalf("update: got error %v", err)
	}

	head = repo.branches["origin/other.branch"].Head

	if head.Hash != hash(0) {
		t.Fatalf("expected head to have hash %s, got %s.", hash(0), head.Hash)
	}

	if len(head.children) != 1 {
		t.Fatalf("expected head to have 1 child commit, got %d.", len(head.children))
	}

	for i := 0; i < 5; i++ {
		if i != 0 {
			if repo.commits[hash(i)].parent == nil {
				t.Errorf("expected commit %d to have a parent commit.", i)
			}
		}
		if i != 4 {
			if len(repo.commits[hash(i)].children) == 0 {
				t.Errorf("expected commit %d to have child commits.", i)
			}
		}
	}
}

func newTestRepo() *Repo {
	return &Repo{
		path:     "",
		root:     "/usr/local/home/go",
		commits:  make(map[string]*Commit),
		branches: make(map[string]*Branch),
		mirror:   false,
		dash:     false,
	}
}
