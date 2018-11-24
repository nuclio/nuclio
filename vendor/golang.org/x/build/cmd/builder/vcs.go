// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/tools/go/vcs"
)

// Repo represents a git repository.
type Repo struct {
	Path   string
	Master *vcs.RepoRoot
	sync.Mutex
}

// RemoteRepo constructs a *Repo representing a remote repository.
func RemoteRepo(url, path string) (*Repo, error) {
	rr, err := vcs.RepoRootForImportPath(url, *verbose)
	if err != nil {
		return nil, err
	}
	return &Repo{
		Path:   path,
		Master: rr,
	}, nil
}

// Clone clones the current Repo to a new destination
// returning a new *Repo if successful.
func (r *Repo) Clone(path, rev string) (*Repo, error) {
	r.Lock()
	defer r.Unlock()

	err := timeout(*cmdTimeout, func() error {
		downloadPath := r.Path
		if !r.Exists() {
			downloadPath = r.Master.Repo
		}
		if rev == "" {
			return r.Master.VCS.Create(path, downloadPath)
		}
		return r.Master.VCS.CreateAtRev(path, downloadPath, rev)
	})
	if err != nil {
		return nil, err
	}
	return &Repo{
		Path:   path,
		Master: r.Master,
	}, nil
}

// Export exports the current Repo at revision rev to a new destination.
func (r *Repo) Export(path, rev string) error {
	// TODO(adg,cmang): implement Export in go/vcs
	_, err := r.Clone(path, rev)
	return err
}

// UpdateTo updates the working copy of this Repo to the
// supplied revision.
func (r *Repo) UpdateTo(hash string) error {
	r.Lock()
	defer r.Unlock()

	if r.Master.VCS.Cmd == "git" {
		cmd := exec.Command("git", "reset", "--hard", hash)
		var log bytes.Buffer
		err := run(cmd, runTimeout(*cmdTimeout), runDir(r.Path), allOutput(&log))
		if err != nil {
			return fmt.Errorf("Error running git update -C %v: %v ; output=%s", hash, err, log.Bytes())
		}
		return nil
	}

	// Else go down three more levels of abstractions, at
	// least two of which are broken for git.
	return timeout(*cmdTimeout, func() error {
		return r.Master.VCS.TagSync(r.Path, hash)
	})
}

// Exists reports whether this Repo represents a valid Mecurial repository.
func (r *Repo) Exists() bool {
	fi, err := os.Stat(filepath.Join(r.Path, "."+r.Master.VCS.Cmd))
	if err != nil {
		return false
	}
	return fi.IsDir()
}

// Pull pulls changes from the default path, that is, the path
// this Repo was cloned from.
func (r *Repo) Pull() error {
	r.Lock()
	defer r.Unlock()

	return timeout(*cmdTimeout, func() error {
		return r.Master.VCS.Download(r.Path)
	})
}

// FullHash returns the full hash for the given Git revision.
func (r *Repo) FullHash(rev string) (string, error) {
	r.Lock()
	defer r.Unlock()

	var hash string
	err := timeout(*cmdTimeout, func() error {
		var data []byte
		// Avoid the vcs package for git, since it's broken
		// for git, and and we're trying to remove levels of
		// abstraction which are increasingly getting
		// difficult to navigate.
		if r.Master.VCS.Cmd == "git" {
			cmd := exec.Command("git", "rev-parse", rev)
			var out bytes.Buffer
			err := run(cmd, runTimeout(*cmdTimeout), runDir(r.Path), allOutput(&out))
			data = out.Bytes()
			if err != nil {
				return fmt.Errorf("Failed to find FullHash of %q; git rev-parse: %v, %s", rev, err, data)
			}
		} else {
			var err error
			data, err = r.Master.VCS.LogAtRev(r.Path, rev, "{node}")
			if err != nil {
				return err
			}
		}
		s := strings.TrimSpace(string(data))
		if s == "" {
			return fmt.Errorf("cannot find revision")
		}
		if len(s) != 40 { // correct for both hg and git
			return fmt.Errorf("%s returned invalid hash: %s", r.Master.VCS, s)
		}
		hash = s
		return nil
	})
	if err != nil {
		return "", err
	}
	return hash, nil
}
