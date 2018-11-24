// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package gitauth writes gitcookies files so git will authenticate
// to Gerrit as gopherbot for quota purposes.
package gitauth

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"cloud.google.com/go/compute/metadata"
)

func Init() error {
	cookieFile := filepath.Join(homeDir(), ".gitcookies")
	if err := exec.Command("git", "config", "--global", "http.cookiefile", cookieFile).Run(); err != nil {
		return fmt.Errorf("running git config to set cookiefile: %v", err)
	}
	if !metadata.OnGCE() {
		// Do nothing for now.
		return nil
	}
	slurp, err := metadata.ProjectAttributeValue("gobot-password")
	if err != nil {
		proj, _ := metadata.ProjectID()
		if proj != "symbolic-datum-552" { // TODO: don't hard-code this; use buildenv package
			log.Printf("gitauth: ignoring 'gobot-password' GCE metadata lookup on non-prod project: %v", err)
			return nil
		}
		return fmt.Errorf("gitauth: getting gobot-password GCE metadata: %v", err)
	}
	slurp = strings.TrimSpace(slurp)
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "go.googlesource.com\tFALSE\t/\tTRUE\t2147483647\to\tgit-gobot.gmail.com=%s\n", slurp)
	fmt.Fprintf(&buf, "go-review.googlesource.com\tFALSE\t/\tTRUE\t2147483647\to\tgit-gobot.gmail.com=%s\n", slurp)
	return ioutil.WriteFile(cookieFile, buf.Bytes(), 0644)
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	log.Fatalf("No HOME set in environment.")
	panic("unreachable")
}
