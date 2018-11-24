// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The retrybuilds command clears build failures from the build.golang.org dashboard
// to force them to be rebuilt.
//
// Valid usage modes:
//
//   retrybuilds -loghash=f45f0eb8
//   retrybuilds -builder=openbsd-amd64
//   retrybuilds -builder=openbsd-amd64 -hash=6fecb7
//   retrybuilds -redo-flaky
//   retrybuilds -redo-flaky -builder=linux-amd64-clang
package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	masterKeyFile = flag.String("masterkey", filepath.Join(os.Getenv("HOME"), "keys", "gobuilder-master.key"), "path to Go builder master key. If present, the key argument is not necessary")
	keyFile       = flag.String("key", "", "path to key file")
	builder       = flag.String("builder", "", "builder to wipe a result for.")
	hash          = flag.String("hash", "", "Hash to wipe. If empty, all will be wiped.")
	redoFlaky     = flag.Bool("redo-flaky", false, "Reset all flaky builds. If builder is empty, the master key is required.")
	builderPrefix = flag.String("builder-prefix", "https://build.golang.org", "builder URL prefix")
	logHash       = flag.String("loghash", "", "If non-empty, clear the build that failed with this loghash prefix")
	sendMasterKey = flag.Bool("sendmaster", false, "send the master key in request instead of a builder-specific key; allows overriding actions of revoked keys")
	branch        = flag.String("branch", "master", "branch to find flakes from (for use with -redo-flaky)")
)

type Failure struct {
	Builder string
	Hash    string
	LogURL  string
}

func main() {
	flag.Parse()
	*builderPrefix = strings.TrimSuffix(*builderPrefix, "/")
	if *logHash != "" {
		substr := "/log/" + *logHash
		for _, f := range failures() {
			if strings.Contains(f.LogURL, substr) {
				wipe(f.Builder, f.Hash)
			}
		}
		return
	}
	if *redoFlaky {
		fixTheFlakes()
		return
	}
	if *builder == "" {
		log.Fatalf("Missing -builder, -redo-flaky, or -loghash flag.")
	}
	wipe(*builder, fullHash(*hash))
}

func fixTheFlakes() {
	gate := make(chan bool, 50)
	var wg sync.WaitGroup
	for _, f := range failures() {
		f := f
		if *builder != "" && f.Builder != *builder {
			continue
		}
		gate <- true
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-gate }()
			res, err := http.Get(f.LogURL)
			if err != nil {
				log.Fatalf("Error fetching %s: %v", f.LogURL, err)
			}
			failLog, err := ioutil.ReadAll(res.Body)
			res.Body.Close()
			if err != nil {
				log.Fatalf("Error reading %s: %v", f.LogURL, err)
			}
			if isFlaky(string(failLog)) {
				log.Printf("Restarting flaky %+v", f)
				wipe(f.Builder, f.Hash)
			}
		}()
	}
	wg.Wait()
}

var flakePhrases = []string{
	"No space left on device",
	"no space left on device", // solaris case apparently
	"fatal error: error in backend: IO failure on output stream",
	"Boffset: unknown state 0",
	"Bseek: unknown state 0",
	"error exporting repository: exit status",
	"remote error: User Is Over Quota",
	"fatal: remote did not send all necessary objects",
	"Failed to schedule \"", // e.g. Failed to schedule "go_test:archive/tar" test after 3 tries.
	"lookup _xmpp-server._tcp.google.com. on 8.8.8.8:53: dial udp 8.8.8.8:53: i/o timeout",
	"lookup _xmpp-server._tcp.google.com on",
	"lookup gmail.com. on 8.8.8.8:53: dial udp 8.8.8.8:53: i/o timeout",
	"lookup gmail.com on 8.8.8.8:53",
	"lookup www.mit.edu on ",
	"undefined: runtime.SetMutexProfileFraction", // ppc64 builders had not-quite-go1.8 bootstrap
	"make.bat: The parameter is incorrect",
}

func isFlaky(failLog string) bool {
	if strings.Count(strings.TrimSpace(failLog), "\n") < 2 {
		return true
	}
	if strings.HasPrefix(failLog, "exit status ") {
		return true
	}
	if strings.HasPrefix(failLog, "timed out after ") {
		return true
	}
	if strings.HasPrefix(failLog, "Failed to schedule ") {
		return true
	}
	for _, phrase := range flakePhrases {
		if strings.Contains(failLog, phrase) {
			return true
		}
	}
	numLines := strings.Count(failLog, "\n")
	if numLines < 20 && strings.Contains(failLog, "error: exit status") {
		return true
	}
	// e.g. fatal: destination path 'go.tools.TMP' already exists and is not an empty directory.
	// To be fixed in golang.org/issue/9407
	if strings.Contains(failLog, "fatal: destination path '") &&
		strings.Contains(failLog, "' already exists and is not an empty directory.") {
		return true
	}
	return false
}

func fullHash(h string) string {
	if h == "" || len(h) == 40 {
		return h
	}
	for _, f := range failures() {
		if strings.HasPrefix(f.Hash, h) {
			return f.Hash
		}
	}
	log.Fatalf("invalid hash %q; failed to finds its full hash. Not a recent failure?", h)
	panic("unreachable")
}

// hash may be empty
func wipe(builder, hash string) {
	if hash != "" {
		log.Printf("Clearing %s, hash %s", builder, hash)
	} else {
		log.Printf("Clearing all builds for %s", builder)
	}
	vals := url.Values{
		"builder": {builder},
		"hash":    {hash},
		"key":     {builderKey(builder)},
	}
	for i := 0; i < 10; i++ {
		res, err := http.PostForm(*builderPrefix+"/clear-results?"+vals.Encode(), nil)
		if err != nil {
			log.Fatal(err)
		}
		body, err := ioutil.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			log.Fatal(err)
		}
		if res.StatusCode != 200 {
			log.Fatalf("Error clearing %v hash %q: %v", builder, hash, res.Status)
		}
		var dashResponse struct {
			Error string
		}
		if err := json.Unmarshal(body, &dashResponse); err != nil {
			log.Fatalf("Bad dashboard response: %v\nBody: %s", err, body)
		}

		switch e := dashResponse.Error; e {
		case "datastore: concurrent transaction":
			log.Printf("Concurrent datastore transaction wiping %v %v: retrying in 1 second", builder, hash)
			time.Sleep(time.Second)
			continue
		default:
			log.Fatalf("Dashboard error: %v", e)
		case "":
			return
		}
	}
	log.Fatalf("Too many datastore transaction issues wiping %v %v", builder, hash)
}

func builderKey(builder string) string {
	if v, ok := builderKeyFromMaster(builder); ok {
		return v
	}
	if *keyFile == "" {
		log.Fatalf("No --key specified for builder %s", builder)
	}
	slurp, err := ioutil.ReadFile(*keyFile)
	if err != nil {
		log.Fatalf("Error reading builder key %s: %v", builder, err)
	}
	return strings.TrimSpace(string(slurp))
}

func builderKeyFromMaster(builder string) (key string, ok bool) {
	if *masterKeyFile == "" {
		return
	}
	slurp, err := ioutil.ReadFile(*masterKeyFile)
	if err != nil {
		return
	}
	if *sendMasterKey {
		return string(slurp), true
	}
	h := hmac.New(md5.New, bytes.TrimSpace(slurp))
	h.Write([]byte(builder))
	return fmt.Sprintf("%x", h.Sum(nil)), true
}

var (
	failMu    sync.Mutex
	failCache []Failure
)

func failures() (ret []Failure) {
	failMu.Lock()
	ret = failCache
	failMu.Unlock()
	if ret != nil {
		return
	}
	ret = []Failure{} // non-nil

	res, err := http.Get(*builderPrefix + "/?mode=failures&branch=" + url.QueryEscape(*branch))
	if err != nil {
		log.Fatal(err)
	}
	slurp, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		log.Fatal(err)
	}
	body := string(slurp)
	for _, line := range strings.Split(body, "\n") {
		f := strings.Fields(line)
		if len(f) == 3 {
			ret = append(ret, Failure{
				Hash:    f[0],
				Builder: f[1],
				LogURL:  f[2],
			})
		}
	}

	failMu.Lock()
	failCache = ret
	failMu.Unlock()
	return ret
}
