// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// Package pubsubtypes contains types published by pubsubhelper.
package pubsubtypes

import (
	"go4.org/types"
)

// Event is the type of event that comes out of pubsubhelper.
type Event struct {
	// Time is the time the event was received, or the time of the
	// long poll timeout. This is what clients should send as the
	// "after" URL parameter for the next event.
	Time types.Time3339

	// LongPollTimeout indicates that no event occurred and the
	// client should retry with ?after=<Time>.
	LongPollTimeout bool `json:",omitempty"`

	// Gerrit is non-nil for Gerrit events.
	Gerrit *GerritEvent `json:",omitempty"`

	// Github is non-nil for GitHub events.
	GitHub *GitHubEvent `json:",omitempty"`
}

// GerritEvent is a type of Event.
type GerritEvent struct {
	// URL is of the form "https://go-review.googlesource.com/39551".
	URL string

	// Project is the Gerrit project on the server, such as "go",
	// "net", "crypto".
	Project string

	// CommitHash is in the Gerrit email headers, so it's included here.
	// I don't dare specify what it means. It seems to be the commit hash
	// that's new or being commented upon. Notably, it doesn't ever appear
	// to be the meta hash for comments.
	CommitHash string

	// ChangeNumber is the number of the change (e.g. 39551).
	ChangeNumber int `json:",omitempty"`
}

type GitHubEvent struct {
	Action            string
	RepoOwner         string // "golang"
	Repo              string // "go"
	IssueNumber       int    `json:",omitempty"`
	PullRequestNumber int    `json:",omitempty"`
}
