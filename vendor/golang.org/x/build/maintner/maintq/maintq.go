// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The maintq command queries a maintnerd gRPC server.
// This tool is mostly for debugging.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"

	"golang.org/x/build/maintner/maintnerd/apipb"
	"golang.org/x/net/http2"
	"grpc.go4.org"
)

var (
	server = flag.String("server", "maintner.golang.org", "maintnerd server")
)

var (
	mc  apipb.MaintnerServiceClient
	ctx = context.Background()
)

func main() {
	flag.Parse()

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			NextProtos:         []string{"h2"},
			InsecureSkipVerify: strings.HasPrefix(*server, "localhost:"),
		},
	}
	hc := &http.Client{Transport: tr}
	http2.ConfigureTransport(tr)

	cc, err := grpc.NewClient(hc, "https://"+*server)
	if err != nil {
		log.Fatal(err)
	}
	mc = apipb.NewMaintnerServiceClient(cc)

	cmdFunc := map[string]func(args []string) error{
		"has-ancestor":  callHasAncestor,
		"get-ref":       callGetRef,
		"try-work":      callTryWork,
		"list-releases": callListReleases,
	}
	log.SetFlags(0)
	if flag.NArg() == 0 || cmdFunc[flag.Arg(0)] == nil {
		var cmds []string
		for cmd := range cmdFunc {
			cmds = append(cmds, cmd)
		}
		sort.Strings(cmds)
		log.Fatalf(`Usage: maintq %v ...`, cmds)
	}
	if err := cmdFunc[flag.Arg(0)](flag.Args()[1:]); err != nil {
		log.Fatal(err)
	}
}

func callHasAncestor(args []string) error {
	if len(args) != 2 {
		return errors.New("Usage: maintq has-ancestor <commit> <ancestor>")
	}
	res, err := mc.HasAncestor(ctx, &apipb.HasAncestorRequest{
		Commit:   args[0],
		Ancestor: args[1],
	})
	if err != nil {
		return err
	}
	fmt.Println(res)
	return nil
}

func callGetRef(args []string) error {
	if len(args) != 2 {
		return errors.New("Usage: maintq get-ref <project> <ref>")
	}
	res, err := mc.GetRef(ctx, &apipb.GetRefRequest{
		GerritServer:  "go.googlesource.com",
		GerritProject: args[0],
		Ref:           args[1],
	})
	if err != nil {
		return err
	}
	if res.Value == "" {
		return errors.New("ref not found")
	}
	fmt.Println(res.Value)
	return nil
}

func callTryWork(args []string) error {
	staging := len(args) == 1 && args[0] == "staging"
	if !staging && len(args) > 0 {
		return errors.New(`Usage: maintq try-work ["staging"]  # prod is default`)
	}
	res, err := mc.GoFindTryWork(ctx, &apipb.GoFindTryWorkRequest{ForStaging: staging})
	if err != nil {
		return err
	}
	fmt.Println(res)
	return nil
}

func callListReleases(args []string) error {
	if len(args) != 0 {
		return errors.New("Usage: maintq list-releases")
	}
	res, err := mc.ListGoReleases(ctx, &apipb.ListGoReleasesRequest{})
	if err != nil {
		return err
	}
	for _, r := range res.Releases {
		fmt.Println(r)
	}
	return nil
}
