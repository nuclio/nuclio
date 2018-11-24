// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The gitmirror binary watches the specified Gerrit repositories for
// new commits and reports them to the build dashboard.
//
// It also serves tarballs over HTTP for the build system, and pushes
// new commits to Github.
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/compute/metadata"
	"golang.org/x/build/buildenv"
	"golang.org/x/build/gerrit"
	"golang.org/x/build/internal/gitauth"
	"golang.org/x/build/maintner"
	"golang.org/x/build/maintner/godata"
)

const (
	goBase         = "https://go.googlesource.com/"
	watcherVersion = 3        // must match dashboard/app/build/handler.go's watcherVersion
	master         = "master" // name of the master branch
)

var (
	httpAddr = flag.String("http", "", "If non-empty, the listen address to run an HTTP server on")
	cacheDir = flag.String("cachedir", "", "git cache directory. If empty a temp directory is made.")

	dashFlag = flag.String("dash", "", "Dashboard URL (must end in /). If unset, will be automatically derived from the GCE project name.")
	keyFile  = flag.String("key", defaultKeyFile, "Build dashboard key file. If empty, automatic from GCE project metadata")

	pollInterval = flag.Duration("poll", 60*time.Second, "Remote repo poll interval")

	// TODO(bradfitz): these three are all kinda the same and
	// redundant. Unify after research.
	network = flag.Bool("network", true, "Enable network calls (disable for testing)")
	mirror  = flag.Bool("mirror", false, "whether to mirror to github")
	report  = flag.Bool("report", true, "Report updates to build dashboard (use false for development dry-run mode)")

	filter   = flag.String("filter", "", "If non-empty, a comma-separated list of directories or files to watch for new commits (only works on main repo). If empty, watch all files in repo.")
	branches = flag.String("branches", "", "If non-empty, a comma-separated list of branches to watch. If empty, watch changes on every branch.")
)

var (
	defaultKeyFile = filepath.Join(homeDir(), ".gobuildkey")
	dashboardKey   = ""
	networkSeen    = make(map[string]bool) // testing mode only (-network=false); known hashes
)

var httpClient = &http.Client{
	Timeout: 30 * time.Second, // overkill
}

var gerritClient = gerrit.NewClient(goBase, gerrit.NoAuth)

var (
	// gitLogFn returns the list of unseen Commits on a Repo,
	// typically by shelling out to `git log`.
	// gitLogFn is a global var so we can stub it out for tests.
	gitLogFn = gitLog

	// gitRemotesFn returns a slice of remote branches known to the git repo.
	// gitRemotesFn is a global var so we can stub it out for tests.
	gitRemotesFn = gitRemotes
)

func main() {
	flag.Parse()
	if err := gitauth.Init(); err != nil {
		log.Fatalf("gitauth: %v", err)
	}

	if *dashFlag == "" && metadata.OnGCE() {
		project, err := metadata.ProjectID()
		if err != nil {
			log.Fatalf("metadata.ProjectID: %v", err)
		}
		*dashFlag = buildenv.ByProjectID(project).DashBase()
	}
	if *dashFlag == "" {
		log.Fatal("-dash must be specified and could not be autodetected")
	}

	log.Printf("gitmirror running.")

	go pollGerritAndTickle()
	go subscribeToMaintnerAndTickleLoop()
	err := runGitMirror()
	log.Fatalf("gitmirror exiting after failure: %v", err)
}

// runGitMirror is a little wrapper so we can use defer and return to signal
// errors. It should only return a non-nil error.
func runGitMirror() error {
	if !strings.HasSuffix(*dashFlag, "/") {
		return errors.New("dashboard URL (-dashboard) must end in /")
	}

	if *mirror {
		sshDir := filepath.Join(homeDir(), ".ssh")
		sshKey := filepath.Join(sshDir, "id_ed25519")
		if _, err := os.Stat(sshKey); err == nil {
			log.Printf("Using github ssh key at %v", sshKey)
		} else {
			if privKey, err := metadata.ProjectAttributeValue("github-ssh"); err == nil && len(privKey) > 0 {
				if err := os.MkdirAll(sshDir, 0700); err != nil {
					return err
				}
				if err := ioutil.WriteFile(sshKey, []byte(privKey+"\n"), 0600); err != nil {
					return err
				}
				log.Printf("Wrote %s from GCE metadata.", sshKey)
			} else {
				return fmt.Errorf("Can't mirror to github without 'github-ssh' GCE metadata or file %v", sshKey)
			}
		}
	}

	if *cacheDir == "" {
		dir, err := ioutil.TempDir("", "gitmirror")
		if err != nil {
			log.Fatal(err)
		}
		defer os.RemoveAll(dir)
		*cacheDir = dir
	} else {
		fi, err := os.Stat(*cacheDir)
		if os.IsNotExist(err) {
			if err := os.MkdirAll(*cacheDir, 0755); err != nil {
				return fmt.Errorf("failed to create watcher's git cache dir: %v", err)
			}
		} else {
			if err != nil {
				return fmt.Errorf("invalid -cachedir: %v", err)
			}
			if !fi.IsDir() {
				return fmt.Errorf("invalid -cachedir=%q; not a directory", *cacheDir)
			}
		}
	}

	if *report {
		if k, err := readKey(); err != nil {
			return err
		} else {
			dashboardKey = k
		}
	}

	if *httpAddr != "" {
		http.HandleFunc("/debug/env", handleDebugEnv)
		http.HandleFunc("/debug/goroutines", handleDebugGoroutines)
		ln, err := net.Listen("tcp", *httpAddr)
		if err != nil {
			return err
		}
		go http.Serve(ln, nil)
	}

	errc := make(chan error)

	subrepos, err := subrepoList()
	if err != nil {
		return err
	}

	startRepo := func(name, path string, dash bool) {
		log.Printf("Starting watch of repo %s", name)
		url := goBase + name
		var dst string
		if *mirror {
			if shouldMirror(name) {
				log.Printf("Starting mirror of subrepo %s", name)
				dst = "git@github.com:golang/" + name + ".git"
			} else {
				log.Printf("Not mirroring repo %s", name)
			}
		}
		r, err := NewRepo(url, dst, path, dash)
		if err != nil {
			errc <- err
			return
		}
		http.Handle("/"+name+".tar.gz", r)
		reposMu.Lock()
		repos = append(repos, r)
		sort.Slice(repos, func(i, j int) bool { return repos[i].name() < repos[j].name() })
		reposMu.Unlock()
		r.Loop()
	}

	go startRepo("go", "", true)

	seen := map[string]bool{"go": true}
	for _, path := range subrepos {
		name := strings.TrimPrefix(path, "golang.org/x/")
		seen[name] = true
		go startRepo(name, path, true)
	}
	if *mirror {
		for name := range gerritMetaMap() {
			if seen[name] {
				// Repo already picked up by dashboard list.
				continue
			}
			path := "golang.org/x/" + name
			if name == "dl" {
				// This subrepo is different from others in that
				// it doesn't use the /x/ path element.
				path = "golang.org/" + name
			}
			go startRepo(name, path, false)
		}
	}

	http.HandleFunc("/", handleRoot)

	// Blocks forever if all the NewRepo calls succeed:
	return <-errc
}

var (
	reposMu sync.Mutex
	repos   []*Repo
)

// GET /
// or:
// GET /debug/watcher/
func handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/debug/watcher/" {
		http.NotFound(w, r)
		return
	}
	reposMu.Lock()
	defer reposMu.Unlock()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, "<html><body><pre>")
	for _, r := range repos {
		fmt.Fprintf(w, "<a href='/debug/watcher/%s'>%s</a> - %s\n", r.name(), r.name(), r.statusLine())
	}
	fmt.Fprint(w, "</pre></body></html>")
}

// shouldMirror reports whether the named repo should be mirrored from
// Gerrit to Github.
func shouldMirror(name string) bool {
	switch name {
	case
		"arch",
		"benchmarks",
		"blog",
		"build",
		"crypto",
		"debug",
		"dl",
		"example",
		"exp",
		"gddo",
		"go",
		"gofrontend",
		"image",
		"lint",
		"mobile",
		"net",
		"oauth2",
		"playground",
		"proposal",
		"review",
		"scratch",
		"sync",
		"sys",
		"talks",
		"term",
		"text",
		"time",
		"tools",
		"tour",
		"vgo":
		return true
	}
	// Else, see if it appears to be a subrepo:
	r, err := httpClient.Get("https://golang.org/x/" + name)
	if err != nil {
		log.Printf("repo %v doesn't seem to exist: %v", name, err)
		return false
	}
	r.Body.Close()
	return r.StatusCode/100 == 2
}

// a statusEntry is a status string at a specific time.
type statusEntry struct {
	status string
	t      time.Time
}

// statusRing is a ring buffer of timestamped status messages.
type statusRing struct {
	mu   sync.Mutex      // guards rest
	head int             // next position to fill
	ent  [50]statusEntry // ring buffer of entries; zero time means unpopulated
}

func (r *statusRing) add(status string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.ent[r.head] = statusEntry{status, time.Now()}
	r.head++
	if r.head == len(r.ent) {
		r.head = 0
	}
}

func (r *statusRing) foreachDesc(fn func(statusEntry)) {
	r.mu.Lock()
	defer r.mu.Unlock()

	i := r.head
	for {
		i--
		if i < 0 {
			i = len(r.ent) - 1
		}
		if i == r.head || r.ent[i].t.IsZero() {
			return
		}
		fn(r.ent[i])
	}
}

// Repo represents a repository to be watched.
type Repo struct {
	root     string             // on-disk location of the git repo, *cacheDir/name
	path     string             // base import path for repo (blank for main repo)
	commits  map[string]*Commit // keyed by full commit hash (40 lowercase hex digits)
	branches map[string]*Branch // keyed by branch name, eg "release-branch.go1.3" (or empty for default)
	dash     bool               // push new commits to the dashboard
	mirror   bool               // push new commits to 'dest' remote
	status   statusRing

	mu        sync.Mutex
	err       error
	firstBad  time.Time
	lastBad   time.Time
	firstGood time.Time
	lastGood  time.Time
}

// NewRepo checks out a new instance of the git repository
// specified by srcURL.
//
// If dstURL is not empty, changes from the source repository will
// be mirrored to the specified destination repository.
// The importPath argument is the base import path of the repository,
// and should be empty for the main Go repo.
// The dash argument should be set true if commits to this
// repo should be reported to the build dashboard.
func NewRepo(srcURL, dstURL, importPath string, dash bool) (*Repo, error) {
	name := path.Base(srcURL) // "go", "net", etc
	root := filepath.Join(*cacheDir, name)
	r := &Repo{
		path:     importPath,
		root:     root,
		commits:  make(map[string]*Commit),
		branches: make(map[string]*Branch),
		mirror:   dstURL != "",
		dash:     dash,
	}

	http.Handle("/debug/watcher/"+r.name(), r)

	needClone := true
	if r.shouldTryReuseGitDir(dstURL) {
		r.setStatus("reusing git dir; running git fetch")
		cmd := exec.Command("git", "fetch", "origin")
		cmd.Dir = r.root
		r.logf("running git fetch")
		t0 := time.Now()
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err != nil {
			r.logf("git fetch failed; proceeding to wipe + clone instead; err: %v, stderr: %s", err, stderr.Bytes())
		} else {
			needClone = false
			r.logf("ran git fetch in %v", time.Since(t0))
		}
	}
	if needClone {
		r.setStatus("need clone; removing cache root")
		os.RemoveAll(r.root)
		t0 := time.Now()
		r.setStatus("running fresh git clone --mirror")
		r.logf("cloning %v into %s", srcURL, r.root)
		cmd := exec.Command("git", "clone", "--mirror", srcURL, r.root)
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("cloning %s: %v\n\n%s", srcURL, err, out)
		}
		r.setStatus("cloned")
		r.logf("cloned in %v", time.Since(t0))
	}

	if r.mirror {
		r.setStatus("adding dest remote")
		if err := r.addRemote("dest", dstURL); err != nil {
			r.setStatus("failed to add dest")
			return nil, fmt.Errorf("adding remote: %v", err)
		}
		r.setStatus("added dest remote")
	}

	if r.dash {
		r.logf("loading commit log")
		if err := r.update(false); err != nil {
			return nil, err
		}
		r.logf("found %v branches among %v commits\n", len(r.branches), len(r.commits))
	}

	return r, nil
}

func (r *Repo) setErr(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	change := (r.err != nil) != (err != nil)
	now := time.Now()
	if err != nil {
		if change {
			r.firstBad = now
		}
		r.lastBad = now
	} else {
		if change {
			r.firstGood = now
		}
		r.lastGood = now
	}
	r.err = err
}

var startTime = time.Now()

func (r *Repo) statusLine() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.lastGood.IsZero() {
		if r.err != nil {
			return fmt.Sprintf("broken; permanently? always failing, for %v", time.Since(r.firstBad))
		}
		if time.Since(startTime) < 5*time.Minute {
			return "ok; starting up, no report yet"
		}
		return fmt.Sprintf("hung; hang at start-up? no report since start %v ago", time.Since(startTime))
	}
	if r.err == nil {
		if sinceGood := time.Since(r.lastGood); sinceGood > 6*time.Minute {
			return fmt.Sprintf("hung? no activity since last success %v ago", sinceGood)
		}
		if r.lastBad.After(time.Now().Add(-1 * time.Hour)) {
			return fmt.Sprintf("ok; recent failure %v ago", time.Since(r.lastBad))
		}
		return "ok"
	}
	return fmt.Sprintf("broken for %v", time.Since(r.lastGood))
}

func (r *Repo) setStatus(status string) {
	r.status.add(status)
}

// shouldTryReuseGitDir reports whether we should try to reuse r.root as the git
// directory. (The directory may be corrupt, though.)
// dstURL is optional, and is the desired remote URL for a remote named "dest".
func (r *Repo) shouldTryReuseGitDir(dstURL string) bool {
	if _, err := os.Stat(filepath.Join(r.root, "FETCH_HEAD")); err != nil {
		if os.IsNotExist(err) {
			r.logf("not reusing git dir; no FETCH_HEAD at %s", r.root)
		} else {
			r.logf("not reusing git dir; %v", err)
		}
		return false
	}
	if dstURL == "" {
		r.logf("not reusing git dir because dstURL is empty")
		return true
	}

	// Does the "dest" remote match? If not, we return false and nuke
	// the world and re-clone out of laziness.
	cmd := exec.Command("git", "remote", "-v")
	cmd.Dir = r.root
	out, err := cmd.Output()
	if err != nil {
		log.Printf("git remote -v: %v", err)
	}
	foundWrong := false
	for _, ln := range strings.Split(string(out), "\n") {
		if !strings.HasPrefix(ln, "dest") {
			continue
		}
		f := strings.Fields(ln)
		if len(f) < 2 {
			continue
		}
		if f[0] == "dest" {
			if f[1] == dstURL {
				return true
			}
			if !foundWrong {
				foundWrong = true
				r.logf("found dest of %q, which doesn't equal sought %q", f[1], dstURL)
			}
		}
	}
	r.logf("not reusing old repo: remote \"dest\" URL doesn't match")
	return false
}

func (r *Repo) addRemote(name, url string) error {
	gitConfig := filepath.Join(r.root, "config")
	f, err := os.OpenFile(gitConfig, os.O_WRONLY|os.O_APPEND, os.ModePerm)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(f, "\n[remote %q]\n\turl = %v\n", name, url)
	if err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// Loop continuously runs "git fetch" in the repo, checks for
// new commits, posts any new commits to the dashboard (if enabled),
// and mirrors commits to a destination repo (if enabled).
func (r *Repo) Loop() {
	tickler := repoTickler(r.name())
	for {
		if err := r.fetch(); err != nil {
			r.setErr(err)
			time.Sleep(10 * time.Second)
			continue
		}
		if r.mirror {
			if err := r.push(); err != nil {
				r.setErr(err)
				time.Sleep(10 * time.Second)
				continue
			}
		}
		if r.dash {
			if err := r.updateDashboard(); err != nil {
				r.setErr(err)
				time.Sleep(10 * time.Second)
				continue
			}
		}

		r.setErr(nil)
		r.setStatus("waiting")
		// We still run a timer but a very slow one, just
		// in case the mechanism updating the repo tickler
		// breaks for some reason.
		timer := time.NewTimer(5 * time.Minute)
		select {
		case <-tickler:
			r.setStatus("got update tickle")
			timer.Stop()
		case <-timer.C:
			r.setStatus("poll timer fired")
		}
	}
}

func (r *Repo) updateDashboard() (err error) {
	r.setStatus("updating dashboard")
	defer func() {
		if err == nil {
			r.setStatus("updated dashboard")
		}
	}()
	if err := r.update(true); err != nil {
		return err
	}
	remotes, err := gitRemotesFn(r)
	if err != nil {
		return err
	}
	for _, name := range remotes {
		b, ok := r.branches[name]
		if !ok {
			// skip branch; must be already merged
			continue
		}
		if err := r.postNewCommits(b); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repo) name() string {
	if r.path == "" {
		return "go"
	}
	return path.Base(r.path)
}

func (r *Repo) logf(format string, args ...interface{}) {
	log.Printf(r.name()+": "+format, args...)
}

// postNewCommits looks for unseen commits on the specified branch and
// posts them to the dashboard.
func (r *Repo) postNewCommits(b *Branch) error {
	if b.Head == b.LastSeen {
		return nil
	}
	c := b.LastSeen
	if c == nil {
		// Haven't seen anything on this branch yet:
		if b.Name == master {
			// For the master branch, bootstrap by creating a dummy
			// commit with a lone child that is the initial commit.
			c = &Commit{}
			for _, c2 := range r.commits {
				if c2.Parent == "" {
					c.children = []*Commit{c2}
					break
				}
			}
			if c.children == nil {
				return fmt.Errorf("couldn't find initial commit")
			}
		} else {
			// Find the commit that this branch forked from.
			base, err := r.mergeBase("heads/"+b.Name, master)
			if err != nil {
				return err
			}
			var ok bool
			c, ok = r.commits[base]
			if !ok {
				return fmt.Errorf("couldn't find base commit: %v", base)
			}
		}
	}
	if err := r.postChildren(b, c); err != nil {
		return err
	}
	b.LastSeen = b.Head
	return nil
}

// postChildren posts to the dashboard all descendants of the given parent.
// It ignores descendants that are not on the given branch.
func (r *Repo) postChildren(b *Branch, parent *Commit) error {
	for _, c := range parent.children {
		if c.Branch != b.Name {
			continue
		}
		if err := r.postCommit(c); err != nil {
			if strings.Contains(err.Error(), "this package already has a first commit; aborting") {
				return nil
			}
			return err
		}
	}
	for _, c := range parent.children {
		if err := r.postChildren(b, c); err != nil {
			return err
		}
	}
	return nil
}

// postCommit sends a commit to the build dashboard.
func (r *Repo) postCommit(c *Commit) error {
	if !*report {
		r.logf("dry-run mode; NOT posting commit to dashboard: %v", c)
		return nil
	}
	r.logf("sending commit to dashboard: %v", c)

	t, err := time.Parse("Mon, 2 Jan 2006 15:04:05 -0700", c.Date)
	if err != nil {
		return fmt.Errorf("postCommit: parsing date %q for commit %v: %v", c.Date, c, err)
	}
	dc := struct {
		PackagePath string // (empty for main repo commits)
		Hash        string
		ParentHash  string

		User   string
		Desc   string
		Time   time.Time
		Branch string

		NeedsBenchmarking bool
	}{
		PackagePath: r.path,
		Hash:        c.Hash,
		ParentHash:  c.Parent,

		User:   c.Author,
		Desc:   c.Desc,
		Time:   t,
		Branch: c.Branch,

		NeedsBenchmarking: c.NeedsBenchmarking(),
	}
	b, err := json.Marshal(dc)
	if err != nil {
		return fmt.Errorf("postCommit: marshaling request body: %v", err)
	}

	if !*network {
		if c.Parent != "" {
			if !networkSeen[c.Parent] {
				r.logf("%v: %v", c.Parent, r.commits[c.Parent])
				return fmt.Errorf("postCommit: no parent %v found on dashboard for %v", c.Parent, c)
			}
		}
		if networkSeen[c.Hash] {
			return fmt.Errorf("postCommit: already seen %v", c)
		}
		networkSeen[c.Hash] = true
		return nil
	}

	v := url.Values{"version": {fmt.Sprint(watcherVersion)}, "key": {dashboardKey}}
	u := *dashFlag + "commit?" + v.Encode()
	resp, err := http.Post(u, "text/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return fmt.Errorf("postCommit: reading body: %v", err)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("postCommit: status: %v\nbody: %s", resp.Status, body)
	}

	var s struct {
		Error string
	}
	if err := json.Unmarshal(body, &s); err != nil {
		return fmt.Errorf("postCommit: decoding response: %v", err)
	}
	if s.Error != "" {
		return fmt.Errorf("postCommit: error: %v", s.Error)
	}
	return nil
}

// update looks for new commits and branches,
// and updates the commits and branches maps.
func (r *Repo) update(noisy bool) error {
	remotes, err := gitRemotesFn(r)
	if err != nil {
		return err
	}
	for _, name := range remotes {
		b := r.branches[name]

		// Find all unseen commits on this branch.
		revspec := "heads/" + name
		if b != nil {
			// If we know about this branch,
			// only log commits down to the known head.
			revspec = b.Head.Hash + ".." + revspec
		}
		log, err := gitLogFn(r, "--topo-order", revspec)
		if err != nil {
			return err
		}
		if len(log) == 0 {
			// No commits to handle; carry on.
			continue
		}

		var nDups, nDrops int

		// Add unknown commits to r.commits.
		var added []*Commit
		for _, c := range log {
			if noisy {
				r.logf("found new commit %v", c)
			}
			// If we've already seen this commit,
			// only store the master one in r.commits.
			if _, ok := r.commits[c.Hash]; ok {
				nDups++
				if name != master {
					nDrops++
					continue
				}
			}
			c.Branch = name
			r.commits[c.Hash] = c
			added = append(added, c)
		}

		if nDups > 0 {
			r.logf("saw %v duplicate commits; dropped %v of them", nDups, nDrops)
		}

		// Link added commits.
		for _, c := range added {
			if c.Parent == "" {
				// This is the initial commit; no parent.
				r.logf("no parents for initial commit %v", c)
				continue
			}
			// Find parent commit.
			p, ok := r.commits[c.Parent]
			if !ok {
				return fmt.Errorf("can't find parent %q for %v", c.Parent, c)
			}
			// Link parent Commit.
			c.parent = p
			// Link child Commits.
			p.children = append(p.children, c)
		}

		// Update branch head, or add newly discovered branch.
		// If we had already seen log[0], eg. on a different branch,
		// we would never have linked it in the loop above.
		// We therefore fetch the commit from r.commits to ensure we have
		// the right address.
		head := r.commits[log[0].Hash]
		if b != nil {
			// Known branch; update head.
			b.Head = head
			r.logf("updated branch head: %v", b)
		} else {
			// It's a new branch; add it.
			seen, err := r.lastSeen(head.Hash)
			if err != nil {
				return err
			}
			b = &Branch{Name: name, Head: head, LastSeen: seen}
			r.branches[name] = b
			r.logf("found branch: %v", b)
		}
	}

	return nil
}

// lastSeen finds the most recent commit the dashboard has seen,
// starting at the specified head. If the dashboard hasn't seen
// any of the commits from head to the beginning, it returns nil.
func (r *Repo) lastSeen(head string) (*Commit, error) {
	h, ok := r.commits[head]
	if !ok {
		return nil, fmt.Errorf("lastSeen: can't find %q in commits", head)
	}

	var s []*Commit
	for c := h; c != nil; c = c.parent {
		s = append(s, c)
	}

	var err error
	i := sort.Search(len(s), func(i int) bool {
		if err != nil {
			return false
		}
		ok, err = r.dashSeen(s[i].Hash)
		return ok
	})
	switch {
	case err != nil:
		return nil, fmt.Errorf("lastSeen: %v", err)
	case i < len(s):
		return s[i], nil
	default:
		// Dashboard saw no commits.
		return nil, nil
	}
}

// dashSeen reports whether the build dashboard knows the specified commit.
func (r *Repo) dashSeen(hash string) (bool, error) {
	if !*network {
		return networkSeen[hash], nil
	}
	v := url.Values{"hash": {hash}, "packagePath": {r.path}}
	u := *dashFlag + "commit?" + v.Encode()
	resp, err := httpClient.Get(u)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return false, fmt.Errorf("status: %v", resp.Status)
	}
	var s struct {
		Error string
	}
	err = json.NewDecoder(resp.Body).Decode(&s)
	if err != nil {
		return false, err
	}
	switch s.Error {
	case "":
		// Found one.
		return true, nil
	case "Commit not found":
		// Commit not found, keep looking for earlier commits.
		return false, nil
	default:
		return false, fmt.Errorf("dashboard: %v", s.Error)
	}
}

// mergeBase returns the hash of the merge base for revspecs a and b.
func (r *Repo) mergeBase(a, b string) (string, error) {
	cmd := exec.Command("git", "merge-base", a, b)
	cmd.Dir = r.root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git merge-base %s..%s: %v", a, b, err)
	}
	return string(bytes.TrimSpace(out)), nil
}

// gitRemotes returns a slice of remote branches known to the git repo.
// It always puts "origin/master" first.
func gitRemotes(r *Repo) ([]string, error) {
	if *branches != "" {
		return strings.Split(*branches, ","), nil
	}

	cmd := exec.Command("git", "branch")
	cmd.Dir = r.root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git branch: %v", err)
	}
	bs := []string{master}
	for _, b := range strings.Split(string(out), "\n") {
		b = strings.TrimPrefix(b, "* ")
		b = strings.TrimSpace(b)
		// Ignore aliases, blank lines, and master (it's already in bs).
		if b == "" || strings.Contains(b, "->") || b == master {
			continue
		}
		// Ignore pre-go1 release branches; they are just noise.
		if strings.HasPrefix(b, "release-branch.r") {
			continue
		}
		bs = append(bs, b)
	}
	return bs, nil
}

const logFormat = `--format=format:` + logBoundary + `%H
%P
%an <%ae>
%cD
%B
` + fileBoundary

const logBoundary = `_-_- magic boundary -_-_`
const fileBoundary = `_-_- file boundary -_-_`

// gitLog runs "git log" with the supplied arguments
// and parses the output into Commit values.
func gitLog(r *Repo, dir string, args ...string) ([]*Commit, error) {
	args = append([]string{"log", "--date=rfc", "--name-only", "--parents", logFormat}, args...)
	if r.path == "" && *filter != "" {
		paths := strings.Split(*filter, ",")
		args = append(args, "--")
		args = append(args, paths...)
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = r.root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git %v: %v\n%s", strings.Join(args, " "), err, out)
	}

	// We have a commit with description that contains 0x1b byte.
	// Mercurial does not escape it, but xml.Unmarshal does not accept it.
	// TODO(adg): do we still need to scrub this? Probably.
	out = bytes.Replace(out, []byte{0x1b}, []byte{'?'}, -1)

	var cs []*Commit
	for _, text := range strings.Split(string(out), logBoundary) {
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		p := strings.SplitN(text, "\n", 5)
		if len(p) != 5 {
			return nil, fmt.Errorf("git log %v: malformed commit: %q", strings.Join(args, " "), text)
		}

		// The change summary contains the change description and files
		// modified in this commit.  There is no way to directly refer
		// to the modified files in the log formatting string, so we look
		// for the file boundary after the description.
		changeSummary := p[4]
		descAndFiles := strings.SplitN(changeSummary, fileBoundary, 2)
		desc := strings.TrimSpace(descAndFiles[0])

		// For branch merges, the list of files can still be empty
		// because there are no changed files.
		files := strings.Replace(strings.TrimSpace(descAndFiles[1]), "\n", " ", -1)

		cs = append(cs, &Commit{
			Hash: p[0],
			// TODO(adg): This may break with branch merges.
			Parent: strings.Split(p[1], " ")[0],
			Author: p[2],
			Date:   p[3],
			Desc:   desc,
			Files:  files,
		})
	}
	return cs, nil
}

// fetch runs "git fetch" in the repository root.
// It tries three times, just in case it failed because of a transient error.
func (r *Repo) fetch() (err error) {
	n := 0
	r.setStatus("running git fetch origin")
	defer func() {
		if err != nil {
			r.setStatus("git fetch failed")
		} else {
			r.setStatus("ran git fetch")
		}
	}()
	return try(3, func() error {
		n++
		if n > 1 {
			r.setStatus(fmt.Sprintf("running git fetch origin, attempt %d", n))
		}
		cmd := exec.Command("git", "fetch", "origin")
		cmd.Dir = r.root
		if out, err := cmd.CombinedOutput(); err != nil {
			err = fmt.Errorf("%v\n\n%s", err, out)
			r.logf("git fetch: %v", err)
			return err
		}
		return nil
	})
}

// push effectively runs "git push -f --mirror dest" in the repository
// root, but does things in smaller batches of refs, to work around
// performance problems we saw in the past. (They might have been
// fixed by later version of git; they seemed to only affect the HTTP
// transport, but not ssh)
//
// It tries three times, just in case it failed because of a transient error.
func (r *Repo) push() (err error) {
	n := 0
	r.setStatus("syncing to github")
	defer func() {
		if err != nil {
			r.setStatus("sync to github failed")
		} else {
			r.setStatus("did sync to github")
		}
	}()
	return try(3, func() error {
		n++
		if n > 1 {
			r.setStatus(fmt.Sprintf("syncing to github, attempt %d", n))
		}
		r.setStatus("sync: fetching local refs")
		local, err := r.getLocalRefs()
		if err != nil {
			r.logf("failed to get local refs: %v", err)
			return err
		}
		r.setStatus(fmt.Sprintf("sync: got %d local refs", len(local)))

		r.setStatus("sync: fetching remote refs")
		remote, err := r.getRemoteRefs("dest")
		if err != nil {
			r.logf("failed to get remote refs: %v", err)
			return err
		}
		r.setStatus(fmt.Sprintf("sync: got %d remote refs", len(remote)))

		var pushRefs []string
		for ref, hash := range local {
			if strings.Contains(ref, "refs/users/") ||
				strings.Contains(ref, "refs/cache-auto") ||
				strings.Contains(ref, "refs/changes/") {
				continue
			}
			if remote[ref] != hash {
				pushRefs = append(pushRefs, ref)
			}
		}
		sort.Slice(pushRefs, func(i, j int) bool {
			p1 := priority[refType(pushRefs[i])]
			p2 := priority[refType(pushRefs[j])]
			if p1 != p2 {
				return p1 > p2
			}
			return pushRefs[i] <= pushRefs[j]
		})
		if len(pushRefs) == 0 {
			r.setStatus("nothing to sync")
			return nil
		}
		for len(pushRefs) > 0 {
			r.setStatus(fmt.Sprintf("%d refs to push; pushing batch", len(pushRefs)))
			r.logf("%d refs remain to sync to github", len(pushRefs))
			args := []string{"push", "-f", "dest"}
			n := 0
			for _, ref := range pushRefs {
				args = append(args, "+"+local[ref]+":"+ref)
				n++
				if n == 200 {
					break
				}
			}
			pushRefs = pushRefs[n:]
			cmd := exec.Command("git", args...)
			cmd.Dir = r.root
			cmd.Stderr = os.Stderr
			out, err := cmd.Output()
			if err != nil {
				r.logf("git push failed, running git %s: %s", args, out)
				r.setStatus("git push failure")
				return err
			}
		}
		r.setStatus("sync complete")
		return nil
	})
}

// hasRev returns true if the repo contains the commit-ish rev.
func (r *Repo) hasRev(ctx context.Context, rev string) bool {
	cmd := exec.CommandContext(ctx, "git", "cat-file", "-t", rev)
	cmd.Dir = r.root
	return cmd.Run() == nil
}

// if non-nil, used by r.archive to create a "git archive" command.
var testHookArchiveCmd func(context.Context, string, ...string) *exec.Cmd

// if non-nil, used by r.archive to create a "git fetch" command.
var testHookFetchCmd func(context.Context, string, ...string) *exec.Cmd

// archive exports the git repository at the given rev and returns the
// compressed repository.
func (r *Repo) archive(ctx context.Context, rev string) ([]byte, error) {
	var cmd *exec.Cmd
	if testHookArchiveCmd == nil {
		cmd = exec.CommandContext(ctx, "git", "archive", "--format=tgz", rev)
	} else {
		cmd = testHookArchiveCmd(ctx, "git", "archive", "--format=tgz", rev)
	}
	cmd.Dir = r.root
	return cmd.Output()
}

// fetchRev attempts to fetch rev from remote.
func (r *Repo) fetchRev(ctx context.Context, remote, rev string) error {
	var cmd *exec.Cmd
	if testHookFetchCmd == nil {
		cmd = exec.CommandContext(ctx, "git", "fetch", remote, rev)
	} else {
		cmd = testHookFetchCmd(ctx, "git", "fetch", remote, rev)
	}
	cmd.Dir = r.root
	return cmd.Run()
}

func (r *Repo) fetchRevIfNeeded(ctx context.Context, rev string) error {
	if r.hasRev(ctx, rev) {
		return nil
	}
	r.logf("attempting to fetch missing revision %s from origin", rev)
	return r.fetchRev(ctx, "origin", rev)
}

// GET /<name>.tar.gz
// GET /debug/watcher/<name>
func (r *Repo) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != "GET" && req.Method != "HEAD" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if strings.HasPrefix(req.URL.Path, "/debug/watcher/") {
		r.serveStatus(w, req)
		return
	}
	rev := req.FormValue("rev")
	if rev == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 30*time.Second)
	defer cancel()
	if err := r.fetchRevIfNeeded(ctx, rev); err != nil {
		// Try the archive anyway, it might work
		r.logf("error fetching revision %s: %v", rev, err)
	}
	tgz, err := r.archive(ctx, rev)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(tgz)))
	w.Header().Set("Content-Type", "application/x-compressed")
	w.Write(tgz)
}

func (r *Repo) serveStatus(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, "<html><head><title>watcher: %s</title><body><h1>watcher status for repo: %q</h1>\n",
		r.name(), r.name())
	fmt.Fprintf(w, "<pre>\n")
	nowRound := time.Now().Round(time.Second)
	r.status.foreachDesc(func(ent statusEntry) {
		fmt.Fprintf(w, "%v   %-20s %v\n",
			ent.t.In(time.UTC).Format(time.RFC3339),
			nowRound.Sub(ent.t.Round(time.Second)).String()+" ago",
			ent.status)
	})
	fmt.Fprintf(w, "\n</pre></body></html>")
}

func try(n int, fn func() error) error {
	var err error
	for tries := 0; tries < n; tries++ {
		time.Sleep(time.Duration(tries) * 5 * time.Second) // Linear back-off.
		if err = fn(); err == nil {
			break
		}
	}
	return err
}

// Branch represents a Mercurial branch.
type Branch struct {
	Name     string
	Head     *Commit
	LastSeen *Commit // the last commit posted to the dashboard
}

func (b *Branch) String() string {
	return fmt.Sprintf("%q(Head: %v LastSeen: %v)", b.Name, b.Head, b.LastSeen)
}

// Commit represents a single Git commit.
type Commit struct {
	Hash   string
	Author string
	Date   string // Format: "Mon, 2 Jan 2006 15:04:05 -0700"
	Desc   string // Plain text, first line is a short description.
	Parent string
	Branch string
	Files  string

	// For walking the graph.
	parent   *Commit
	children []*Commit
}

func (c *Commit) String() string {
	s := c.Hash
	if c.Branch != "" {
		s += fmt.Sprintf("[%v]", c.Branch)
	}
	s += fmt.Sprintf("(%q)", strings.SplitN(c.Desc, "\n", 2)[0])
	return s
}

// NeedsBenchmarking reports whether the Commit needs benchmarking.
func (c *Commit) NeedsBenchmarking() bool {
	// Do not benchmark branch commits, they are usually not interesting
	// and fall out of the trunk succession.
	if c.Branch != master {
		return false
	}
	// Do not benchmark commits that do not touch source files (e.g. CONTRIBUTORS).
	for _, f := range strings.Split(c.Files, " ") {
		if (strings.HasPrefix(f, "include") || strings.HasPrefix(f, "src")) &&
			!strings.HasSuffix(f, "_test.go") && !strings.Contains(f, "testdata") {
			return true
		}
	}
	return false
}

func homeDir() string {
	switch runtime.GOOS {
	case "plan9":
		return os.Getenv("home")
	case "windows":
		return os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
	}
	return os.Getenv("HOME")
}

func readKey() (string, error) {
	c, err := ioutil.ReadFile(*keyFile)
	if os.IsNotExist(err) && metadata.OnGCE() {
		key, err := metadata.ProjectAttributeValue("builder-master-key")
		if err != nil {
			return "", fmt.Errorf("-key=%s doesn't exist, and key can't be loaded from GCE metadata: %v", *keyFile, err)
		}
		return strings.TrimSpace(key), nil
	}
	if err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(bytes.SplitN(c, []byte("\n"), 2)[0])), nil
}

// subrepoList fetches a list of sub-repositories from the dashboard
// and returns them as a slice of base import paths.
// Eg, []string{"golang.org/x/tools", "golang.org/x/net"}.
func subrepoList() ([]string, error) {
	if !*network {
		return nil, nil
	}

	r, err := httpClient.Get(*dashFlag + "packages?kind=subrepo")
	if err != nil {
		return nil, fmt.Errorf("subrepo list: %v", err)
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return nil, fmt.Errorf("subrepo list: got status %v", r.Status)
	}
	var resp struct {
		Response []struct {
			Path string
		}
		Error string
	}
	err = json.NewDecoder(r.Body).Decode(&resp)
	if err != nil {
		return nil, fmt.Errorf("subrepo list: %v", err)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("subrepo list: %v", resp.Error)
	}
	var pkgs []string
	for _, r := range resp.Response {
		pkgs = append(pkgs, r.Path)
	}
	return pkgs, nil
}

var (
	ticklerMu sync.Mutex
	ticklers  = make(map[string]chan bool)
)

// repo is the gerrit repo: e.g. "go", "net", "crypto", ...
func repoTickler(repo string) chan bool {
	ticklerMu.Lock()
	defer ticklerMu.Unlock()
	if c, ok := ticklers[repo]; ok {
		return c
	}
	c := make(chan bool, 1)
	ticklers[repo] = c
	return c
}

// pollGerritAndTickle polls Gerrit's JSON meta URL of all its URLs
// and their current branch heads.  When this sees that one has
// changed, it tickles the channel for that repo and wakes up its
// poller, if its poller is in a sleep.
func pollGerritAndTickle() {
	last := map[string]string{} // repo -> last seen hash
	for {
		for repo, hash := range gerritMetaMap() {
			if hash != last[repo] {
				last[repo] = hash
				select {
				case repoTickler(repo) <- true:
				default:
				}
			}
		}
		time.Sleep(*pollInterval)
	}
}

// subscribeToMaintnerAndTickleLoop subscribes to maintner.golang.org
// and watches for any ref changes in realtime.
func subscribeToMaintnerAndTickleLoop() {
	for {
		if err := subscribeToMaintnerAndTickle(); err != nil {
			log.Printf("maintner loop: %v; retrying in 30 seconds", err)
			time.Sleep(30 * time.Second)
		}
	}
}

func subscribeToMaintnerAndTickle() error {
	log.Printf("Loading maintner data.")
	t0 := time.Now()
	ctx := context.Background()
	corpus, err := godata.Get(ctx)
	if err != nil {
		return err
	}
	log.Printf("Loaded maintner data in %v", time.Since(t0))
	last := map[string]string{} // go.googlesource.com repo base => digest of all refs
	for {
		corpus.Gerrit().ForeachProjectUnsorted(func(gp *maintner.GerritProject) error {
			proj := path.Base(gp.ServerSlashProject())
			s1 := sha1.New()
			gp.ForeachNonChangeRef(func(ref string, hash maintner.GitHash) error {
				io.WriteString(s1, string(hash))
				return nil
			})
			sum := fmt.Sprintf("%x", s1.Sum(nil))
			lastSum := last[proj]
			if lastSum == sum {
				return nil
			}
			last[proj] = sum
			if lastSum == "" {
				return nil
			}
			log.Printf("maintner refs for %s changed", gp.ServerSlashProject())
			select {
			case repoTickler(proj) <- true:
			default:
			}
			return nil
		})
		if err := corpus.Update(ctx); err != nil {
			return err
		}
	}
}

// gerritMetaMap returns the map from repo name (e.g. "go") to its
// latest master hash.
// The returned map is nil on any transient error.
func gerritMetaMap() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	meta, err := gerritClient.GetProjects(ctx, "master")
	if err != nil {
		return nil
	}
	m := map[string]string{}
	for repo, v := range meta {
		if master, ok := v.Branches["master"]; ok {
			m[repo] = master
		}
	}
	return m
}

func (r *Repo) getLocalRefs() (map[string]string, error) {
	cmd := exec.Command("git", "show-ref")
	cmd.Dir = r.root
	return parseRefs(cmd)
}

// getRemoteRefs tells you which references are available in the remote
// named dest, and returns a map of refs to the matching commit, e.g.
// "refs/heads/master" => "6b27048ae5e6ad1ef927e72e437531493de612fe".
func (r *Repo) getRemoteRefs(dest string) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "ls-remote", dest)
	cmd.Dir = r.root
	return parseRefs(cmd)
}

func parseRefs(cmd *exec.Cmd) (map[string]string, error) {
	refHash := map[string]string{}
	var errBuf bytes.Buffer
	if cmd.Stderr == nil {
		cmd.Stderr = &errBuf
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	bs := bufio.NewScanner(out)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	for bs.Scan() {
		f := strings.Fields(bs.Text())
		refHash[f[1]] = f[0]
	}
	if err := bs.Err(); err != nil {
		go cmd.Wait() // prevent zombies
		return nil, err
	}
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("wait err: %v, stderr: %s", err, errBuf.Bytes())
	}
	return refHash, nil
}

func refType(s string) string {
	s = strings.TrimPrefix(s, "refs/")
	if i := strings.IndexByte(s, '/'); i != -1 {
		return s[:i]
	}
	return s
}

var priority = map[string]int{
	"heads":   5,
	"tags":    4,
	"changes": 3,
}

// GET /debug/goroutines
func handleDebugGoroutines(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	buf := make([]byte, 1<<20)
	w.Write(buf[:runtime.Stack(buf, true)])
}

// GET /debug/env
func handleDebugEnv(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	for _, kv := range os.Environ() {
		fmt.Fprintf(w, "%s\n", kv)
	}
}
