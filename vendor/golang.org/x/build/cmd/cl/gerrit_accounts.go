// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/build/gerrit"
	"golang.org/x/build/maintner/godata"
)

// GerritAccounts holds a mapping of Gerrit account IDs to
// the corresponding gerrit.AccountInfo object.
// A call to Initialize must be made in order for the map to be populated.
type GerritAccounts struct {
	accounts    map[int64]*gerrit.AccountInfo // Gerrit account ID to AccountInfo.
	refreshTime time.Time
}

// ErrNotFound is the error returned when no mapping for a Gerrit email address is available.
var ErrNotFound = errors.New("no mapping found for the given Gerrit email address")

// LookupByGerritEmail translates a Gerrit email address in the format of
// <Gerrit User ID>@<Gerrit server UUID> into the actual email address of the person.
// If the cache is out of date, and fetchUpdates is true, it'll download a fresh mapping from Gerrit,
// and persist it as well. If fetchUpdates is false, then ErrNotFound is returned.
// After downloading a fresh mapping, and a mapping for an account ID is not found,
// then ErrNotFound is returned.
func (ga *GerritAccounts) LookupByGerritEmail(gerritEmail string, fetchUpdates bool) (*gerrit.AccountInfo, error) {
	if gerritEmail == "" {
		return nil, errors.New("gerritEmail cannot be empty")
	}

	atIdx := strings.LastIndex(gerritEmail, "@")
	if atIdx == -1 {
		return nil, fmt.Errorf("LookupByGerritEmail: %q is not a valid email address", gerritEmail)
	}

	accountId, err := strconv.Atoi(gerritEmail[0:atIdx])
	if err != nil {
		return nil, fmt.Errorf("LookupByGerritEmail: %q is not of the form <Gerrit User ID>@<Gerrit server UUID>", gerritEmail)
	}

	account := ga.accounts[int64(accountId)]
	if account != nil {
		// The cached mapping might be the same as gerritEmail (as it's the default if a mapping is missing).
		// Return ErrNotFound in that case.
		if account.Email == gerritEmail {
			return nil, ErrNotFound
		}
		return account, nil
	}

	if !fetchUpdates {
		return nil, ErrNotFound
	}

	// Cache miss, let's sync up with Gerrit.

	// We should also add a default value for this email address - in case
	// Gerrit doesn't have this account ID (which would be rare - or the account is inactive),
	// we don't want to keep making network calls.
	// As GerritAccounts holds a map, if Gerrit returns a valid mapping,
	// it will be overridden.
	ga.accounts[int64(accountId)] = &gerrit.AccountInfo{
		Email:     gerritEmail,
		NumericID: int64(accountId),
		Name:      gerritEmail,
		Username:  gerritEmail,
	}

	// If we've recently hit Gerrit for a fresh mapping already, then skip a network call,
	// and persist the default version for this gerritEmail.
	if time.Now().Sub(ga.refreshTime).Minutes() < 5 {
		log.Println("Skipping Gerrit account info lookup for", gerritEmail)
		err = ga.cacheMappingToDisk()
		if err != nil {
			return nil, err
		}

		return nil, ErrNotFound
	}

	if err := ga.fetchAndPersist(); err != nil {
		return nil, err
	}

	if ga.accounts[int64(accountId)].Email == gerritEmail {
		return nil, ErrNotFound
	}

	return ga.accounts[int64(accountId)], nil
}

// refresh makes a call to the Gerrit server, and updates the mapping.
// It also updates refreshTime, after the update has completed.
func (ga *GerritAccounts) refresh() error {
	if ga.accounts == nil {
		ga.accounts = map[int64]*gerrit.AccountInfo{}
	}

	c := gerrit.NewClient("https://go-review.googlesource.com", gerrit.NoAuth)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if ctx.Err() != nil {
		return ctx.Err()
	}

	start := 0
	for {
		accounts, err := c.QueryAccounts(ctx, "is:active",
			gerrit.QueryAccountsOpt{Fields: []string{"DETAILS"}, Start: start})

		if err != nil {
			return ctx.Err()
		}

		start += len(accounts)

		for _, account := range accounts {
			ga.accounts[account.NumericID] = account
		}

		log.Println("Fetched", start, "accounts from Gerrit")

		if accounts[len(accounts)-1].MoreAccounts == false {
			break
		}
	}

	ga.refreshTime = time.Now()

	return nil
}

// cacheMappingToDisk serializes the map and writes it to the cache directory.
func (ga *GerritAccounts) cacheMappingToDisk() error {
	cachePath, err := cachePath()

	if err != nil {
		return err
	}

	var out bytes.Buffer
	encoder := gob.NewEncoder(&out)

	err = encoder.Encode(ga.accounts)

	if err != nil {
		return err
	}

	err = ioutil.WriteFile(cachePath, out.Bytes(), 0600)
	if err != nil {
		return err
	}

	return nil
}

// Initialize does either one of the following two things, in order:
// 1. If a cached mapping exists, then restore the map from the cache and return.
// 2. If the cached mapping does not exist, hit Gerrit (call refresh()), and then persist the mapping.
func (ga *GerritAccounts) Initialize() error {
	cachePath, err := cachePath()
	if err != nil {
		return err
	}

	if cache, err := ioutil.ReadFile(cachePath); err == nil {
		d := gob.NewDecoder(bytes.NewReader(cache))

		if err := d.Decode(&ga.accounts); err != nil {
			return err
		}
		log.Println("Read Gerrit accounts information from disk cache")
		return nil
	}

	if err := ga.fetchAndPersist(); err != nil {
		return err
	}

	return nil
}

func (ga *GerritAccounts) fetchAndPersist() error {
	log.Println("Fetching accounts mapping from Gerrit. This will take some time...")

	err := ga.refresh()
	if err != nil {
		return err
	}

	err = ga.cacheMappingToDisk()
	if err != nil {
		return err
	}

	return nil
}

func cachePath() (string, error) {
	targetDir := godata.XdgCacheDir()
	targetDir = filepath.Join(targetDir, "golang-build-cmd-cl")
	if err := os.MkdirAll(targetDir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(targetDir, "accounts"), nil
}
