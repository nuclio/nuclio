// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/github"
	"golang.org/x/build/maintner"
)

func newPullRequest(title, body string) *github.PullRequest {
	return &github.PullRequest{
		Title:  github.String(title),
		Body:   github.String(body),
		Number: github.Int(42),
		Head:   &github.PullRequestBranch{SHA: github.String("deadbeef")},
		Base: &github.PullRequestBranch{
			Repo: &github.Repository{
				Owner: &github.User{
					Login: github.String("golang"),
				},
				Name: github.String("go"),
			},
		},
	}
}

func newMaintnerCL() *maintner.GerritCL {
	return &maintner.GerritCL{
		Commit: &maintner.GitCommit{
			Msg: `cmd/gerritbot: previous commit messsage

Hello there

Change-Id: If751ce3ffa3a4d5e00a3138211383d12cb6b23fc
`,
		},
	}
}

func TestCommitMessage(t *testing.T) {
	testCases := []struct {
		desc     string
		pr       *github.PullRequest
		cl       *maintner.GerritCL
		expected string
	}{
		{
			"simple",
			newPullRequest("cmd/gerritbot: title of change", "Body text"),
			nil,
			`cmd/gerritbot: title of change

Body text

Change-Id: I8ef4fc7aa2b40846583a9cbf175d75d023b5564e
GitHub-Last-Rev: deadbeef
GitHub-Pull-Request: golang/go#42
`,
		},
		{
			"change with Change-Id",
			newPullRequest("cmd/gerritbot: change with change ID", "Body text"),
			newMaintnerCL(),
			`cmd/gerritbot: change with change ID

Body text

Change-Id: If751ce3ffa3a4d5e00a3138211383d12cb6b23fc
GitHub-Last-Rev: deadbeef
GitHub-Pull-Request: golang/go#42
`,
		},
		{
			"Change-Id in body text",
			newPullRequest("cmd/gerritbot: change with change ID in body text",
				"Change-Id: I30e0a6ec666a06eae3e8444490d96fabcab3333e"),
			nil,
			`cmd/gerritbot: change with change ID in body text

Change-Id: I30e0a6ec666a06eae3e8444490d96fabcab3333e
GitHub-Last-Rev: deadbeef
GitHub-Pull-Request: golang/go#42
`,
		},
		{
			"Change-Id in body text with an existing CL",
			newPullRequest("cmd/gerritbot: change with change ID in body text and an existing CL",
				"Change-Id: I30e0a6ec666a06eae3e8444490d96fabcab3333e"),
			newMaintnerCL(),
			`cmd/gerritbot: change with change ID in body text and an existing CL

Change-Id: I30e0a6ec666a06eae3e8444490d96fabcab3333e

Change-Id: If751ce3ffa3a4d5e00a3138211383d12cb6b23fc
GitHub-Last-Rev: deadbeef
GitHub-Pull-Request: golang/go#42
`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			msg, err := commitMessage(tc.pr, tc.cl)
			if err != nil {
				t.Fatalf("got unexpected error from commitMessage: %v", err)
			}
			if diff := cmp.Diff(msg, tc.expected); diff != "" {
				t.Errorf("got unexpected commit message (-got +want)\n%s", diff)
			}
		})
	}
}

// Test that gerritChangeRE matches the URL to the Change within
// the git output from Gerrit after successfully creating a new CL.
// Whenever Gerrit changes the Change URL format in its output,
// we need to update gerritChangeRE and this test accordingly.
//
// See https://golang.org/issue/27561.
func TestFindChangeURL(t *testing.T) {
	// Sample git output from Gerrit, extracted from production logs on 2018/09/07.
	const out = "remote: \rremote: Processing changes: new: 1 (\\)\rremote: Processing changes: new: 1 (|)\rremote: Processing changes: refs: 1, new: 1 (|)\rremote: Processing changes: refs: 1, new: 1 (|)        \rremote: Processing changes: refs: 1, new: 1, done            \nremote: \nremote: SUCCESS        \nremote: \nremote: New Changes:        \nremote:   https://go-review.googlesource.com/c/dl/+/134117 remove blank line from codereview.cfg        \nTo https://go.googlesource.com/dl\n * [new branch]      HEAD -> refs/for/master"
	got := gerritChangeRE.FindString(out)
	want := "https://go-review.googlesource.com/c/dl/+/134117"
	if got != want {
		t.Errorf("could not find change URL in command output: %q\n\ngot %q, want %q", out, got, want)
	}
}
