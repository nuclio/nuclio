// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package godata_test

import (
	"context"
	"fmt"
	"log"

	"golang.org/x/build/maintner"
	"golang.org/x/build/maintner/godata"
)

func ExampleGet_numComments() {
	corpus, err := godata.Get(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	num := 0
	corpus.GitHub().ForeachRepo(func(gr *maintner.GitHubRepo) error {
		return gr.ForeachIssue(func(gi *maintner.GitHubIssue) error {
			return gi.ForeachComment(func(*maintner.GitHubComment) error {
				num++
				return nil
			})
		})
	})
	fmt.Printf("%d GitHub comments on Go repos.\n", num)
}
