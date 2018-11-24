// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package maintner

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"

	"golang.org/x/build/maintner/maintpb"
)

func TestParseGithubEvents(t *testing.T) {
	tests := []struct {
		name string                    // test
		j    string                    // JSON from Github API
		e    *GitHubIssueEvent         // in-memory
		p    *maintpb.GithubIssueEvent // on disk
	}{
		{
			name: "labeled",
			j: `{
    "id": 998144526,
    "url": "https://api.github.com/repos/bradfitz/go-issue-mirror/issues/events/998144526",
    "actor": {
      "login": "bradfitz",
      "id": 2621
    },
    "event": "labeled",
    "commit_id": null,
    "commit_url": null,
    "created_at": "2017-03-13T22:39:28Z",
    "label": {
      "name": "enhancement",
      "color": "84b6eb"
    }
  }
`,
			e: &GitHubIssueEvent{
				ID:      998144526,
				Type:    "labeled",
				Created: t3339("2017-03-13T22:39:28Z"),
				Actor: &GitHubUser{
					ID:    2621,
					Login: "bradfitz",
				},
				Label: "enhancement",
			},
			p: &maintpb.GithubIssueEvent{
				Id:        998144526,
				EventType: "labeled",
				ActorId:   2621,
				Created:   p3339("2017-03-13T22:39:28Z"),
				Label:     &maintpb.GithubLabel{Name: "enhancement"},
			},
		},

		{
			name: "unlabeled",
			j: `{
    "id": 998144526,
    "url": "https://api.github.com/repos/bradfitz/go-issue-mirror/issues/events/998144526",
    "actor": {
      "login": "bradfitz",
      "id": 2621
    },
    "event": "unlabeled",
    "commit_id": null,
    "commit_url": null,
    "created_at": "2017-03-13T22:39:28Z",
    "label": {
      "name": "enhancement",
      "color": "84b6eb"
    }
  }
`,
			e: &GitHubIssueEvent{
				ID:      998144526,
				Type:    "unlabeled",
				Created: t3339("2017-03-13T22:39:28Z"),
				Actor: &GitHubUser{
					ID:    2621,
					Login: "bradfitz",
				},
				Label: "enhancement",
			},
			p: &maintpb.GithubIssueEvent{
				Id:        998144526,
				EventType: "unlabeled",
				ActorId:   2621,
				Created:   p3339("2017-03-13T22:39:28Z"),
				Label:     &maintpb.GithubLabel{Name: "enhancement"},
			},
		},

		{
			name: "milestoned",
			j: `{
    "id": 998144529,
    "url": "https://api.github.com/repos/bradfitz/go-issue-mirror/issues/events/998144529",
    "actor": {
      "login": "bradfitz",
      "id": 2621
    },
    "event": "milestoned",
    "commit_id": null,
    "commit_url": null,
    "created_at": "2017-03-13T22:39:28Z",
    "milestone": {
      "title": "World Domination"
    }}`,
			e: &GitHubIssueEvent{
				ID:      998144529,
				Type:    "milestoned",
				Created: t3339("2017-03-13T22:39:28Z"),
				Actor: &GitHubUser{
					ID:    2621,
					Login: "bradfitz",
				},
				Milestone: "World Domination",
			},
			p: &maintpb.GithubIssueEvent{
				Id:        998144529,
				EventType: "milestoned",
				ActorId:   2621,
				Created:   p3339("2017-03-13T22:39:28Z"),
				Milestone: &maintpb.GithubMilestone{Title: "World Domination"},
			},
		},

		{
			name: "demilestoned",
			j: `{
    "id": 998144529,
    "url": "https://api.github.com/repos/bradfitz/go-issue-mirror/issues/events/998144529",
    "actor": {
      "login": "bradfitz",
      "id": 2621
    },
    "event": "demilestoned",
    "commit_id": null,
    "commit_url": null,
    "created_at": "2017-03-13T22:39:28Z",
    "milestone": {
      "title": "World Domination"
    }}`,
			e: &GitHubIssueEvent{
				ID:      998144529,
				Type:    "demilestoned",
				Created: t3339("2017-03-13T22:39:28Z"),
				Actor: &GitHubUser{
					ID:    2621,
					Login: "bradfitz",
				},
				Milestone: "World Domination",
			},
			p: &maintpb.GithubIssueEvent{
				Id:        998144529,
				EventType: "demilestoned",
				ActorId:   2621,
				Created:   p3339("2017-03-13T22:39:28Z"),
				Milestone: &maintpb.GithubMilestone{Title: "World Domination"},
			},
		},

		{
			name: "assigned",
			j: `{
    "id": 998144530,
    "url": "https://api.github.com/repos/bradfitz/go-issue-mirror/issues/events/998144530",
    "actor": {
      "login": "bradfitz",
      "id": 2621
    },
    "event": "assigned",
    "commit_id": null,
    "commit_url": null,
    "created_at": "2017-03-13T22:39:28Z",
    "assignee": {
      "login": "bradfitz",
      "id": 2621
    },
    "assigner": {
      "login": "bradfitz",
      "id": 2621
    }}`,
			e: &GitHubIssueEvent{
				ID:      998144530,
				Type:    "assigned",
				Created: t3339("2017-03-13T22:39:28Z"),
				Actor: &GitHubUser{
					ID:    2621,
					Login: "bradfitz",
				},
				Assignee: &GitHubUser{
					ID:    2621,
					Login: "bradfitz",
				},
				Assigner: &GitHubUser{
					ID:    2621,
					Login: "bradfitz",
				},
			},
			p: &maintpb.GithubIssueEvent{
				Id:         998144530,
				EventType:  "assigned",
				ActorId:    2621,
				Created:    p3339("2017-03-13T22:39:28Z"),
				AssigneeId: 2621,
				AssignerId: 2621,
			},
		},

		{
			name: "unassigned",
			j: `{
    "id": 1000077586,
    "url": "https://api.github.com/repos/bradfitz/go-issue-mirror/issues/events/1000077586",
    "actor": {
      "login": "dmitshur",
      "id": 1924134
    },
    "event": "unassigned",
    "commit_id": null,
    "commit_url": null,
    "created_at": "2017-03-15T00:31:42Z",
    "assignee": {
      "login": "dmitshur",
      "id": 1924134
    },
    "assigner": {
      "login": "bradfitz",
      "id": 2621
    }
  }`,
			e: &GitHubIssueEvent{
				ID:      1000077586,
				Type:    "unassigned",
				Created: t3339("2017-03-15T00:31:42Z"),
				Actor: &GitHubUser{
					ID:    1924134,
					Login: "dmitshur",
				},
				Assignee: &GitHubUser{
					ID:    1924134,
					Login: "dmitshur",
				},
				Assigner: &GitHubUser{
					ID:    2621,
					Login: "bradfitz",
				},
			},
			p: &maintpb.GithubIssueEvent{
				Id:         1000077586,
				EventType:  "unassigned",
				ActorId:    1924134,
				Created:    p3339("2017-03-15T00:31:42Z"),
				AssigneeId: 1924134,
				AssignerId: 2621,
			},
		},

		{
			name: "locked",
			j: `{
    "id": 998144646,
    "url": "https://api.github.com/repos/bradfitz/go-issue-mirror/issues/events/998144646",
    "actor": {
      "login": "bradfitz",
      "id": 2621
    },
    "event": "locked",
    "commit_id": null,
    "commit_url": null,
    "created_at": "2017-03-13T22:39:36Z"
  }`,
			e: &GitHubIssueEvent{
				ID:      998144646,
				Type:    "locked",
				Created: t3339("2017-03-13T22:39:36Z"),
				Actor: &GitHubUser{
					ID:    2621,
					Login: "bradfitz",
				},
			},
			p: &maintpb.GithubIssueEvent{
				Id:        998144646,
				EventType: "locked",
				ActorId:   2621,
				Created:   p3339("2017-03-13T22:39:36Z"),
			},
		},

		{
			name: "unlocked",
			j: `{
    "id": 1000014895,
    "url": "https://api.github.com/repos/bradfitz/go-issue-mirror/issues/events/1000014895",
    "actor": {
      "login": "bradfitz",
      "id": 2621
    },
    "event": "unlocked",
    "commit_id": null,
    "commit_url": null,
    "created_at": "2017-03-14T23:26:21Z"
 }`,
			e: &GitHubIssueEvent{
				ID:      1000014895,
				Type:    "unlocked",
				Created: t3339("2017-03-14T23:26:21Z"),
				Actor: &GitHubUser{
					ID:    2621,
					Login: "bradfitz",
				},
			},
			p: &maintpb.GithubIssueEvent{
				Id:        1000014895,
				EventType: "unlocked",
				ActorId:   2621,
				Created:   p3339("2017-03-14T23:26:21Z"),
			},
		},

		{
			name: "closed",
			j: `  {
    "id": 1006040931,
    "url": "https://api.github.com/repos/bradfitz/go-issue-mirror/issues/events/1006040931",
    "actor": {
      "login": "bradfitz",
      "id": 2621
    },
    "event": "closed",
    "commit_id": "e4d70f7e8892f024e4ed3e8b99ee6c5a9f16e126",
    "commit_url": "https://api.github.com/repos/bradfitz/go-issue-mirror/commits/e4d70f7e8892f024e4ed3e8b99ee6c5a9f16e126",
    "created_at": "2017-03-19T23:40:33Z"
  }`,
			e: &GitHubIssueEvent{
				ID:      1006040931,
				Type:    "closed",
				Created: t3339("2017-03-19T23:40:33Z"),
				Actor: &GitHubUser{
					ID:    2621,
					Login: "bradfitz",
				},
				CommitID:  "e4d70f7e8892f024e4ed3e8b99ee6c5a9f16e126",
				CommitURL: "https://api.github.com/repos/bradfitz/go-issue-mirror/commits/e4d70f7e8892f024e4ed3e8b99ee6c5a9f16e126",
			},
			p: &maintpb.GithubIssueEvent{
				Id:        1006040931,
				EventType: "closed",
				ActorId:   2621,
				Created:   p3339("2017-03-19T23:40:33Z"),
				Commit: &maintpb.GithubCommit{
					Owner:    "bradfitz",
					Repo:     "go-issue-mirror",
					CommitId: "e4d70f7e8892f024e4ed3e8b99ee6c5a9f16e126",
				},
			},
		},

		{
			name: "reopened",
			j: `{
    "id": 1000014895,
    "url": "https://api.github.com/repos/bradfitz/go-issue-mirror/issues/events/1000014895",
    "actor": {
      "login": "bradfitz",
      "id": 2621
    },
    "event": "reopened",
    "commit_id": null,
    "commit_url": null,
    "created_at": "2017-03-14T23:26:21Z"
 }`,
			e: &GitHubIssueEvent{
				ID:      1000014895,
				Type:    "reopened",
				Created: t3339("2017-03-14T23:26:21Z"),
				Actor: &GitHubUser{
					ID:    2621,
					Login: "bradfitz",
				},
			},
			p: &maintpb.GithubIssueEvent{
				Id:        1000014895,
				EventType: "reopened",
				ActorId:   2621,
				Created:   p3339("2017-03-14T23:26:21Z"),
			},
		},

		{
			name: "referenced",
			j: `{
    "id": 1006040930,
    "url": "https://api.github.com/repos/bradfitz/go-issue-mirror/issues/events/1006040930",
    "actor": {
      "login": "bradfitz",
      "id": 2621
    },
    "event": "referenced",
    "commit_id": "e4d70f7e8892f024e4ed3e8b99ee6c5a9f16e126",
    "commit_url": "https://api.github.com/repos/bradfitz/go-issue-mirror/commits/e4d70f7e8892f024e4ed3e8b99ee6c5a9f16e126",
    "created_at": "2017-03-19T23:40:32Z"
  }`,
			e: &GitHubIssueEvent{
				ID:      1006040930,
				Type:    "referenced",
				Created: t3339("2017-03-19T23:40:32Z"),
				Actor: &GitHubUser{
					ID:    2621,
					Login: "bradfitz",
				},
				CommitID:  "e4d70f7e8892f024e4ed3e8b99ee6c5a9f16e126",
				CommitURL: "https://api.github.com/repos/bradfitz/go-issue-mirror/commits/e4d70f7e8892f024e4ed3e8b99ee6c5a9f16e126",
			},
			p: &maintpb.GithubIssueEvent{
				Id:        1006040930,
				EventType: "referenced",
				ActorId:   2621,
				Created:   p3339("2017-03-19T23:40:32Z"),
				Commit: &maintpb.GithubCommit{
					Owner:    "bradfitz",
					Repo:     "go-issue-mirror",
					CommitId: "e4d70f7e8892f024e4ed3e8b99ee6c5a9f16e126",
				},
			},
		},

		{
			name: "renamed",
			j: `{
    "id": 1006107803,
    "url": "https://api.github.com/repos/bradfitz/go-issue-mirror/issues/events/1006107803",
    "actor": {
      "login": "bradfitz",
      "id": 2621
    },
    "event": "renamed",
    "commit_id": null,
    "commit_url": null,
    "created_at": "2017-03-20T02:53:43Z",
    "rename": {
      "from": "test-2",
      "to": "test-2 new name"
    }
  }`,
			e: &GitHubIssueEvent{
				ID:      1006107803,
				Type:    "renamed",
				Created: t3339("2017-03-20T02:53:43Z"),
				Actor: &GitHubUser{
					ID:    2621,
					Login: "bradfitz",
				},
				From: "test-2",
				To:   "test-2 new name",
			},
			p: &maintpb.GithubIssueEvent{
				Id:         1006107803,
				EventType:  "renamed",
				ActorId:    2621,
				Created:    p3339("2017-03-20T02:53:43Z"),
				RenameFrom: "test-2",
				RenameTo:   "test-2 new name",
			},
		},
	}

	var eventTypes []string

	for _, tt := range tests {
		evts, err := parseGithubEvents(strings.NewReader("[" + tt.j + "]"))
		if err != nil {
			t.Errorf("%s: parse JSON: %v", tt.name, err)
			continue
		}
		if len(evts) != 1 {
			t.Errorf("%s: parse JSON = %v entries; want 1", tt.name, len(evts))
			continue
		}
		gote := evts[0]
		if !reflect.DeepEqual(gote, tt.e) {
			t.Errorf("%s: JSON -> githubEvent differs: %v", tt.name, DeepDiff(gote, tt.e))
			continue
		}
		eventTypes = append(eventTypes, gote.Type)

		gotp := gote.Proto()
		if !reflect.DeepEqual(gotp, tt.p) {
			t.Errorf("%s: githubEvent -> proto differs: %v", tt.name, DeepDiff(gotp, tt.p))
			continue
		}

		var c Corpus
		c.initGithub()
		c.github.getOrCreateUserID(2621).Login = "bradfitz"
		c.github.getOrCreateUserID(1924134).Login = "dmitshur"
		gr := c.github.getOrCreateRepo("foowner", "bar")
		e2 := gr.newGithubEvent(gotp)

		if !reflect.DeepEqual(e2, tt.e) {
			t.Errorf("%s: proto -> githubEvent differs: %v", tt.name, DeepDiff(e2, tt.e))
			continue
		}
	}

	t.Logf("Tested event types: %q", eventTypes)
}

func TestCacheableURL(t *testing.T) {
	tests := []struct {
		v    string
		want bool
	}{
		{"https://api.github.com/repos/OWNER/RePO/milestones?page=1", true},
		{"https://api.github.com/repos/OWNER/RePO/milestones?page=2", false},
		{"https://api.github.com/repos/OWNER/RePO/milestones?", false},
		{"https://api.github.com/repos/OWNER/RePO/milestones", false},

		{"https://api.github.com/repos/OWNER/RePO/labels?page=1", true},
		{"https://api.github.com/repos/OWNER/RePO/labels?page=2", false},
		{"https://api.github.com/repos/OWNER/RePO/labels?", false},
		{"https://api.github.com/repos/OWNER/RePO/labels", false},

		{"https://api.github.com/repos/OWNER/RePO/foos?page=1", false},

		{"https://api.github.com/repos/OWNER/RePO/issues?page=1", false},
		{"https://api.github.com/repos/OWNER/RePO/issues?page=1&sort=updated&direction=desc", true},
	}

	for _, tt := range tests {
		got := cacheableURL(tt.v)
		if got != tt.want {
			t.Errorf("cacheableURL(%q) = %v; want %v", tt.v, got, tt.want)
		}
	}
}

func t3339(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t.UTC()
}

func p3339(s string) *timestamp.Timestamp {
	tp, err := ptypes.TimestampProto(t3339(s))
	if err != nil {
		panic(err)
	}
	return tp
}

func TestParseGithubRefs(t *testing.T) {
	tests := []struct {
		gerritProj string // "go.googlesource.com/go", etc
		msg        string
		want       []string
	}{
		{"go.googlesource.com/go", "\nFixes #1234\n", []string{"golang/go#1234"}},
		{"go.googlesource.com/go", "Fixes #1234\n", []string{"golang/go#1234"}},
		{"go.googlesource.com/go", "Fixes #1234", []string{"golang/go#1234"}},
		{"go.googlesource.com/go", "Fixes golang/go#1234", []string{"golang/go#1234"}},
		{"go.googlesource.com/go", "Fixes golang/go#1234\n", []string{"golang/go#1234"}},
		{"go.googlesource.com/go", "Fixes golang/go#1234.", []string{"golang/go#1234"}},
		{"go.googlesource.com/go", "Mention issue #1234 a second time.\n\nFixes #1234.", []string{"golang/go#1234"}},
		{"go.googlesource.com/go", "Mention issue #1234 a second time.\n\nFixes #1234.\nUpdates #1235.", []string{"golang/go#1234", "golang/go#1235"}},
		{"go.googlesource.com/net", "Fixes golang/go#1234.", []string{"golang/go#1234"}},
		{"go.googlesource.com/net", "Fixes #1234", nil},
	}
	for _, tt := range tests {
		c := new(Corpus)
		var got []string
		for _, ref := range c.parseGithubRefs(tt.gerritProj, tt.msg) {
			got = append(got, ref.String())
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("parseGithubRefs(%q, %q) = %q; want %q", tt.gerritProj, tt.msg, got, tt.want)
		}
	}
}
