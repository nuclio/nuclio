// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package maintner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/google/go-github/github"
	"github.com/gregjones/httpcache"

	"golang.org/x/build/maintner/maintpb"
	"golang.org/x/oauth2"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
)

// xFromCache is the synthetic response header added by the httpcache
// package for responses fulfilled from cache due to a 304 from the server.
const xFromCache = "X-From-Cache"

// GitHubRepoID is a GitHub org & repo, lowercase.
type GitHubRepoID struct {
	Owner, Repo string
}

func (id GitHubRepoID) String() string { return id.Owner + "/" + id.Repo }

func (id GitHubRepoID) valid() bool {
	if id.Owner == "" || id.Repo == "" {
		// TODO: more validation. whatever GitHub requires.
		return false
	}
	return true
}

// GitHub holds data about a GitHub repo.
type GitHub struct {
	c     *Corpus
	users map[int64]*GitHubUser
	teams map[int64]*GitHubTeam
	repos map[GitHubRepoID]*GitHubRepo
}

// ForeachRepo calls fn serially for each GitHubRepo, stopping if fn
// returns an error. The function is called with lexically increasing
// repo IDs.
func (g *GitHub) ForeachRepo(fn func(*GitHubRepo) error) error {
	var ids []GitHubRepoID
	for id := range g.repos {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if ids[i].Owner < ids[j].Owner {
			return true
		}
		return ids[i].Owner == ids[j].Owner && ids[i].Repo < ids[j].Repo
	})
	for _, id := range ids {
		if err := fn(g.repos[id]); err != nil {
			return err
		}
	}
	return nil
}

// Repo returns the repo if it's known. Otherwise it returns nil.
func (g *GitHub) Repo(owner, repo string) *GitHubRepo {
	return g.repos[GitHubRepoID{owner, repo}]
}

func (g *GitHub) getOrCreateRepo(owner, repo string) *GitHubRepo {
	if g == nil {
		panic("cannot call methods on nil GitHub")
	}
	id := GitHubRepoID{owner, repo}
	if !id.valid() {
		return nil
	}
	r, ok := g.repos[id]
	if ok {
		return r
	}
	r = &GitHubRepo{
		github: g,
		id:     id,
		issues: map[int32]*GitHubIssue{},
	}
	g.repos[id] = r
	return r
}

type GitHubRepo struct {
	github     *GitHub
	id         GitHubRepoID
	issues     map[int32]*GitHubIssue // num -> issue
	milestones map[int64]*GitHubMilestone
	labels     map[int64]*GitHubLabel
}

func (gr *GitHubRepo) ID() GitHubRepoID { return gr.id }

// Issue returns the the provided issue number, or nil if it's not known.
func (gr *GitHubRepo) Issue(n int32) *GitHubIssue { return gr.issues[n] }

// ForeachLabel calls fn for each label in the repo, in unsorted order.
//
// Iteration ends if fn returns an error, with that error.
func (gr *GitHubRepo) ForeachLabel(fn func(*GitHubLabel) error) error {
	for _, lb := range gr.labels {
		if err := fn(lb); err != nil {
			return err
		}
	}
	return nil
}

// ForeachMilestone calls fn for each milestone in the repo, in unsorted order.
//
// Iteration ends if fn returns an error, with that error.
func (gr *GitHubRepo) ForeachMilestone(fn func(*GitHubMilestone) error) error {
	for _, m := range gr.milestones {
		if err := fn(m); err != nil {
			return err
		}
	}
	return nil
}

// ForeachIssue calls fn for each issue in the repo.
//
// If fn returns an error, iteration ends and ForeachIssue returns
// with that error.
//
// The fn function is called serially, with increasingly numbered
// issues.
func (gr *GitHubRepo) ForeachIssue(fn func(*GitHubIssue) error) error {
	s := make([]*GitHubIssue, 0, len(gr.issues))
	for _, gi := range gr.issues {
		s = append(s, gi)
	}
	sort.Slice(s, func(i, j int) bool { return s[i].Number < s[j].Number })
	for _, gi := range s {
		if err := fn(gi); err != nil {
			return err
		}
	}
	return nil
}

func (g *GitHubRepo) getOrCreateMilestone(id int64) *GitHubMilestone {
	if id == 0 {
		panic("zero id")
	}
	m, ok := g.milestones[id]
	if ok {
		return m
	}
	if g.milestones == nil {
		g.milestones = map[int64]*GitHubMilestone{}
	}
	m = &GitHubMilestone{ID: id}
	g.milestones[id] = m
	return m
}

func (g *GitHubRepo) getOrCreateLabel(id int64) *GitHubLabel {
	if id == 0 {
		panic("zero id")
	}
	lb, ok := g.labels[id]
	if ok {
		return lb
	}
	if g.labels == nil {
		g.labels = map[int64]*GitHubLabel{}
	}
	lb = &GitHubLabel{ID: id}
	g.labels[id] = lb
	return lb
}

// GitHubUser represents a GitHub user.
// It is a subset of https://developer.github.com/v3/users/#get-a-single-user
type GitHubUser struct {
	ID    int64
	Login string
}

// GitHubTeam represents a GitHub team.
// It is a subset of https://developer.github.com/v3/orgs/teams/#get-team
type GitHubTeam struct {
	ID int64

	// Slug is a URL-friendly representation of the team name.
	// It is unique across a GitHub organization.
	Slug string
}

// GitHubIssueRef is a reference to an issue (or pull request) number
// in a repo. These are parsed from text making references such as
// "golang/go#1234" or just "#1234" (with an implicit Repo).
type GitHubIssueRef struct {
	Repo   *GitHubRepo // must be non-nil
	Number int32       // GitHubIssue.Number
}

func (r GitHubIssueRef) String() string { return fmt.Sprintf("%s#%d", r.Repo.ID(), r.Number) }

// GitHubIssue represents a GitHub issue.
// This is maintner's in-memory representation. It differs slightly
// from the API's *github.Issue type, notably in the lack of pointers
// for all fields.
// See https://developer.github.com/v3/issues/#get-a-single-issue
type GitHubIssue struct {
	ID          int64
	Number      int32
	NotExist    bool // if true, rest of fields should be ignored.
	Closed      bool
	Locked      bool
	PullRequest bool // if true, this issue is a Pull Request. All PRs are issues, but not all issues are PRs.
	User        *GitHubUser
	Assignees   []*GitHubUser
	Created     time.Time
	Updated     time.Time
	ClosedAt    time.Time
	ClosedBy    *GitHubUser // TODO(dmitshur): Implement (see golang.org/issue/28745).
	Title       string
	Body        string
	Milestone   *GitHubMilestone       // nil for unknown, noMilestone for none
	Labels      map[int64]*GitHubLabel // label ID => label

	commentsUpdatedTil time.Time                   // max comment modtime seen
	commentsSyncedAsOf time.Time                   // as of server's Date header
	comments           map[int64]*GitHubComment    // by comment.ID
	eventMaxTime       time.Time                   // latest time of any event in events map
	eventsSyncedAsOf   time.Time                   // as of server's Date header
	events             map[int64]*GitHubIssueEvent // by event.ID
}

// LastModified reports the most recent time that any known metadata was updated.
// In contrast to the Updated field, LastModified includes comments and events.
//
// TODO(bradfitz): this seems to not be working, at least events
// aren't updating it. Investigate.
func (gi *GitHubIssue) LastModified() time.Time {
	ret := gi.Updated
	if gi.commentsUpdatedTil.After(ret) {
		ret = gi.commentsUpdatedTil
	}
	if gi.eventMaxTime.After(ret) {
		ret = gi.eventMaxTime
	}
	return ret
}

// HasEvent reports whether there's any GitHubIssueEvent in this
// issue's history of the given type.
func (gi *GitHubIssue) HasEvent(eventType string) bool {
	for _, e := range gi.events {
		if e.Type == eventType {
			return true
		}
	}
	return false
}

// ForeachEvent calls fn for each event on the issue.
//
// If fn returns an error, iteration ends and ForeachEvent returns
// with that error.
//
// The fn function is called serially, in order of the event's time.
func (gi *GitHubIssue) ForeachEvent(fn func(*GitHubIssueEvent) error) error {
	// TODO: keep these sorted in the corpus
	s := make([]*GitHubIssueEvent, 0, len(gi.events))
	for _, e := range gi.events {
		s = append(s, e)
	}
	sort.Slice(s, func(i, j int) bool {
		ci, cj := s[i].Created, s[j].Created
		if ci.Before(cj) {
			return true
		}
		return ci.Equal(cj) && s[i].ID < s[j].ID
	})
	for _, e := range s {
		if err := fn(e); err != nil {
			return err
		}
	}
	return nil
}

// ForeachComment calls fn for each event on the issue.
//
// If fn returns an error, iteration ends and ForeachComment returns
// with that error.
//
// The fn function is called serially, in order of the comment's time.
func (gi *GitHubIssue) ForeachComment(fn func(*GitHubComment) error) error {
	// TODO: keep these sorted in the corpus
	s := make([]*GitHubComment, 0, len(gi.comments))
	for _, e := range gi.comments {
		s = append(s, e)
	}
	sort.Slice(s, func(i, j int) bool {
		ci, cj := s[i].Created, s[j].Created
		if ci.Before(cj) {
			return true
		}
		return ci.Equal(cj) && s[i].ID < s[j].ID
	})
	for _, e := range s {
		if err := fn(e); err != nil {
			return err
		}
	}
	return nil
}

// HasLabel reports whether the issue is labeled with the given label.
func (gi *GitHubIssue) HasLabel(label string) bool {
	for _, lb := range gi.Labels {
		if lb.Name == label {
			return true
		}
	}
	return false
}

// HasLabelID returns whether the issue has a label with the given ID.
func (gi *GitHubIssue) HasLabelID(id int64) bool {
	_, ok := gi.Labels[id]
	return ok
}

func (gi *GitHubIssue) getCreatedAt() time.Time {
	if gi == nil {
		return time.Time{}
	}
	return gi.Created
}

func (gi *GitHubIssue) getUpdatedAt() time.Time {
	if gi == nil {
		return time.Time{}
	}
	return gi.Updated
}

func (gi *GitHubIssue) getClosedAt() time.Time {
	if gi == nil {
		return time.Time{}
	}
	return gi.ClosedAt
}

// noMilestone is a sentinel value to explicitly mean no milestone.
var noMilestone = new(GitHubMilestone)

type GitHubLabel struct {
	ID   int64
	Name string
	// TODO: color?
}

// GenMutationDiff generates a diff from in-memory state 'a' (which
// may be nil) to the current (non-nil) state b from GitHub. It
// returns nil if there's no difference.
func (a *GitHubLabel) GenMutationDiff(b *github.Label) *maintpb.GithubLabel {
	id := int64(b.GetID())
	if a != nil && a.ID == id && a.Name == b.GetName() {
		// No change.
		return nil
	}
	return &maintpb.GithubLabel{Id: id, Name: b.GetName()}
}

func (lb *GitHubLabel) processMutation(mut maintpb.GithubLabel) {
	if lb.ID == 0 {
		panic("bogus label ID 0")
	}
	if lb.ID != mut.Id {
		panic(fmt.Sprintf("label ID = %v != mutation ID = %v", lb.ID, mut.Id))
	}
	if mut.Name != "" {
		lb.Name = mut.Name
	}
}

type GitHubMilestone struct {
	ID     int64
	Title  string
	Number int32
	Closed bool
}

// IsNone reports whether ms represents the sentinel "no milestone" milestone.
func (ms *GitHubMilestone) IsNone() bool { return ms == noMilestone }

// IsUnknown reports whether ms is nil, which represents the unknown
// state. Milestones should never be in this state, though.
func (ms *GitHubMilestone) IsUnknown() bool { return ms == nil }

// emptyMilestone is a non-nil *githubMilestone with zero values for
// all fields.
var emptyMilestone = new(GitHubMilestone)

// GenMutationDiff generates a diff from in-memory state 'a' (which
// may be nil) to the current (non-nil) state b from GitHub. It
// returns nil if there's no difference.
func (a *GitHubMilestone) GenMutationDiff(b *github.Milestone) *maintpb.GithubMilestone {
	var ret *maintpb.GithubMilestone // lazily inited by diff
	diff := func() *maintpb.GithubMilestone {
		if ret == nil {
			ret = &maintpb.GithubMilestone{Id: int64(b.GetID())}
		}
		return ret
	}
	if a == nil {
		a = emptyMilestone
	}
	if a.Title != b.GetTitle() {
		diff().Title = b.GetTitle()
	}
	if a.Number != int32(b.GetNumber()) {
		diff().Number = int64(b.GetNumber())
	}
	if closed := b.GetState() == "closed"; a.Closed != closed {
		diff().Closed = &maintpb.BoolChange{Val: closed}
	}
	return ret
}

func (ms *GitHubMilestone) processMutation(mut maintpb.GithubMilestone) {
	if ms.ID == 0 {
		panic("bogus milestone ID 0")
	}
	if ms.ID != mut.Id {
		panic(fmt.Sprintf("milestone ID = %v != mutation ID = %v", ms.ID, mut.Id))
	}
	if mut.Title != "" {
		ms.Title = mut.Title
	}
	if mut.Number != 0 {
		ms.Number = int32(mut.Number)
	}
	if mut.Closed != nil {
		ms.Closed = mut.Closed.Val
	}
}

type GitHubComment struct {
	ID      int64
	User    *GitHubUser
	Created time.Time
	Updated time.Time
	Body    string
}

// GitHubDismissedReview is the contents of a dismissed review event. For more
// details, see https://developer.github.com/v3/issues/events/.
type GitHubDismissedReviewEvent struct {
	ReviewID         int64
	State            string // commented, approved, changes_requested
	DismissalMessage string
}

type GitHubIssueEvent struct {
	// TODO: this struct is a little wide. change it to an interface
	// instead?  Maybe later, if memory profiling suggests it would help.

	// ID is the ID of the event.
	ID int64

	// Type is one of:
	// * labeled, unlabeled
	// * milestoned, demilestoned
	// * assigned, unassigned
	// * locked, unlocked
	// * closed
	// * referenced
	// * renamed
	// * reopened
	// * comment_deleted
	// * head_ref_restored
	// * base_ref_changed
	// * subscribed
	// * mentioned
	// * review_requested, review_request_removed, review_dismissed
	Type string

	// OtherJSON optionally contains a JSON object of GitHub's API
	// response for any fields maintner was unable to extract at
	// the time. It is empty if maintner supported all the fields
	// when the mutation was created.
	OtherJSON string

	Created time.Time
	Actor   *GitHubUser

	Label               string      // for type: "unlabeled", "labeled"
	Assignee            *GitHubUser // for type "assigned", "unassigned"
	Assigner            *GitHubUser // for type "assigned", "unassigned"
	Milestone           string      // for type: "milestoned", "demilestoned"
	From, To            string      // for type: "renamed"
	CommitID, CommitURL string      // for type: "closed", "referenced" ... ?

	Reviewer        *GitHubUser
	TeamReviewer    *GitHubTeam
	ReviewRequester *GitHubUser
	DismissedReview *GitHubDismissedReviewEvent
}

func (e *GitHubIssueEvent) Proto() *maintpb.GithubIssueEvent {
	p := &maintpb.GithubIssueEvent{
		Id:         e.ID,
		EventType:  e.Type,
		RenameFrom: e.From,
		RenameTo:   e.To,
	}
	if e.OtherJSON != "" {
		p.OtherJson = []byte(e.OtherJSON)
	}
	if !e.Created.IsZero() {
		if tp, err := ptypes.TimestampProto(e.Created); err == nil {
			p.Created = tp
		}
	}
	if e.Actor != nil {
		p.ActorId = e.Actor.ID
	}
	if e.Assignee != nil {
		p.AssigneeId = e.Assignee.ID
	}
	if e.Assigner != nil {
		p.AssignerId = e.Assigner.ID
	}
	if e.Label != "" {
		p.Label = &maintpb.GithubLabel{Name: e.Label}
	}
	if e.Milestone != "" {
		p.Milestone = &maintpb.GithubMilestone{Title: e.Milestone}
	}
	if e.CommitID != "" {
		c := &maintpb.GithubCommit{CommitId: e.CommitID}
		if m := rxGithubCommitURL.FindStringSubmatch(e.CommitURL); m != nil {
			c.Owner = m[1]
			c.Repo = m[2]
		}
		p.Commit = c
	}
	if e.Reviewer != nil {
		p.ReviewerId = e.Reviewer.ID
	}
	if e.TeamReviewer != nil {
		p.TeamReviewer = &maintpb.GithubTeam{
			Id:   e.TeamReviewer.ID,
			Slug: e.TeamReviewer.Slug,
		}
	}
	if e.ReviewRequester != nil {
		p.ReviewRequesterId = e.ReviewRequester.ID
	}
	if e.DismissedReview != nil {
		p.DismissedReview = &maintpb.GithubDismissedReviewEvent{
			ReviewId:         e.DismissedReview.ReviewID,
			State:            e.DismissedReview.State,
			DismissalMessage: e.DismissedReview.DismissalMessage,
		}
	}
	return p
}

var rxGithubCommitURL = regexp.MustCompile(`^https://api\.github\.com/repos/([^/]+)/([^/]+)/commits/`)

// r.github.c.mu must be held.
func (r *GitHubRepo) newGithubEvent(p *maintpb.GithubIssueEvent) *GitHubIssueEvent {
	g := r.github
	e := &GitHubIssueEvent{
		ID:              p.Id,
		Type:            p.EventType,
		Actor:           g.getOrCreateUserID(p.ActorId),
		Assignee:        g.getOrCreateUserID(p.AssigneeId),
		Assigner:        g.getOrCreateUserID(p.AssignerId),
		Reviewer:        g.getOrCreateUserID(p.ReviewerId),
		TeamReviewer:    g.getTeam(p.TeamReviewer),
		ReviewRequester: g.getOrCreateUserID(p.ReviewRequesterId),
		From:            p.RenameFrom,
		To:              p.RenameTo,
	}
	if p.Created != nil {
		e.Created, _ = ptypes.Timestamp(p.Created)
	}
	if len(p.OtherJson) > 0 {
		// TODO: parse it and see if we've since learned how
		// to deal with it?
		log.Printf("Unknown JSON in log: %s", p.OtherJson)
	}
	if p.Label != nil {
		e.Label = g.c.str(p.Label.Name)
	}
	if p.Milestone != nil {
		e.Milestone = g.c.str(p.Milestone.Title)
	}
	if c := p.Commit; c != nil {
		e.CommitID = c.CommitId
		if c.Owner != "" && c.Repo != "" {
			// TODO: this field is dumb. break it down.
			e.CommitURL = "https://api.github.com/repos/" + c.Owner + "/" + c.Repo + "/commits/" + c.CommitId
		}
	}
	if d := p.DismissedReview; d != nil {
		e.DismissedReview = &GitHubDismissedReviewEvent{
			ReviewID:         d.ReviewId,
			State:            d.State,
			DismissalMessage: d.DismissalMessage,
		}
	}
	return e
}

// (requires corpus be locked for reads)
func (gi *GitHubIssue) commentsSynced() bool {
	if gi.NotExist {
		// Issue doesn't exist, so can't sync its non-issues,
		// so consider it done.
		return true
	}
	return gi.commentsSyncedAsOf.After(gi.Updated)
}

// (requires corpus be locked for reads)
func (gi *GitHubIssue) eventsSynced() bool {
	if gi.NotExist {
		// Issue doesn't exist, so can't sync its non-issues,
		// so consider it done.
		return true
	}
	return gi.eventsSyncedAsOf.After(gi.Updated)
}

func (c *Corpus) initGithub() {
	if c.github != nil {
		return
	}
	c.github = &GitHub{
		c:     c,
		repos: map[GitHubRepoID]*GitHubRepo{},
	}
}

// GitHubLimiter sets a limiter that controls the rate of requests made
// to GitHub APIs. If nil, requests are not limited. Only valid in leader mode.
// The limiter must only be set before Sync or SyncLoop is called.
func (c *Corpus) SetGitHubLimiter(l *rate.Limiter) {
	c.githubLimiter = l
}

// TrackGitHub registers the named GitHub repo as a repo to
// watch and append to the mutation log. Only valid in leader mode.
// The token is the auth token to use to make API calls.
func (c *Corpus) TrackGitHub(owner, repo, token string) {
	if c.mutationLogger == nil {
		panic("can't TrackGitHub in non-leader mode")
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.initGithub()
	gr := c.github.getOrCreateRepo(owner, repo)
	if gr == nil {
		log.Fatalf("invalid github owner/repo %q/%q", owner, repo)
	}
	c.watchedGithubRepos = append(c.watchedGithubRepos, watchedGithubRepo{
		gr:    gr,
		token: token,
	})
}

type watchedGithubRepo struct {
	gr    *GitHubRepo
	token string
}

// g.c.mu must be held
func (g *GitHub) getUser(pu *maintpb.GithubUser) *GitHubUser {
	if pu == nil {
		return nil
	}
	if u := g.users[pu.Id]; u != nil {
		if pu.Login != "" && pu.Login != u.Login {
			u.Login = pu.Login
		}
		return u
	}
	if g.users == nil {
		g.users = make(map[int64]*GitHubUser)
	}
	u := &GitHubUser{
		ID:    pu.Id,
		Login: pu.Login,
	}
	g.users[pu.Id] = u
	return u
}

func (g *GitHub) getOrCreateUserID(id int64) *GitHubUser {
	if id == 0 {
		return nil
	}
	if u := g.users[id]; u != nil {
		return u
	}
	if g.users == nil {
		g.users = make(map[int64]*GitHubUser)
	}
	u := &GitHubUser{ID: id}
	g.users[id] = u
	return u
}

// g.c.mu must be held
func (g *GitHub) getTeam(pt *maintpb.GithubTeam) *GitHubTeam {
	if pt == nil {
		return nil
	}
	if g.teams == nil {
		g.teams = make(map[int64]*GitHubTeam)
	}

	t := g.teams[pt.Id]
	if t == nil {
		t = &GitHubTeam{
			ID: pt.Id,
		}
		g.teams[pt.Id] = t
	}
	if pt.Slug != "" {
		t.Slug = pt.Slug
	}
	return t
}

// newGithubUserProto creates a GithubUser with the minimum diff between
// existing and g. The return value is nil if there were no changes. existing
// may also be nil.
func newGithubUserProto(existing *GitHubUser, g *github.User) *maintpb.GithubUser {
	if g == nil {
		return nil
	}
	id := int64(g.GetID())
	if existing == nil {
		return &maintpb.GithubUser{
			Id:    id,
			Login: g.GetLogin(),
		}
	}
	hasChanges := false
	u := &maintpb.GithubUser{Id: id}
	if login := g.GetLogin(); existing.Login != login {
		u.Login = login
		hasChanges = true
	}
	// Add more fields here
	if hasChanges {
		return u
	}
	return nil
}

// deletedAssignees returns an array of user ID's that are present in existing
// but not present in new.
func deletedAssignees(existing []*GitHubUser, new []*github.User) []int64 {
	mp := make(map[int64]bool, len(existing))
	for _, u := range new {
		id := int64(u.GetID())
		mp[id] = true
	}
	toDelete := []int64{}
	for _, u := range existing {
		if _, ok := mp[u.ID]; !ok {
			toDelete = append(toDelete, u.ID)
		}
	}
	return toDelete
}

// newAssignees returns an array of diffs between existing and new. New users in
// new will be present in the returned array in their entirety. Modified users
// will appear containing only the ID field and changed fields. Unmodified users
// will not appear in the returned array.
func newAssignees(existing []*GitHubUser, new []*github.User) []*maintpb.GithubUser {
	mp := make(map[int64]*GitHubUser, len(existing))
	for _, u := range existing {
		mp[u.ID] = u
	}
	changes := []*maintpb.GithubUser{}
	for _, u := range new {
		if existingUser, ok := mp[int64(u.GetID())]; ok {
			diffUser := &maintpb.GithubUser{
				Id: int64(u.GetID()),
			}
			hasDiff := false
			if login := u.GetLogin(); existingUser.Login != login {
				diffUser.Login = login
				hasDiff = true
			}
			// check more User fields for diffs here, as we add them to the proto

			if hasDiff {
				changes = append(changes, diffUser)
			}
		} else {
			changes = append(changes, &maintpb.GithubUser{
				Id:    int64(u.GetID()),
				Login: u.GetLogin(),
			})
		}
	}
	return changes
}

// setAssigneesFromProto returns a new array of assignees according to the
// instructions in new (adds or modifies users in existing), and toDelete
// (deletes them). c.mu must be held.
func (g *GitHub) setAssigneesFromProto(existing []*GitHubUser, new []*maintpb.GithubUser, toDelete []int64) []*GitHubUser {
	c := g.c
	mp := make(map[int64]*GitHubUser)
	for _, u := range existing {
		mp[u.ID] = u
	}
	for _, u := range new {
		if existingUser, ok := mp[u.Id]; ok {
			if u.Login != "" {
				existingUser.Login = u.Login
			}
			// TODO: add other fields here when we add them for user.
		} else {
			c.debugf("adding assignee %q", u.Login)
			existing = append(existing, g.getUser(u))
		}
	}
	// IDs to delete, in descending order
	idxsToDelete := []int{}
	// this is quadratic but the number of assignees is very unlikely to exceed,
	// say, 5.
	for _, id := range toDelete {
		for i, u := range existing {
			if u.ID == id {
				idxsToDelete = append([]int{i}, idxsToDelete...)
			}
		}
	}
	for _, idx := range idxsToDelete {
		existing = append(existing[:idx], existing[idx+1:]...)
	}
	return existing
}

// githubIssueDiffer generates a minimal diff (protobuf mutation) to
// get a GitHub Issue from its in-memory state 'a' to the current
// GitHub API state 'b'.
type githubIssueDiffer struct {
	gr *GitHubRepo
	a  *GitHubIssue  // may be nil if no current state
	b  *github.Issue // may NOT be nil
}

func (d githubIssueDiffer) verbose() bool {
	return d.gr.github != nil && d.gr.github.c != nil && d.gr.github.c.verbose
}

// returns nil if no changes.
func (d githubIssueDiffer) Diff() *maintpb.GithubIssueMutation {
	var changed bool
	m := &maintpb.GithubIssueMutation{
		Owner:       d.gr.id.Owner,
		Repo:        d.gr.id.Repo,
		Number:      int32(d.b.GetNumber()),
		PullRequest: d.b.IsPullRequest(),
	}
	for _, f := range issueDiffMethods {
		if f(d, m) {
			if d.verbose() {
				fname := strings.TrimPrefix(runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name(), "golang.org/x/build/maintner.githubIssueDiffer.")
				log.Printf("Issue %d changed: %v", d.b.GetNumber(), fname)
			}
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return m
}

// issueDiffMethods are the different steps githubIssueDiffer.Diff
// goes through to compute a diff. The methods should return true if
// any change was made. The order is irrelevant unless otherwise
// documented in comments in the list below.
var issueDiffMethods = []func(githubIssueDiffer, *maintpb.GithubIssueMutation) bool{
	githubIssueDiffer.diffCreatedAt,
	githubIssueDiffer.diffUpdatedAt,
	githubIssueDiffer.diffUser,
	githubIssueDiffer.diffBody,
	githubIssueDiffer.diffTitle,
	githubIssueDiffer.diffMilestone,
	githubIssueDiffer.diffAssignees,
	githubIssueDiffer.diffClosedState,
	githubIssueDiffer.diffClosedAt,
	githubIssueDiffer.diffClosedBy,
	githubIssueDiffer.diffLockedState,
	githubIssueDiffer.diffLabels,
}

func (d githubIssueDiffer) diffCreatedAt(m *maintpb.GithubIssueMutation) bool {
	return d.diffTimeField(&m.Created, d.a.getCreatedAt(), d.b.GetCreatedAt())
}

func (d githubIssueDiffer) diffUpdatedAt(m *maintpb.GithubIssueMutation) bool {
	return d.diffTimeField(&m.Updated, d.a.getUpdatedAt(), d.b.GetUpdatedAt())
}

func (d githubIssueDiffer) diffClosedAt(m *maintpb.GithubIssueMutation) bool {
	return d.diffTimeField(&m.ClosedAt, d.a.getClosedAt(), d.b.GetClosedAt())
}

func (d githubIssueDiffer) diffTimeField(dst **timestamp.Timestamp, memTime, githubTime time.Time) bool {
	if githubTime.IsZero() || memTime.Equal(githubTime) {
		return false
	}
	tproto, err := ptypes.TimestampProto(githubTime)
	if err != nil {
		panic(err)
	}
	*dst = tproto
	return true
}

func (d githubIssueDiffer) diffUser(m *maintpb.GithubIssueMutation) bool {
	var existing *GitHubUser
	if d.a != nil {
		existing = d.a.User
	}
	m.User = newGithubUserProto(existing, d.b.User)
	return m.User != nil
}

func (d githubIssueDiffer) diffClosedBy(m *maintpb.GithubIssueMutation) bool {
	var existing *GitHubUser
	if d.a != nil {
		existing = d.a.ClosedBy
	}
	m.ClosedBy = newGithubUserProto(existing, d.b.ClosedBy)
	return m.ClosedBy != nil
}

func (d githubIssueDiffer) diffBody(m *maintpb.GithubIssueMutation) bool {
	if d.a != nil && d.a.Body == d.b.GetBody() {
		return false
	}
	m.Body = d.b.GetBody()
	return true
}

func (d githubIssueDiffer) diffTitle(m *maintpb.GithubIssueMutation) bool {
	if d.a != nil && d.a.Title == d.b.GetTitle() {
		return false
	}
	m.Title = d.b.GetTitle()
	return true
}

func (d githubIssueDiffer) diffMilestone(m *maintpb.GithubIssueMutation) bool {
	if d.a != nil && d.a.Milestone != nil {
		ma, mb := d.a.Milestone, d.b.Milestone
		if ma == noMilestone && d.b.Milestone == nil {
			// Unchanged. Still no milestone.
			return false
		}
		if mb != nil && ma.ID == int64(mb.GetID()) {
			// Unchanged. Same milestone.
			// TODO: detect milestone renames and emit mutation for that?
			return false
		}

	}
	if mb := d.b.Milestone; mb != nil {
		m.MilestoneId = int64(mb.GetID())
		m.MilestoneNum = int64(mb.GetNumber())
		m.MilestoneTitle = mb.GetTitle()
	} else {
		m.NoMilestone = true
	}
	return true
}

func (d githubIssueDiffer) diffAssignees(m *maintpb.GithubIssueMutation) bool {
	if d.a == nil {
		m.Assignees = newAssignees(nil, d.b.Assignees)
		return true
	}
	m.Assignees = newAssignees(d.a.Assignees, d.b.Assignees)
	m.DeletedAssignees = deletedAssignees(d.a.Assignees, d.b.Assignees)
	return len(m.Assignees) > 0 || len(m.DeletedAssignees) > 0
}

func (d githubIssueDiffer) diffLabels(m *maintpb.GithubIssueMutation) bool {
	// Common case: no changes. Return false quickly without allocations.
	if d.a != nil && len(d.a.Labels) == len(d.b.Labels) {
		missing := false
		for _, gl := range d.b.Labels {
			if _, ok := d.a.Labels[int64(gl.GetID())]; !ok {
				missing = true
				break
			}
		}
		if !missing {
			return false
		}
	}

	toAdd := map[int64]*maintpb.GithubLabel{}
	for _, gl := range d.b.Labels {
		id := int64(gl.GetID())
		if id == 0 {
			panic("zero label ID")
		}
		toAdd[id] = &maintpb.GithubLabel{Id: id, Name: gl.GetName()}
	}

	var toDelete []int64
	if d.a != nil {
		for id := range d.a.Labels {
			if _, ok := toAdd[id]; ok {
				// Already had it.
				delete(toAdd, id)
			} else {
				// We had it, but no longer.
				toDelete = append(toDelete, id)
			}
		}
	}

	m.RemoveLabel = toDelete
	for _, labpb := range toAdd {
		m.AddLabel = append(m.AddLabel, labpb)
	}

	return len(m.RemoveLabel) > 0 || len(m.AddLabel) > 0
}

func (d githubIssueDiffer) diffClosedState(m *maintpb.GithubIssueMutation) bool {
	bclosed := d.b.GetState() == "closed"
	if d.a != nil && d.a.Closed == bclosed {
		return false
	}
	m.Closed = &maintpb.BoolChange{Val: bclosed}
	return true
}

func (d githubIssueDiffer) diffLockedState(m *maintpb.GithubIssueMutation) bool {
	if d.a != nil && d.a.Locked == d.b.GetLocked() {
		return false
	}
	if d.a == nil && !d.b.GetLocked() {
		return false
	}
	m.Locked = &maintpb.BoolChange{Val: d.b.GetLocked()}
	return true
}

// newMutationFromIssue generates a GithubIssueMutation using the
// smallest possible diff between a (the state we have in memory in
// the corpus) and b (the current GitHub API state).
//
// If newMutationFromIssue returns nil, the provided github.Issue is no newer
// than the data we have in the corpus. 'a'. may be nil.
func (r *GitHubRepo) newMutationFromIssue(a *GitHubIssue, b *github.Issue) *maintpb.Mutation {
	if b == nil || b.Number == nil {
		panic(fmt.Sprintf("github issue with nil number: %#v", b))
	}
	gim := githubIssueDiffer{gr: r, a: a, b: b}.Diff()
	if gim == nil {
		// No changes.
		return nil
	}
	return &maintpb.Mutation{GithubIssue: gim}
}

func (r *GitHubRepo) missingIssues() []int32 {
	c := r.github.c
	c.mu.RLock()
	defer c.mu.RUnlock()

	var maxNum int32
	for num := range r.issues {
		if num > maxNum {
			maxNum = num
		}
	}

	var missing []int32
	for num := int32(1); num < maxNum; num++ {
		if _, ok := r.issues[num]; !ok {
			missing = append(missing, num)
		}
	}
	return missing
}

// processGithubMutation updates the corpus with the information in m.
func (c *Corpus) processGithubMutation(m *maintpb.GithubMutation) {
	if c == nil {
		panic("nil corpus")
	}
	c.initGithub()
	gr := c.github.getOrCreateRepo(m.Owner, m.Repo)
	if gr == nil {
		log.Printf("bogus Owner/Repo %q/%q in mutation: %v", m.Owner, m.Repo, m)
		return
	}
	for _, lp := range m.Labels {
		lb := gr.getOrCreateLabel(lp.Id)
		lb.processMutation(*lp)
	}
	for _, mp := range m.Milestones {
		ms := gr.getOrCreateMilestone(mp.Id)
		ms.processMutation(*mp)
	}
}

// processGithubIssueMutation updates the corpus with the information in m.
func (c *Corpus) processGithubIssueMutation(m *maintpb.GithubIssueMutation) {
	if c == nil {
		panic("nil corpus")
	}
	c.initGithub()
	gr := c.github.getOrCreateRepo(m.Owner, m.Repo)
	if gr == nil {
		log.Printf("bogus Owner/Repo %q/%q in mutation: %v", m.Owner, m.Repo, m)
		return
	}
	if m.Number == 0 {
		log.Printf("bogus zero Number in mutation: %v", m)
		return
	}
	gi, ok := gr.issues[m.Number]
	if !ok {
		gi = &GitHubIssue{
			// User added below
			Number: m.Number,
			ID:     m.Id,
		}
		if gr.issues == nil {
			gr.issues = make(map[int32]*GitHubIssue)
		}
		gr.issues[m.Number] = gi

		if m.NotExist {
			gi.NotExist = true
			return
		}

		var err error
		gi.Created, err = ptypes.Timestamp(m.Created)
		if err != nil {
			panic(err)
		}
	}
	if m.NotExist != gi.NotExist {
		gi.NotExist = m.NotExist
	}
	if gi.NotExist {
		return
	}

	// Check Updated before all other fields so they don't update if this
	// Mutation is stale
	// (ignoring Created since it *should* never update)
	if m.Updated != nil {
		t, err := ptypes.Timestamp(m.Updated)
		if err != nil {
			panic(err)
		}
		gi.Updated = t
	}
	if m.ClosedAt != nil {
		t, err := ptypes.Timestamp(m.ClosedAt)
		if err != nil {
			panic(err)
		}
		gi.ClosedAt = t
	}
	if m.User != nil {
		gi.User = c.github.getUser(m.User)
	}
	if m.NoMilestone {
		gi.Milestone = noMilestone
	} else if m.MilestoneId != 0 {
		ms := gr.getOrCreateMilestone(m.MilestoneId)
		ms.processMutation(maintpb.GithubMilestone{
			Id:     m.MilestoneId,
			Title:  m.MilestoneTitle,
			Number: m.MilestoneNum,
		})
		gi.Milestone = ms
	}
	if m.ClosedBy != nil {
		gi.ClosedBy = c.github.getUser(m.ClosedBy)
	}
	if b := m.Closed; b != nil {
		gi.Closed = b.Val
	}
	if b := m.Locked; b != nil {
		gi.Locked = b.Val
	}
	if m.PullRequest {
		gi.PullRequest = true
	}

	gi.Assignees = c.github.setAssigneesFromProto(gi.Assignees, m.Assignees, m.DeletedAssignees)

	if m.Body != "" {
		gi.Body = m.Body
	}
	if m.Title != "" {
		gi.Title = m.Title
	}
	if len(m.RemoveLabel) > 0 || len(m.AddLabel) > 0 {
		if gi.Labels == nil {
			gi.Labels = make(map[int64]*GitHubLabel)
		}
		for _, lid := range m.RemoveLabel {
			delete(gi.Labels, lid)
		}
		for _, lp := range m.AddLabel {
			lb := gr.getOrCreateLabel(lp.Id)
			lb.processMutation(*lp)
			gi.Labels[lp.Id] = lb
		}
	}

	for _, cmut := range m.Comment {
		if cmut.Id == 0 {
			log.Printf("Ignoring bogus comment mutation lacking Id: %v", cmut)
			continue
		}
		gc, ok := gi.comments[cmut.Id]
		if !ok {
			if gi.comments == nil {
				gi.comments = make(map[int64]*GitHubComment)
			}
			gc = &GitHubComment{ID: cmut.Id}
			gi.comments[gc.ID] = gc
		}
		if cmut.User != nil {
			gc.User = c.github.getUser(cmut.User)
		}
		if cmut.Created != nil {
			gc.Created, _ = ptypes.Timestamp(cmut.Created)
			gc.Created = gc.Created.UTC()
		}
		if cmut.Updated != nil {
			gc.Updated, _ = ptypes.Timestamp(cmut.Updated)
			gc.Updated = gc.Updated.UTC()
		}
		if cmut.Body != "" {
			gc.Body = cmut.Body
		}
	}
	if m.CommentStatus != nil && m.CommentStatus.ServerDate != nil {
		if serverDate, err := ptypes.Timestamp(m.CommentStatus.ServerDate); err == nil {
			gi.commentsSyncedAsOf = serverDate.UTC()
		}
	}

	for _, emut := range m.Event {
		if emut.Id == 0 {
			log.Printf("Ignoring bogus event mutation lacking Id: %v", emut)
			continue
		}
		if gi.events == nil {
			gi.events = make(map[int64]*GitHubIssueEvent)
		}
		gie := gr.newGithubEvent(emut)
		gi.events[emut.Id] = gie
		if gie.Created.After(gi.eventMaxTime) {
			gi.eventMaxTime = gie.Created
		}
	}
	if m.EventStatus != nil && m.EventStatus.ServerDate != nil {
		if serverDate, err := ptypes.Timestamp(m.EventStatus.ServerDate); err == nil {
			gi.eventsSyncedAsOf = serverDate.UTC()
		}
	}
}

// githubCache is an httpcache.Cache wrapper that only
// stores responses for:
//   * https://api.github.com/repos/$OWNER/$REPO/issues?direction=desc&page=1&sort=updated
//   * https://api.github.com/repos/$OWNER/$REPO/milestones?page=1
//   * https://api.github.com/repos/$OWNER/$REPO/labels?page=1
type githubCache struct {
	httpcache.Cache
}

var rxGithubCacheURLs = regexp.MustCompile(`^https://api.github.com/repos/\w+/\w+/(issues|milestones|labels)\?(.+)`)

func cacheableURL(urlStr string) bool {
	m := rxGithubCacheURLs.FindStringSubmatch(urlStr)
	if m == nil {
		return false
	}
	v, _ := url.ParseQuery(m[2])
	if v.Get("page") != "1" {
		return false
	}
	switch m[1] {
	case "issues":
		return v.Get("sort") == "updated" && v.Get("direction") == "desc"
	case "milestones", "labels":
		return true
	default:
		panic("unexpected cache key base " + m[1])
	}
}

func (c *githubCache) Set(urlKey string, res []byte) {
	// TODO: verify that the httpcache package guarantees that the
	// first string parameter to Set here is actually a
	// URL. Empirically they appear to be.
	if cacheableURL(urlKey) {
		c.Cache.Set(urlKey, res)
	}
}

// sync checks for new changes on a single GitHub repository and
// updates the Corpus with any changes. If loop is true, it runs
// forever.
func (gr *GitHubRepo) sync(ctx context.Context, token string, loop bool) error {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	hc := oauth2.NewClient(ctx, ts)
	if tr, ok := hc.Transport.(*http.Transport); ok {
		defer tr.CloseIdleConnections()
	}
	directTransport := hc.Transport
	if gr.github.c.githubLimiter != nil {
		directTransport = limitTransport{gr.github.c.githubLimiter, hc.Transport}
	}
	cachingTransport := &httpcache.Transport{
		Transport:           directTransport,
		Cache:               &githubCache{Cache: httpcache.NewMemoryCache()},
		MarkCachedResponses: true, // adds "X-From-Cache: 1" response header.
	}

	p := &githubRepoPoller{
		c:             gr.github.c,
		token:         token,
		gr:            gr,
		githubDirect:  github.NewClient(&http.Client{Transport: directTransport}),
		githubCaching: github.NewClient(&http.Client{Transport: cachingTransport}),
	}
	activityCh := gr.github.c.activityChan("github:" + gr.id.String())
	var expectChanges bool // got webhook update, but haven't seen new data yet
	var sleepDelay time.Duration
	for {
		prevLastUpdate := p.lastUpdate
		err := p.sync(ctx, expectChanges)
		if err == context.Canceled || !loop {
			return err
		}
		sawChanges := !p.lastUpdate.Equal(prevLastUpdate)
		if sawChanges {
			expectChanges = false
		}
		// If we got woken up by a webhook, sometimes
		// immediately polling GitHub for the data results in
		// a cache hit saying nothing's changed. Don't believe
		// it. Polling quickly with exponential backoff until
		// we see what we're expecting.
		if expectChanges {
			if sleepDelay == 0 {
				sleepDelay = 1 * time.Second
			} else {
				sleepDelay *= 2
				if sleepDelay > 15*time.Minute {
					sleepDelay = 15 * time.Minute
				}
			}
			p.logf("expect changes; re-polling in %v", sleepDelay)
		} else {
			sleepDelay = 15 * time.Minute
		}
		p.logf("sync = %v; sleeping", err)
		timer := time.NewTimer(sleepDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-activityCh:
			timer.Stop()
			expectChanges = true
			sleepDelay = 0
		case <-timer.C:
		}
	}
}

// A githubRepoPoller updates the Corpus (gr.c) to have the latest
// version of the GitHub repo rp, using the GitHub client ghc.
type githubRepoPoller struct {
	c             *Corpus // shortcut for gr.github.c
	gr            *GitHubRepo
	token         string
	lastUpdate    time.Time // modified by sync
	githubCaching *github.Client
	githubDirect  *github.Client // not caching
}

func (p *githubRepoPoller) Owner() string { return p.gr.id.Owner }
func (p *githubRepoPoller) Repo() string  { return p.gr.id.Repo }

func (p *githubRepoPoller) logf(format string, args ...interface{}) {
	log.Printf("sync github "+p.gr.id.String()+": "+format, args...)
}

func (p *githubRepoPoller) sync(ctx context.Context, expectChanges bool) error {
	p.logf("Beginning sync.")
	if err := p.syncIssues(ctx, expectChanges); err != nil {
		return err
	}
	if err := p.syncComments(ctx); err != nil {
		return err
	}
	if err := p.syncEvents(ctx); err != nil {
		return err
	}
	return nil
}

func (p *githubRepoPoller) syncMilestones(ctx context.Context) error {
	var mut *maintpb.GithubMutation // lazy init
	var changes int
	err := p.foreachItem(ctx, 1, p.getMilestonePage, func(e interface{}) error {
		ms := e.(*github.Milestone)
		id := int64(ms.GetID())
		p.c.mu.RLock()
		diff := p.gr.milestones[id].GenMutationDiff(ms)
		p.c.mu.RUnlock()
		if diff == nil {
			return nil
		}
		if mut == nil {
			mut = &maintpb.GithubMutation{
				Owner: p.Owner(),
				Repo:  p.Repo(),
			}
		}
		mut.Milestones = append(mut.Milestones, diff)
		changes++
		return nil
	})
	if err != nil {
		return err
	}
	p.logf("%d milestone changes.", changes)
	if changes == 0 {
		return nil
	}
	p.c.addMutation(&maintpb.Mutation{Github: mut})
	return nil
}

func (p *githubRepoPoller) syncLabels(ctx context.Context) error {
	var mut *maintpb.GithubMutation // lazy init
	var changes int
	err := p.foreachItem(ctx, 1, p.getLabelPage, func(e interface{}) error {
		lb := e.(*github.Label)
		id := int64(lb.GetID())
		p.c.mu.RLock()
		diff := p.gr.labels[id].GenMutationDiff(lb)
		p.c.mu.RUnlock()
		if diff == nil {
			return nil
		}
		if mut == nil {
			mut = &maintpb.GithubMutation{
				Owner: p.Owner(),
				Repo:  p.Repo(),
			}
		}
		mut.Labels = append(mut.Labels, diff)
		changes++
		return nil
	})
	if err != nil {
		return err
	}
	p.logf("%d label changes.", changes)
	if changes == 0 {
		return nil
	}
	p.c.addMutation(&maintpb.Mutation{Github: mut})
	return nil
}

func (p *githubRepoPoller) getMilestonePage(ctx context.Context, page int) ([]interface{}, *github.Response, error) {
	ms, res, err := p.githubCaching.Issues.ListMilestones(ctx, p.Owner(), p.Repo(), &github.MilestoneListOptions{
		State:       "all",
		ListOptions: github.ListOptions{Page: page},
	})
	if err != nil {
		return nil, nil, err
	}
	its := make([]interface{}, len(ms))
	for i, m := range ms {
		its[i] = m
	}
	return its, res, err
}

func (p *githubRepoPoller) getLabelPage(ctx context.Context, page int) ([]interface{}, *github.Response, error) {
	ls, res, err := p.githubCaching.Issues.ListLabels(ctx, p.Owner(), p.Repo(), &github.ListOptions{
		Page: page,
	})
	if err != nil {
		return nil, nil, err
	}
	its := make([]interface{}, len(ls))
	for i, lb := range ls {
		its[i] = lb
	}
	return its, res, err
}

// foreach walks over all pages of items from getPage and calls fn for each item.
// If the first page's response was cached, fn is never called.
func (p *githubRepoPoller) foreachItem(
	ctx context.Context,
	page int,
	getPage func(ctx context.Context, page int) ([]interface{}, *github.Response, error),
	fn func(interface{}) error) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		items, res, err := getPage(ctx, page)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		fromCache := page == 1 && res.Response.Header.Get(xFromCache) == "1"
		if fromCache {
			log.Printf("no new items of type %T", items[0])
			// No need to walk over these again.
			return nil
		}
		// TODO: use res.Rate (sleep until Reset if Limit == 0)
		for _, it := range items {
			if err := fn(it); err != nil {
				return err
			}
		}
		if res.NextPage == 0 {
			return nil
		}
		page = res.NextPage
	}
}

func (p *githubRepoPoller) syncIssues(ctx context.Context, expectChanges bool) error {
	c := p.gr.github.c
	page := 1
	seen := make(map[int64]bool)
	keepGoing := true
	owner, repo := p.gr.id.Owner, p.gr.id.Repo
	for keepGoing {
		ghc := p.githubCaching
		if expectChanges {
			ghc = p.githubDirect
		}
		issues, res, err := ghc.Issues.ListByRepo(ctx, owner, repo, &github.IssueListByRepoOptions{
			State:     "all",
			Sort:      "updated",
			Direction: "desc",
			ListOptions: github.ListOptions{
				Page:    page,
				PerPage: 100,
			},
		})
		if err != nil {
			return err
		}
		// See https://developer.github.com/v3/activity/events/ for X-Poll-Interval:
		if pi := res.Response.Header.Get("X-Poll-Interval"); pi != "" {
			nsec, _ := strconv.Atoi(pi)
			d := time.Duration(nsec) * time.Second
			p.logf("Requested to adjust poll interval to %v", d)
			// TODO: return an error type up that the sync loop can use
			// to adjust its default interval.
			// For now, ignore.
		}
		fromCache := res.Response.Header.Get(xFromCache) == "1"
		if len(issues) == 0 {
			p.logf("issues: reached end.")
			break
		}

		didMilestoneLabelSync := false
		changes := 0
		for _, is := range issues {
			id := int64(is.GetID())
			if seen[id] {
				// If an issue gets updated (and bumped to the top) while we
				// are paging, it's possible the last issue from page N can
				// appear as the first issue on page N+1. Don't process that
				// issue twice.
				// https://github.com/google/go-github/issues/566
				continue
			}
			seen[id] = true

			var mp *maintpb.Mutation
			c.mu.RLock()
			{
				gi := p.gr.issues[int32(*is.Number)]
				mp = p.gr.newMutationFromIssue(gi, is)
			}
			c.mu.RUnlock()

			if mp == nil {
				continue
			}

			// If there's something new (not a cached response),
			// then check for updated milestones and labels before
			// creating issue mutations below. Doesn't matter
			// much, but helps to have it all loaded.
			if !fromCache && !didMilestoneLabelSync {
				didMilestoneLabelSync = true
				group, ctx := errgroup.WithContext(ctx)
				group.Go(func() error { return p.syncMilestones(ctx) })
				group.Go(func() error { return p.syncLabels(ctx) })
				if err := group.Wait(); err != nil {
					return err
				}
			}

			changes++
			p.logf("changed issue %d: %s", is.GetNumber(), is.GetTitle())
			c.addMutation(mp)
			p.lastUpdate = time.Now()
		}

		if changes == 0 {
			missing := p.gr.missingIssues()
			if len(missing) == 0 {
				p.logf("no changed issues; cached=%v", fromCache)
				return nil
			}
			if len(missing) > 0 {
				p.logf("%d missing github issues.", len(missing))
			}
			if len(missing) < 100 {
				keepGoing = false
			}
		}

		c.mu.RLock()
		num := len(p.gr.issues)
		c.mu.RUnlock()
		p.logf("After page %d: %v issues, %v changes, %v issues in memory", page, len(issues), changes, num)

		page++
	}

	missing := p.gr.missingIssues()
	if len(missing) > 0 {
		p.logf("remaining issues: %v", missing)
		for _, num := range missing {
			p.logf("getting issue %v ...", num)
			issue, _, err := p.githubDirect.Issues.Get(ctx, owner, repo, int(num))
			if ge, ok := err.(*github.ErrorResponse); ok && ge.Message == "Not Found" {
				mp := &maintpb.Mutation{
					GithubIssue: &maintpb.GithubIssueMutation{
						Owner:    owner,
						Repo:     repo,
						Number:   num,
						NotExist: true,
					},
				}
				c.addMutation(mp)
				continue
			} else if err != nil {
				return err
			}
			mp := p.gr.newMutationFromIssue(nil, issue)
			if mp == nil {
				continue
			}
			p.logf("modified issue %d: %s", issue.GetNumber(), issue.GetTitle())
			c.addMutation(mp)
			p.lastUpdate = time.Now()
		}
	}

	return nil
}

func (p *githubRepoPoller) issueNumbersWithStaleCommentSync() (issueNums []int32) {
	p.c.mu.RLock()
	defer p.c.mu.RUnlock()

	for n, gi := range p.gr.issues {
		if !gi.commentsSynced() {
			issueNums = append(issueNums, n)
		}
	}
	sort.Slice(issueNums, func(i, j int) bool {
		return issueNums[i] < issueNums[j]
	})
	return issueNums
}

func (p *githubRepoPoller) syncComments(ctx context.Context) error {
	for {
		nums := p.issueNumbersWithStaleCommentSync()
		if len(nums) == 0 {
			return nil
		}
		remain := len(nums)
		for _, num := range nums {
			p.logf("comment sync: %d issues remaining; syncing issue %v", remain, num)
			if err := p.syncCommentsOnIssue(ctx, num); err != nil {
				p.logf("comment sync on issue %d: %v", num, err)
				return err
			}
			remain--
		}
	}
}

func (p *githubRepoPoller) syncCommentsOnIssue(ctx context.Context, issueNum int32) error {
	p.c.mu.RLock()
	issue := p.gr.issues[issueNum]
	if issue == nil {
		p.c.mu.RUnlock()
		return fmt.Errorf("unknown issue number %v", issueNum)
	}
	since := issue.commentsUpdatedTil
	p.c.mu.RUnlock()

	owner, repo := p.gr.id.Owner, p.gr.id.Repo
	morePages := true // at least try the first. might be empty.
	for morePages {
		ics, res, err := p.githubDirect.Issues.ListComments(ctx, owner, repo, int(issueNum), &github.IssueListCommentsOptions{
			Since:       since,
			Direction:   "asc",
			Sort:        "updated",
			ListOptions: github.ListOptions{PerPage: 100},
		})
		// TODO: use res.Rate.* (https://godoc.org/github.com/google/go-github/github#Rate) to sleep
		// and retry if we're out of tokens. Probably need to make an HTTP RoundTripper that does
		// that automatically.
		if err != nil {
			return err
		}
		serverDate, err := http.ParseTime(res.Header.Get("Date"))
		if err != nil {
			return fmt.Errorf("invalid server Date response: %v", err)
		}
		serverDate = serverDate.UTC()
		p.logf("Number of comments on issue %d since %v: %v", issueNum, since, len(ics))

		mut := &maintpb.Mutation{
			GithubIssue: &maintpb.GithubIssueMutation{
				Owner:  owner,
				Repo:   repo,
				Number: issueNum,
			},
		}

		p.c.mu.RLock()
		for _, ic := range ics {
			if ic.ID == nil || ic.Body == nil || ic.User == nil || ic.CreatedAt == nil || ic.UpdatedAt == nil {
				// Bogus.
				p.logf("bogus comment: %v", ic)
				continue
			}
			created, err := ptypes.TimestampProto(*ic.CreatedAt)
			if err != nil {
				continue
			}
			updated, err := ptypes.TimestampProto(*ic.UpdatedAt)
			if err != nil {
				continue
			}
			since = *ic.UpdatedAt // for next round

			id := int64(*ic.ID)
			cur := issue.comments[id]

			// TODO: does a reaction update a comment's UpdatedAt time?
			var cmut *maintpb.GithubIssueCommentMutation
			if cur == nil {
				cmut = &maintpb.GithubIssueCommentMutation{
					Id: id,
					User: &maintpb.GithubUser{
						Id:    int64(*ic.User.ID),
						Login: *ic.User.Login,
					},
					Body:    *ic.Body,
					Created: created,
					Updated: updated,
				}
			} else if !cur.Updated.Equal(*ic.UpdatedAt) || cur.Body != *ic.Body {
				cmut = &maintpb.GithubIssueCommentMutation{
					Id: id,
				}
				if !cur.Updated.Equal(*ic.UpdatedAt) {
					cmut.Updated = updated
				}
				if cur.Body != *ic.Body {
					cmut.Body = *ic.Body
				}
			}
			if cmut != nil {
				mut.GithubIssue.Comment = append(mut.GithubIssue.Comment, cmut)
			}
		}
		p.c.mu.RUnlock()

		if res.NextPage == 0 {
			sdp, _ := ptypes.TimestampProto(serverDate)
			mut.GithubIssue.CommentStatus = &maintpb.GithubIssueSyncStatus{ServerDate: sdp}
			morePages = false
		}

		p.c.addMutation(mut)
	}
	return nil
}

func (p *githubRepoPoller) issueNumbersWithStaleEventSync() (issueNums []int32) {
	p.c.mu.RLock()
	defer p.c.mu.RUnlock()

	for n, gi := range p.gr.issues {
		if !gi.eventsSynced() {
			issueNums = append(issueNums, n)
		}
	}
	sort.Slice(issueNums, func(i, j int) bool {
		return issueNums[i] < issueNums[j]
	})
	return issueNums
}

func (p *githubRepoPoller) syncEvents(ctx context.Context) error {
	for {
		nums := p.issueNumbersWithStaleEventSync()
		if len(nums) == 0 {
			return nil
		}
		remain := len(nums)
		for _, num := range nums {
			p.logf("event sync: %d issues remaining; syncing issue %v", remain, num)
			if err := p.syncEventsOnIssue(ctx, num); err != nil {
				p.logf("event sync on issue %d: %v", num, err)
				return err
			}
			remain--
		}
	}
}

func (p *githubRepoPoller) syncEventsOnIssue(ctx context.Context, issueNum int32) error {
	const perPage = 100
	p.c.mu.RLock()
	gi := p.gr.issues[issueNum]
	if gi == nil {
		panic(fmt.Sprintf("bogus issue %v", issueNum))
	}
	have := len(gi.events)
	p.c.mu.RUnlock()

	skipPages := have / perPage

	mut := &maintpb.Mutation{
		GithubIssue: &maintpb.GithubIssueMutation{
			Owner:  p.Owner(),
			Repo:   p.Repo(),
			Number: issueNum,
		},
	}

	err := p.foreachItem(ctx,
		1+skipPages,
		func(ctx context.Context, page int) ([]interface{}, *github.Response, error) {
			u := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%v/events?per_page=%v&page=%v",
				p.Owner(), p.Repo(), issueNum, perPage, page)
			req, _ := http.NewRequest("GET", u, nil)

			req.Header.Set("Authorization", "Bearer "+p.token)
			req.Header.Set("User-Agent", "golang-x-build-maintner/1.0")
			ctx, cancel := context.WithTimeout(ctx, time.Minute)
			defer cancel()
			req = req.WithContext(ctx)
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				log.Printf("Fetching %s: %v", u, err)
				return nil, nil, err
			}
			log.Printf("Fetching %s: %v", u, res.Status)
			if res.StatusCode != http.StatusOK {
				log.Printf("Fetching %s: %v: %+v", u, res.Status, res.Header)
				// TODO: rate limiting, etc.
				return nil, nil, fmt.Errorf("%s: %v", u, res.Status)
			}
			evts, err := parseGithubEvents(res.Body)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: parse github events: %v", u, err)
			}
			is := make([]interface{}, len(evts))
			for i, v := range evts {
				is[i] = v
			}
			serverDate, err := http.ParseTime(res.Header.Get("Date"))
			if err != nil {
				return nil, nil, fmt.Errorf("invalid server Date response: %v", err)
			}
			sdp, _ := ptypes.TimestampProto(serverDate.UTC())
			mut.GithubIssue.EventStatus = &maintpb.GithubIssueSyncStatus{ServerDate: sdp}

			return is, makeGithubResponse(res), err
		},
		func(v interface{}) error {
			ge := v.(*GitHubIssueEvent)
			p.c.mu.RLock()
			_, ok := gi.events[ge.ID]
			p.c.mu.RUnlock()
			if ok {
				// Already have it. And they're
				// assumed to be immutable, so the
				// copy we already have should be
				// good. Don't add to mutation log.
				return nil
			}
			mut.GithubIssue.Event = append(mut.GithubIssue.Event, ge.Proto())
			return nil
		})
	if err != nil {
		return err
	}
	p.c.addMutation(mut)
	return nil
}

// parseGithubEvents parses the JSON array of GitHub events in r.  It
// does this the very manual way (using map[string]interface{})
// instead of using nice types because https://golang.org/issue/15314
// isn't implemented yet and also because even if it were implemented,
// this code still wants to preserve any unknown fields to store in
// the "OtherJSON" field for future updates of the code to parse. (If
// GitHub adds new Event types in the future, we want to archive them,
// even if we don't understand them)
func parseGithubEvents(r io.Reader) ([]*GitHubIssueEvent, error) {
	var jevents []map[string]interface{}
	jd := json.NewDecoder(r)
	jd.UseNumber()
	if err := jd.Decode(&jevents); err != nil {
		return nil, err
	}
	var evts []*GitHubIssueEvent
	for _, em := range jevents {
		for k, v := range em {
			if v == nil {
				delete(em, k)
			}
		}
		delete(em, "url")

		e := &GitHubIssueEvent{}

		e.Type, _ = em["event"].(string)
		delete(em, "event")

		e.ID = jint64(em["id"])
		delete(em, "id")

		// TODO: store these two more compactly:
		e.CommitID, _ = em["commit_id"].(string) // "5383ecf5a0824649ffcc0349f00f0317575753d0"
		delete(em, "commit_id")
		e.CommitURL, _ = em["commit_url"].(string) // "https://api.github.com/repos/bradfitz/go-issue-mirror/commits/5383ecf5a0824649ffcc0349f00f0317575753d0"
		delete(em, "commit_url")

		getUser := func(field string, gup **GitHubUser) {
			am, ok := em[field].(map[string]interface{})
			if !ok {
				return
			}
			delete(em, field)
			gu := &GitHubUser{ID: jint64(am["id"])}
			gu.Login, _ = am["login"].(string)
			*gup = gu
		}

		getUser("actor", &e.Actor)
		getUser("assignee", &e.Assignee)
		getUser("assigner", &e.Assigner)
		getUser("requested_reviewer", &e.Reviewer)
		getUser("review_requester", &e.ReviewRequester)

		if lm, ok := em["label"].(map[string]interface{}); ok {
			delete(em, "label")
			e.Label, _ = lm["name"].(string)
		}

		if mm, ok := em["milestone"].(map[string]interface{}); ok {
			delete(em, "milestone")
			e.Milestone, _ = mm["title"].(string)
		}

		if rm, ok := em["rename"].(map[string]interface{}); ok {
			delete(em, "rename")
			e.From, _ = rm["from"].(string)
			e.To, _ = rm["to"].(string)
		}

		if createdStr, ok := em["created_at"].(string); ok {
			delete(em, "created_at")
			var err error
			e.Created, err = time.Parse(time.RFC3339, createdStr)
			if err != nil {
				return nil, err
			}
			e.Created = e.Created.UTC()
		}
		if dr, ok := em["dismissed_review"]; ok {
			delete(em, "dismissed_review")
			drm := dr.(map[string]interface{})
			dro := &GitHubDismissedReviewEvent{}
			dro.ReviewID = jint64(drm["review_id"])
			if state, ok := drm["state"].(string); ok {
				dro.State = state
			} else {
				log.Printf("got type %T for 'state' field, expected string in %+v", drm["state"], drm)
			}
			dro.DismissalMessage, _ = drm["dismissal_message"].(string)
			e.DismissedReview = dro
		}
		if rt, ok := em["requested_team"]; ok {
			delete(em, "requested_team")
			rtm, ok := rt.(map[string]interface{})
			if !ok {
				log.Printf("got value %+v for 'requested_team' field, wanted a map with 'id' and 'slug' fields", rt)
			} else {
				t := &GitHubTeam{}
				t.ID = jint64(rtm["id"])
				t.Slug, _ = rtm["slug"].(string)
				e.TeamReviewer = t
			}
		}
		delete(em, "node_id") // not sure what it is, but don't need to store it

		otherJSON, _ := json.Marshal(em)
		e.OtherJSON = string(otherJSON)
		if e.OtherJSON == "{}" {
			e.OtherJSON = ""
		}
		if e.OtherJSON != "" {
			log.Printf("warning: storing unknown field(s) in GitHub event: %s", e.OtherJSON)
		}
		evts = append(evts, e)
	}
	return evts, nil
}

// jint64 return an int64 from the provided JSON object value v.
func jint64(v interface{}) int64 {
	switch v := v.(type) {
	case nil:
		return 0
	case json.Number:
		n, _ := strconv.ParseInt(string(v), 10, 64)
		return n
	default:
		panic(fmt.Sprintf("unexpected type %T", v))
	}
}

// Copy of go-github's func newResponse, basically.
func makeGithubResponse(res *http.Response) *github.Response {
	gr := &github.Response{Response: res}
	for _, lv := range res.Header["Link"] {
		for _, link := range strings.Split(lv, ",") {
			segs := strings.Split(strings.TrimSpace(link), ";")
			if len(segs) < 2 {
				continue
			}
			// ensure href is properly formatted
			if !strings.HasPrefix(segs[0], "<") || !strings.HasSuffix(segs[0], ">") {
				continue
			}

			// try to pull out page parameter
			u, err := url.Parse(segs[0][1 : len(segs[0])-1])
			if err != nil {
				continue
			}
			page := u.Query().Get("page")
			if page == "" {
				continue
			}

			for _, seg := range segs[1:] {
				switch strings.TrimSpace(seg) {
				case `rel="next"`:
					gr.NextPage, _ = strconv.Atoi(page)
				case `rel="prev"`:
					gr.PrevPage, _ = strconv.Atoi(page)
				case `rel="first"`:
					gr.FirstPage, _ = strconv.Atoi(page)
				case `rel="last"`:
					gr.LastPage, _ = strconv.Atoi(page)
				}
			}
		}
	}
	return gr
}

var rxReferences = regexp.MustCompile(`(?:\b([\w\-]+)/([\w\-]+))?\#(\d+)\b`)

// parseGithubRefs parses references to GitHub issues from commit message commitMsg.
// Multiple references to the same issue are deduplicated.
func (c *Corpus) parseGithubRefs(gerritProj string, commitMsg string) []GitHubIssueRef {
	// Use of rxReferences by itself caused this function to take 20% of the CPU time.
	// TODO(bradfitz): stop using regexps here.
	// But in the meantime, help the regexp engine with this one weird trick:
	// Reduce the length of the string given to FindAllStringSubmatch.
	// Discard all lines before the first line containing a '#'.
	// The "Fixes #nnnn" is usually at the end, so this discards most of the input.
	// Now CPU is only 2% instead of 20%.
	hash := strings.IndexByte(commitMsg, '#')
	if hash == -1 {
		return nil
	}
	nl := strings.LastIndexByte(commitMsg[:hash], '\n')
	commitMsg = commitMsg[nl+1:]

	// TODO: use FindAllStringSubmatchIndex instead, so we can
	// back up and see what's behind it and ignore "#1", "#2",
	// "#3" 'references' which are actually bullets or ARM
	// disassembly, and only respect them as real if they have the
	// word "Fixes " or "Issue " or similar before them.
	ms := rxReferences.FindAllStringSubmatch(commitMsg, -1)
	if len(ms) == 0 {
		return nil
	}
	/* e.g.
	2017/03/30 21:42:07 matches: [["golang/go#9327" "golang" "go" "9327"]]
	2017/03/30 21:42:07 matches: [["golang/go#16512" "golang" "go" "16512"] ["golang/go#18404" "golang" "go" "18404"]]
	2017/03/30 21:42:07 matches: [["#1" "" "" "1"]]
	2017/03/30 21:42:07 matches: [["#10234" "" "" "10234"]]
	2017/03/30 21:42:31 matches: [["GoogleCloudPlatform/gcloud-golang#262" "GoogleCloudPlatform" "gcloud-golang" "262"]]
	2017/03/30 21:42:31 matches: [["GoogleCloudPlatform/google-cloud-go#481" "GoogleCloudPlatform" "google-cloud-go" "481"]]
	*/
	c.initGithub()
	github := c.GitHub()
	refs := make([]GitHubIssueRef, 0, len(ms))
	for _, m := range ms {
		owner, repo, numStr := strings.ToLower(m[1]), strings.ToLower(m[2]), m[3]
		num, err := strconv.ParseInt(numStr, 10, 32)
		if err != nil {
			continue
		}
		if owner == "" {
			if gerritProj == "go.googlesource.com/go" {
				owner, repo = "golang", "go"
			} else {
				continue
			}
		}
		ref := GitHubIssueRef{github.getOrCreateRepo(owner, repo), int32(num)}
		if contains(refs, ref) {
			continue
		}
		refs = append(refs, ref)
	}
	return refs
}

// contains reports whether refs contains the reference ref.
func contains(refs []GitHubIssueRef, ref GitHubIssueRef) bool {
	for _, r := range refs {
		if r == ref {
			return true
		}
	}
	return false
}

type limitTransport struct {
	limiter *rate.Limiter
	base    http.RoundTripper
}

func (t limitTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	limiter := t.limiter
	// NOTE(cbro): limiter should not be nil, but check defensively.
	if limiter != nil {
		if err := limiter.Wait(r.Context()); err != nil {
			return nil, err
		}
	}
	return t.base.RoundTrip(r)
}
