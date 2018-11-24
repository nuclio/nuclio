// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
CL prints a list of open Go code reviews (also known as change lists, or CLs).

Usage:

	cl [-closed] [-dnr] [-r] [-url] [-cl 12345] [-project build]

CL searches Gerrit for CLs matching the query and then
prints a line for each CL that is waiting for review
(as opposed to waiting for revisions by the author).

The output line looks like:

	CL 9225    0/ 2d  go   rsc   austin*   cmd/internal/gc: emit write barrier

From left to right, the columns show the CL number,
the number of days the CL has been in the current waiting state
(waiting for author or waiting for review),
the number of days since the CL was created,
the project name ("go" or the name of a subrepository),
the author, the reviewer, and the subject.
If the CL is waiting for revisions by the author,
the author column has an asterisk.
If the CL is waiting for a reviewer, the reviewer column
has an asterisk.
If the CL has been reviewed by the reviewer,
the reviewer column shows the current score.

By default, CL omits closed CLs, those with an R=close reply
and no subsequent upload of a new patch set.
If the -closed flag is specified, CL adds closed CLs to the output.

By default, CL omits CLs containing ``DO NOT REVIEW'' in the
latest patch's commit message.
If the -dnr flag is specified, CL includes those CLs in its output.

If the -r flag is specified, CL shows only CLs that need review,
not those waiting for the author. In this mode, the
redundant ``waiting for reviewer'' asterisk is elided.

If the -url flag is specified, CL replaces "CL 1234" at the beginning
of each output line with a full URL, "https://golang.org/cl/1234".

If the -cl flag is specified, CL prints the status of just one particular CL.

If the -project flag is specified, CL prints the CLs only from the given project.

By default, CL sorts the output first by the combination of
project name and change subject.
The -sort flag changes the sort order. The choices are
"delay", to sort by the time the change has been in the current
waiting state, and "age", to sort by creation time.
When sorting, ties are broken by CL number.

TODO: Support do-not-review, output as JSON.
*/
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/build/maintner"
	"golang.org/x/build/maintner/godata"
)

var (
	flagClosed      = flag.Bool("closed", false, "include CLs that are closed or DO NOT REVIEW")
	flagDoNotReview = flag.Bool("dnr", false, "print only CLs in need of review")
	flagNeedsReview = flag.Bool("r", false, "print only CLs in need of review")
	flagJSON        = flag.Bool("json", false, "print CLs in JSON format")
	flagURL         = flag.Bool("url", false, "print full URLs for CLs")
	flagSort        = flag.String("sort", "", "sort by `order` (age or delay) instead of project+subject")
	flagCL          = flag.Int("cl", 0, "include only the CL specified (-cl 2130)")
	flagProject     = flag.String("project", "", "include only CLs from the project specified")
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: cl [query]\n")
	fmt.Fprintf(os.Stderr, "Flags:\n")
	flag.PrintDefaults()
	os.Exit(2)
}

var now = time.Now() // so time stays the same during computations.

// CL is a wrapper for a GerritCL object.
// It holds some meta information which is required for writing the output.
type CL struct {
	// gerritCL is the actual CL object as returned by the Corpus.
	gerritCL *maintner.GerritCL

	// needsReview indicates whether or not this CL requires a review.
	needsReview bool

	// needsReviewChanged is the last time when needsReview was set.
	needsReviewChanged time.Time

	// reviewerEmail is the email address of the person responsible for reviewing this CL.
	reviewerEmail string
	closed        bool
	closedReason  string

	// scores is a map of the last scores given
	// by the reviewers for this particular CL (+1, -1, +2, -2).
	// It is keyed by the reviewer's email address.
	scores map[string]int
}

func (cl *CL) age(now time.Time) time.Duration {
	return now.Sub(cl.gerritCL.Created)
}

func (cl *CL) delaySinceLastUpdated(now time.Time) time.Duration {
	return now.Sub(cl.needsReviewChanged)
}

const maxUsernameLen = 12

func main() {
	log.SetFlags(0)
	log.SetPrefix("cl: ")

	flag.Usage = usage
	flag.Parse()

	gerritAccounts := &GerritAccounts{}

	if err := gerritAccounts.Initialize(); err != nil {
		log.Fatal("couldn't initialise Gerrit account mapping", err)
	}

	switch *flagSort {
	case "", "age", "delay":
		// ok
	default:
		log.Fatal("unknown sort order")
	}

	corpus, err := godata.Get(context.Background())
	if err != nil {
		log.Fatal("couldn't initialise the Corpus", err)
	}

	cls := []*CL{}

	corpus.Gerrit().ForeachProjectUnsorted(func(gp *maintner.GerritProject) error {
		if *flagProject != "" && *flagProject != gp.Project() {
			return nil
		}

		gp.ForeachCLUnsorted(func(cl *maintner.GerritCL) error {
			if cl.Status == "abandoned" { // Don't display abandoned CLs.
				return nil
			}

			if cl.Meta == nil { // Occurs infrequently. See https://golang.org/issue/22060.
				return nil
			}

			if *flagCL > 0 && int32(*flagCL) != cl.Number {
				return nil
			}

			ourCL := &CL{gerritCL: cl}
			updateReviewStatus(ourCL, gerritAccounts)

			if (*flagNeedsReview && !ourCL.needsReview) || (!*flagClosed && cl.Status == "merged") {
				return nil
			}

			cls = append(cls, ourCL)

			return nil
		})

		return nil
	})

	switch *flagSort {
	case "":
		sort.Sort(byRepoAndSubject(cls))
	case "age":
		sort.Sort(byAge(cls))
	case "delay":
		sort.Sort(byDelay(cls))
	}

	clPrefix := "CL "
	if *flagURL {
		clPrefix = "https://golang.org/cl/"
	}

	var projectLen, authorLen, reviewerLen int
	for _, cl := range cls {
		projectLen = max(projectLen, len(cl.gerritCL.Project.Project()))
		authorLen = max(authorLen, len(cl.gerritCL.Meta.Commit.Author.Email()))
		if cl.reviewerEmail != "" {
			reviewerLen = max(reviewerLen, len(cl.reviewerEmail))
		}
	}

	if authorLen > maxUsernameLen {
		authorLen = maxUsernameLen
	}

	if reviewerLen > maxUsernameLen {
		reviewerLen = maxUsernameLen
	}

	authorLen += 1   // For *.
	reviewerLen += 3 // For +2*.

	var buf bytes.Buffer
	for _, cl := range cls {
		fmt.Fprintf(&buf, "%s%-5d %3.0f/%3.0fd %-*s  %-*s %-*s %s\n",
			clPrefix, cl.gerritCL.Number,
			cl.delaySinceLastUpdated(now).Hours()/24, cl.age(now).Hours()/24,
			projectLen, cl.gerritCL.Project.Project(),
			authorLen, authorString(cl, gerritAccounts, authorLen),
			reviewerLen, reviewerString(cl, reviewerLen),
			cl.gerritCL.Subject())
	}
	os.Stdout.Write(buf.Bytes())
}

const tagCodeReview = "Label: Code-Review="

// updateReviewStatus guesses the reviewer, and then decides
// whether or not the given CL is waiting for a review or not.
func updateReviewStatus(cl *CL, gerritAccounts *GerritAccounts) {
	var initialReviewer, firstResponder string

	cl.scores = map[string]int{}

	authorEmail, err := gerritAccounts.LookupByGerritEmail(cl.gerritCL.Metas[0].Commit.Author.Email(), true)
	if err != nil {
		return // We can't resolve the author.
	}

	// Find the initial reviewer, and the first responder (always exclude the author in both cases).
	// Also update the scores map.
	for _, meta := range cl.gerritCL.Metas {
		if firstResponder == "" {
			responder, err := gerritAccounts.LookupByGerritEmail(meta.Commit.Author.Email(), true)
			if err == nil && responder.Email != authorEmail.Email {
				firstResponder = responder.Email
			}
		}

		if meta.Commit.Reviewer == nil {
			continue
		}

		reviewer, err := gerritAccounts.LookupByGerritEmail(meta.Commit.Reviewer.Email(), true)
		if err != nil {
			continue
		}

		codeReviewIdx := strings.Index(meta.Commit.Msg, tagCodeReview)

		if codeReviewIdx > 0 {
			prefix := len(tagCodeReview)
			// Extract and convert the point(s). This line takes the form "Label: Code-Review=+1".
			val, err := strconv.Atoi(meta.Commit.Msg[codeReviewIdx+prefix : codeReviewIdx+prefix+2])
			if err == nil {
				cl.scores[reviewer.Email] = val
			}
		}

		if initialReviewer == "" && reviewer.Email != "" && authorEmail.Email != reviewer.Email {
			initialReviewer = reviewer.Email
		}
	}

	if initialReviewer != "" {
		cl.reviewerEmail = initialReviewer
	}

	// maybe sets the reviewerEmail if it's not set yet.
	maybe := func(who string) {
		// The initial reviewer always gets the highest priority.
		if cl.reviewerEmail == "" || who == initialReviewer {
			cl.reviewerEmail = who
		}
	}

	// Determine reviewer, in priority order.

	// 1. Anyone who -2'ed the CL.
	for who, score := range cl.scores {
		if score == -2 {
			maybe(who)
		}
	}

	// 2. Anyone who +2'ed the CL.
	for who, score := range cl.scores {
		if score == +2 {
			maybe(who)
		}
	}

	// 3. Whoever responds first.
	if firstResponder != "" {
		maybe(firstResponder)
	}

	// Now that we know who the reviewer is,
	// figure out whether the CL is in need of review
	// (or else is waiting for the author to do more work).
	for _, meta := range cl.gerritCL.Metas {
		if meta.Commit.Author == nil { // Happens for Gerrit-generated messages.
			continue
		}

		accountInfo, err := gerritAccounts.LookupByGerritEmail(meta.Commit.Author.Email(), true)
		if err != nil {
			continue
		}

		if strings.Contains(meta.Commit.Msg, "Uploaded patch set ") || accountInfo.Email != cl.reviewerEmail {
			cl.needsReview = true
			cl.needsReviewChanged = meta.Commit.CommitTime
		}

		if accountInfo.Email == cl.reviewerEmail {
			cl.needsReview = false
			cl.needsReviewChanged = meta.Commit.CommitTime
		}
	}

	// TODO: Support do not review, close, and postpone to next go release
}

func max(i, j int) int {
	if i < j {
		return j
	}
	return i
}

// authorString returns the author column, limited to n bytes.
func authorString(cl *CL, gerritAccounts *GerritAccounts, n int) string {
	suffix := ""
	if !cl.needsReview {
		suffix = "*"
	}

	first := cl.gerritCL.Meta.Commit

	for first.Parents != nil && len(first.Parents) > 0 {
		first = first.Parents[0]
	}

	// Lookup the real account ID.
	accountInfo, err := gerritAccounts.LookupByGerritEmail(first.Author.Email(), true)
	if err != nil {
		return ""
	}

	return truncate(username(accountInfo.Email), n-len(suffix)) + suffix
}

// username returns the ideal username from the email address.
// This might not be the actual username of the person, but merely a short name
// that can be displayed in the output.
func username(email string) string {
	idx := strings.Index(email, "@")
	if idx != -1 {
		return email[0:idx]
	}

	return email
}

// reviewerString returns the reviewer column, limited to n bytes.
func reviewerString(cl *CL, n int) string {
	suffix := ""
	if cl.needsReview && !*flagNeedsReview {
		suffix = "*"
	}

	if score := (cl.scores)[cl.reviewerEmail]; score != 0 {
		suffix = fmt.Sprintf("%+d", score) + suffix
	}
	return truncate(username(cl.reviewerEmail), n-len(suffix)) + suffix
}

// truncate returns the name truncated to n bytes.
func truncate(text string, n int) string {
	if len(text) <= n {
		return text
	}
	return text[:n-3] + "..."
}

// Sort interfaces.

type byRepoAndSubject []*CL

func (x byRepoAndSubject) Len() int      { return len(x) }
func (x byRepoAndSubject) Swap(i, j int) { x[i], x[j] = x[j], x[i] }
func (x byRepoAndSubject) Less(i, j int) bool {
	if x[i].gerritCL.Project.Project() != x[j].gerritCL.Project.Project() {
		return projectOrder(x[i].gerritCL.Project.Project()) < projectOrder(x[j].gerritCL.Project.Project())
	}

	if x[i].gerritCL.Subject() != x[j].gerritCL.Subject() {
		return x[i].gerritCL.Subject() < x[j].gerritCL.Subject()
	}
	return x[i].gerritCL.Number < x[j].gerritCL.Number
}

type byAge []*CL

func (x byAge) Len() int      { return len(x) }
func (x byAge) Swap(i, j int) { x[i], x[j] = x[j], x[i] }
func (x byAge) Less(i, j int) bool {
	if !x[i].gerritCL.Created.Equal(x[j].gerritCL.Created) {
		return x[i].gerritCL.Created.Before(x[j].gerritCL.Created)
	}
	return x[i].gerritCL.Number > x[j].gerritCL.Number
}

type byDelay []*CL

func (x byDelay) Len() int      { return len(x) }
func (x byDelay) Swap(i, j int) { x[i], x[j] = x[j], x[i] }
func (x byDelay) Less(i, j int) bool {
	if !x[i].needsReviewChanged.Equal(x[j].needsReviewChanged) {
		return x[i].needsReviewChanged.Before(x[j].needsReviewChanged)
	}
	return x[i].gerritCL.Number < x[j].gerritCL.Number
}

func projectOrder(name string) string {
	if name == "go" {
		return "\x00" // Sort before everything except empty string.
	}
	return name
}
