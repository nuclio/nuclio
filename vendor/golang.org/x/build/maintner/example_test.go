// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package maintner_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/build/maintner"
)

func Example() {
	cacheDir := filepath.Join(os.Getenv("HOME"), "var", "maintnerd")
	// maintner.golang.org contains the metadata for the Go project and related
	// Github repositories and issues.
	mutSrc := maintner.NewNetworkMutationSource("https://maintner.golang.org/logs", cacheDir)
	corpus := new(maintner.Corpus)
	if err := corpus.Initialize(context.Background(), mutSrc); err != nil {
		log.Fatal(err)
	}
	err := corpus.GitHub().ForeachRepo(func(r *maintner.GitHubRepo) error {
		fmt.Println(r.ID())
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
}

func Example_fromDisk() {
	logger := maintner.NewDiskMutationLogger(filepath.Join(os.Getenv("HOME"), "var", "maintnerd"))
	corpus := new(maintner.Corpus)
	corpus.Initialize(context.Background(), logger)
	err := corpus.GitHub().ForeachRepo(func(r *maintner.GitHubRepo) error {
		fmt.Println(r.ID())
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
}

func ExampleDiskMutationLogger() {
	logger := maintner.NewDiskMutationLogger(filepath.Join(os.Getenv("HOME"), "var", "maintnerd"))
	corpus := new(maintner.Corpus)
	corpus.Initialize(context.Background(), logger)
	err := corpus.GitHub().ForeachRepo(func(r *maintner.GitHubRepo) error {
		fmt.Println(r.ID())
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
}
