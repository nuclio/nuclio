// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The buildstats command syncs build logs from Datastore to Bigquery.
//
// It will eventually also do more stats.
package main // import "golang.org/x/build/cmd/buildstats"

import (
	"context"
	"flag"
	"log"

	"golang.org/x/build/buildenv"
	"golang.org/x/build/internal/buildstats"
)

var (
	doSync  = flag.Bool("sync", false, "sync build stats data from Datastore to BigQuery")
	verbose = flag.Bool("v", false, "verbose")
)

var env *buildenv.Environment

func main() {
	buildenv.RegisterFlags()
	flag.Parse()
	buildstats.Verbose = *verbose

	env = buildenv.FromFlags()

	ctx := context.Background()
	if *doSync {
		if err := buildstats.SyncBuilds(ctx, env); err != nil {
			log.Fatalf("SyncBuilds: %v", err)
		}
		if err := buildstats.SyncSpans(ctx, env); err != nil {
			log.Fatalf("SyncSpans: %v", err)
		}
	} else {
		log.Fatalf("the buildstats command doesn't yet do anything except the --sync mode")
	}

}
