// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"cloud.google.com/go/storage"
)

const releaseBucket = "golang-release-staging"

var gcsClient *storage.Client

func loadGCSAuth() {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Test early that we can write to the bucket.
	name := fmt.Sprintf(".writable/%d", time.Now().UnixNano())
	err = client.Bucket(releaseBucket).Object(name).NewWriter(ctx).Close()
	if err != nil {
		log.Fatalf("cannot write to %s: %v", releaseBucket, err)
	}
	err = client.Bucket(releaseBucket).Object(name).Delete(ctx)
	if err != nil {
		log.Fatalf("cannot delete from %s: %v", releaseBucket, err)
	}

	gcsClient = client
}

func gcsUpload(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	sum := h.Sum(nil)
	if _, err := f.Seek(0, 0); err != nil {
		return err
	}

	ctx := context.Background()
	obj := gcsClient.Bucket("golang-release-staging").Object(dst)
	if attrs, err := obj.Attrs(ctx); err == nil && bytes.Equal(attrs.MD5, sum[:]) {
		return nil
	}

	cloud := obj.NewWriter(ctx)
	if _, err := io.Copy(cloud, f); err != nil {
		cloud.Close()
		return err
	}
	if err := cloud.Close(); err != nil {
		return err
	}
	if attrs, err := obj.Attrs(ctx); err != nil || !bytes.Equal(attrs.MD5, sum[:]) {
		if err == nil {
			err = fmt.Errorf("md5 mismatch")
		}
		return fmt.Errorf("upload %s: %v", dst, err)
	}

	return nil
}
