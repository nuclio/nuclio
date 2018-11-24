// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"cloud.google.com/go/compute/metadata"
	"golang.org/x/build/cmd/pubsubhelper/pubsubtypes"
)

func handleGithubWebhook(w http.ResponseWriter, r *http.Request) {
	if r.TLS == nil {
		http.Error(w, "HTTPS required", http.StatusBadRequest)
		return
	}
	body, err := validateGithubRequest(w, r)
	if err != nil {
		log.Printf("failed to validate github webhook request: %v", err)
		// But send a 200 OK anyway, so they don't queue up on
		// Github's side if they're real.
		return
	}

	var payload githubWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("error unmarshalling payload: %v; payload=%s", err, body)
		// But send a 200 OK anyway. Our fault.
		return
	}
	back, _ := json.MarshalIndent(payload, "", "\t")
	log.Printf("github verified webhook: %s", back)

	if payload.Repository == nil || (payload.Issue == nil && payload.PullRequest == nil) {
		// Ignore.
		return
	}

	f := strings.Split(payload.Repository.FullName, "/")
	if len(f) != 2 {
		log.Printf("bogus repository name %q", payload.Repository.FullName)
		return
	}
	owner, repo := f[0], f[1]

	var issueNumber int
	if payload.Issue != nil {
		issueNumber = payload.Issue.Number
	}
	var prNumber int
	if payload.PullRequest != nil {
		prNumber = payload.PullRequest.Number
	}

	publish(&pubsubtypes.Event{
		GitHub: &pubsubtypes.GitHubEvent{
			Action:            payload.Action,
			RepoOwner:         owner,
			Repo:              repo,
			IssueNumber:       issueNumber,
			PullRequestNumber: prNumber,
		},
	})
}

// validate compares the signature in the request header with the body.
func validateGithubRequest(w http.ResponseWriter, r *http.Request) (body []byte, err error) {
	// Decode signature header.
	sigHeader := r.Header.Get("X-Hub-Signature")
	sigParts := strings.SplitN(sigHeader, "=", 2)
	if len(sigParts) != 2 {
		return nil, fmt.Errorf("Bad signature header: %q", sigHeader)
	}
	var h func() hash.Hash
	switch alg := sigParts[0]; alg {
	case "sha1":
		h = sha1.New
	case "sha256":
		h = sha256.New
	default:
		return nil, fmt.Errorf("Unsupported hash algorithm: %q", alg)
	}
	gotSig, err := hex.DecodeString(sigParts[1])
	if err != nil {
		return nil, err
	}

	// Compute expected signature.
	key, err := metadata.ProjectAttributeValue("pubsubhelper-webhook-secret")
	if err != nil {
		return nil, err
	}
	body, err = ioutil.ReadAll(http.MaxBytesReader(w, r.Body, 5<<20))
	if err != nil {
		return nil, err
	}
	mac := hmac.New(h, []byte(key))
	mac.Write(body)
	expectSig := mac.Sum(nil)

	if !hmac.Equal(gotSig, expectSig) {
		return nil, fmt.Errorf("Invalid signature %X, want %x", gotSig, expectSig)
	}
	return body, nil
}

type githubWebhookPayload struct {
	Action      string             `json:"action"`
	Repository  *githubRepository  `json:"repository"`
	Issue       *githubIssue       `json:"issue"`
	PullRequest *githubPullRequest `json:"pull_request"`
}

type githubRepository struct {
	FullName string `json:"full_name"` // "golang/go"
}

type githubIssue struct {
	URL    string `json:"url"`    // https://api.github.com/repos/baxterthehacker/public-repo/issues/2
	Number int    `json:"number"` // 2
}

type githubPullRequest struct {
	URL    string `json:"url"`    // https://api.github.com/repos/baxterthehacker/public-repo/pulls/8
	Number int    `json:"number"` // 8
}
