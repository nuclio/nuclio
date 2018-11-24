// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build appengine

package cache

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/gob"
	"fmt"
	"net/http"
	"time"

	"google.golang.org/appengine/log"
	"google.golang.org/appengine/memcache"
)

// TimeKey specifies the memcache entity that keeps the logical datastore time.
var TimeKey = "cachetime"

const (
	nocache = "nocache"
	expiry  = 10 * time.Minute
)

func newTime() uint64 { return uint64(time.Now().Unix()) << 32 }

// Now returns the current logical datastore time to use for cache lookups.
func Now(c context.Context) uint64 {
	t, err := memcache.Increment(c, TimeKey, 0, newTime())
	if err != nil {
		log.Errorf(c, "cache.Now: %v", err)
		return 0
	}
	return t
}

// Tick sets the current logical datastore time to a never-before-used time
// and returns that time. It should be called to invalidate the cache.
func Tick(c context.Context) uint64 {
	t, err := memcache.Increment(c, TimeKey, 1, newTime())
	if err != nil {
		log.Errorf(c, "cache.Tick: %v", err)
		return 0
	}
	return t
}

// Get fetches data for name at time now from memcache and unmarshals it into
// value. It reports whether it found the cache record and logs any errors to
// the admin console.
func Get(c context.Context, r *http.Request, now uint64, name string, value interface{}) bool {
	if now == 0 || r.FormValue(nocache) != "" {
		return false
	}
	key := fmt.Sprintf("%s.%d", name, now)
	_, err := gzipGobCodec.Get(c, key, value)
	if err == nil {
		log.Debugf(c, "cache hit %q", key)
		return true
	}
	log.Debugf(c, "cache miss %q", key)
	if err != memcache.ErrCacheMiss {
		log.Errorf(c, "get cache %q: %v", key, err)
	}
	return false
}

// Set puts value into memcache under name at time now.
// It logs any errors to the admin console.
func Set(c context.Context, r *http.Request, now uint64, name string, value interface{}) {
	if now == 0 || r.FormValue(nocache) != "" {
		return
	}
	key := fmt.Sprintf("%s.%d", name, now)
	err := gzipGobCodec.Set(c, &memcache.Item{
		Key:        key,
		Object:     value,
		Expiration: expiry,
	})
	if err != nil {
		log.Errorf(c, "set cache %q: %v", key, err)
	}
}

var gzipGobCodec = memcache.Codec{Marshal: marshal, Unmarshal: unmarshal}

func marshal(v interface{}) ([]byte, error) {
	var b bytes.Buffer
	zw := gzip.NewWriter(&b)
	if err := gob.NewEncoder(zw).Encode(v); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func unmarshal(b []byte, v interface{}) error {
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return err
	}
	return gob.NewDecoder(zr).Decode(v)
}
