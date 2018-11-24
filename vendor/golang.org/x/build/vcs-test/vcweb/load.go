// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
)

var (
	ctx    = context.Background()
	client *storage.Client
	bucket *storage.BucketHandle
)

var cache struct {
	sync.Mutex
	entry map[string]*cacheEntry
}

type cacheEntry struct {
	sync.Mutex
	expire time.Time
	md5    []byte
}

func init() {
	var err error
	client, err = storage.NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}
	bucket = client.Bucket("vcs-test")
	cache.entry = map[string]*cacheEntry{}
}

func loadFS(dir1, dir2 string, force bool) {
	name := dir1 + "/" + dir2
	defer func() {
		if err := recover(); err != nil {
			log.Printf("%s: %v", name, err)
		}
	}()
	check := func(err error) {
		if err != nil {
			panic(err)
		}
	}

	check(os.MkdirAll(filepath.Join(*dir, dir1), 0777))

	cache.Lock()
	entry := cache.entry[name]
	if entry == nil {
		entry = new(cacheEntry)
		cache.entry[name] = entry
	}
	cache.Unlock()

	entry.Lock()
	defer entry.Unlock()

	if time.Now().Before(entry.expire) && !force {
		return
	}

	entry.expire = time.Now().Add(5 * time.Minute)

	obj := bucket.Object(name + ".zip")
	attrs, err := obj.Attrs(ctx)
	check(err)
	if bytes.Equal(attrs.MD5, entry.md5) {
		return
	}

	r, err := obj.NewReader(ctx)
	check(err)
	defer r.Close()

	zipFile := filepath.Join(*dir, name+".zip")
	zf, err := os.Create(zipFile)
	check(err)

	h := md5.New()
	_, err = io.Copy(io.MultiWriter(zf, h), r)
	check(zf.Close())
	check(err)
	sum := h.Sum(nil)

	if !bytes.Equal(attrs.MD5, sum) {
		panic(fmt.Sprintf("load: unexpected md5 %x != %x", sum, attrs.MD5))
	}

	zf, err = os.Open(zipFile)
	check(err)
	info, err := zf.Stat()
	check(err)
	zr, err := zip.NewReader(zf, info.Size())
	check(err)

	tmp := filepath.Join(*dir, dir1+"/_"+dir2)
	check(os.RemoveAll(tmp))
	check(os.MkdirAll(tmp, 0777))

	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "/") {
			check(os.MkdirAll(filepath.Join(tmp, f.Name), 0777))
			continue
		}
		w, err := os.Create(filepath.Join(tmp, f.Name))
		check(err)
		r, err := f.Open()
		check(err)
		_, err = io.Copy(w, r)
		check(err)
		check(w.Close())
	}

	real := filepath.Join(*dir, dir1+"/"+dir2)
	check(os.RemoveAll(real))
	check(os.Rename(tmp, real))
	entry.md5 = sum
}
