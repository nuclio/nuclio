// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Fetchlogs downloads build failure logs from the Go dashboard so
// they can be accessed and searched from the local file system.
//
// It organizes these logs into two directories created in the
// directory specified by the -dir flag (which typically defaults to
// ~/.cache/fetchlogs). The log/ directory contains all log files
// named the same way they are named by the dashboard (which happens
// to be the SHA-1 of their contents). The rev/ directory contains
// symlinks back to these logs named
//
//    rev/<ISO 8601 commit date>-<git revision>/<builder>
//
// Fetchlogs will reuse existing log files and revision symlinks, so
// it only has to download logs that are new since the last time it
// was run.
//
// This makes failures easily searchable with standard tools. For
// example, to list the revisions and builders with a particular
// failure, use:
//
//    grep -lR <regexp> rev | sort
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/build/types"
)

var defaultDir = filepath.Join(xdgCacheDir(), "fetchlogs")

var (
	flagN   = flag.Int("n", 300, "limit to most recent `N` commits")
	flagPar = flag.Int("j", 5, "number of concurrent download `jobs`")
	flagDir = flag.String("dir", defaultDir, "`directory` to save logs to")
)

func main() {
	log.SetPrefix("fetchlogs: ")
	log.SetFlags(0)

	flag.Parse()
	if flag.NArg() != 0 {
		flag.Usage()
		os.Exit(2)
	}

	// If the top-level directory is the default XDG cache
	// directory, make sure it exists.
	if *flagDir == defaultDir {
		if err := xdgCreateDir(*flagDir); err != nil {
			log.Fatal(err)
		}
	}

	// Create directory structure.
	if err := os.Chdir(*flagDir); err != nil {
		log.Fatal(err)
	}
	ensureDir("log")
	ensureDir("rev")

	// Set up fetchers.
	fetcher := newFetcher(*flagPar)
	wg := sync.WaitGroup{}

	// Fetch dashboard pages.
	haveCommits := 0
	for page := 0; haveCommits < *flagN; page++ {
		url := fmt.Sprintf("https://build.golang.org/?mode=json&page=%d", page)
		index, err := fetcher.get(url)
		if err != nil {
			log.Fatal(err)
		}

		var status types.BuildStatus
		if err = json.NewDecoder(index).Decode(&status); err != nil {
			log.Fatal("error unmarshalling result: ", err)
		}
		index.Close()

		for _, rev := range status.Revisions {
			haveCommits++
			if haveCommits > *flagN {
				break
			}
			if rev.Repo != "go" {
				continue
			}

			// Create a revision directory. This way we
			// have a record of commits with no failures.
			date, err := parseRevDate(rev.Date)
			if err != nil {
				log.Fatal("malformed revision date: ", err)
			}
			revDir := revToDir(rev.Revision, date)
			ensureDir(revDir)

			// Save revision metadata.
			buf := bytes.Buffer{}
			enc := json.NewEncoder(&buf)
			if err = enc.Encode(rev); err != nil {
				log.Fatal(err)
			}
			if err = writeFileAtomic(filepath.Join(revDir, ".rev.json"), &buf); err != nil {
				log.Fatal("error saving revision metadata: ", err)
			}

			// Save builders list so Results list can be
			// interpreted.
			if err = enc.Encode(status.Builders); err != nil {
				log.Fatal(err)
			}
			if err = writeFileAtomic(filepath.Join(revDir, ".builders.json"), &buf); err != nil {
				log.Fatal("error saving builders metadata: ", err)
			}

			// Fetch revision logs.
			for i, res := range rev.Results {
				if res == "" || res == "ok" {
					continue
				}

				wg.Add(1)
				go func(rev, builder, logURL string) {
					defer wg.Done()
					logPath := filepath.Join("log", filepath.Base(logURL))
					err := fetcher.getFile(logURL, logPath)
					if err != nil {
						log.Fatal("error fetching log: ", err)
					}
					if err := linkLog(revDir, builder, logPath); err != nil {
						log.Fatal("error linking log: ", err)
					}
				}(revDir, status.Builders[i], res)
			}
		}
	}

	wg.Wait()
}

// A fetcher downloads files over HTTP concurrently. It allows
// limiting the number of concurrent downloads and correctly handles
// multiple (possibly concurrent) fetches from the same URL to the
// same file.
type fetcher struct {
	tokens chan struct{}

	pending struct {
		sync.Mutex
		m map[string]*pendingFetch
	}
}

type pendingFetch struct {
	wchan chan struct{} // closed when fetch completes

	// err is the error, if any, that occurred during this fetch.
	// It will be set before wchan is closed.
	err error
}

func newFetcher(jobs int) *fetcher {
	f := new(fetcher)

	f.tokens = make(chan struct{}, *flagPar)
	for i := 0; i < jobs; i++ {
		f.tokens <- struct{}{}
	}

	f.pending.m = make(map[string]*pendingFetch)

	return f
}

// get performs an HTTP GET for URL and returns the body, while
// obeying the job limit on fetcher.
func (f *fetcher) get(url string) (io.ReadCloser, error) {
	<-f.tokens
	fmt.Println("fetching", url)
	resp, err := http.Get(url)
	f.tokens <- struct{}{}
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

// getFile performs an HTTP GET for URL and writes it to filename. If
// the destination file already exists, this returns immediately. If
// another goroutine is currently fetching filename, this blocks until
// the fetch is done and then returns.
func (f *fetcher) getFile(url string, filename string) error {
	// Do we already have it?
	if _, err := os.Stat(filename); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	// Check if another fetcher is working on it.
	f.pending.Lock()
	if p, ok := f.pending.m[filename]; ok {
		f.pending.Unlock()
		<-p.wchan
		return p.err
	}

	p := &pendingFetch{wchan: make(chan struct{})}
	f.pending.m[filename] = p
	f.pending.Unlock()

	r, err := f.get(url)
	if err == nil {
		err = writeFileAtomic(filename, r)
		r.Close()
	}
	p.err = err

	close(p.wchan)
	return p.err
}

// ensureDir creates directory name if it does not exist.
func ensureDir(name string) {
	err := os.Mkdir(name, 0777)
	if err != nil && !os.IsExist(err) {
		log.Fatal("error creating directory ", name, ": ", err)
	}
}

// writeFileAtomic atomically creates a file called filename and
// copies the data from r to the file.
func writeFileAtomic(filename string, r io.Reader) error {
	tmpPath := filename + ".tmp"
	if f, err := os.Create(tmpPath); err != nil {
		return err
	} else {
		_, err := io.Copy(f, r)
		if err == nil {
			err = f.Sync()
		}
		err2 := f.Close()
		if err == nil {
			err = err2
		}
		if err != nil {
			os.Remove(tmpPath)
			return err
		}
	}
	if err := os.Rename(tmpPath, filename); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

// linkLog creates a symlink for finding logPath based on its git
// revision and builder.
func linkLog(revDir, builder, logPath string) error {
	// Create symlink.
	err := os.Symlink("../../"+logPath, filepath.Join(revDir, builder))
	if err != nil && !os.IsExist(err) {
		return err
	}

	return nil
}

// parseRevDate parses a revision date in RFC3339.
func parseRevDate(date string) (time.Time, error) {
	return time.Parse(time.RFC3339, date)
}

// revToDir returns the path of the revision directory for revision.
func revToDir(revision string, date time.Time) string {
	return filepath.Join("rev", date.Format("2006-01-02T15:04:05")+"-"+revision[:7])
}
