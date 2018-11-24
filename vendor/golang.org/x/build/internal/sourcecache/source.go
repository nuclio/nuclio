// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package sourcecache provides a cache of code found in Git repositories.
// It takes directly to the Gerrit instance at go.googlesource.com.
// If RegisterGitMirrorDial is called, it will first try to get code from gitmirror before falling back on Gerrit.
package sourcecache

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"time"

	"golang.org/x/build/cmd/coordinator/spanlog"
	"golang.org/x/build/internal/lru"
	"golang.org/x/build/internal/singleflight"
)

var processStartTime = time.Now()

var sourceGroup singleflight.Group

var sourceCache = lru.New(40) // git rev -> []byte

// GetSourceTgz returns a Reader that provides a tgz of the requested source revision.
// repo is go.googlesource.com repo ("go", "net", etc)
// rev is git revision.
func GetSourceTgz(sl spanlog.Logger, repo, rev string) (tgz io.Reader, err error) {
	sp := sl.CreateSpan("get_source")
	defer func() { sp.Done(err) }()

	key := fmt.Sprintf("%v-%v", repo, rev)
	vi, err, _ := sourceGroup.Do(key, func() (interface{}, error) {
		if tgzBytes, ok := sourceCache.Get(key); ok {
			return tgzBytes, nil
		}

		if gitMirrorClient != nil {
			sp := sl.CreateSpan("get_source_from_gitmirror")
			tgzBytes, err := getSourceTgzFromGitMirror(repo, rev)
			if err == nil {
				sourceCache.Add(key, tgzBytes)
				sp.Done(nil)
				return tgzBytes, nil
			}
			log.Printf("Error fetching source %s/%s from watcher (after %v uptime): %v",
				repo, rev, time.Since(processStartTime), err)
			sp.Done(errors.New("timeout"))
		}

		sp := sl.CreateSpan("get_source_from_gerrit", fmt.Sprintf("%v from gerrit", key))
		tgzBytes, err := getSourceTgzFromGerrit(repo, rev)
		sp.Done(err)
		if err == nil {
			sourceCache.Add(key, tgzBytes)
		}
		return tgzBytes, err
	})
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(vi.([]byte)), nil
}

var gitMirrorClient *http.Client

// RegisterGitMirrorDial registers a dial function which will be used to reach gitmirror.
// If used, this function must be called before GetSourceTgz.
func RegisterGitMirrorDial(dial func(context.Context) (net.Conn, error)) {
	gitMirrorClient = &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			IdleConnTimeout: 30 * time.Second,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dial(ctx)
			},
		},
	}
}

var gerritHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
}

func getSourceTgzFromGerrit(repo, rev string) (tgz []byte, err error) {
	return getSourceTgzFromURL(gerritHTTPClient, "gerrit", repo, rev, "https://go.googlesource.com/"+repo+"/+archive/"+rev+".tar.gz")
}

func getSourceTgzFromGitMirror(repo, rev string) (tgz []byte, err error) {
	for i := 0; i < 2; i++ { // two tries; different pods maybe?
		if i > 0 {
			time.Sleep(1 * time.Second)
		}
		// The "gitmirror" hostname is unused:
		tgz, err = getSourceTgzFromURL(gitMirrorClient, "gitmirror", repo, rev, "http://gitmirror/"+repo+".tar.gz?rev="+rev)
		if err == nil {
			return tgz, nil
		}
		if tr, ok := http.DefaultTransport.(*http.Transport); ok {
			tr.CloseIdleConnections()
		}
	}
	return nil, err
}

func getSourceTgzFromURL(hc *http.Client, source, repo, rev, urlStr string) (tgz []byte, err error) {
	res, err := hc.Get(urlStr)
	if err != nil {
		return nil, fmt.Errorf("fetching %s/%s from %s: %v", repo, rev, source, err)
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		slurp, _ := ioutil.ReadAll(io.LimitReader(res.Body, 4<<10))
		return nil, fmt.Errorf("fetching %s/%s from %s: %v; body: %s", repo, rev, source, res.Status, slurp)
	}
	// TODO(bradfitz): finish golang.org/issue/11224
	const maxSize = 50 << 20 // talks repo is over 25MB; go source is 7.8MB on 2015-06-15
	slurp, err := ioutil.ReadAll(io.LimitReader(res.Body, maxSize+1))
	if len(slurp) > maxSize && err == nil {
		err = fmt.Errorf("body over %d bytes", maxSize)
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s/%s from %s: %v", repo, rev, source, err)
	}
	return slurp, nil
}
