// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/go-github/github"
	"golang.org/x/build/maintner"
	"golang.org/x/build/maintner/godata"
	"golang.org/x/oauth2"
)

const (
	projectOwner = "golang"
	projectRepo  = "go"
)

var githubClient *github.Client

// GitHub personal access token, from https://github.com/settings/applications.
var githubAuthToken string

var goRepo *maintner.GitHubRepo

func loadMaintner() {
	corpus, err := godata.Get(context.Background())
	if err != nil {
		log.Fatal("failed to load maintner data:", err)
	}
	goRepo = corpus.GitHub().Repo(projectOwner, projectRepo)
}

func loadGithubAuth() {
	const short = ".github-issue-token"
	filename := filepath.Clean(os.Getenv("HOME") + "/" + short)
	shortFilename := filepath.Clean("$HOME/" + short)
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatal("reading token: ", err, "\n\n"+
			"Please create a personal access token at https://github.com/settings/tokens/new\n"+
			"and write it to ", shortFilename, " to use this program.\n"+
			"** The token only needs the public_repo scope. **\n"+
			"The benefit of using a personal access token over using your GitHub\n"+
			"password directly is that you can limit its use and revoke it at any time.\n\n")
	}
	fi, err := os.Stat(filename)
	if err != nil {
		log.Fatalln("reading token:", err)
	}
	if fi.Mode()&0077 != 0 {
		log.Fatalf("reading token: %s mode is %#o, want %#o", shortFilename, fi.Mode()&0777, fi.Mode()&0700)
	}
	githubAuthToken = strings.TrimSpace(string(data))
	t := &oauth2.Transport{
		Source: &tokenSource{AccessToken: githubAuthToken},
	}
	githubClient = github.NewClient(&http.Client{Transport: t})
}

// releaseStatusTitle returns the title of the release status issue
// for the given milestone.
// If you change this function, releasebot will not be able to find an
// existing tracking issue using the old name and will create a new one.
func (w *Work) releaseStatusTitle() string {
	return "all: " + strings.Replace(w.Version, "go", "Go ", -1) + " release status"
}

type tokenSource oauth2.Token

func (t *tokenSource) Token() (*oauth2.Token, error) {
	return (*oauth2.Token)(t), nil
}

func (w *Work) findOrCreateReleaseIssue() {
	w.log.Printf("Release status issue title: %q", w.releaseStatusTitle())
	if dryRun {
		return
	}
	if w.ReleaseIssue == 0 {
		title := w.releaseStatusTitle()
		body := fmt.Sprintf("Issue tracking the %s release by releasebot.", w.Version)
		num, err := w.createGitHubIssue(title, body)
		if err != nil {
			w.log.Panic(err)
		}
		w.ReleaseIssue = num
		w.log.Printf("Release status issue: https://golang.org/issue/%d", num)
	}
}

// createGitHubIssue creates an issue in the release milestone and returns its number.
func (w *Work) createGitHubIssue(title, msg string) (int, error) {
	if dryRun {
		return 0, errors.New("attemted write operation in dry-run mode")
	}
	var dup int
	goRepo.ForeachIssue(func(gi *maintner.GitHubIssue) error {
		if gi.Title == title {
			dup = int(gi.Number)
			return errors.New("stop iteration")
		}
		return nil
	})
	if dup != 0 {
		return dup, nil
	}
	opts := &github.IssueListByRepoOptions{
		State:       "all",
		ListOptions: github.ListOptions{PerPage: 100},
	}
	if !w.BetaRelease && !w.RCRelease {
		opts.Milestone = strconv.Itoa(int(w.Milestone.Number))
	}
	is, _, err := githubClient.Issues.ListByRepo(context.TODO(), "golang", "go", opts)
	if err != nil {
		return 0, err
	}
	for _, i := range is {
		if i.GetTitle() == title {
			// Dup.
			return i.GetNumber(), nil
		}
	}
	copts := &github.IssueRequest{
		Title: github.String(title),
		Body:  github.String(msg),
	}
	if !w.BetaRelease && !w.RCRelease {
		copts.Milestone = github.Int(int(w.Milestone.Number))
	}
	i, _, err := githubClient.Issues.Create(context.TODO(), "golang", "go", copts)
	return i.GetNumber(), err
}

// pushIssues moves open issues to the next release.
func (w *Work) pushIssues() {
	if err := goRepo.ForeachIssue(func(gi *maintner.GitHubIssue) error {
		if gi.Milestone == nil || gi.Milestone.Title != w.Milestone.Title {
			return nil
		}
		if gi.Closed || gi.Title == w.releaseStatusTitle() {
			return nil
		}
		w.log.Printf("changing milestone of issue %d to %s", gi.Number, w.NextMilestone.Title)
		if dryRun {
			return nil
		}
		_, _, err := githubClient.Issues.Edit(context.TODO(), projectOwner, projectRepo, int(gi.Number), &github.IssueRequest{
			Milestone: github.Int(int(w.NextMilestone.Number)),
		})
		if err != nil {
			return fmt.Errorf("#%d: %s", gi.Number, err)
		}
		return nil
	}); err != nil {
		w.logError("error moving issues to the next minor release: %v", err)
		return
	}
}

func (w *Work) closeMilestone() {
	w.log.Printf("closing milestone %s", w.Milestone.Title)
	if dryRun {
		return
	}
	closed := "closed"
	_, _, err := githubClient.Issues.EditMilestone(context.TODO(), projectOwner, projectRepo, int(w.Milestone.Number), &github.Milestone{
		State: &closed,
	})
	if err != nil {
		w.logError("closing milestone: %v", err)
	}

}

func postGithubComment(number int, body string) error {
	if dryRun {
		return errors.New("attemted write operation in dry-run mode")
	}
	_, _, err := githubClient.Issues.CreateComment(context.TODO(), projectOwner, projectRepo, number, &github.IssueComment{
		Body: &body,
	})
	return err
}
