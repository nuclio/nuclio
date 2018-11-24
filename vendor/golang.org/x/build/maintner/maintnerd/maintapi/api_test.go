// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package maintapi

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/build/maintner"
	"golang.org/x/build/maintner/godata"
	"golang.org/x/build/maintner/maintnerd/apipb"
)

func TestGetRef(t *testing.T) {
	c := getGoData(t)
	s := apiService{c}
	req := &apipb.GetRefRequest{
		GerritServer:  "go.googlesource.com",
		GerritProject: "go",
		Ref:           "refs/heads/master",
	}
	res, err := s.GetRef(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Value) != 40 {
		t.Errorf("go master ref = %q; want length 40 string", res.Value)
	}

	// Bogus ref
	req.Ref = "NOT EXIST REF"
	res, err = s.GetRef(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Value) != 0 {
		t.Errorf("go bogus ref = %q; want empty string", res.Value)
	}

	// Bogus project
	req.GerritProject = "NOT EXIST PROJ"
	_, err = s.GetRef(context.Background(), req)
	if got, want := fmt.Sprint(err), "unknown gerrit project"; got != want {
		t.Errorf("error for bogus project = %q; want %q", got, want)
	}
}

var hitGerrit = flag.Bool("hit_gerrit", false, "query production Gerrit in TestFindTryWork")

func TestFindTryWork(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	if !*hitGerrit {
		t.Skip("skipping without flag --hit_gerrit")
	}
	c := getGoData(t)
	s := apiService{c}
	req := &apipb.GoFindTryWorkRequest{}
	t0 := time.Now()
	res, err := s.GoFindTryWork(context.Background(), req)
	d0 := time.Since(t0)
	if err != nil {
		t.Fatal(err)
	}

	// Just for interactive debugging. This is using live data.
	// The stable tests are in TestTryWorkItem and TestTryBotStatus.
	t.Logf("Current: %v", res)

	t1 := time.Now()
	res, err = s.GoFindTryWork(context.Background(), req)
	d1 := time.Since(t1)
	t.Logf("Latency: %v, then %v", d0, d1)
	t.Logf("Cached: %v, %v", res, err)
}

func TestTryBotStatus(t *testing.T) {
	c := getGoData(t)
	tests := []struct {
		proj      string
		clnum     int32
		msgCutoff int
		wantTry   bool
		wantDone  bool
	}{
		{"go", 51430, 1, true, false},
		{"go", 51430, 2, true, false},
		{"go", 51430, 3, true, true},

		{"build", 48968, 5, true, false},  // adding trybot (coordinator ignores for "build" repo)
		{"build", 48968, 6, false, false}, // removing it
	}
	for _, tt := range tests {
		cl := c.Gerrit().Project("go.googlesource.com", tt.proj).CL(tt.clnum)
		if cl == nil {
			t.Errorf("CL %d in %s not found", tt.clnum, tt.proj)
			continue
		}
		old := *cl // save before mutations
		cl.Version = cl.Messages[tt.msgCutoff-1].Version
		cl.Messages = cl.Messages[:tt.msgCutoff]
		gotTry, gotDone := tryBotStatus(cl, false /* not staging */)
		if gotTry != tt.wantTry || gotDone != tt.wantDone {
			t.Errorf("tryBotStatus(%q, %d) after %d messages = try/done %v, %v; want %v, %v",
				tt.proj, tt.clnum, tt.msgCutoff, gotTry, gotDone, tt.wantTry, tt.wantDone)
			for _, msg := range cl.Messages {
				t.Logf("  msg ver=%d, text=%q", msg.Version, msg.Message)
			}
		}
		*cl = old // restore
	}

}

func TestTryWorkItem(t *testing.T) {
	c := getGoData(t)
	tests := []struct {
		proj  string
		clnum int32
		want  string
	}{
		// Same Change-Id, different branch:
		{"go", 51430, `project:"go" branch:"master" change_id:"I0bcae339624e7d61037d9ea0885b7bd07491bbb6" commit:"45a4609c0ae214e448612e0bc0846e2f2682f1b2" `},
		{"go", 51450, `project:"go" branch:"release-branch.go1.9" change_id:"I0bcae339624e7d61037d9ea0885b7bd07491bbb6" commit:"7320506bc58d3a55eff2c67b2ec65cfa94f7b0a7" `},
		// Different project:
		{"build", 51432, `project:"build" branch:"master" change_id:"I1f71836da7008e58d3e76e2cc3170e96cd57ddf6" commit:"9251bc9950baff61d95da0761e2e4bfab61ed210" `},
	}
	for _, tt := range tests {
		cl := c.Gerrit().Project("go.googlesource.com", tt.proj).CL(tt.clnum)
		if cl == nil {
			t.Errorf("CL %d in %s not found", tt.clnum, tt.proj)
			continue
		}
		got := fmt.Sprint(tryWorkItem(cl))
		if got != tt.want {
			t.Errorf("tryWorkItem(%q, %v) = %#q; want %#q", tt.proj, tt.clnum, got, tt.want)
		}
	}
}

var (
	corpusMu    sync.Mutex
	corpusCache *maintner.Corpus
)

func getGoData(tb testing.TB) *maintner.Corpus {
	corpusMu.Lock()
	defer corpusMu.Unlock()
	if corpusCache != nil {
		return corpusCache
	}
	var err error
	corpusCache, err = godata.Get(context.Background())
	if err != nil {
		tb.Fatalf("getting corpus: %v", err)
	}
	return corpusCache
}

func TestSupportedGoReleases(t *testing.T) {
	tests := []struct {
		goProj nonChangeRefLister
		want   []*apipb.GoRelease
	}{
		// A sample of real data from maintner.
		{
			goProj: gerritProject{
				refs: []refHash{
					{"HEAD", gitHash("5168fcf63f5001b38f9ac64ce5c5e3c2d397363d")},
					{"refs/heads/dev.boringcrypto", gitHash("13bf5b80e8d8841a2a3c9b0d5dec65a0c8636253")},
					{"refs/heads/dev.boringcrypto.go1.10", gitHash("2e2a04a605b6c3fc6e733810bdcd0200d8ed25a8")},
					{"refs/heads/dev.boringcrypto.go1.11", gitHash("685dc1638240af70c86a146b0ddb86d51d64f269")},
					{"refs/heads/dev.typealias", gitHash("8a5ef1501dee0715093e87cdc1c9b6becb81c882")},
					{"refs/heads/master", gitHash("5168fcf63f5001b38f9ac64ce5c5e3c2d397363d")},
					{"refs/heads/release-branch.go1", gitHash("08b97d4061dd75ceec1d44e4335183cd791c9306")},
					{"refs/heads/release-branch.go1.1", gitHash("1d6d8fca241bb611af51e265c1b5a2e9ae904702")},
					{"refs/heads/release-branch.go1.10", gitHash("e97b7d68f107ff60152f5bd5701e0286f221ee93")},
					{"refs/heads/release-branch.go1.11", gitHash("97781d2ed116d2cd9cb870d0b84fc0ec598c9abc")},
					{"refs/heads/release-branch.go1.2", gitHash("43d00b0942c1c6f43993ac71e1eea48e62e22b8d")},
					{"refs/heads/release-branch.r59", gitHash("5d9765785dff74784bbdad43f7847b6825509032")},
					{"refs/heads/release-branch.r60", gitHash("394b383a1ee0ac3fec5e453a7dbe590d3ce6d6b0")},
					{"refs/notes/review", gitHash("c46ab9dacb2ac618d86f1c1f719bc2de46010e86")},
					{"refs/tags/1.10beta1.mailed", gitHash("2df74db61620771e4f878c9e1db7aeecc00808ba")},
					{"refs/tags/andybons/blog.mailed", gitHash("707a89416af909a3af6c26df93995bc17bf9ce81")},
					{"refs/tags/go1", gitHash("6174b5e21e73714c63061e66efdbe180e1c5491d")},
					{"refs/tags/go1.0.1", gitHash("2fffba7fe19690e038314d17a117d6b87979c89f")},
					{"refs/tags/go1.0.2", gitHash("cb6c6570b73a1c4d19cad94570ed277f7dae55ac")},
					{"refs/tags/go1.0.3", gitHash("30be9b4313622c2077539e68826194cb1028c691")},
					{"refs/tags/go1.1", gitHash("205f850ceacfc39d1e9d76a9569416284594ce8c")},
					{"refs/tags/go1.10", gitHash("bf86aec25972f3a100c3aa58a6abcbcc35bdea49")},
					{"refs/tags/go1.10.1", gitHash("ac7c0ee26dda18076d5f6c151d8f920b43340ae3")},
					{"refs/tags/go1.10.2", gitHash("71bdbf431b79dff61944f22c25c7e085ccfc25d5")},
					{"refs/tags/go1.10.3", gitHash("fe8a0d12b14108cbe2408b417afcaab722b0727c")},
					{"refs/tags/go1.10.4", gitHash("2191fce26a7fd1cd5b4975e7bd44ab44b1d9dd78")},
					{"refs/tags/go1.10beta1", gitHash("9ce6b5c2ed5d3d5251b9a6a0c548d5fb2c8567e8")},
					{"refs/tags/go1.10beta2", gitHash("594668a5a96267a46282ce3007a584ec07adf705")},
					{"refs/tags/go1.10rc1", gitHash("5348aed83e39bd1d450d92d7f627e994c2db6ebf")},
					{"refs/tags/go1.10rc2", gitHash("20e228f2fdb44350c858de941dff4aea9f3127b8")},
					{"refs/tags/go1.11", gitHash("41e62b8c49d21659b48a95216e3062032285250f")},
					{"refs/tags/go1.11.1", gitHash("26957168c4c0cdcc7ca4f0b19d0eb19474d224ac")},
					{"refs/tags/go1.11beta1", gitHash("a12c1f26e4cc602dae62ec065a237172a5b8f926")},
					{"refs/tags/go1.11beta2", gitHash("c814ac44c0571f844718f07aa52afa47e37fb1ed")},
					{"refs/tags/go1.11beta3", gitHash("1b870077c896379c066b41657d3c9062097a6943")},
					{"refs/tags/go1.11rc1", gitHash("807e7f2420c683384dc9c6db498808ba1b7aab17")},
					{"refs/tags/go1.11rc2", gitHash("02c0c32960f65d0b9c66ec840c612f5f9623dc51")},
					{"refs/tags/go1.9.7", gitHash("7df09b4a03f9e53334672674ba7983d5e7128646")},
					{"refs/tags/go1.9beta1", gitHash("952ecbe0a27aadd184ca3e2c342beb464d6b1653")},
					{"refs/tags/go1.9beta2", gitHash("eab99a8d548f8ba864647ab171a44f0a5376a6b3")},
					{"refs/tags/go1.9rc1", gitHash("65c6c88a9442b91d8b2fd0230337b1fda4bb6cdf")},
					{"refs/tags/go1.9rc2", gitHash("048c9cfaacb6fe7ac342b0acd8ca8322b6c49508")},
					{"refs/tags/release.r59", gitHash("5d9765785dff74784bbdad43f7847b6825509032")},
					{"refs/tags/release.r60", gitHash("5464bfebe723752dfc09a6dd6b361b8e79db5995")},
					{"refs/tags/release.r60.1", gitHash("4af7136fcf874e212d66c72178a68db969918b25")},
					{"refs/tags/weekly", gitHash("3895b5051df256b442d0b0af50debfffd8d75164")},
					{"refs/tags/weekly.2009-11-10", gitHash("78c47c36b2984058c1bec0bd72e0b127b24fcd44")},
					{"refs/tags/weekly.2009-11-10.1", gitHash("c57054f7b49539ca4ed6533267c1c20c39aaaaa5")},
				},
			},
			want: []*apipb.GoRelease{
				{
					Major: 1, Minor: 11, Patch: 1,
					TagName:      "go1.11.1",
					TagCommit:    "26957168c4c0cdcc7ca4f0b19d0eb19474d224ac",
					BranchName:   "release-branch.go1.11",
					BranchCommit: "97781d2ed116d2cd9cb870d0b84fc0ec598c9abc",
				},
				{
					Major: 1, Minor: 10, Patch: 4,
					TagName:      "go1.10.4",
					TagCommit:    "2191fce26a7fd1cd5b4975e7bd44ab44b1d9dd78",
					BranchName:   "release-branch.go1.10",
					BranchCommit: "e97b7d68f107ff60152f5bd5701e0286f221ee93",
				},
			},
		},

		// Detect and handle a new major version.
		{
			goProj: gerritProject{
				refs: []refHash{
					{"refs/tags/go1.5", gitHash("9b82ca331d1fa30e3428e7914ba780ae7f75a702")},
					{"refs/tags/go1.42.1", gitHash("23982c09ae5ac811d1dd0099e1626596ade61000")},
					{"refs/tags/go1", gitHash("5c503fde0aa534d3259533802052f936c95fa782")},
					{"refs/tags/go2", gitHash("43126518de2eb0dadc0917a593f08637318986bf")},
					{"refs/tags/go1.11.111", gitHash("c59f000d9bb66592ff84a942014afd1a7be4c953")}, // The onesiest release ever!
					{"refs/heads/release-branch.go1", gitHash("b0f2d801c19fc8798ecf67e50364a44dba606fcd")},
					{"refs/heads/release-branch.go1.5", gitHash("a6ae58c93408bcc17758d397eed0ace973de8481")},
					{"refs/heads/release-branch.go1.11", gitHash("f4f148ef7962271ff8ffcebf13400ded535e9957")},
					{"refs/heads/release-branch.go1.42", gitHash("362986e7a4b5edc911ed55324c37106c40abe3fb")},
					{"refs/heads/release-branch.go2", gitHash("cfbe0f14bcbf1e773f8dd9a968c80cf0b9238c59")},
					{"refs/heads/release-branch.go1.2", gitHash("6523e1eb33ef792df04e08462ed332b95311261e")},

					// It doesn't count as a release if there's no corresponding release-branch.go1.43 release branch.
					{"refs/tags/go1.43", gitHash("3aa7f7065ecf717b1dd6512bb7a9f40625fc8cb5")},
				},
			},
			want: []*apipb.GoRelease{
				{
					Major: 2, Minor: 0, Patch: 0,
					TagName:      "go2",
					TagCommit:    "43126518de2eb0dadc0917a593f08637318986bf",
					BranchName:   "release-branch.go2",
					BranchCommit: "cfbe0f14bcbf1e773f8dd9a968c80cf0b9238c59",
				},
				{
					Major: 1, Minor: 42, Patch: 1,
					TagName:      "go1.42.1",
					TagCommit:    "23982c09ae5ac811d1dd0099e1626596ade61000",
					BranchName:   "release-branch.go1.42",
					BranchCommit: "362986e7a4b5edc911ed55324c37106c40abe3fb",
				},
			},
		},
	}
	for i, tt := range tests {
		got, err := supportedGoReleases(tt.goProj)
		if err != nil {
			t.Fatalf("%d: supportedGoReleases: %v", i, err)
		}
		if diff := cmp.Diff(got, tt.want); diff != "" {
			t.Errorf("%d: supportedGoReleases: (-got +want)\n%s", i, diff)
		}
	}
}

type gerritProject struct {
	refs []refHash
}

func (gp gerritProject) ForeachNonChangeRef(fn func(ref string, hash maintner.GitHash) error) error {
	for _, r := range gp.refs {
		err := fn(r.Ref, r.Hash)
		if err != nil {
			return err
		}
	}
	return nil
}

type refHash struct {
	Ref  string
	Hash maintner.GitHash
}

func gitHash(hexa string) maintner.GitHash {
	if len(hexa) != 40 {
		panic(fmt.Errorf("bogus git hash %q", hexa))
	}
	binary, err := hex.DecodeString(hexa)
	if err != nil {
		panic(fmt.Errorf("bogus git hash %q: %v", hexa, err))
	}
	return maintner.GitHash(binary)
}
