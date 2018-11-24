// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The gerritbot binary converts GitHub Pull Requests to Gerrit Changes,
// updating the PR and Gerrit Change as appropriate.
package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/compute/metadata"
	"github.com/google/go-github/github"
	"golang.org/x/build/gerrit"
	"golang.org/x/build/internal/https"
	"golang.org/x/build/maintner"
	"golang.org/x/build/maintner/godata"
	"golang.org/x/oauth2"
)

var (
	listen          = flag.String("listen", "localhost:6343", "listen address")
	autocertBucket  = flag.String("autocert-bucket", "", "if non-empty, listen on port 443 and serve a LetsEncrypt TLS cert using this Google Cloud Storage bucket as a cache")
	workdir         = flag.String("workdir", defaultWorkdir(), "where git repos and temporary worktrees are created")
	githubTokenFile = flag.String("github-token-file", filepath.Join(defaultWorkdir(), "github-token"), "file to load GitHub token from; should only contain the token text")
	gerritTokenFile = flag.String("gerrit-token-file", filepath.Join(defaultWorkdir(), "gerrit-token"), "file to load Gerrit token from; should be of form <git-email>:<token>")
	gitcookiesFile  = flag.String("gitcookies-file", "", "if non-empty, write a git http cookiefile to this location using compute metadata")
	dryRun          = flag.Bool("dry-run", false, "print out mutating actions but don’t perform any")
)

func main() {
	flag.Parse()
	if err := writeCookiesFile(); err != nil {
		log.Fatalf("writeCookiesFile(): %v", err)
	}
	ghc, err := githubClient()
	if err != nil {
		log.Fatalf("githubClient(): %v", err)
	}
	gc, err := gerritClient()
	if err != nil {
		log.Fatalf("gerritClient(): %v", err)
	}
	b := newBot(ghc, gc)

	ctx := context.Background()
	b.initCorpus(ctx)
	go b.corpusUpdateLoop(ctx)

	https.ListenAndServe(http.HandlerFunc(handleIndex), &https.Options{
		Addr:                *listen,
		AutocertCacheBucket: *autocertBucket,
	})
}

func defaultWorkdir() string {
	// TODO(andybons): Use os.UserCacheDir (issue 22536) when it's available.
	return filepath.Join(home(), ".gerritbot")
}

func home() string {
	h := os.Getenv("HOME")
	if h != "" {
		return h
	}
	u, err := user.Current()
	if err != nil {
		log.Fatalf("user.Current(): %v", err)
	}
	return u.HomeDir
}

func writeCookiesFile() error {
	if *gitcookiesFile == "" {
		return nil
	}
	log.Printf("Writing git http cookies file %q ...", *gitcookiesFile)
	if !metadata.OnGCE() {
		return fmt.Errorf("cannot write git http cookies file %q from metadata: not on GCE", *gitcookiesFile)
	}
	k := "gerritbot-gitcookies"
	cookies, err := metadata.ProjectAttributeValue(k)
	if cookies == "" {
		return fmt.Errorf("metadata.ProjectAttribtueValue(%q) returned an empty value", k)
	}
	if err != nil {
		return fmt.Errorf("metadata.ProjectAttribtueValue(%q): %v", k, err)
	}
	return ioutil.WriteFile(*gitcookiesFile, []byte(cookies), 0600)
}

func githubClient() (*github.Client, error) {
	token, err := githubToken()
	if err != nil {
		return nil, err
	}
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)
	return github.NewClient(tc), nil
}

func githubToken() (string, error) {
	if metadata.OnGCE() {
		token, err := metadata.ProjectAttributeValue("maintner-github-token")
		if err == nil {
			return token, nil
		}
	}
	slurp, err := ioutil.ReadFile(*githubTokenFile)
	if err != nil {
		return "", err
	}
	tok := strings.TrimSpace(string(slurp))
	if len(tok) == 0 {
		return "", fmt.Errorf("token from file %q cannot be empty", *githubTokenFile)
	}
	return tok, nil
}

func gerritClient() (*gerrit.Client, error) {
	username, token, err := gerritAuth()
	if err != nil {
		return nil, err
	}
	c := gerrit.NewClient("https://go-review.googlesource.com", gerrit.BasicAuth(username, token))
	return c, nil
}

func gerritAuth() (string, string, error) {
	var slurp string
	if metadata.OnGCE() {
		var err error
		slurp, err = metadata.ProjectAttributeValue("gobot-password")
		if err != nil {
			log.Printf(`Error retrieving Project Metadata "gobot-password": %v`, err)
		}
	}
	if len(slurp) == 0 {
		slurpBytes, err := ioutil.ReadFile(*gerritTokenFile)
		if err != nil {
			return "", "", err
		}
		slurp = string(slurpBytes)
	}
	f := strings.SplitN(strings.TrimSpace(slurp), ":", 2)
	if len(f) == 1 {
		// Assume the whole thing is the token.
		return "git-gobot.golang.org", f[0], nil
	}
	if len(f) != 2 || f[0] == "" || f[1] == "" {
		return "", "", fmt.Errorf("expected Gerrit token to be of form <git-email>:<token>")
	}
	return f[0], f[1], nil
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	r.Header.Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintln(w, "Hello, GerritBot! 🤖")
}

const (
	// Footer that contains the last revision from GitHub that was successfully
	// imported to Gerrit.
	prefixGitFooterLastRev = "GitHub-Last-Rev:"

	// Footer containing the GitHub PR associated with the Gerrit Change.
	prefixGitFooterPR = "GitHub-Pull-Request:"

	// Footer containing the Gerrit Change ID.
	prefixGitFooterChangeID = "Change-Id:"
)

// Gerrit projects we accept PRs for.
var gerritProjectWhitelist = map[string]bool{
	"arch":           true,
	"benchmarks":     true,
	"blog":           true,
	"build":          true,
	"crypto":         true,
	"debug":          true,
	"dl":             true,
	"example":        true,
	"exp":            true,
	"gddo":           true,
	"go":             true,
	"image":          true,
	"lint":           true,
	"mobile":         true,
	"net":            true,
	"oauth2":         true,
	"perf":           true,
	"playground":     true,
	"proposal":       true,
	"review":         true,
	"scratch":        true,
	"sublime-build":  true,
	"sublime-config": true,
	"sync":           true,
	"sys":            true,
	"talks":          true,
	"term":           true,
	"text":           true,
	"time":           true,
	"tools":          true,
	"tour":           true,
	"vgo":            true,
}

type cachedPullRequest struct {
	pr   *github.PullRequest
	etag string
}

type bot struct {
	githubClient *github.Client
	gerritClient *gerrit.Client

	sync.RWMutex // Protects all fields below
	corpus       *maintner.Corpus

	// importedPRs and cachedPRs should share the same method for determining keys
	// given the same Pull Request, as the presence of a key in the former determined
	// whether it should remain in the latter.
	importedPRs map[string]*maintner.GerritCL // GitHub owner/repo#n -> Gerrit CL

	// Pull Requests that have been cached locally since maintner doesn’t support
	// PRs and this is used to make conditional requests to the API.
	cachedPRs map[string]*cachedPullRequest // GitHub owner/repo#n -> GitHub Pull Request

	// CLs that have been created/updated on Gerrit for GitHub PRs but are not yet
	// reflected in the maintner corpus yet.
	pendingCLs map[string]string // GitHub owner/repo#n -> Commit message from PR

	// Cache of Gerrit Account IDs to AccountInfo structs.
	cachedGerritAccounts map[int]*gerrit.AccountInfo // 1234 -> Detailed Account Info
}

func newBot(githubClient *github.Client, gerritClient *gerrit.Client) *bot {
	return &bot{
		githubClient:         githubClient,
		gerritClient:         gerritClient,
		importedPRs:          map[string]*maintner.GerritCL{},
		pendingCLs:           map[string]string{},
		cachedPRs:            map[string]*cachedPullRequest{},
		cachedGerritAccounts: map[int]*gerrit.AccountInfo{},
	}
}

// initCorpus fetches a full maintner corpus, overwriting any existing data.
func (b *bot) initCorpus(ctx context.Context) {
	b.Lock()
	defer b.Unlock()
	var err error
	b.corpus, err = godata.Get(ctx)
	if err != nil {
		log.Fatalf("godata.Get: %v", err)
	}
}

// corpusUpdateLoop continuously updates the server’s corpus until ctx’s Done
// channel is closed.
func (b *bot) corpusUpdateLoop(ctx context.Context) {
	log.Println("Starting corpus update loop ...")
	for {
		b.checkPullRequests()
		err := b.corpus.UpdateWithLocker(ctx, &b.RWMutex)
		if err != nil {
			if err == maintner.ErrSplit {
				log.Println("Corpus out of sync. Re-fetching corpus.")
				b.initCorpus(ctx)
			} else {
				log.Printf("corpus.Update: %v; sleeping 15s", err)
				time.Sleep(15 * time.Second)
				continue
			}
		}

		select {
		case <-ctx.Done():
			return
		default:
			continue
		}
	}
}

func (b *bot) checkPullRequests() {
	b.Lock()
	defer b.Unlock()
	b.importedPRs = map[string]*maintner.GerritCL{}
	b.corpus.Gerrit().ForeachProjectUnsorted(func(p *maintner.GerritProject) error {
		pname := p.Project()
		if !gerritProjectWhitelist[pname] {
			return nil
		}
		return p.ForeachOpenCL(func(cl *maintner.GerritCL) error {
			prv := cl.Footer(prefixGitFooterPR)
			if prv == "" {
				return nil
			}
			b.importedPRs[prv] = cl
			return nil
		})
	})

	// Remove any cached PRs that are no longer being checked.
	for k := range b.cachedPRs {
		if b.importedPRs[k] == nil {
			delete(b.cachedPRs, k)
		}
	}

	b.corpus.GitHub().ForeachRepo(func(ghr *maintner.GitHubRepo) error {
		id := ghr.ID()
		if id.Owner != "golang" || !gerritProjectWhitelist[id.Repo] {
			return nil
		}
		return ghr.ForeachIssue(func(issue *maintner.GitHubIssue) error {
			if issue.PullRequest && issue.Closed {
				// Clean up any reference of closed CLs within pendingCLs.
				shortLink := githubShortLink(id.Owner, id.Repo, int(issue.Number))
				delete(b.pendingCLs, shortLink)
				return nil
			}
			if issue.Closed || !issue.PullRequest || !issue.HasLabel("cla: yes") {
				return nil
			}
			ctx := context.Background()
			pr, err := b.getFullPR(ctx, id.Owner, id.Repo, int(issue.Number))
			if err != nil {
				log.Printf("getFullPR(ctx, %q, %q, %d): %v", id.Owner, id.Repo, issue.Number, err)
				return nil
			}
			if err := b.processPullRequest(ctx, pr); err != nil {
				log.Printf("processPullRequest: %v", err)
				return nil
			}
			return nil
		})
	})
}

// prShortLink returns text referencing an Issue or Pull Request that will be
// automatically converted into a link by GitHub.
func githubShortLink(owner, repo string, number int) string {
	return fmt.Sprintf("%s#%d", owner+"/"+repo, number)
}

// prShortLink returns text referencing the given Pull Request that will be
// automatically converted into a link by GitHub.
func prShortLink(pr *github.PullRequest) string {
	repo := pr.GetBase().GetRepo()
	return githubShortLink(repo.GetOwner().GetLogin(), repo.GetName(), pr.GetNumber())
}

// processPullRequest is the entry point to the state machine of mirroring a PR
// with Gerrit. PRs that are up to date with their respective Gerrit changes are
// skipped, and any with a HEAD commit SHA unequal to its Gerrit equivalent are
// imported. If the Gerrit change associated with a PR has been merged, the PR
// is closed. Those that have no associated open or merged Gerrit changes will
// result in one being created.
// b.RWMutex must be Lock'ed.
func (b *bot) processPullRequest(ctx context.Context, pr *github.PullRequest) error {
	log.Printf("Processing PR %s ...", pr.GetHTMLURL())
	shortLink := prShortLink(pr)
	cl := b.importedPRs[shortLink]

	if cl != nil && b.pendingCLs[shortLink] == cl.Commit.Msg {
		delete(b.pendingCLs, shortLink)
	}
	if b.pendingCLs[shortLink] != "" {
		log.Printf("Changes for PR %s have yet to be mirrored in the maintner corpus. Skipping for now.", shortLink)
		return nil
	}

	cmsg, err := commitMessage(pr, cl)
	if err != nil {
		return fmt.Errorf("commitMessage: %v", err)
	}

	if cl == nil {
		gcl, err := b.gerritChangeForPR(pr)
		if err != nil {
			return fmt.Errorf("gerritChangeForPR(%+v): %v", pr, err)
		}
		if gcl != nil && gcl.Status != "NEW" {
			if err := b.closePR(ctx, pr, gcl); err != nil {
				return fmt.Errorf("b.closePR(ctx, %+v, %+v): %v", pr, gcl, err)
			}
		}
		if gcl != nil {
			b.pendingCLs[shortLink] = cmsg
			return nil
		}
		if err := b.importGerritChangeFromPR(ctx, pr, nil); err != nil {
			return fmt.Errorf("importGerritChangeFromPR(%v, nil): %v", shortLink, err)
		}
		b.pendingCLs[shortLink] = cmsg
		return nil
	}

	if err := b.syncGerritCommentsToGitHub(ctx, pr, cl); err != nil {
		return fmt.Errorf("syncGerritCommentsToGitHub: %v", err)
	}

	if cmsg == cl.Commit.Msg {
		log.Printf("Change https://go-review.googlesource.com/q/%s is up to date; nothing to do.",
			cl.ChangeID())
		return nil
	}
	// Import PR to existing Gerrit Change.
	if err := b.importGerritChangeFromPR(ctx, pr, cl); err != nil {
		return fmt.Errorf("importGerritChangeFromPR(%v, %v): %v", shortLink, cl, err)
	}
	b.pendingCLs[shortLink] = cmsg
	return nil
}

// gerritMessageAuthorID returns the Gerrit Account ID of the author of m.
func gerritMessageAuthorID(m *maintner.GerritMessage) (int, error) {
	email := m.Author.Email()
	if strings.Index(email, "@") == -1 {
		return -1, fmt.Errorf("message author email %q does not contain '@' character", email)
	}
	i, err := strconv.Atoi(strings.Split(email, "@")[0])
	if err != nil {
		return -1, fmt.Errorf("strconv.Atoi: %v (email: %q)", err, email)
	}
	return i, nil
}

// gerritMessageAuthorName returns a message author's display name. To prevent a
// thundering herd of redundant comments created by posting a different message
// via postGitHubMessageNoDup in syncGerritCommentsToGitHub, it will only return
// the correct display name for messages posted after a hard-coded date.
// b.RWMutex must be Lock'ed.
func (b *bot) gerritMessageAuthorName(ctx context.Context, m *maintner.GerritMessage) (string, error) {
	t := time.Date(2018, time.November, 9, 0, 0, 0, 0, time.UTC)
	if m.Date.Before(t) {
		return m.Author.Name(), nil
	}
	id, err := gerritMessageAuthorID(m)
	if err != nil {
		return "", fmt.Errorf("gerritMessageAuthorID: %v", err)
	}
	account := b.cachedGerritAccounts[id]
	if account != nil {
		return account.Name, nil
	}
	ai, err := b.gerritClient.GetAccountInfo(ctx, strconv.Itoa(id))
	if err != nil {
		return "", fmt.Errorf("b.gerritClient.GetAccountInfo: %v", err)
	}
	b.cachedGerritAccounts[id] = &ai
	return ai.Name, nil
}

// b.RWMutex must be Lock'ed.
func (b *bot) syncGerritCommentsToGitHub(ctx context.Context, pr *github.PullRequest, cl *maintner.GerritCL) error {
	if *dryRun {
		log.Printf("[dry run] would sync Gerrit comments to %v", prShortLink(pr))
		return nil
	}
	repo := pr.GetBase().GetRepo()
	for _, m := range cl.Messages {
		id, err := gerritMessageAuthorID(m)
		if err != nil {
			return fmt.Errorf("gerritMessageAuthorID: %v", err)
		}
		if id == cl.OwnerID() {
			continue
		}
		authorName, err := b.gerritMessageAuthorName(ctx, m)
		if err != nil {
			return fmt.Errorf("b.gerritMessageAuthorName: %v", err)
		}
		msg := fmt.Sprintf(`Message from %s:

%s

---
Please don’t reply on this GitHub thread. Visit [golang.org/cl/%d](https://go-review.googlesource.com/c/%s/+/%d#message-%s).
After addressing review feedback, remember to [publish your drafts](https://github.com/golang/go/wiki/GerritBot#i-left-a-reply-to-a-comment-in-gerrit-but-no-one-but-me-can-see-it)!`,
			authorName, m.Message, cl.Number, cl.Project.Project(), cl.Number, m.Meta.Hash.String())
		if err := b.postGitHubMessageNoDup(ctx, repo.GetOwner().GetLogin(), repo.GetName(), pr.GetNumber(), msg); err != nil {
			return fmt.Errorf("postGitHubMessageNoDup: %v", err)
		}
	}
	return nil
}

// gerritChangeForPR returns the Gerrit Change info associated with the given PR.
// If no change exists for pr, it returns nil (with a nil error). If multiple
// changes exist it will return the first open change, and if no open changes
// are available, the first closed change is returned.
func (b *bot) gerritChangeForPR(pr *github.PullRequest) (*gerrit.ChangeInfo, error) {
	q := fmt.Sprintf(`"%s %s"`, prefixGitFooterPR, prShortLink(pr))
	cs, err := b.gerritClient.QueryChanges(context.Background(), q)
	if err != nil {
		return nil, fmt.Errorf("c.QueryChanges(ctx, %q): %v", q, err)
	}
	if len(cs) == 0 {
		return nil, nil
	}
	for _, c := range cs {
		if c.Status == gerrit.ChangeStatusNew {
			return c, nil
		}
	}
	// All associated changes are closed. It doesn’t matter which one is returned.
	return cs[0], nil
}

// closePR closes pr using the information from the given Gerrit change.
func (b *bot) closePR(ctx context.Context, pr *github.PullRequest, ch *gerrit.ChangeInfo) error {
	if *dryRun {
		log.Printf("[dry run] would close PR %v", prShortLink(pr))
		return nil
	}
	msg := fmt.Sprintf(`This PR is being closed because [golang.org/cl/%d](https://go-review.googlesource.com/c/%s/+/%d) has been %s.`,
		ch.ChangeNumber, ch.Project, ch.ChangeNumber, strings.ToLower(ch.Status))
	if ch.Status != gerrit.ChangeStatusAbandoned && ch.Status != gerrit.ChangeStatusMerged {
		return fmt.Errorf("invalid status for closed Gerrit change: %q", ch.Status)
	}

	repo := pr.GetBase().GetRepo()
	if err := b.postGitHubMessageNoDup(ctx, repo.GetOwner().GetLogin(), repo.GetName(), pr.GetNumber(), msg); err != nil {
		return fmt.Errorf("postGitHubMessageNoDup: %v", err)
	}

	req := &github.IssueRequest{
		State: github.String("closed"),
	}
	_, resp, err := b.githubClient.Issues.Edit(ctx, repo.GetOwner().GetLogin(), repo.GetName(), pr.GetNumber(), req)
	if err != nil {
		return fmt.Errorf("b.githubClient.Issues.Edit(ctx, %q, %q, %d, %+v): %v",
			repo.GetOwner().GetLogin(), repo.GetName(), pr.GetNumber(), req, err)
	}
	logGitHubRateLimits(resp)
	return nil
}

// downloadRef calls the Gerrit API to retrieve the ref (such as refs/changes/16/81116/1)
// of the most recent patch set of the change with changeID.
func (b *bot) downloadRef(ctx context.Context, changeID string) (string, error) {
	opt := gerrit.QueryChangesOpt{Fields: []string{"CURRENT_REVISION"}}
	ch, err := b.gerritClient.GetChange(ctx, changeID, opt)
	if err != nil {
		return "", fmt.Errorf("c.GetChange(ctx, %q, %+v): %v", changeID, opt, err)
	}
	rev, ok := ch.Revisions[ch.CurrentRevision]
	if !ok {
		return "", fmt.Errorf("revisions[current_revision] is not present in %+v", ch)
	}
	return rev.Ref, nil
}

func runCmd(c *exec.Cmd) error {
	log.Printf("Executing %v", c.Args)
	if b, err := c.CombinedOutput(); err != nil {
		return fmt.Errorf("running %v: output: %s; err: %v", c.Args, b, err)
	}
	return nil
}

const gerritHostBase = "https://go.googlesource.com/"

// gerritChangeRE matches the URL to the Change within the git output when pushing to Gerrit.
var gerritChangeRE = regexp.MustCompile(`https:\/\/go-review\.googlesource\.com\/c\/\w+\/\+\/\d+`)

// importGerritChangeFromPR mirrors the latest state of pr to cl. If cl is nil,
// then a new Gerrit Change is created.
func (b *bot) importGerritChangeFromPR(ctx context.Context, pr *github.PullRequest, cl *maintner.GerritCL) error {
	if *dryRun {
		log.Printf("[dry run] import Gerrit Change from PR %v", prShortLink(pr))
		return nil
	}
	githubRepo := pr.GetBase().GetRepo()
	gerritRepo := gerritHostBase + githubRepo.GetName() // GitHub repo name should match Gerrit repo name.
	repoDir := filepath.Join(reposRoot(), url.PathEscape(gerritRepo))

	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		cmds := []*exec.Cmd{
			exec.Command("git", "clone", "--bare", gerritRepo, repoDir),
			exec.Command("git", "-C", repoDir, "remote", "add", "github", githubRepo.GetGitURL()),
		}
		for _, c := range cmds {
			if err := runCmd(c); err != nil {
				return err
			}
		}
	}

	worktree := fmt.Sprintf("worktree_%s_%s_%d", githubRepo.GetOwner().GetLogin(), githubRepo.GetName(), pr.GetNumber())
	worktreeDir := filepath.Join(*workdir, "tmp", worktree)
	// workTreeDir is created by the `git worktree add` command.
	defer func() {
		log.Println("Cleaning up...")
		for _, c := range []*exec.Cmd{
			exec.Command("git", "-C", worktreeDir, "checkout", "master"),
			exec.Command("git", "-C", worktreeDir, "branch", "-D", prShortLink(pr)),
			exec.Command("rm", "-rf", worktreeDir),
			exec.Command("git", "-C", repoDir, "worktree", "prune"),
			exec.Command("git", "-C", repoDir, "branch", "-D", worktree),
		} {
			if err := runCmd(c); err != nil {
				log.Print(err)
			}
		}
	}()
	prBaseRef := pr.GetBase().GetRef()
	for _, c := range []*exec.Cmd{
		exec.Command("rm", "-rf", worktreeDir),
		exec.Command("git", "-C", repoDir, "worktree", "prune"),
		exec.Command("git", "-C", repoDir, "worktree", "add", worktreeDir),
		exec.Command("git", "-C", worktreeDir, "fetch", "origin", fmt.Sprintf("+%s:%s", prBaseRef, prBaseRef)),
		exec.Command("git", "-C", worktreeDir, "fetch", "github", fmt.Sprintf("pull/%d/head", pr.GetNumber())),
	} {
		if err := runCmd(c); err != nil {
			return err
		}
	}

	mergeBaseSHA, err := cmdOut(exec.Command("git", "-C", worktreeDir, "merge-base", prBaseRef, "FETCH_HEAD"))
	if err != nil {
		return err
	}

	author, err := cmdOut(exec.Command("git", "-C", worktreeDir, "diff-tree", "--always", "--no-patch", "--format=%an <%ae>", "FETCH_HEAD"))
	if err != nil {
		return err
	}

	cmsg, err := commitMessage(pr, cl)
	if err != nil {
		return fmt.Errorf("commitMessage: %v", err)
	}
	for _, c := range []*exec.Cmd{
		exec.Command("git", "-C", worktreeDir, "checkout", "-B", prShortLink(pr), mergeBaseSHA),
		exec.Command("git", "-C", worktreeDir, "merge", "--squash", "--no-commit", "FETCH_HEAD"),
		exec.Command("git", "-C", worktreeDir, "commit", "--author", author, "-m", cmsg),
	} {
		if err := runCmd(c); err != nil {
			return err
		}
	}

	var pushOpts string
	if cl == nil {
		// Add this informational message only on CL creation.
		msg := fmt.Sprintf("This Gerrit CL corresponds to GitHub PR %s.\n\nAuthor: %s", prShortLink(pr), author)
		pushOpts = "%m=" + url.QueryEscape(msg)
	}

	// nokeycheck is specified to avoid failing silently when a review is created
	// with what appears to be a private key. Since there are cases where a user
	// would want a private key checked in (tests).
	out, err := cmdOut(exec.Command("git", "-C", worktreeDir, "push", "-o", "nokeycheck", "origin", "HEAD:refs/for/"+prBaseRef+pushOpts))
	if err != nil {
		return fmt.Errorf("could not create change: %v", err)
	}
	changeURL := gerritChangeRE.FindString(out)
	if changeURL == "" {
		return fmt.Errorf("could not find change URL in command output: %q", out)
	}
	repo := pr.GetBase().GetRepo()
	msg := fmt.Sprintf(`This PR (HEAD: %v) has been imported to Gerrit for code review.

Please visit %s to see it.

Tip: You can toggle comments from me using the %s slash command (e.g. %s)
See the [Wiki page](https://golang.org/wiki/GerritBot) for more info`,
		pr.Head.GetSHA(), changeURL, "`comments`", "`/comments off`")
	return b.postGitHubMessageNoDup(ctx, repo.GetOwner().GetLogin(), repo.GetName(), pr.GetNumber(), msg)
}

var changeIdentRE = regexp.MustCompile(`(?m)^Change-Id: (I[0-9a-fA-F]{40})\n?`)

// commitMessage returns the text used when creating the squashed commit for pr.
// A non-nil cl indicates that pr is associated with an existing Gerrit Change.
func commitMessage(pr *github.PullRequest, cl *maintner.GerritCL) (string, error) {
	prBody := pr.GetBody()
	var changeID string
	if cl != nil {
		changeID = cl.ChangeID()
	} else {
		sms := changeIdentRE.FindStringSubmatch(prBody)
		if sms != nil {
			changeID = sms[1]
			prBody = strings.Replace(prBody, sms[0], "", -1)
		}
	}
	if changeID == "" {
		changeID = genChangeID(pr)
	}

	var msg bytes.Buffer
	fmt.Fprintf(&msg, "%s\n\n%s\n\n", pr.GetTitle(), prBody)
	fmt.Fprintf(&msg, "%s %s\n", prefixGitFooterChangeID, changeID)
	fmt.Fprintf(&msg, "%s %s\n", prefixGitFooterLastRev, pr.Head.GetSHA())
	fmt.Fprintf(&msg, "%s %s\n", prefixGitFooterPR, prShortLink(pr))

	// Clean the commit message up.
	cmd := exec.Command("git", "stripspace")
	cmd.Stdin = &msg
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("could not execute command %v: %v", cmd.Args, err)
	}
	return string(out), nil
}

// genChangeID returns a new Gerrit Change ID using the Pull Request’s ID.
// Change IDs are SHA-1 hashes prefixed by an "I" character.
func genChangeID(pr *github.PullRequest) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "golang_github_pull_request_id_%d", pr.GetID())
	return fmt.Sprintf("I%x", sha1.Sum(buf.Bytes()))
}

func cmdOut(cmd *exec.Cmd) (string, error) {
	log.Printf("Executing %v", cmd.Args)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("running %v: output: %s; err: %v", cmd.Args, out, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func reposRoot() string {
	return filepath.Join(*workdir, "repos")
}

// getFullPR retrieves a Pull Request via GitHub’s API.
// b.RWMutex must be Lock'ed.
func (b *bot) getFullPR(ctx context.Context, owner, repo string, number int) (*github.PullRequest, error) {
	shortLink := githubShortLink(owner, repo, number)
	cpr := b.cachedPRs[shortLink]
	var etag string
	if cpr != nil {
		etag = cpr.etag
		log.Printf("Retrieving PR %s from GitHub using Etag %q ...", shortLink, etag)
	} else {
		log.Printf("Retrieving PR %s from GitHub without an Etag ...", shortLink)
	}

	u := fmt.Sprintf("repos/%v/%v/pulls/%d", owner, repo, number)
	req, err := b.githubClient.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("b.githubClient.NewRequest(%q, %q, nil): %v", http.MethodGet, u, err)
	}
	if etag != "" {
		req.Header.Add("If-None-Match", etag)
	}

	pr := new(github.PullRequest)
	resp, err := b.githubClient.Do(ctx, req, pr)
	logGitHubRateLimits(resp)
	if err != nil {
		if resp.StatusCode == http.StatusNotModified {
			log.Println("Returning cached version of", shortLink)
			return cpr.pr, nil
		}
		return nil, fmt.Errorf("b.githubClient.Do: %v", err)
	}

	b.cachedPRs[shortLink] = &cachedPullRequest{
		etag: resp.Header.Get("Etag"),
		pr:   pr,
	}
	return pr, nil
}

func logGitHubRateLimits(resp *github.Response) {
	if resp == nil {
		return
	}
	log.Printf("GitHub: %d/%d calls remaining; Reset in %v", resp.Rate.Remaining, resp.Rate.Limit, resp.Rate.Reset.Sub(time.Now()))
}

// postGitHubMessageNoDup ensures that the message being posted on an issue does not already have the
// same exact content. These comments can be toggled by the user via a slash command /comments {on|off}
// at the beginning of a message.
// TODO(andybons): This logic is shared by gopherbot. Consolidate it somewhere.
func (b *bot) postGitHubMessageNoDup(ctx context.Context, org, repo string, issueNum int, msg string) error {
	gr := b.corpus.GitHub().Repo(org, repo)
	if gr == nil {
		return fmt.Errorf("unknown github repo %s/%s", org, repo)
	}
	var since time.Time
	var noComment bool
	var ownerID int64
	if gi := gr.Issue(int32(issueNum)); gi != nil {
		ownerID = gi.User.ID
		var dup bool
		gi.ForeachComment(func(c *maintner.GitHubComment) error {
			since = c.Updated
			// TODO: check for exact match?
			if strings.Contains(c.Body, msg) {
				dup = true
				return nil
			}
			if c.User.ID == ownerID && strings.HasPrefix(c.Body, "/comments ") {
				if strings.HasPrefix(c.Body, "/comments off") {
					noComment = true
				} else if strings.HasPrefix(c.Body, "/comments on") {
					noComment = false
				}
			}
			return nil
		})
		if dup {
			// Comment's already been posted. Nothing to do.
			return nil
		}
	}
	// See if there is a dup comment from when GerritBot last got
	// its data from maintner.
	ics, resp, err := b.githubClient.Issues.ListComments(ctx, org, repo, int(issueNum), &github.IssueListCommentsOptions{
		Since:       since,
		ListOptions: github.ListOptions{PerPage: 1000},
	})
	if err != nil {
		return err
	}
	logGitHubRateLimits(resp)
	for _, ic := range ics {
		if strings.Contains(ic.GetBody(), msg) {
			// Dup.
			return nil
		}
	}

	if ownerID == 0 {
		issue, resp, err := b.githubClient.Issues.Get(ctx, org, repo, issueNum)
		if err != nil {
			return err
		}
		logGitHubRateLimits(resp)
		ownerID = int64(issue.GetUser().GetID())
	}
	for _, ic := range ics {
		if strings.Contains(ic.GetBody(), msg) {
			// Dup.
			return nil
		}
		body := ic.GetBody()
		if int64(ic.GetUser().GetID()) == ownerID && strings.HasPrefix(body, "/comments ") {
			if strings.HasPrefix(body, "/comments off") {
				noComment = true
			} else if strings.HasPrefix(body, "/comments on") {
				noComment = false
			}
		}
	}
	if noComment {
		return nil
	}
	_, resp, err = b.githubClient.Issues.CreateComment(ctx, org, repo, int(issueNum), &github.IssueComment{
		Body: github.String(msg),
	})
	if err != nil {
		return err
	}
	logGitHubRateLimits(resp)
	return nil
}
