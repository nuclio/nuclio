// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The rmplaysnippet binary removes a code snippet from play.golang.org given its URL
// or ID.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"cloud.google.com/go/datastore"
	"golang.org/x/build/buildenv"
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: %s {https://play.golang.org/p/<id> | <id>}\n", os.Args[0])
}

func main() {
	if len(os.Args) != 2 {
		usage()
		os.Exit(2)
	}

	snippetID := strings.TrimPrefix(os.Args[1], "https://play.golang.org/p/")
	if snippetID == "" {
		usage()
		os.Exit(2)
	}

	fmt.Printf("Really delete Snippet with ID %q? [y,N]: ", snippetID)
	var confirm string
	fmt.Scanln(&confirm)
	if !strings.HasPrefix(strings.ToLower(confirm), "y") {
		fmt.Printf("Aborting ...\n")
		os.Exit(0)
	}

	buildenv.CheckUserCredentials()
	ctx := context.Background()
	client, err := datastore.NewClient(ctx, "golang-org")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create Datastore client: %v\n", err)
		os.Exit(1)
	}
	k := datastore.NameKey("Snippet", snippetID, nil)
	if err := client.Delete(ctx, k); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to delete Snippet with ID %q: %v\n", snippetID, err)
		fmt.Fprintf(os.Stderr, "rmplaysnippet requires Application Default Credentials.\n")
		fmt.Fprintf(os.Stderr, "Did you run `gcloud auth application-default login`?\n")
		os.Exit(1)
	}
	fmt.Printf("Snippet with ID %q deleted\n", snippetID)
}
