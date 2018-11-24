// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TODO(adg): packages at weekly/release
// TODO(adg): some means to register new packages

// +build appengine

package build

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/build/types"

	"golang.org/x/build/app/cache"

	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/memcache"
)

func init() {
	handleFunc("/", uiHandler)
}

// uiHandler draws the build status page.
func uiHandler(w http.ResponseWriter, r *http.Request) {
	d := dashboardForRequest(r)
	c := d.Context(appengine.NewContext(r))
	now := cache.Now(c)
	key := "build-ui"

	mode := r.FormValue("mode")

	page, _ := strconv.Atoi(r.FormValue("page"))
	if page < 0 {
		page = 0
	}
	key += fmt.Sprintf("-page%v", page)

	repo := r.FormValue("repo")
	if repo != "" {
		key += "-repo-" + repo
	}

	branch := r.FormValue("branch")
	switch branch {
	case "all":
		branch = ""
	case "":
		branch = "master"
	}
	if repo != "" || mode == "json" {
		// Don't filter on branches in sub-repos.
		// TODO(adg): figure out how to make this work sensibly.
		// Don't filter on branches in json mode.
		branch = ""
	}
	if branch != "" {
		key += "-branch-" + branch
	}

	hashes := r.Form["hash"]

	var data uiTemplateData
	if len(hashes) > 0 || !cache.Get(c, r, now, key, &data) {

		pkg := &Package{} // empty package is the main repository
		if repo != "" {
			var err error
			pkg, err = GetPackage(c, repo)
			if err != nil {
				logErr(w, r, err)
				return
			}
		}
		var commits []*Commit
		var err error
		if len(hashes) > 0 {
			commits, err = fetchCommits(c, pkg, hashes)
		} else {
			commits, err = dashCommits(c, pkg, page, branch)
		}
		if err != nil {
			logErr(w, r, err)
			return
		}
		builders := commitBuilders(commits)

		branches := listBranches(c)

		var tagState []*TagState
		// Only show sub-repo state on first page of normal repo view.
		if pkg.Kind == "" && len(hashes) == 0 && page == 0 && (branch == "" || branch == "master") {
			s, err := GetTagState(c, "tip", "")
			if err != nil {
				if err == datastore.ErrNoSuchEntity {
					err = fmt.Errorf("tip tag not found")
				}
				logErr(w, r, err)
				return
			}
			tagState = []*TagState{s}
			for _, b := range branches {
				if !strings.HasPrefix(b, "release-branch.") {
					continue
				}
				if hiddenBranches[b] {
					continue
				}
				s, err := GetTagState(c, "release", b)
				if err == datastore.ErrNoSuchEntity {
					continue
				}
				if err != nil {
					logErr(w, r, err)
					return
				}
				tagState = append(tagState, s)
			}
		}

		p := &Pagination{}
		if len(commits) == commitsPerPage {
			p.Next = page + 1
		}
		if page > 0 {
			p.Prev = page - 1
			p.HasPrev = true
		}

		data = uiTemplateData{
			Package:    pkg,
			Commits:    commits,
			Builders:   builders,
			TagState:   tagState,
			Pagination: p,
			Branches:   branches,
			Branch:     branch,
		}
		if len(hashes) == 0 {
			cache.Set(c, r, now, key, &data)
		}
	}
	data.Dashboard = d

	switch mode {
	case "failures":
		failuresHandler(w, r, &data)
		return
	case "json":
		jsonHandler(w, r, &data)
		return
	}

	// Populate building URLs for the HTML UI only.
	data.populateBuildingURLs(c)

	var buf bytes.Buffer
	if err := uiTemplate.Execute(&buf, &data); err != nil {
		logErr(w, r, err)
		return
	}
	buf.WriteTo(w)
}

func listBranches(c context.Context) (branches []string) {
	var commits []*Commit
	_, err := datastore.NewQuery("Commit").Distinct().Project("Branch").GetAll(c, &commits)
	if err != nil {
		log.Errorf(c, "listBranches: %v", err)
		return
	}
	for _, c := range commits {
		branches = append(branches, c.Branch)
	}
	return
}

// failuresHandler is https://build.golang.org/?mode=failures , where it outputs
// one line per failure on the front page, in the form:
//    hash builder failure-url
func failuresHandler(w http.ResponseWriter, r *http.Request, data *uiTemplateData) {
	w.Header().Set("Content-Type", "text/plain")
	d := dashboardForRequest(r)
	for _, c := range data.Commits {
		for _, b := range data.Builders {
			res := c.Result(b, "")
			if res == nil || res.OK || res.LogHash == "" {
				continue
			}
			url := fmt.Sprintf("https://%v%v/log/%v", r.Host, d.Prefix, res.LogHash)
			fmt.Fprintln(w, c.Hash, b, url)
		}
	}
}

// jsonHandler is https://build.golang.org/?mode=json
// The output is a types.BuildStatus JSON object.
func jsonHandler(w http.ResponseWriter, r *http.Request, data *uiTemplateData) {
	d := dashboardForRequest(r)

	// cell returns one of "" (no data), "ok", or a failure URL.
	cell := func(res *Result) string {
		switch {
		case res == nil:
			return ""
		case res.OK:
			return "ok"
		}
		return fmt.Sprintf("https://%v%v/log/%v", r.Host, d.Prefix, res.LogHash)
	}

	var res types.BuildStatus
	res.Builders = data.Builders

	// First the commits from the main section (the "go" repo)
	for _, c := range data.Commits {
		rev := types.BuildRevision{
			Repo:    "go",
			Results: make([]string, len(data.Builders)),
		}
		commitToBuildRevision(c, &rev)
		for i, b := range data.Builders {
			rev.Results[i] = cell(c.Result(b, ""))
		}
		res.Revisions = append(res.Revisions, rev)
	}

	// Then the one commit each for the subrepos for each of the tracked tags.
	// (tip, Go 1.4, etc)
	for _, ts := range data.TagState {
		for _, pkgState := range ts.Packages {
			goRev := ts.Tag.Hash
			goBranch := ts.Name
			if goBranch == "tip" {
				// Normalize old hg terminology into
				// our git branch name.
				goBranch = "master"
			}
			rev := types.BuildRevision{
				Repo:       pkgState.Package.Name,
				GoRevision: goRev,
				Results:    make([]string, len(data.Builders)),
				GoBranch:   goBranch,
			}
			commitToBuildRevision(pkgState.Commit, &rev)
			for i, b := range res.Builders {
				rev.Results[i] = cell(pkgState.Commit.Result(b, goRev))
			}
			res.Revisions = append(res.Revisions, rev)
		}
	}

	v, _ := json.MarshalIndent(res, "", "\t")
	w.Header().Set("Content-Type", "text/json; charset=utf-8")
	w.Write(v)
}

// commitToBuildRevision fills in the fields of BuildRevision rev that
// are derived from Commit c.
func commitToBuildRevision(c *Commit, rev *types.BuildRevision) {
	rev.Revision = c.Hash
	// TODO: A comment may have more than one parent.
	rev.ParentRevisions = []string{c.ParentHash}
	rev.Date = c.Time.Format(time.RFC3339)
	rev.Branch = c.Branch
	rev.Author = c.User
	rev.Desc = c.Desc
}

type Pagination struct {
	Next, Prev int
	HasPrev    bool
}

// dashCommits gets a slice of the latest Commits to the current dashboard.
// If page > 0 it paginates by commitsPerPage.
func dashCommits(c context.Context, pkg *Package, page int, branch string) ([]*Commit, error) {
	offset := page * commitsPerPage
	q := datastore.NewQuery("Commit").
		Ancestor(pkg.Key(c)).
		Order("-Num")

	if branch != "" {
		q = q.Filter("Branch =", branch)
	}

	var commits []*Commit
	_, err := q.Limit(commitsPerPage).Offset(offset).
		GetAll(c, &commits)
	return commits, err
}

// fetchCommits gets a slice of the specific commit hashes
func fetchCommits(c context.Context, pkg *Package, hashes []string) ([]*Commit, error) {
	var out []*Commit
	var keys []*datastore.Key
	for _, hash := range hashes {
		commit := &Commit{
			Hash:        hash,
			PackagePath: pkg.Path,
		}
		out = append(out, commit)
		keys = append(keys, commit.Key(c))
	}
	err := datastore.GetMulti(c, keys, out)
	return out, err
}

// commitBuilders returns the names of the builders that provided
// Results for the provided commits.
func commitBuilders(commits []*Commit) []string {
	builders := make(map[string]bool)
	for _, commit := range commits {
		for _, r := range commit.Results() {
			builders[r.Builder] = true
		}
	}
	k := keys(builders)
	sort.Sort(builderOrder(k))
	return k
}

func keys(m map[string]bool) (s []string) {
	for k := range m {
		s = append(s, k)
	}
	sort.Strings(s)
	return
}

// builderOrder implements sort.Interface, sorting builder names
// ("darwin-amd64", etc) first by builderPriority and then alphabetically.
type builderOrder []string

func (s builderOrder) Len() int      { return len(s) }
func (s builderOrder) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s builderOrder) Less(i, j int) bool {
	pi, pj := builderPriority(s[i]), builderPriority(s[j])
	if pi == pj {
		return s[i] < s[j]
	}
	return pi < pj
}

func builderPriority(builder string) (p int) {
	// Put -temp builders at the end, always.
	if strings.HasSuffix(builder, "-temp") {
		defer func() { p += 20 }()
	}
	// Group race builders together.
	if isRace(builder) {
		return 2
	}
	// If the OS has a specified priority, use it.
	if p, ok := osPriority[builderOS(builder)]; ok {
		return p
	}
	// The rest.
	return 10
}

func isRace(s string) bool {
	return strings.Contains(s, "-race-") || strings.HasSuffix(s, "-race")
}

func unsupported(builder string) bool {
	if strings.HasSuffix(builder, "-temp") {
		return true
	}
	return unsupportedOS(builderOS(builder))
}

func unsupportedOS(os string) bool {
	if os == "race" || os == "android" || os == "all" {
		return false
	}
	p, ok := osPriority[os]
	return !ok || p > 1
}

// Priorities for specific operating systems.
var osPriority = map[string]int{
	"all":     0,
	"darwin":  1,
	"freebsd": 1,
	"linux":   1,
	"windows": 1,
	// race == 2
	"android":   3,
	"openbsd":   4,
	"netbsd":    5,
	"dragonfly": 6,
}

// TagState represents the state of all Packages at a Tag.
type TagState struct {
	Name     string // "tip", "release-branch.go1.4", etc
	Tag      *Commit
	Packages []*PackageState
}

// PackageState represents the state of a Package at a Tag.
type PackageState struct {
	Package *Package
	Commit  *Commit
}

// GetTagState fetches the results for all Go subrepos at the specified Tag.
// (Kind is "tip" or "release"; name is like "release-branch.go1.4".)
func GetTagState(c context.Context, kind, name string) (*TagState, error) {
	tag, err := GetTag(c, kind, name)
	if err != nil {
		return nil, err
	}
	pkgs, err := Packages(c, "subrepo")
	if err != nil {
		return nil, err
	}
	st := TagState{Name: tag.String()}
	for _, pkg := range pkgs {
		com, err := pkg.LastCommit(c)
		if err != nil {
			log.Warningf(c, "%v: no Commit found: %v", pkg, err)
			continue
		}
		st.Packages = append(st.Packages, &PackageState{pkg, com})
	}
	st.Tag, err = tag.Commit(c)
	if err != nil {
		return nil, err
	}
	return &st, nil
}

type uiTemplateData struct {
	Dashboard  *Dashboard
	Package    *Package
	Commits    []*Commit
	Builders   []string
	TagState   []*TagState
	Pagination *Pagination
	Branches   []string
	Branch     string
}

// buildingKey returns a memcache key that points to the log URL
// of an inflight build for the given hash, goHash, and builder.
func buildingKey(hash, goHash, builder string) string {
	return fmt.Sprintf("building|%v|%v|%v", hash, goHash, builder)
}

// populateBuildingURLs populates each commit in Commits' buildingURLs map with the
// URLs of builds which are currently in progress.
func (td *uiTemplateData) populateBuildingURLs(ctx context.Context) {
	// need are memcache keys: "building|<hash>|<gohash>|<builder>"
	// The hash is of the main "go" repo, or the subrepo commit hash.
	// The gohash is empty for the main repo, else it's the Go hash.
	var need []string

	commit := map[string]*Commit{} // commit hash -> Commit

	// Gather pending commits for main repo.
	for _, b := range td.Builders {
		for _, c := range td.Commits {
			if c.Result(b, "") == nil {
				commit[c.Hash] = c
				need = append(need, buildingKey(c.Hash, "", b))
			}
		}
	}

	// Gather pending commits for sub-repos.
	for _, ts := range td.TagState {
		goHash := ts.Tag.Hash
		for _, b := range td.Builders {
			for _, pkg := range ts.Packages {
				c := pkg.Commit
				commit[c.Hash] = c
				if c.Result(b, goHash) == nil {
					need = append(need, buildingKey(c.Hash, goHash, b))
				}
			}
		}
	}

	if len(need) == 0 {
		return
	}

	m, err := memcache.GetMulti(ctx, need)
	if err != nil {
		// oh well. this is a cute non-critical feature anyway.
		log.Debugf(ctx, "GetMulti of building keys: %v", err)
		return
	}
	for k, it := range m {
		f := strings.SplitN(k, "|", 4)
		if len(f) != 4 {
			continue
		}
		hash, goHash, builder := f[1], f[2], f[3]
		c, ok := commit[hash]
		if !ok {
			continue
		}
		m := c.buildingURLs
		if m == nil {
			m = make(map[builderAndGoHash]string)
			c.buildingURLs = m
		}
		m[builderAndGoHash{builder, goHash}] = string(it.Value)
	}

}

var uiTemplate = template.Must(
	template.New("ui.html").Funcs(tmplFuncs).ParseFiles("ui.html"),
)

var tmplFuncs = template.FuncMap{
	"buildDashboards":   buildDashboards,
	"builderOS":         builderOS,
	"builderSpans":      builderSpans,
	"builderSubheading": builderSubheading,
	"builderTitle":      builderTitle,
	"shortDesc":         shortDesc,
	"shortHash":         shortHash,
	"shortUser":         shortUser,
	"tail":              tail,
	"unsupported":       unsupported,
}

func splitDash(s string) (string, string) {
	i := strings.Index(s, "-")
	if i >= 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

// builderOS returns the os tag for a builder string
func builderOS(s string) string {
	os, _ := splitDash(s)
	return os
}

// builderOSOrRace returns the builder OS or, if it is a race builder, "race".
func builderOSOrRace(s string) string {
	if isRace(s) {
		return "race"
	}
	return builderOS(s)
}

// builderArch returns the arch tag for a builder string
func builderArch(s string) string {
	_, arch := splitDash(s)
	arch, _ = splitDash(arch) // chop third part
	return arch
}

// builderSubheading returns a short arch tag for a builder string
// or, if it is a race builder, the builder OS.
func builderSubheading(s string) string {
	if isRace(s) {
		return builderOS(s)
	}
	arch := builderArch(s)
	switch arch {
	case "amd64":
		return "x64"
	}
	return arch
}

// builderArchChar returns the architecture letter for a builder string
func builderArchChar(s string) string {
	arch := builderArch(s)
	switch arch {
	case "386":
		return "8"
	case "amd64":
		return "6"
	case "arm":
		return "5"
	}
	return arch
}

type builderSpan struct {
	N           int
	OS          string
	Unsupported bool
}

// builderSpans creates a list of tags showing
// the builder's operating system names, spanning
// the appropriate number of columns.
func builderSpans(s []string) []builderSpan {
	var sp []builderSpan
	for len(s) > 0 {
		i := 1
		os := builderOSOrRace(s[0])
		u := unsupportedOS(os) || strings.HasSuffix(s[0], "-temp")
		for i < len(s) && builderOSOrRace(s[i]) == os {
			i++
		}
		sp = append(sp, builderSpan{i, os, u})
		s = s[i:]
	}
	return sp
}

// builderTitle formats "linux-amd64-foo" as "linux amd64 foo".
func builderTitle(s string) string {
	return strings.Replace(s, "-", " ", -1)
}

// buildDashboards returns the known public dashboards.
func buildDashboards() []*Dashboard {
	return dashboards
}

// shortDesc returns the first line of a description.
func shortDesc(desc string) string {
	if i := strings.Index(desc, "\n"); i != -1 {
		desc = desc[:i]
	}
	return limitStringLength(desc, 100)
}

// shortHash returns a short version of a hash.
func shortHash(hash string) string {
	if len(hash) > 7 {
		hash = hash[:7]
	}
	return hash
}

// shortUser returns a shortened version of a user string.
func shortUser(user string) string {
	if i, j := strings.Index(user, "<"), strings.Index(user, ">"); 0 <= i && i < j {
		user = user[i+1 : j]
	}
	if i := strings.Index(user, "@"); i >= 0 {
		return user[:i]
	}
	return user
}

// tail returns the trailing n lines of s.
func tail(n int, s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) < n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
