// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The gopherbot command runs Go's gopherbot role account on
// GitHub and Gerrit.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"cloud.google.com/go/compute/metadata"
	"github.com/google/go-github/github"
	"go4.org/strutil"
	"golang.org/x/build/devapp/owners"
	"golang.org/x/build/gerrit"
	"golang.org/x/build/maintner"
	"golang.org/x/build/maintner/godata"
	"golang.org/x/build/maintner/maintnerd/apipb"
	"golang.org/x/oauth2"
	"grpc.go4.org"
)

var (
	dryRun          = flag.Bool("dry-run", false, "just report what would've been done, without changing anything")
	daemon          = flag.Bool("daemon", false, "run in daemon mode")
	githubTokenFile = flag.String("github-token-file", filepath.Join(os.Getenv("HOME"), "keys", "github-gobot"), `File to load Github token from. File should be of form <username>:<token>`)
	// go here: https://go-review.googlesource.com/settings#HTTPCredentials
	// click "Obtain Password"
	// The next page will have a .gitcookies file - look for the part that has
	// "git-youremail@yourcompany.com=password". Copy and paste that to the
	// token file with a colon in between the email and password.
	gerritTokenFile = flag.String("gerrit-token-file", filepath.Join(os.Getenv("HOME"), "keys", "gerrit-gobot"), `File to load Gerrit token from. File should be of form <git-email>:<token>`)

	onlyRun = flag.String("only-run", "", "if non-empty, the name of a task to run. Mostly for debugging, but tasks (like 'kicktrain') may choose to only run in explicit mode")
)

// GitHub Label IDs for the golang/go repo.
const (
	needsDecisionID      = 373401956
	needsFixID           = 373399998
	needsInvestigationID = 373402289
)

// Label names (that are used in multiple places).
const (
	frozenDueToAge = "FrozenDueToAge"
)

// GitHub Milestone numbers for the golang/go repo.
var (
	proposal   = milestone{30, "Proposal"}
	unreleased = milestone{22, "Unreleased"}
	unplanned  = milestone{6, "Unplanned"}
	gccgo      = milestone{23, "Gccgo"}
	vgo        = milestone{71, "vgo"}
)

type milestone struct {
	Number int
	Name   string
}

func getGithubToken() (string, error) {
	if metadata.OnGCE() {
		for _, key := range []string{"gopherbot-github-token", "maintner-github-token"} {
			token, err := metadata.ProjectAttributeValue(key)
			if token != "" && err == nil {
				return token, nil
			}
		}
	}
	slurp, err := ioutil.ReadFile(*githubTokenFile)
	if err != nil {
		return "", err
	}
	f := strings.SplitN(strings.TrimSpace(string(slurp)), ":", 2)
	if len(f) != 2 || f[0] == "" || f[1] == "" {
		return "", fmt.Errorf("Expected token %q to be of form <username>:<token>", slurp)
	}
	return f[1], nil
}

func getGerritAuth() (username string, password string, err error) {
	var slurp string
	if metadata.OnGCE() {
		for _, key := range []string{"gopherbot-gerrit-token", "maintner-gerrit-token", "gobot-password"} {
			slurp, err = metadata.ProjectAttributeValue(key)
			if slurp != "" && err == nil {
				break
			}
		}
	}
	if len(slurp) == 0 {
		var slurpBytes []byte
		slurpBytes, err = ioutil.ReadFile(*gerritTokenFile)
		if err != nil {
			return "", "", err
		}
		slurp = string(slurpBytes)
	}
	f := strings.SplitN(strings.TrimSpace(slurp), ":", 2)
	if len(f) == 1 {
		// assume the whole thing is the token
		return "git-gobot.golang.org", f[0], nil
	}
	if len(f) != 2 || f[0] == "" || f[1] == "" {
		return "", "", fmt.Errorf("Expected Gerrit token %q to be of form <git-email>:<token>", slurp)
	}
	return f[0], f[1], nil
}

func getGithubClient() (*github.Client, error) {
	token, err := getGithubToken()
	if err != nil {
		if *dryRun {
			return github.NewClient(http.DefaultClient), nil
		}
		return nil, err
	}
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)
	return github.NewClient(tc), nil
}

func getGerritClient() (*gerrit.Client, error) {
	username, token, err := getGerritAuth()
	if err != nil {
		if *dryRun {
			c := gerrit.NewClient("https://go-review.googlesource.com", gerrit.NoAuth)
			return c, nil
		}
		return nil, err
	}
	c := gerrit.NewClient("https://go-review.googlesource.com", gerrit.BasicAuth(username, token))
	return c, nil
}

func getMaintnerClient() (apipb.MaintnerServiceClient, error) {
	cc, err := grpc.NewClient(nil, "https://maintner.golang.org")
	if err != nil {
		return nil, err
	}
	return apipb.NewMaintnerServiceClient(cc), nil
}

func init() {
	flag.Usage = func() {
		os.Stderr.WriteString("gopherbot runs Go's gopherbot role account on GitHub and Gerrit.\n\n")
		flag.PrintDefaults()
	}
}

type gerritChange struct {
	project string
	num     int32
}

func (c gerritChange) ID() string {
	// https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#change-id
	return fmt.Sprintf("%s~%d", c.project, c.num)
}

func (c gerritChange) String() string {
	return c.ID()
}

func main() {
	flag.Parse()

	ghc, err := getGithubClient()
	if err != nil {
		log.Fatal(err)
	}
	gerrit, err := getGerritClient()
	if err != nil {
		log.Fatal(err)
	}
	mc, err := getMaintnerClient()
	if err != nil {
		log.Fatal(err)
	}

	bot := &gopherbot{
		ghc:    ghc,
		gerrit: gerrit,
		mc:     mc,
		deletedChanges: map[gerritChange]bool{
			{"crypto", 35958}: true,
		},
	}
	bot.initCorpus()

	ctx := context.Background()
	for {
		t0 := time.Now()
		err := bot.doTasks(ctx)
		if err != nil {
			log.Print(err)
		}
		botDur := time.Since(t0)
		log.Printf("gopherbot ran in %v", botDur)
		if !*daemon {
			if err != nil {
				os.Exit(1)
			}
			return
		}
		if err != nil {
			log.Printf("sleeping 30s after previous error.")
			time.Sleep(30 * time.Second)
		}
		for {
			t0 := time.Now()
			err := bot.corpus.Update(ctx)
			if err != nil {
				if err == maintner.ErrSplit {
					log.Print("Corpus out of sync. Re-fetching corpus.")
					bot.initCorpus()
				} else {
					log.Printf("corpus.Update: %v; sleeping 15s", err)
					time.Sleep(15 * time.Second)
					continue
				}
			}
			log.Printf("got corpus update after %v", time.Since(t0))
			break
		}
		lastTask = ""
	}
}

type gopherbot struct {
	ghc    *github.Client
	gerrit *gerrit.Client
	mc     apipb.MaintnerServiceClient
	corpus *maintner.Corpus
	gorepo *maintner.GitHubRepo

	knownContributors map[string]bool

	// Until golang.org/issue/22635 is fixed, keep a map of changes that were deleted
	// to prevent calls to Gerrit that will always 404.
	deletedChanges map[gerritChange]bool

	releases struct {
		sync.Mutex
		lastUpdate time.Time
		major      []string // last two releases and the next upcoming release, like: "1.9", "1.10", "1.11"
	}
}

var tasks = []struct {
	name string
	fn   func(*gopherbot, context.Context) error
}{
	{"kicktrain", (*gopherbot).getOffKickTrain},
	{"unwait-release", (*gopherbot).unwaitRelease},
	{"freeze old issues", (*gopherbot).freezeOldIssues},
	{"label proposals", (*gopherbot).labelProposals},
	{"set subrepo milestones", (*gopherbot).setSubrepoMilestones},
	{"set misc milestones", (*gopherbot).setMiscMilestones},
	{"label build issues", (*gopherbot).labelBuildIssues},
	{"label mobile issues", (*gopherbot).labelMobileIssues},
	{"label documentation issues", (*gopherbot).labelDocumentationIssues},
	{"close stale WaitingForInfo", (*gopherbot).closeStaleWaitingForInfo},
	{"cl2issue", (*gopherbot).cl2issue},
	{"update needs", (*gopherbot).updateNeeds},
	{"congratulate new contributors", (*gopherbot).congratulateNewContributors},
	{"un-wait CLs", (*gopherbot).unwaitCLs},
	{"open cherry pick issues", (*gopherbot).openCherryPickIssues},
	{"apply minor release milestones", (*gopherbot).setMinorMilestones},
	{"close cherry pick issues", (*gopherbot).closeCherryPickIssues},
	{"apply labels from comments", (*gopherbot).applyLabelsFromComments},
	{"assign reviewers to CLs", (*gopherbot).assignReviewersToCLs},
	{"abandon scratch reviews", (*gopherbot).abandonScratchReviews},
}

func (b *gopherbot) initCorpus() {
	ctx := context.Background()
	corpus, err := godata.Get(ctx)
	if err != nil {
		log.Fatalf("godata.Get: %v", err)
	}

	repo := corpus.GitHub().Repo("golang", "go")
	if repo == nil {
		log.Fatal("Failed to find Go repo in Corpus.")
	}

	b.corpus = corpus
	b.gorepo = repo
}

func (b *gopherbot) doTasks(ctx context.Context) error {
	for _, task := range tasks {
		if *onlyRun != "" && task.name != *onlyRun {
			continue
		}
		if err := task.fn(b, ctx); err != nil {
			log.Printf("%s: %v", task.name, err)
			return err
		}
	}

	return nil
}

func (b *gopherbot) addLabel(ctx context.Context, gi *maintner.GitHubIssue, label string) error {
	return b.addLabels(ctx, gi, []string{label})
}

func (b *gopherbot) addLabels(ctx context.Context, gi *maintner.GitHubIssue, labels []string) error {
	var toAdd []string
	for _, label := range labels {
		if gi.HasLabel(label) {
			log.Printf("Issue %d already has label %q; no need to send request to add it", gi.Number, label)
			continue
		}
		printIssue("label-"+label, gi)
		toAdd = append(toAdd, label)
	}

	if *dryRun || len(toAdd) == 0 {
		return nil
	}

	return addLabelsToIssue(ctx, b.ghc.Issues, int(gi.Number), toAdd)
}

// addLabelsToIssue adds labels to the issue in golang/go with the given issueNum.
// TODO: Proper stubs via interfaces.
var addLabelsToIssue = func(ctx context.Context, issues *github.IssuesService, issueNum int, labels []string) error {
	_, _, err := issues.AddLabelsToIssue(ctx, "golang", "go", issueNum, labels)
	return err
}

// removeLabel removes the label from the given issue in golang/go.
func (b *gopherbot) removeLabel(ctx context.Context, gi *maintner.GitHubIssue, label string) error {
	return b.removeLabels(ctx, gi, []string{label})
}

func (b *gopherbot) removeLabels(ctx context.Context, gi *maintner.GitHubIssue, labels []string) error {
	var removeLabels bool
	for _, l := range labels {
		if !gi.HasLabel(l) {
			log.Printf("Issue %d (in maintner) does not have label %q; no need to send request to remove it", gi.Number, l)
			continue
		}
		printIssue("label-"+l, gi)
		removeLabels = true
	}

	if *dryRun || !removeLabels {
		return nil
	}

	ghLabels, err := labelsForIssue(ctx, b.ghc.Issues, int(gi.Number))
	if err != nil {
		return err
	}
	toRemove := make(map[string]bool)
	for _, l := range labels {
		toRemove[l] = true
	}

	for _, l := range ghLabels {
		if toRemove[l] {
			if err := removeLabelFromIssue(ctx, b.ghc.Issues, int(gi.Number), l); err != nil {
				log.Printf("Could not remove label %q from issue %d: %v", l, gi.Number, err)
				continue
			}
		}
	}
	return nil
}

// labelsForIssue returns all labels for the given issue in the golang/go repo.
// TODO: Proper stubs via interfaces.
var labelsForIssue = func(ctx context.Context, issues *github.IssuesService, issueNum int) ([]string, error) {
	ghLabels, _, err := issues.ListLabelsByIssue(ctx, "golang", "go", issueNum, &github.ListOptions{PerPage: 100})
	if err != nil {
		return nil, fmt.Errorf("could not list labels for golang/go#%d: %v", issueNum, err)
	}
	var labels []string
	for _, l := range ghLabels {
		labels = append(labels, l.GetName())
	}
	return labels, nil
}

// removeLabelForIssue removes the given label from golang/go with the given issueNum.
// If the issue did not have the label already (or the label didn't exist), return nil.
// TODO: Proper stubs via interfaces.
var removeLabelFromIssue = func(ctx context.Context, issues *github.IssuesService, issueNum int, label string) error {
	_, err := issues.RemoveLabelForIssue(ctx, "golang", "go", issueNum, label)
	if ge, ok := err.(*github.ErrorResponse); ok && ge.Response != nil && ge.Response.StatusCode == http.StatusNotFound {
		return nil
	}
	return err
}

func (b *gopherbot) setMilestone(ctx context.Context, gi *maintner.GitHubIssue, m milestone) error {
	printIssue("milestone-"+m.Name, gi)
	if *dryRun {
		return nil
	}
	_, _, err := b.ghc.Issues.Edit(ctx, "golang", "go", int(gi.Number), &github.IssueRequest{
		Milestone: github.Int(m.Number),
	})
	return err
}

func (b *gopherbot) addGitHubComment(ctx context.Context, org, repo string, issueNum int32, msg string) error {
	gr := b.corpus.GitHub().Repo(org, repo)
	if gr == nil {
		return fmt.Errorf("unknown github repo %s/%s", org, repo)
	}
	var since time.Time
	if gi := gr.Issue(issueNum); gi != nil {
		dup := false
		gi.ForeachComment(func(c *maintner.GitHubComment) error {
			since = c.Updated
			// TODO: check for gopherbot as author? check for exact match?
			// This seems fine for now.
			if strings.Contains(c.Body, msg) {
				dup = true
				return errStopIteration
			}
			return nil
		})
		if dup {
			// Comment's already been posted. Nothing to do.
			return nil
		}
	}
	// See if there is a dup comment from when gopherbot last got
	// its data from maintner.
	ics, _, err := b.ghc.Issues.ListComments(ctx, org, repo, int(issueNum), &github.IssueListCommentsOptions{
		Since:       since,
		ListOptions: github.ListOptions{PerPage: 1000},
	})
	if err != nil {
		return err
	}
	for _, ic := range ics {
		if strings.Contains(ic.GetBody(), msg) {
			// Dup.
			return nil
		}
	}
	if *dryRun {
		log.Printf("[dry-run] would add comment to github.com/%s/%s/issues/%d: %v", org, repo, issueNum, msg)
		return nil
	}
	_, _, err = b.ghc.Issues.CreateComment(ctx, org, repo, int(issueNum), &github.IssueComment{
		Body: github.String(msg),
	})
	return err
}

// createGitHubIssue returns the number of the created issue, or 4242 in dry-run mode.
// baseEvent is the timestamp of the event causing this action, and is used for de-duplication.
func (b *gopherbot) createGitHubIssue(ctx context.Context, title, msg string, labels []string, baseEvent time.Time) (int, error) {
	var dup int
	b.gorepo.ForeachIssue(func(gi *maintner.GitHubIssue) error {
		// TODO: check for gopherbot as author? check for exact match?
		// This seems fine for now.
		if gi.Title == title {
			dup = int(gi.Number)
			return errStopIteration
		}
		return nil
	})
	if dup != 0 {
		// Issue's already been posted. Nothing to do.
		return dup, nil
	}
	// See if there is a dup issue from when gopherbot last got its data from maintner.
	is, _, err := b.ghc.Issues.ListByRepo(ctx, "golang", "go", &github.IssueListByRepoOptions{
		State:       "all",
		ListOptions: github.ListOptions{PerPage: 100},
		Since:       baseEvent,
	})
	if err != nil {
		return 0, err
	}
	for _, i := range is {
		if i.GetTitle() == title {
			// Dup.
			return i.GetNumber(), nil
		}
	}
	if *dryRun {
		log.Printf("[dry-run] would create issue with title %s and labels %v\n%s", title, labels, msg)
		return 4242, nil
	}
	i, _, err := b.ghc.Issues.Create(ctx, "golang", "go", &github.IssueRequest{
		Title:  github.String(title),
		Body:   github.String(msg),
		Labels: &labels,
	})
	return i.GetNumber(), err
}

func (b *gopherbot) closeGitHubIssue(ctx context.Context, number int32) error {
	if *dryRun {
		log.Printf("[dry-run] would close golang.org/issue/%v", number)
		return nil
	}
	_, _, err := b.ghc.Issues.Edit(ctx, "golang", "go", int(number), &github.IssueRequest{State: github.String("closed")})
	return err
}

type gerritCommentOpts struct {
	OldPhrases []string
	Version    string // if empty, latest version is used
}

var emptyGerritCommentOpts gerritCommentOpts

// addGerritComment adds the given comment to the CL specified by the changeID
// and the patch set identified by the version.
//
// As an idempotence check, before adding the comment the comment and the list
// of oldPhrases are checked against the CL to ensure that no phrase in the list
// has already been added to the list as a comment.
func (b *gopherbot) addGerritComment(ctx context.Context, changeID, comment string, opts *gerritCommentOpts) error {
	if b == nil {
		panic("nil gopherbot")
	}
	if *dryRun {
		log.Printf("[dry-run] would add comment to golang.org/cl/%s: %v", changeID, comment)
		return nil
	}
	if opts == nil {
		opts = &emptyGerritCommentOpts
	}
	// One final staleness check before sending a message: get the list
	// of comments from the API and check whether any of them match.
	info, err := b.gerrit.GetChange(ctx, changeID, gerrit.QueryChangesOpt{
		Fields: []string{"MESSAGES", "CURRENT_REVISION"},
	})
	if err != nil {
		return err
	}
	for _, msg := range info.Messages {
		if strings.Contains(msg.Message, comment) {
			return nil // Our comment is already there
		}
		for j := range opts.OldPhrases {
			// Message looks something like "Patch set X:\n\n(our text)"
			if strings.Contains(msg.Message, opts.OldPhrases[j]) {
				return nil // Our comment is already there
			}
		}
	}
	var rev string
	if opts.Version != "" {
		rev = opts.Version
	} else {
		rev = info.CurrentRevision
	}
	return b.gerrit.SetReview(ctx, changeID, rev, gerrit.ReviewInput{
		Message: comment,
	})
}

// Move any issue to "Unplanned" if it looks like it keeps getting kicked along between releases.
func (b *gopherbot) getOffKickTrain(ctx context.Context) error {
	// We only run this task if it was explicitly requested via
	// the --only-run flag.
	if *onlyRun == "" {
		return nil
	}
	type match struct {
		url   string
		title string
		gi    *maintner.GitHubIssue
	}
	var matches []match
	b.gorepo.ForeachIssue(func(gi *maintner.GitHubIssue) error {
		if gi.PullRequest || gi.Closed || gi.NotExist {
			return nil
		}
		curMilestone := gi.Milestone.Title
		if !strings.HasPrefix(curMilestone, "Go1.") || strings.Count(curMilestone, ".") != 1 {
			return nil
		}
		if gi.HasLabel("release-blocker") || gi.HasLabel("Security") {
			return nil
		}
		if len(gi.Assignees) > 0 {
			return nil
		}
		was := map[string]bool{}
		gi.ForeachEvent(func(e *maintner.GitHubIssueEvent) error {
			if e.Type == "milestoned" {
				switch e.Milestone {
				case "Unreleased", "Unplanned", "Proposal":
					return nil
				}
				if strings.Count(e.Milestone, ".") > 1 {
					return nil
				}
				ms := strings.TrimSuffix(e.Milestone, "Maybe")
				ms = strings.TrimSuffix(ms, "Early")
				was[ms] = true
			}
			return nil
		})
		if len(was) > 2 {
			var mss []string
			for ms := range was {
				mss = append(mss, ms)
			}
			sort.Slice(mss, func(i, j int) bool {
				if len(mss[i]) == len(mss[j]) {
					return mss[i] < mss[j]
				}
				return len(mss[i]) < len(mss[j])
			})
			matches = append(matches, match{
				url:   fmt.Sprintf("https://golang.org/issue/%d", gi.Number),
				title: fmt.Sprintf("%s - %v", gi.Title, mss),
				gi:    gi,
			})
		}
		return nil
	})
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].title < matches[j].title
	})
	fmt.Printf("%d issues:\n", len(matches))
	for _, m := range matches {
		fmt.Printf("%-30s - %s\n", m.url, m.title)
		if !*dryRun {
			if err := b.setMilestone(ctx, m.gi, unplanned); err != nil {
				return err
			}
		}
	}
	return nil
}

// unwaitRelease changes any Gerrit CL with hashtag "wait-release"
// into "ex-wait-release". This is run manually (with --only-run)
// at the opening of a release cycle.
func (b *gopherbot) unwaitRelease(ctx context.Context) error {
	// We only run this task if it was explicitly requested via
	// the --only-run flag.
	if *onlyRun == "" {
		return nil
	}
	cis, err := b.gerrit.QueryChanges(ctx, "hashtag:wait-release status:open")
	if err != nil {
		return nil
	}
	for _, ci := range cis {
		if *dryRun {
			log.Printf("[dry run] would remove hashtag 'wait-release' from CL %d", ci.ChangeNumber)
			continue
		}
		_, err := b.gerrit.SetHashtags(ctx, ci.ID, gerrit.HashtagsInput{
			Add:    []string{"ex-wait-release"},
			Remove: []string{"wait-release"},
		})
		if err != nil {
			log.Printf("https://golang.org/cl/%d: modifying hash tags: %v", ci.ChangeNumber, err)
			return err
		}
		log.Printf("https://golang.org/cl/%d: removed wait-release", ci.ChangeNumber)
	}
	return nil
}

// freezeOldIssues locks any issue that's old and closed.
// (Otherwise people find ancient bugs via searches and start asking questions
// into a void and it's sad for everybody.)
// This method doesn't need to explicitly avoid edit wars with humans because
// it bails out if the issue was edited recently. A human unlocking an issue
// causes the updated time to bump, which means the bot wouldn't try to lock it
// again for another year.
func (b *gopherbot) freezeOldIssues(ctx context.Context) error {
	tooOld := time.Now().Add(-365 * 24 * time.Hour)
	return b.gorepo.ForeachIssue(func(gi *maintner.GitHubIssue) error {
		if !gi.Closed || gi.PullRequest || gi.Locked {
			return nil
		}
		if gi.Updated.After(tooOld) {
			return nil
		}
		printIssue("freeze", gi)
		if *dryRun {
			return nil
		}
		_, err := b.ghc.Issues.Lock(ctx, "golang", "go", int(gi.Number), nil)
		if err != nil {
			return err
		}
		return b.addLabel(ctx, gi, frozenDueToAge)
	})
}

// labelProposals adds the "Proposal" label and "Proposal" milestone
// to open issues with title beginning with "Proposal:". It tries not
// to get into an edit war with a human.
func (b *gopherbot) labelProposals(ctx context.Context) error {
	return b.gorepo.ForeachIssue(func(gi *maintner.GitHubIssue) error {
		if gi.Closed || gi.PullRequest {
			return nil
		}
		if !strings.HasPrefix(gi.Title, "proposal:") && !strings.HasPrefix(gi.Title, "Proposal:") {
			return nil
		}
		// Add Milestone if missing:
		if gi.Milestone.IsNone() && !gi.HasEvent("milestoned") && !gi.HasEvent("demilestoned") {
			if err := b.setMilestone(ctx, gi, proposal); err != nil {
				return err
			}
		}
		// Add Proposal label if missing:
		if !gi.HasLabel("Proposal") && !gi.HasEvent("unlabeled") {
			if err := b.addLabel(ctx, gi, "Proposal"); err != nil {
				return err
			}
		}
		return nil
	})
}

func (b *gopherbot) setSubrepoMilestones(ctx context.Context) error {
	return b.gorepo.ForeachIssue(func(gi *maintner.GitHubIssue) error {
		if gi.Closed || gi.PullRequest || !gi.Milestone.IsNone() || gi.HasEvent("demilestoned") || gi.HasEvent("milestoned") {
			return nil
		}
		if !strings.HasPrefix(gi.Title, "x/") {
			return nil
		}
		pkg := gi.Title
		if colon := strings.IndexByte(pkg, ':'); colon >= 0 {
			pkg = pkg[:colon]
		}
		if sp := strings.IndexByte(pkg, ' '); sp >= 0 {
			pkg = pkg[:sp]
		}
		switch pkg {
		case "",
			"x/arch",
			"x/crypto/chacha20poly1305",
			"x/crypto/curve25519",
			"x/crypto/poly1305",
			"x/net/http2",
			"x/net/idna",
			"x/net/lif",
			"x/net/proxy",
			"x/net/route",
			"x/text/unicode/norm",
			"x/text/width":
			// These get vendored in. Don't mess with them.
			return nil
		case "x/vgo":
			// Handled by setMiscMilestones
			return nil
		}
		return b.setMilestone(ctx, gi, unreleased)
	})
}

func (b *gopherbot) setMiscMilestones(ctx context.Context) error {
	return b.gorepo.ForeachIssue(func(gi *maintner.GitHubIssue) error {
		if gi.Closed || gi.PullRequest || !gi.Milestone.IsNone() || gi.HasEvent("demilestoned") || gi.HasEvent("milestoned") {
			return nil
		}
		if strings.Contains(gi.Title, "gccgo") { // TODO: better gccgo bug report heuristic?
			return b.setMilestone(ctx, gi, gccgo)
		}
		if strings.HasPrefix(gi.Title, "x/vgo") {
			return b.setMilestone(ctx, gi, vgo)
		}
		return nil
	})
}

func (b *gopherbot) labelBuildIssues(ctx context.Context) error {
	return b.gorepo.ForeachIssue(func(gi *maintner.GitHubIssue) error {
		if gi.Closed || gi.PullRequest || !strings.HasPrefix(gi.Title, "x/build") || gi.HasLabel("Builders") || gi.HasEvent("unlabeled") {
			return nil
		}
		return b.addLabel(ctx, gi, "Builders")
	})
}

func (b *gopherbot) labelMobileIssues(ctx context.Context) error {
	return b.gorepo.ForeachIssue(func(gi *maintner.GitHubIssue) error {
		if gi.Closed || gi.PullRequest || !strings.HasPrefix(gi.Title, "x/mobile") || gi.HasLabel("mobile") || gi.HasEvent("unlabeled") {
			return nil
		}
		return b.addLabel(ctx, gi, "mobile")
	})
}

func (b *gopherbot) labelDocumentationIssues(ctx context.Context) error {
	return b.gorepo.ForeachIssue(func(gi *maintner.GitHubIssue) error {
		if gi.Closed || gi.PullRequest || !isDocumentationTitle(gi.Title) || gi.HasLabel("Documentation") || gi.HasEvent("unlabeled") {
			return nil
		}
		return b.addLabel(ctx, gi, "Documentation")
	})
}

func (b *gopherbot) closeStaleWaitingForInfo(ctx context.Context) error {
	const waitingForInfo = "WaitingForInfo"
	now := time.Now()
	return b.gorepo.ForeachIssue(func(gi *maintner.GitHubIssue) error {
		if gi.Closed || gi.PullRequest || !gi.HasLabel("WaitingForInfo") {
			return nil
		}
		var waitStart time.Time
		gi.ForeachEvent(func(e *maintner.GitHubIssueEvent) error {
			if e.Type == "reopened" {
				// Ignore any previous WaitingForInfo label if it's reopend.
				waitStart = time.Time{}
				return nil
			}
			if e.Label == waitingForInfo {
				switch e.Type {
				case "unlabeled":
					waitStart = time.Time{}
				case "labeled":
					waitStart = e.Created
				}
				return nil
			}
			return nil
		})
		if waitStart.IsZero() {
			return nil
		}

		deadline := waitStart.AddDate(0, 1, 0) // 1 month
		if now.Before(deadline) {
			return nil
		}

		var lastOPComment time.Time
		gi.ForeachComment(func(c *maintner.GitHubComment) error {
			if c.User.ID == gi.User.ID {
				lastOPComment = c.Created
			}
			return nil
		})
		if lastOPComment.After(waitStart) {
			return nil
		}

		printIssue("close-stale-waiting-for-info", gi)
		// TODO: write a task that reopens issues if the OP speaks up.
		if err := b.addGitHubComment(ctx, "golang", "go", gi.Number,
			"Timed out in state WaitingForInfo. Closing.\n\n(I am just a bot, though. Please speak up if this is a mistake or you have the requested information.)"); err != nil {
			return err
		}
		return b.closeGitHubIssue(ctx, gi.Number)
	})

}

// cl2issue writes "Change https://golang.org/issue/NNNN mentions this issue"\
// and the change summary on GitHub when a new Gerrit change references a GitHub issue.
func (b *gopherbot) cl2issue(ctx context.Context) error {
	monthAgo := time.Now().Add(-30 * 24 * time.Hour)
	return b.corpus.Gerrit().ForeachProjectUnsorted(func(gp *maintner.GerritProject) error {
		if gp.Server() != "go.googlesource.com" {
			return nil
		}
		return gp.ForeachCLUnsorted(func(cl *maintner.GerritCL) error {
			if cl.Meta.Commit.AuthorTime.Before(monthAgo) {
				// If the CL was last updated over a
				// month ago, assume (as an
				// optimization) that gopherbot
				// already processed this issue.
				return nil
			}
			for _, ref := range cl.GitHubIssueRefs {
				if id := ref.Repo.ID(); id.Owner != "golang" || id.Repo != "go" {
					continue
				}
				gi := ref.Repo.Issue(ref.Number)
				if gi == nil || gi.PullRequest || gi.HasLabel(frozenDueToAge) {
					continue
				}
				hasComment := false
				substr := fmt.Sprintf("%d mentions this issue", cl.Number)
				gi.ForeachComment(func(c *maintner.GitHubComment) error {
					if strings.Contains(c.Body, substr) {
						hasComment = true
						return errStopIteration
					}
					return nil
				})
				if !hasComment {
					printIssue("cl2issue", gi)
					msg := fmt.Sprintf("Change https://golang.org/cl/%d mentions this issue: `%s`", cl.Number, cl.Commit.Summary())
					if err := b.addGitHubComment(ctx, "golang", "go", gi.Number, msg); err != nil {
						return err
					}
				}
			}
			return nil
		})
	})
}

// canonicalLabelName returns "needsfix" for "needs-fix" or "NeedsFix"
// in prep for future label renaming.
func canonicalLabelName(s string) string {
	return strings.Replace(strings.ToLower(s), "-", "", -1)
}

// If an issue has multiple "needs" labels, remove all but the most recent.
// These were originally called NeedsFix, NeedsDecision, and NeedsInvestigation,
// but are being renamed to "needs-foo".
func (b *gopherbot) updateNeeds(ctx context.Context) error {
	return b.gorepo.ForeachIssue(func(gi *maintner.GitHubIssue) error {
		if gi.Closed || gi.PullRequest {
			return nil
		}
		var numNeeds int
		if gi.Labels[needsDecisionID] != nil {
			numNeeds++
		}
		if gi.Labels[needsFixID] != nil {
			numNeeds++
		}
		if gi.Labels[needsInvestigationID] != nil {
			numNeeds++
		}
		if numNeeds <= 1 {
			return nil
		}

		labels := map[string]int{} // lowercase no-hyphen "needsfix" -> position
		var pos, maxPos int
		gi.ForeachEvent(func(e *maintner.GitHubIssueEvent) error {
			var add bool
			switch e.Type {
			case "labeled":
				add = true
			case "unlabeled":
			default:
				return nil
			}
			if !strings.HasPrefix(e.Label, "Needs") && !strings.HasPrefix(e.Label, "needs-") {
				return nil
			}
			key := canonicalLabelName(e.Label)
			pos++
			if add {
				labels[key] = pos
				maxPos = pos
			} else {
				delete(labels, key)
			}
			return nil
		})
		if len(labels) <= 1 {
			return nil
		}

		// Remove any label that's not the newest (added in
		// last position).
		for _, lab := range gi.Labels {
			key := canonicalLabelName(lab.Name)
			if !strings.HasPrefix(key, "needs") || labels[key] == maxPos {
				continue
			}
			printIssue("updateneeds", gi)
			fmt.Printf("\t... removing label %q\n", lab.Name)
			if err := b.removeLabel(ctx, gi, lab.Name); err != nil {
				return err
			}
		}
		return nil
	})
}

// If any of the messages in this array have been posted on a CL, don't post
// again. If you amend the message even slightly, please prepend the new message
// to this list, to avoid re-spamming people.
//
// The first message is the "current" message.
var congratulatoryMessages = []string{
	// TODO: provide more helpful info? Amend, don't add 2nd commit, link to a
	// review guide?
	//
	// also TODO: make this a template? May want to provide more dynamic
	// information in the future. Would make it tougher to search and see if
	// a comment has been previously posted.
	`Congratulations on opening your first change. Thank you for your contribution!

Next steps:
Within the next week or so, a maintainer will review your change and provide
feedback. See https://golang.org/doc/contribute.html#review for more info and
tips to get your patch through code review.

Most changes in the Go project go through a few rounds of revision. This can be
surprising to people new to the project. The careful, iterative review process
is our way of helping mentor contributors and ensuring that their contributions
have a lasting impact.

During May-July and Nov-Jan the Go project is in a code freeze, during which
little code gets reviewed or merged. If a reviewer responds with a comment like
R=go1.11, it means that this CL will be reviewed as part of the next development
cycle. See https://golang.org/s/release for more details.`, // TODO only show freeze message during freeze
	"It's your first ever CL! Congrats, and thanks for sending!",
}

// Don't want to congratulate people on CL's they submitted a year ago.
var congratsEpoch = time.Date(2017, 6, 17, 0, 0, 0, 0, time.UTC)

func (b *gopherbot) congratulateNewContributors(ctx context.Context) error {
	cls := make(map[string]*maintner.GerritCL)
	b.corpus.Gerrit().ForeachProjectUnsorted(func(gp *maintner.GerritProject) error {
		if gp.Server() != "go.googlesource.com" {
			return nil
		}
		return gp.ForeachCLUnsorted(func(cl *maintner.GerritCL) error {
			// CLs can be returned by maintner in any order. Note also that
			// Gerrit CL numbers are sparse (CL N does not guarantee that CL N-1
			// exists) and Gerrit issues CL's out of order - it may issue CL N,
			// then CL (N - 18), then CL (N - 40).
			if b.knownContributors == nil {
				b.knownContributors = make(map[string]bool)
			}
			if cl.Commit == nil {
				return nil
			}
			email := cl.Commit.Author.Email()
			if email == "" {
				email = cl.Commit.Author.Str
			}
			if b.knownContributors[email] {
				return nil
			}
			if cls[email] != nil {
				// this person has multiple CLs; not a new contributor.
				b.knownContributors[email] = true
				delete(cls, email)
				return nil
			}
			cls[email] = cl
			return nil
		})
	})
	for email, cl := range cls {
		// See golang.org/issue/23865
		if cl.Branch() == "refs/meta/config" {
			b.knownContributors[email] = true
			continue
		}
		if cl.Commit == nil || cl.Commit.CommitTime.Before(congratsEpoch) {
			b.knownContributors[email] = true
			continue
		}
		if cl.Status == "merged" {
			b.knownContributors[email] = true
			continue
		}
		foundMessage := false
		for i := range cl.Messages {
			// TODO: once gopherbot starts posting these messages and we
			// have the author's name for Gopherbot, check the author name
			// matches as well.
			for j := range congratulatoryMessages {
				// Message looks something like "Patch set X:\n\n(our text)"
				if strings.Contains(cl.Messages[i].Message, congratulatoryMessages[j]) {
					foundMessage = true
					break
				}
			}
			if foundMessage {
				break
			}
		}

		if foundMessage {
			b.knownContributors[email] = true
			continue
		}
		opts := &gerritCommentOpts{
			OldPhrases: congratulatoryMessages,
		}
		err := b.addGerritComment(ctx, cl.ChangeID(), congratulatoryMessages[0], opts)
		if err != nil {
			return fmt.Errorf("could not add comment to golang.org/cl/%d: %v", cl.Number, err)
		}
		b.knownContributors[email] = true
	}
	return nil
}

// unwaitCLs removes wait-* hashtags from CLs.
func (b *gopherbot) unwaitCLs(ctx context.Context) error {
	return b.corpus.Gerrit().ForeachProjectUnsorted(func(gp *maintner.GerritProject) error {
		if gp.Server() != "go.googlesource.com" {
			return nil
		}
		return gp.ForeachOpenCL(func(cl *maintner.GerritCL) error {
			tags := cl.Meta.Hashtags()
			if tags.Len() == 0 {
				return nil
			}
			// If the CL is tagged "wait-author", remove
			// that tag if the author has replied since
			// the last time the "wait-author" tag was
			// added.
			if tags.Contains("wait-author") {
				// Figure out othe last index at which "wait-author" was added.
				waitAuthorIndex := -1
				for i := len(cl.Metas) - 1; i >= 0; i-- {
					if cl.Metas[i].HashtagsAdded().Contains("wait-author") {
						waitAuthorIndex = i
						break
					}
				}

				// Find the author has replied since
				author := cl.Metas[0].Commit.Author.Str
				hasReplied := false
				for _, m := range cl.Metas[waitAuthorIndex+1:] {
					if m.Commit.Author.Str == author {
						hasReplied = true
						break
					}
				}
				if hasReplied {
					log.Printf("https://golang.org/cl/%d -- remove wait-author; reply from %s", cl.Number, author)
					err := b.onLatestCL(ctx, cl, func() error {
						if *dryRun {
							log.Printf("[dry run] would remove hashtag 'wait-author' from CL %d", cl.Number)
							return nil
						}
						_, err := b.gerrit.RemoveHashtags(ctx, fmt.Sprint(cl.Number), "wait-author")
						if err != nil {
							log.Printf("https://golang.org/cl/%d: error removing wait-author: %v", cl.Number, err)
							return err
						}
						log.Printf("https://golang.org/cl/%d: removed wait-author", cl.Number)
						return nil
					})
					if err != nil {
						return err
					}
				}
			}
			return nil
		})
	})
}

// onLatestCL checks whether cl's metadata is in sync with Gerrit's
// upstream data and, if so, returns f(). If it's out of sync, it does
// nothing more and returns nil.
func (b *gopherbot) onLatestCL(ctx context.Context, cl *maintner.GerritCL, f func() error) error {
	ci, err := b.gerrit.GetChangeDetail(ctx, fmt.Sprint(cl.Number), gerrit.QueryChangesOpt{Fields: []string{"MESSAGES"}})
	if err != nil {
		return err
	}
	if len(ci.Messages) == 0 {
		log.Printf("onLatestCL: CL %d has no messages. Odd. Ignoring.", cl.Number)
		return nil
	}
	if ci.Messages[len(ci.Messages)-1].ID == cl.Meta.Commit.Hash.String() {
		return f()
	}
	log.Printf("onLatestCL: maintner metadata for CL %d is behind; skipping action for now.", cl.Number)
	return nil
}

// getMajorReleases returns the two most recent major Go 1.x releases, and
// the next upcoming release, sorted and formatted like []string{"1.9", "1.10", "1.11"}.
func (b *gopherbot) getMajorReleases(ctx context.Context) ([]string, error) {
	b.releases.Lock()
	defer b.releases.Unlock()

	if expiry := b.releases.lastUpdate.Add(time.Hour); time.Now().Before(expiry) {
		return b.releases.major, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	resp, err := b.mc.ListGoReleases(ctx, &apipb.ListGoReleasesRequest{})
	if err != nil {
		return nil, err
	}
	rs := resp.Releases // Supported Go releases, sorted with latest first.

	var majorReleases []string
	for i := len(rs) - 1; i >= 0; i-- {
		x, y := rs[i].Major, rs[i].Minor
		majorReleases = append(majorReleases, fmt.Sprintf("%d.%d", x, y))
	}
	// Include the next release in the list of major releases.
	if len(rs) > 0 {
		// Assume the next release will bump the minor version.
		nextX, nextY := rs[0].Major, rs[0].Minor+1
		majorReleases = append(majorReleases, fmt.Sprintf("%d.%d", nextX, nextY))
	}

	b.releases.major = majorReleases
	b.releases.lastUpdate = time.Now()

	return majorReleases, nil
}

// openCherryPickIssues opens CherryPickCandidate issues for backport when
// asked on the main issue.
func (b *gopherbot) openCherryPickIssues(ctx context.Context) error {
	return b.gorepo.ForeachIssue(func(gi *maintner.GitHubIssue) error {
		if gi.HasLabel("CherryPickApproved") && gi.HasLabel("CherryPickCandidate") {
			if err := b.removeLabel(ctx, gi, "CherryPickCandidate"); err != nil {
				return err
			}
		}
		if gi.HasLabel(frozenDueToAge) || gi.PullRequest {
			return nil
		}
		var backportComment *maintner.GitHubComment
		if err := gi.ForeachComment(func(c *maintner.GitHubComment) error {
			if strings.HasPrefix(c.Body, "Backport issue(s) opened") {
				backportComment = nil
				return errStopIteration
			}
			body := strings.ToLower(c.Body)
			if strings.Contains(body, "@gopherbot") &&
				strings.Contains(body, "please") &&
				strings.Contains(body, "backport") {
				backportComment = c
			}
			return nil
		}); err != nil && err != errStopIteration {
			return err
		}
		if backportComment == nil {
			return nil
		}
		majorReleases, err := b.getMajorReleases(ctx)
		if err != nil {
			return err
		}
		var selectedReleases []string
		for _, r := range majorReleases {
			if strings.Contains(backportComment.Body, r) {
				selectedReleases = append(selectedReleases, r)
			}
		}
		if len(selectedReleases) == 0 {
			// Only backport to major releases unless explicitly
			// asked to backport to the upcoming release.
			selectedReleases = majorReleases[:len(majorReleases)-1]
		}
		var openedIssues []string
		for _, rel := range selectedReleases {
			printIssue("open-backport-issue-"+rel, gi)
			id, err := b.createGitHubIssue(ctx,
				fmt.Sprintf("%s [%s backport]", gi.Title, rel),
				fmt.Sprintf("@%s requested issue #%d to be considered for backport to the next %s minor release.\n\n%s\n",
					backportComment.User.Login, gi.Number, rel, blockqoute(backportComment.Body)),
				[]string{"CherryPickCandidate"}, backportComment.Created)
			if err != nil {
				return err
			}
			openedIssues = append(openedIssues, fmt.Sprintf("#%d (for %s)", id, rel))
		}
		return b.addGitHubComment(ctx, "golang", "go", gi.Number, fmt.Sprintf("Backport issue(s) opened: %s.\n\nRemember to create the cherry-pick CL(s) as soon as the patch is submitted to master, according to https://golang.org/wiki/MinorReleases.", strings.Join(openedIssues, ", ")))
	})
}

// setMinorMilestones applies the latest minor release milestone to issue
// with [1.X backport] in the title.
func (b *gopherbot) setMinorMilestones(ctx context.Context) error {
	majorReleases, err := b.getMajorReleases(ctx)
	if err != nil {
		return err
	}
	return b.gorepo.ForeachIssue(func(gi *maintner.GitHubIssue) error {
		if gi.Closed || gi.PullRequest || !gi.Milestone.IsNone() || gi.HasEvent("demilestoned") || gi.HasEvent("milestoned") {
			return nil
		}
		var majorRel string
		for _, r := range majorReleases {
			if strings.Contains(gi.Title, "backport") && strings.HasSuffix(gi.Title, "["+r+" backport]") {
				majorRel = r
			}
		}
		if majorRel == "" {
			return nil
		}
		m, err := b.getMinorMilestoneForMajor(ctx, majorRel)
		if err != nil {
			// Fail silently, the milestone might not exist yet.
			log.Printf("Failed to apply minor release milestone to issue %d: %v", gi.Number, err)
			return nil
		}
		return b.setMilestone(ctx, gi, m)
	})
}

// getMinorMilestoneForMajor returns the latest open minor release milestone
// for a given major release series.
func (b *gopherbot) getMinorMilestoneForMajor(ctx context.Context, majorRel string) (milestone, error) {
	var res milestone
	var minorVers int
	titlePrefix := "Go" + majorRel + "."
	if err := b.gorepo.ForeachMilestone(func(m *maintner.GitHubMilestone) error {
		if m.Closed {
			return nil
		}
		if !strings.HasPrefix(m.Title, titlePrefix) {
			return nil
		}
		n, err := strconv.Atoi(strings.TrimPrefix(m.Title, titlePrefix))
		if err != nil {
			return nil
		}
		if n > minorVers {
			res = milestone{
				Number: int(m.Number),
				Name:   m.Title,
			}
			minorVers = n
		}
		return nil
	}); err != nil {
		return milestone{}, err
	}
	if minorVers == 0 {
		return milestone{}, errors.New("no minor milestone found for release series " + majorRel)
	}
	return res, nil
}

// closeCherryPickIssues closes cherry-pick issues when CLs are merged to
// release branches, as GitHub only does that on merge to master.
func (b *gopherbot) closeCherryPickIssues(ctx context.Context) error {
	openCherryPickIssues := make(map[int32]*maintner.GitHubIssue) // by GitHub Issue Number
	b.gorepo.ForeachIssue(func(gi *maintner.GitHubIssue) error {
		if gi.Closed || gi.PullRequest || gi.NotExist || gi.Milestone.IsNone() {
			return nil
		}
		if strings.Count(gi.Milestone.Title, ".") == 2 { // minor release
			openCherryPickIssues[gi.Number] = gi
		}
		return nil
	})
	monthAgo := time.Now().Add(-30 * 24 * time.Hour)
	return b.corpus.Gerrit().ForeachProjectUnsorted(func(gp *maintner.GerritProject) error {
		if gp.Server() != "go.googlesource.com" {
			return nil
		}
		return gp.ForeachCLUnsorted(func(cl *maintner.GerritCL) error {
			if cl.Commit.CommitTime.Before(monthAgo) {
				// If the CL was last updated over a month ago, assume (as an
				// optimization) that gopherbot already processed this CL.
				return nil
			}
			if cl.Status != "merged" || cl.Private || !strings.HasPrefix(cl.Branch(), "release-branch") {
				return nil
			}
			for _, ref := range cl.GitHubIssueRefs {
				if id := ref.Repo.ID(); id.Owner != "golang" || id.Repo != "go" {
					continue
				}
				gi, ok := openCherryPickIssues[ref.Number]
				if !ok {
					continue
				}
				printIssue("close-cherry-pick", gi)
				if err := b.addGitHubComment(ctx, "golang", "go", gi.Number, fmt.Sprintf(
					"Closed by merging %s to %s.", cl.Commit.Hash, cl.Branch())); err != nil {
					return err
				}
				return b.closeGitHubIssue(ctx, gi.Number)
			}
			return nil
		})
	})
}

type labelCommand struct {
	action  string    // "add" or "remove"
	label   string    // the label name
	created time.Time // creation time of the comment containing the command
	noop    bool      // whether to apply the command or not
}

// applyLabelsFromComments looks within open GitHub issues for commands to add or
// remove labels. Anyone can use the /label <label> or /unlabel <label> commands.
func (b *gopherbot) applyLabelsFromComments(ctx context.Context) error {
	allLabels := make(map[string]string) // lowercase label name -> proper casing
	b.gorepo.ForeachLabel(func(gl *maintner.GitHubLabel) error {
		allLabels[strings.ToLower(gl.Name)] = gl.Name
		return nil
	})

	return b.gorepo.ForeachIssue(func(gi *maintner.GitHubIssue) error {
		if gi.Closed {
			return nil
		}

		var cmds []labelCommand

		cmds = append(cmds, labelCommandsFromBody(gi.Body, gi.Created)...)
		gi.ForeachComment(func(gc *maintner.GitHubComment) error {
			cmds = append(cmds, labelCommandsFromBody(gc.Body, gc.Created)...)
			return nil
		})

		for i, c := range cmds {
			// Does the label even exist? If so, use the proper capitalization.
			// If it doesn't exist, the command is a no-op.
			if l, ok := allLabels[c.label]; ok {
				cmds[i].label = l
			} else {
				cmds[i].noop = true
				continue
			}

			// If any action has been taken on the label since the comment containing
			// the command to add or remove it, then it should be a no-op.
			gi.ForeachEvent(func(ge *maintner.GitHubIssueEvent) error {
				if (ge.Type == "unlabeled" || ge.Type == "labeled") &&
					strings.ToLower(ge.Label) == c.label &&
					ge.Created.After(c.created) {
					cmds[i].noop = true
					return errStopIteration
				}
				return nil
			})
		}

		toAdd, toRemove := mutationsFromCommands(cmds)
		if err := b.addLabels(ctx, gi, toAdd); err != nil {
			log.Printf("Unable to add labels (%v) to issue %d: %v", toAdd, gi.Number, err)
		}
		if err := b.removeLabels(ctx, gi, toRemove); err != nil {
			log.Printf("Unable to remove labels (%v) from issue %d: %v", toRemove, gi.Number, err)
		}

		return nil
	})
}

// labelCommandsFromBody returns a slice of commands inferred by the given body text.
// The format of commands is:
// @gopherbot[,] [please] [add|remove] <label>[{,|;} label... and remove <label>...]
// Omission of add or remove will default to adding a label.
func labelCommandsFromBody(body string, created time.Time) []labelCommand {
	if !strutil.ContainsFold(body, "@gopherbot") {
		return nil
	}
	var cmds []labelCommand
	lines := strings.Split(body, "\n")
	for _, l := range lines {
		if !strutil.ContainsFold(l, "@gopherbot") {
			continue
		}
		l = strings.ToLower(l)
		scanner := bufio.NewScanner(strings.NewReader(l))
		scanner.Split(bufio.ScanWords)
		var (
			add      strings.Builder
			remove   strings.Builder
			inRemove bool
		)
		for scanner.Scan() {
			switch scanner.Text() {
			case "@gopherbot", "@gopherbot,", "@gopherbot:", "please", "and", "label", "labels":
				continue
			case "add":
				inRemove = false
				continue
			case "remove", "unlabel":
				inRemove = true
				continue
			}

			if inRemove {
				remove.WriteString(scanner.Text())
				remove.WriteString(" ") // preserve whitespace within labels
			} else {
				add.WriteString(scanner.Text())
				add.WriteString(" ") // preserve whitespace within labels
			}
		}
		if add.Len() > 0 {
			cmds = append(cmds, labelCommands(add.String(), "add", created)...)
		}
		if remove.Len() > 0 {
			cmds = append(cmds, labelCommands(remove.String(), "remove", created)...)
		}
	}
	return cmds
}

// labelCommands returns a slice of commands for the given action and string of
// text following commands like @gopherbot add/remove.
func labelCommands(s, action string, created time.Time) []labelCommand {
	var cmds []labelCommand
	f := func(c rune) bool {
		return c != '-' && !unicode.IsLetter(c) && !unicode.IsNumber(c) && !unicode.IsSpace(c)
	}
	for _, label := range strings.FieldsFunc(s, f) {
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}
		cmds = append(cmds, labelCommand{action: action, label: label, created: created})
	}
	return cmds
}

// mutationsFromCommands returns two sets of labels to add and remove based on
// the given cmds.
func mutationsFromCommands(cmds []labelCommand) (add, remove []string) {
	// Split the labels into what to add and what to remove.
	// Account for two opposing commands that have yet to be applied canceling
	// each other out.
	var (
		toAdd    map[string]bool
		toRemove map[string]bool
	)
	for _, c := range cmds {
		if c.noop {
			continue
		}
		switch c.action {
		case "add":
			if toRemove[c.label] {
				delete(toRemove, c.label)
				continue
			}
			if toAdd == nil {
				toAdd = make(map[string]bool)
			}
			toAdd[c.label] = true
		case "remove":
			if toAdd[c.label] {
				delete(toAdd, c.label)
				continue
			}
			if toRemove == nil {
				toRemove = make(map[string]bool)
			}
			toRemove[c.label] = true
		default:
			log.Printf("Invalid label action type: %q", c.action)
		}
	}

	for l := range toAdd {
		if toAdd[l] && !labelChangeBlacklisted(l, "add") {
			add = append(add, l)
		}
	}

	for l := range toRemove {
		if toRemove[l] && !labelChangeBlacklisted(l, "remove") {
			remove = append(remove, l)
		}
	}
	return add, remove
}

// labelChangeBlacklisted returns true if an action on the given label is
// forbidden via gopherbot.
func labelChangeBlacklisted(label, action string) bool {
	if action == "remove" && label == "Security" {
		return true
	}
	for _, prefix := range []string{
		"CherryPick",
		"cla:",
		"Proposal-",
	} {
		if strings.HasPrefix(label, prefix) {
			return true
		}
	}
	return false
}

// assignReviewersToCLs looks for CLs with no humans in the reviewer or cc fields
// that have been open for a short amount of time (enough of a signal that the
// author does not intend to add anyone to the review), then assigns reviewers/ccs
// using the golang.org/s/owners API.
func (b *gopherbot) assignReviewersToCLs(ctx context.Context) error {
	const tagNoOwners = "no-owners"
	b.corpus.Gerrit().ForeachProjectUnsorted(func(gp *maintner.GerritProject) error {
		if gp.Project() == "scratch" || gp.Server() != "go.googlesource.com" {
			return nil
		}
		gp.ForeachOpenCL(func(cl *maintner.GerritCL) error {
			if cl.Private || cl.WorkInProgress() || time.Now().Sub(cl.Created) < 10*time.Minute {
				return nil
			}
			tags := cl.Meta.Hashtags()
			if tags.Contains(tagNoOwners) {
				return nil
			}

			gc := gerritChange{gp.Project(), cl.Number}
			if b.deletedChanges[gc] {
				return nil
			}
			if strutil.ContainsFold(cl.Commit.Msg, "do not submit") || strutil.ContainsFold(cl.Commit.Msg, "do not review") {
				return nil
			}

			if b.humanReviewersOnChange(ctx, gc, cl) {
				return nil
			}

			changeURL := fmt.Sprintf("https://go-review.googlesource.com/c/%s/+/%d", gp.Project(), cl.Number)
			log.Printf("No reviewers or cc: %s", changeURL)
			files, err := b.gerrit.ListFiles(ctx, gc.ID(), cl.Commit.Hash.String())
			if err != nil {
				log.Printf("Could not get change %+v: %v", gc, err)
				if httpErr, ok := err.(*gerrit.HTTPError); ok && httpErr.Res.StatusCode == http.StatusNotFound {
					b.deletedChanges[gc] = true
				}
				return nil
			}

			var paths []string
			for f := range files {
				if f == "/COMMIT_MSG" {
					continue
				}
				paths = append(paths, gp.Project()+"/"+f)
			}

			entries, err := getCodeOwners(ctx, paths)
			if err != nil {
				log.Printf("Could not get owners for change %s: %v", changeURL, err)
				return nil
			}

			authorEmail := cl.Commit.Author.Email()
			merged := mergeOwnersEntries(entries, authorEmail)
			if len(merged.Primary) == 0 && len(merged.Secondary) == 0 {
				// No owners found for the change. Add the #no-owners tag.
				log.Printf("Adding no-owners tag to change %s...", changeURL)
				if *dryRun {
					return nil
				}
				if _, err := b.gerrit.AddHashtags(ctx, gc.ID(), tagNoOwners); err != nil {
					log.Printf("Could not add hashtag to change %q: %v", gc.ID(), err)
					return nil
				}
				return nil
			}

			// Assign reviewers.
			var review gerrit.ReviewInput
			for _, owner := range merged.Primary {
				review.Reviewers = append(review.Reviewers, gerrit.ReviewerInput{Reviewer: owner.GerritEmail})
			}
			for _, owner := range merged.Secondary {
				review.Reviewers = append(review.Reviewers, gerrit.ReviewerInput{Reviewer: owner.GerritEmail, State: "CC"})
			}
			if *dryRun {
				log.Printf("[dry run] Would set review on %s: %+v", changeURL, review)
				return nil
			}
			log.Printf("Setting review on %s: %+v", changeURL, review)
			if err := b.gerrit.SetReview(ctx, gc.ID(), "current", review); err != nil {
				log.Printf("Could not set review for change %q: %v", gc.ID(), err)
				return nil
			}
			return nil
		})
		return nil
	})
	return nil
}

// abandonScratchReviews abandons Gerrit CLs in the "scratch" project if they've been open for over a week.
func (b *gopherbot) abandonScratchReviews(ctx context.Context) error {
	tooOld := time.Now().Add(-24 * time.Hour * 7)
	return b.corpus.Gerrit().ForeachProjectUnsorted(func(gp *maintner.GerritProject) error {
		if gp.Project() != "scratch" || gp.Server() != "go.googlesource.com" {
			return nil
		}
		return gp.ForeachOpenCL(func(cl *maintner.GerritCL) error {
			if !cl.Meta.Commit.CommitTime.Before(tooOld) {
				return nil
			}
			if *dryRun {
				log.Printf("[dry-run] would've closed scratch CL https://golang.org/cl/%d ...", cl.Number)
				return nil
			}
			log.Printf("closing scratch CL https://golang.org/cl/%d ...", cl.Number)
			err := b.gerrit.AbandonChange(ctx, fmt.Sprint(cl.Number), "Auto-abandoning old scratch review.")
			if err != nil && strings.Contains(err.Error(), "404 Not Found") {
				return nil
			}
			return err
		})
	})
}

// humanReviewersOnChange returns true if there are (or were any) human reviewers in the given change.
// The gerritChange passed must be used because it’s used as a key to deletedChanges and the ID returned
// by cl.ChangeID() can be associated with multiple changes (cherry-picks, for example).
func (b *gopherbot) humanReviewersOnChange(ctx context.Context, change gerritChange, cl *maintner.GerritCL) bool {
	if found := humanReviewersInMetas(cl.Metas); found {
		return true
	}

	reviewers, err := b.gerrit.ListReviewers(ctx, change.ID())
	if err != nil {
		if httpErr, ok := err.(*gerrit.HTTPError); ok && httpErr.Res.StatusCode == http.StatusNotFound {
			b.deletedChanges[change] = true
		}
		log.Printf("Could not list reviewers on change %q: %v", change.ID(), err)
		return true
	}

	const (
		gobotID     = 5976
		gerritbotID = 12446
	)
	for _, r := range reviewers {
		if r.NumericID != gobotID && r.NumericID != gerritbotID {
			return true
		}
	}
	return false
}

func humanReviewersInMetas(metas []*maintner.GerritMeta) bool {
	// Emails as they appear in maintner (<numeric ID>@<instance ID>)
	var (
		gobotEmail     = "5976@62eb7196-b449-3ce5-99f1-c037f21e1705"
		gerritbotEmail = "12446@62eb7196-b449-3ce5-99f1-c037f21e1705"

		hasHuman bool
	)
	for _, m := range metas {
		if !strings.Contains(m.Commit.Msg, "Reviewer:") && !strings.Contains(m.Commit.Msg, "CC:") {
			continue
		}

		err := maintner.ForeachLineStr(m.Commit.Msg, func(ln string) error {
			if !strings.HasPrefix(ln, "Reviewer:") && !strings.HasPrefix(ln, "CC:") {
				return nil
			}
			if !strings.Contains(ln, gobotEmail) && !strings.Contains(ln, gerritbotEmail) {
				// A human is already on the change.
				hasHuman = true
				return errStopIteration
			}
			return nil
		})
		if err != nil && err != errStopIteration {
			log.Printf("humanReviewersInMetas: got unexpected error from maintner.ForeachLineStr: %v", err)
			return hasHuman
		}
	}
	return hasHuman
}

func getCodeOwners(ctx context.Context, paths []string) ([]*owners.Entry, error) {
	oReq := owners.Request{Version: 1}
	oReq.Payload.Paths = paths

	b, err := json.Marshal(oReq)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", "https://dev.golang.org/owners/", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var oResp owners.Response
	if err := json.NewDecoder(resp.Body).Decode(&oResp); err != nil {
		return nil, fmt.Errorf("could not decode owners response: %v", err)
	}
	if oResp.Error != "" {
		return nil, fmt.Errorf("error from dev.golang.org/owners endpoint: %v", oResp.Error)
	}
	var entries []*owners.Entry
	for _, entry := range oResp.Payload.Entries {
		if entry == nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// mergeOwnersEntries takes multiple owners.Entry structs and aggregates all
// primary and secondary users into a single entry.
// If a user is a primary in one entry but secondary on another, they are
// primary in the returned entry.
// If a users email matches the authorEmail, the the user is omitted from the
// result.
// The resulting order of the entries is non-deterministic.
func mergeOwnersEntries(entries []*owners.Entry, authorEmail string) *owners.Entry {
	var result owners.Entry
	pm := make(map[owners.Owner]bool)
	for _, e := range entries {
		for _, o := range e.Primary {
			pm[o] = true
		}
	}
	sm := make(map[owners.Owner]bool)
	for _, e := range entries {
		for _, o := range e.Secondary {
			if !pm[o] {
				sm[o] = true
			}
		}
	}
	for o := range pm {
		if o.GerritEmail != authorEmail {
			result.Primary = append(result.Primary, o)
		}
	}
	for o := range sm {
		if o.GerritEmail != authorEmail {
			result.Secondary = append(result.Secondary, o)
		}
	}
	return &result
}

func blockqoute(s string) string {
	s = strings.TrimSpace(s)
	s = "> " + s
	s = strings.Replace(s, "\n", "\n> ", -1)
	return s
}

// errStopIteration is used to stop iteration over issues or comments.
// It has no special meaning.
var errStopIteration = errors.New("stop iteration")

func isDocumentationTitle(t string) bool {
	if !strings.Contains(t, "doc") && !strings.Contains(t, "Doc") {
		return false
	}
	t = strings.ToLower(t)
	if strings.HasPrefix(t, "doc:") {
		return true
	}
	if strings.HasPrefix(t, "docs:") {
		return true
	}
	if strings.HasPrefix(t, "cmd/doc:") {
		return false
	}
	if strings.HasPrefix(t, "go/doc:") {
		return false
	}
	if strings.Contains(t, "godoc:") { // in x/tools, or the dozen places people file it as
		return false
	}
	return strings.Contains(t, "document") ||
		strings.Contains(t, "docs ")
}

var lastTask string

func printIssue(task string, gi *maintner.GitHubIssue) {
	if *dryRun {
		task = task + " [dry-run]"
	}
	if task != lastTask {
		fmt.Println(task)
		lastTask = task
	}
	fmt.Printf("\thttps://golang.org/issue/%v  %s\n", gi.Number, gi.Title)
}
