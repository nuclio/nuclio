// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/go-github/github"
	"golang.org/x/build/devapp/owners"
	"golang.org/x/build/maintner"
)

func TestLabelCommandsFromComments(t *testing.T) {
	created := time.Now()
	testCases := []struct {
		desc string
		body string
		cmds []labelCommand
	}{
		{
			"basic add/remove",
			"We should fix this issue, but we need help\n\n@gopherbot please add help wanted, needsfix and remove needsinvestigation",
			[]labelCommand{
				{action: "add", label: "help wanted", created: created},
				{action: "add", label: "needsfix", created: created},
				{action: "remove", label: "needsinvestigation", created: created},
			},
		},
		{
			"no please",
			"@gopherbot add NeedsFix",
			[]labelCommand{
				{action: "add", label: "needsfix", created: created},
			},
		},
		{
			"with comma",
			"@gopherbot, NeedsFix",
			[]labelCommand{
				{action: "add", label: "needsfix", created: created},
			},
		},
		{
			"with semicolons",
			"@gopherbot NeedsFix;help wanted; remove needsinvestigation",
			[]labelCommand{
				{action: "add", label: "needsfix", created: created},
				{action: "add", label: "help wanted", created: created},
				{action: "remove", label: "needsinvestigation", created: created},
			},
		},
		{
			"case insensitive",
			"@gopherbot please add HelP WanteD",
			[]labelCommand{
				{action: "add", label: "help wanted", created: created},
			},
		},
		{
			"fun input",
			"@gopherbot please add help wanted,;needsfix;",
			[]labelCommand{
				{action: "add", label: "help wanted", created: created},
				{action: "add", label: "needsfix", created: created},
			},
		},
		{
			"with hyphen",
			"@gopherbot please add label OS-macOS",
			[]labelCommand{
				{action: "add", label: "os-macos", created: created},
			},
		},
		{
			"unlabel keyword",
			"@gopherbot please unlabel needsinvestigation, NeedsDecision",
			[]labelCommand{
				{action: "remove", label: "needsinvestigation", created: created},
				{action: "remove", label: "needsdecision", created: created},
			},
		},
		{
			"with label[s] keyword",
			"@gopherbot please add label help wanted and remove labels needsinvestigation, NeedsDecision",
			[]labelCommand{
				{action: "add", label: "help wanted", created: created},
				{action: "remove", label: "needsinvestigation", created: created},
				{action: "remove", label: "needsdecision", created: created},
			},
		},
		{
			"no label commands",
			"The cake was a lie",
			nil,
		},
	}
	for _, tc := range testCases {
		cmds := labelCommandsFromBody(tc.body, created)
		if diff := cmp.Diff(cmds, tc.cmds, cmp.AllowUnexported(labelCommand{})); diff != "" {
			t.Errorf("%s: commands differ: (-got +want)\n%s", tc.desc, diff)
		}
	}
}

func TestLabelMutations(t *testing.T) {
	testCases := []struct {
		desc   string
		cmds   []labelCommand
		add    []string
		remove []string
	}{
		{
			"basic",
			[]labelCommand{
				{action: "add", label: "foo"},
				{action: "remove", label: "baz"},
			},
			[]string{"foo"},
			[]string{"baz"},
		},
		{
			"add/remove of same label",
			[]labelCommand{
				{action: "add", label: "foo"},
				{action: "remove", label: "foo"},
				{action: "remove", label: "bar"},
				{action: "add", label: "bar"},
			},
			nil,
			nil,
		},
		{
			"deduplication of labels",
			[]labelCommand{
				{action: "add", label: "foo"},
				{action: "add", label: "foo"},
				{action: "remove", label: "bar"},
				{action: "remove", label: "bar"},
			},
			[]string{"foo"},
			[]string{"bar"},
		},
		{
			"forbidden actions",
			[]labelCommand{
				{action: "add", label: "Proposal-Accepted"},
				{action: "add", label: "CherryPickApproved"},
				{action: "add", label: "cla: yes"},
				{action: "remove", label: "Security"},
			},
			nil,
			nil,
		},
		{
			"can add Security",
			[]labelCommand{
				{action: "add", label: "Security"},
			},
			[]string{"Security"},
			nil,
		},
	}
	for _, tc := range testCases {
		add, remove := mutationsFromCommands(tc.cmds)
		if diff := cmp.Diff(add, tc.add); diff != "" {
			t.Errorf("%s: label additions differ: (-got, +want)\n%s", tc.desc, diff)
		}
		if diff := cmp.Diff(remove, tc.remove); diff != "" {
			t.Errorf("%s: label removals differ: (-got, +want)\n%s", tc.desc, diff)
		}
	}
}

func TestAddLabels(t *testing.T) {
	oldFunc := addLabelsToIssue
	defer func() { addLabelsToIssue = oldFunc }()

	var added []string
	addLabelsToIssue = func(_ context.Context, _ *github.IssuesService, _ int, labels []string) error {
		added = labels
		return nil
	}

	testCases := []struct {
		desc   string
		gi     *maintner.GitHubIssue
		labels []string
		added  []string
	}{
		{
			"basic add",
			&maintner.GitHubIssue{},
			[]string{"foo"},
			[]string{"foo"},
		},
		{
			"some labels already present in maintner",
			&maintner.GitHubIssue{
				Labels: map[int64]*maintner.GitHubLabel{
					0: {Name: "NeedsDecision"},
				},
			},
			[]string{"foo", "NeedsDecision"},
			[]string{"foo"},
		},
		{
			"all labels already present in maintner",
			&maintner.GitHubIssue{
				Labels: map[int64]*maintner.GitHubLabel{
					0: {Name: "NeedsDecision"},
				},
			},
			[]string{"NeedsDecision"},
			nil,
		},
	}

	b := &gopherbot{ghc: github.NewClient(http.DefaultClient)}
	for _, tc := range testCases {
		// Clear any previous state from stubbed addLabelsToIssue function above
		// since some test cases may skip calls to it.
		added = nil

		if err := b.addLabels(nil, tc.gi, tc.labels); err != nil {
			t.Errorf("%s: b.addLabels got unexpected error: %v", tc.desc, err)
			continue
		}
		if diff := cmp.Diff(added, tc.added); diff != "" {
			t.Errorf("%s: labels added differ: (-got, +want)\n%s", tc.desc, diff)
		}
	}
}

func TestRemoveLabels(t *testing.T) {
	oldLabelsForIssue := labelsForIssue
	defer func() { labelsForIssue = oldLabelsForIssue }()

	labelsForIssue = func(_ context.Context, _ *github.IssuesService, _ int) ([]string, error) {
		return []string{"help wanted", "NeedsFix"}, nil
	}

	oldRemoveLabelFromIssue := removeLabelFromIssue
	defer func() { removeLabelFromIssue = oldRemoveLabelFromIssue }()

	var removed []string
	removeLabelFromIssue = func(_ context.Context, _ *github.IssuesService, _ int, label string) error {
		removed = append(removed, label)
		return nil
	}

	testCases := []struct {
		desc     string
		gi       *maintner.GitHubIssue
		toRemove []string
		removed  []string
	}{
		{
			"basic remove",
			&maintner.GitHubIssue{
				Labels: map[int64]*maintner.GitHubLabel{
					0: {Name: "NeedsFix"},
				},
			},
			[]string{"NeedsFix"},
			[]string{"NeedsFix"},
		},
		{
			"label not present in maintner",
			&maintner.GitHubIssue{},
			[]string{"NeedsFix"},
			nil,
		},
		{
			"label not present in GitHub",
			&maintner.GitHubIssue{
				Labels: map[int64]*maintner.GitHubLabel{
					0: {Name: "foo"},
				},
			},
			[]string{"foo"},
			nil,
		},
	}

	b := &gopherbot{ghc: github.NewClient(http.DefaultClient)}
	for _, tc := range testCases {
		// Clear any previous state from stubbed removeLabelFromIssue function above
		// since some test cases may skip calls to it.
		removed = nil

		if err := b.removeLabels(nil, tc.gi, tc.toRemove); err != nil {
			t.Errorf("%s: b.addLabels got unexpected error: %v", tc.desc, err)
			continue
		}
		if diff := cmp.Diff(removed, tc.removed); diff != "" {
			t.Errorf("%s: labels removed differ: (-got, +want)\n%s", tc.desc, diff)
		}
	}
}

func TestHumanReviewersInMetas(t *testing.T) {
	testCases := []struct {
		commitMsg string
		hasHuman  bool
	}{
		{`Patch-set: 6
Reviewer: Andrew Bonventre <22285@62eb7196-b449-3ce5-99f1-c037f21e1705>
`,
			true,
		},
		{`Patch-set: 6
CC: Andrew Bonventre <22285@62eb7196-b449-3ce5-99f1-c037f21e1705>
`,
			true,
		},
		{`Patch-set: 6
Reviewer: Gobot Gobot <5976@62eb7196-b449-3ce5-99f1-c037f21e1705>
`,
			false,
		},
		{`Patch-set: 6
Reviewer: Gobot Gobot <5976@62eb7196-b449-3ce5-99f1-c037f21e1705>
CC: Andrew Bonventre <22285@62eb7196-b449-3ce5-99f1-c037f21e1705>
`,
			true,
		},
		{`Patch-set: 6
Reviewer: Gobot Gobot <5976@62eb7196-b449-3ce5-99f1-c037f21e1705>
Reviewer: Andrew Bonventre <22285@62eb7196-b449-3ce5-99f1-c037f21e1705>
`,
			true,
		},
	}

	for _, tc := range testCases {
		metas := []*maintner.GerritMeta{
			{Commit: &maintner.GitCommit{Msg: tc.commitMsg}},
		}
		if got, want := humanReviewersInMetas(metas), tc.hasHuman; got != want {
			t.Errorf("Unexpected result for meta commit message: got %v; want %v for\n%s", got, want, tc.commitMsg)
		}
	}
}

func TestMergeOwnersEntries(t *testing.T) {
	var (
		andybons = owners.Owner{GitHubUsername: "andybons", GerritEmail: "andybons@golang.org"}
		bradfitz = owners.Owner{GitHubUsername: "bradfitz", GerritEmail: "bradfitz@golang.org"}
		filippo  = owners.Owner{GitHubUsername: "filippo", GerritEmail: "filippo@golang.org"}
	)
	testCases := []struct {
		desc        string
		entries     []*owners.Entry
		authorEmail string
		result      *owners.Entry
	}{
		{
			"no entries",
			nil,
			"",
			&owners.Entry{},
		},
		{
			"primary merge",
			[]*owners.Entry{
				{Primary: []owners.Owner{andybons}},
				{Primary: []owners.Owner{bradfitz}},
			},
			"",
			&owners.Entry{
				Primary: []owners.Owner{andybons, bradfitz},
			},
		},
		{
			"secondary merge",
			[]*owners.Entry{
				{Secondary: []owners.Owner{andybons}},
				{Secondary: []owners.Owner{filippo}},
			},
			"",
			&owners.Entry{
				Secondary: []owners.Owner{andybons, filippo},
			},
		},
		{
			"promote from secondary to primary",
			[]*owners.Entry{
				{Primary: []owners.Owner{andybons, filippo}},
				{Secondary: []owners.Owner{filippo}},
			},
			"",
			&owners.Entry{
				Primary: []owners.Owner{andybons, filippo},
			},
		},
		{
			"primary filter",
			[]*owners.Entry{
				{Primary: []owners.Owner{filippo, andybons}},
			},
			filippo.GerritEmail,
			&owners.Entry{
				Primary: []owners.Owner{andybons},
			},
		},
		{
			"secondary filter",
			[]*owners.Entry{
				{Secondary: []owners.Owner{filippo, andybons}},
			},
			filippo.GerritEmail,
			&owners.Entry{
				Secondary: []owners.Owner{andybons},
			},
		},
	}
	cmpFn := func(a, b owners.Owner) bool {
		return a.GitHubUsername < b.GitHubUsername
	}
	for _, tc := range testCases {
		got := mergeOwnersEntries(tc.entries, tc.authorEmail)
		if diff := cmp.Diff(got, tc.result, cmpopts.SortSlices(cmpFn)); diff != "" {
			t.Errorf("%s: final entry results differ: (-got, +want)\n%s", tc.desc, diff)
		}
	}
}
