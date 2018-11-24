// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package gerrit contains code to interact with Gerrit servers.
//
// The API is not subject to the Go 1 compatibility promise and may change at
// any time.
package gerrit

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Client is a Gerrit client.
type Client struct {
	url  string // URL prefix, e.g. "https://go-review.googlesource.com/a" (without trailing slash)
	auth Auth

	// HTTPClient optionally specifies an HTTP client to use
	// instead of http.DefaultClient.
	HTTPClient *http.Client
}

// NewClient returns a new Gerrit client with the given URL prefix
// and authentication mode.
// The url should be just the scheme and hostname.
// If auth is nil, a default is used, or requests are made unauthenticated.
func NewClient(url string, auth Auth) *Client {
	if auth == nil {
		// TODO(bradfitz): use GitCookies auth, once that exists
		auth = NoAuth
	}
	return &Client{
		url:  strings.TrimSuffix(url, "/"),
		auth: auth,
	}
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

// HTTPError is the error type returned when a Gerrit API call does not return
// the expected status.
type HTTPError struct {
	Res     *http.Response
	Body    []byte // 4KB prefix
	BodyErr error  // any error reading Body
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP status %s; %s", e.Res.Status, e.Body)
}

// doArg is one of urlValues, reqBody, or wantResStatus
type doArg interface {
	isDoArg()
}

type wantResStatus int

func (wantResStatus) isDoArg() {}

type reqBody struct{ body interface{} }

func (reqBody) isDoArg() {}

type urlValues url.Values

func (urlValues) isDoArg() {}

func (c *Client) do(ctx context.Context, dst interface{}, method, path string, opts ...doArg) error {
	var arg url.Values
	var body interface{}
	var wantStatus = http.StatusOK
	for _, opt := range opts {
		switch opt := opt.(type) {
		case wantResStatus:
			wantStatus = int(opt)
		case reqBody:
			body = opt.body
		case urlValues:
			arg = url.Values(opt)
		default:
			panic(fmt.Sprintf("internal error; unsupported type %T", opt))
		}
	}

	var bodyr io.Reader
	var contentType string
	if body != nil {
		v, err := json.MarshalIndent(body, "", "  ")
		if err != nil {
			return err
		}
		bodyr = bytes.NewReader(v)
		contentType = "application/json"
	}
	// slashA is either "/a" (for authenticated requests) or "" for unauthenticated.
	// See https://gerrit-review.googlesource.com/Documentation/rest-api.html#authentication
	slashA := "/a"
	if _, ok := c.auth.(noAuth); ok {
		slashA = ""
	}
	var err error
	u := c.url + slashA + path
	if arg != nil {
		u += "?" + arg.Encode()
	}
	req, err := http.NewRequest(method, u, bodyr)
	if err != nil {
		return err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	c.auth.setAuth(c, req)
	res, err := c.httpClient().Do(req.WithContext(ctx))
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != wantStatus {
		body, err := ioutil.ReadAll(io.LimitReader(res.Body, 4<<10))
		return &HTTPError{res, body, err}
	}

	// The JSON response begins with an XSRF-defeating header
	// like ")]}\n". Read that and skip it.
	br := bufio.NewReader(res.Body)
	if _, err := br.ReadSlice('\n'); err != nil {
		return err
	}
	return json.NewDecoder(br).Decode(dst)
}

// Possible values for the ChangeInfo Status field.
const (
	ChangeStatusNew       = "NEW"
	ChangeStatusAbandoned = "ABANDONED"
	ChangeStatusMerged    = "MERGED"
)

// ChangeInfo is a Gerrit data structure.
// See https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#change-info
type ChangeInfo struct {
	// ID is the ID of the change in the format
	// "'<project>~<branch>~<Change-Id>'", where 'project',
	// 'branch' and 'Change-Id' are URL encoded. For 'branch' the
	// refs/heads/ prefix is omitted.
	ID           string `json:"id"`
	ChangeNumber int    `json:"_number"`
	ChangeID     string `json:"change_id"`

	Project string `json:"project"`

	// Branch is the name of the target branch.
	// The refs/heads/ prefix is omitted.
	Branch string `json:"branch"`

	Topic    string       `json:"topic"`
	Assignee *AccountInfo `json:"assignee"`
	Hashtags []string     `json:"hashtags"`

	// Subject is the subject of the change
	// (the header line of the commit message).
	Subject string `json:"subject"`

	// Status is the status of the change (NEW, SUBMITTED, MERGED,
	// ABANDONED, DRAFT).
	Status string `json:"status"`

	Created    TimeStamp    `json:"created"`
	Updated    TimeStamp    `json:"updated"`
	Submitted  TimeStamp    `json:"submitted"`
	Submitter  *AccountInfo `json:"submitter"`
	SubmitType string       `json:"submit_type"`

	// Mergeable indicates whether the change can be merged.
	// It is not set for already-merged changes,
	// nor if the change is untested, nor if the
	// SKIP_MERGEABLE option has been set.
	Mergeable bool `json:"mergeable"`

	// Submittable indicates whether the change can be submitted.
	// It is only set if requested, using the "SUBMITTABLE" option.
	Submittable bool `json:"submittable"`

	// Insertions and Deletions count inserted and deleted lines.
	Insertions int `json:"insertions"`
	Deletions  int `json:"deletions"`

	// CurrentRevision is the commit ID of the current patch set
	// of this change.  This is only set if the current revision
	// is requested or if all revisions are requested (fields
	// "CURRENT_REVISION" or "ALL_REVISIONS").
	CurrentRevision string `json:"current_revision"`

	// Revisions maps a commit ID of the patch set to a
	// RevisionInfo entity.
	//
	// Only set if the current revision is requested (in which
	// case it will only contain a key for the current revision)
	// or if all revisions are requested.
	Revisions map[string]RevisionInfo `json:"revisions"`

	// Owner is the author of the change.
	// The details are only filled in if field "DETAILED_ACCOUNTS" is requested.
	Owner *AccountInfo `json:"owner"`

	// Messages are included if field "MESSAGES" is requested.
	Messages []ChangeMessageInfo `json:"messages"`

	// Labels maps label names to LabelInfo entries.
	Labels map[string]LabelInfo `json:"labels"`

	// ReviewerUpdates are included if field "REVIEWER_UPDATES" is requested.
	ReviewerUpdates []ReviewerUpdateInfo `json:"reviewer_updates"`

	// Reviewers maps reviewer state ("REVIEWER", "CC", "REMOVED")
	// to a list of accounts.
	// REVIEWER lists users with at least one non-zero vote on the change.
	// CC lists users added to the change who has not voted.
	// REMOVED lists users who were previously reviewers on the change
	// but who have been removed.
	// Reviewers is only included if "DETAILED_LABELS" is requested.
	Reviewers map[string][]*AccountInfo `json:"reviewers"`

	// WorkInProgress indicates that the change is marked as a work in progress.
	// (This means it is not yet ready for review, but it is still publicly visible.)
	WorkInProgress bool `json:"work_in_progress"`

	// HasReviewStarted indicates whether the change has ever been marked
	// ready for review in the past (not as a work in progress).
	HasReviewStarted bool `json:"has_review_started"`

	// RevertOf lists the numeric Change-Id of the change that this change reverts.
	RevertOf int `json:"revert_of"`

	// MoreChanges is set on the last change from QueryChanges if
	// the result set is truncated by an 'n' parameter.
	MoreChanges bool `json:"_more_changes"`
}

// ReviewerUpdateInfo is a Gerrit data structure.
// See https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#review-update-info
type ReviewerUpdateInfo struct {
	Updated   TimeStamp    `json:"updated"`
	UpdatedBy *AccountInfo `json:"updated_by"`
	Reviewer  *AccountInfo `json:"reviewer"`
	State     string       // "REVIEWER", "CC", or "REMOVED"
}

// AccountInfo is a Gerrit data structure. It's used both for getting the details
// for a single account, as well as for querying multiple accounts.
type AccountInfo struct {
	NumericID int64  `json:"_account_id"`
	Name      string `json:"name,omitempty"`
	Email     string `json:"email,omitempty"`
	Username  string `json:"username,omitempty"`

	// MoreAccounts is set on the last account from QueryAccounts if
	// the result set is truncated by an 'n' parameter (or has more).
	MoreAccounts bool `json:"_more_accounts"`

	// TODO: "avatars" is also returned, but not added here yet (add if required)
}

func (ai *AccountInfo) Equal(v *AccountInfo) bool {
	if ai == nil || v == nil {
		return false
	}
	return ai.NumericID == v.NumericID
}

type ChangeMessageInfo struct {
	ID             string       `json:"id"`
	Author         *AccountInfo `json:"author"`
	Time           TimeStamp    `json:"date"`
	Message        string       `json:"message"`
	RevisionNumber int          `json:"_revision_number"`
}

// The LabelInfo entity contains information about a label on a
// change, always corresponding to the current patch set.
//
// There are two options that control the contents of LabelInfo:
// LABELS and DETAILED_LABELS.
//
// For a quick summary of the state of labels, use LABELS.
//
// For detailed information about labels, including exact numeric
// votes for all users and the allowed range of votes for the current
// user, use DETAILED_LABELS.
type LabelInfo struct {
	// Optional means the label may be set, but it’s neither
	// necessary for submission nor does it block submission if
	// set.
	Optional bool `json:"optional"`

	// Fields set by LABELS field option:

	All []ApprovalInfo `json:"all"`
}

type ApprovalInfo struct {
	AccountInfo
	Value int       `json:"value"`
	Date  TimeStamp `json:"date"`
}

// The RevisionInfo entity contains information about a patch set. Not
// all fields are returned by default. Additional fields can be
// obtained by adding o parameters as described at:
// https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#list-changes
type RevisionInfo struct {
	Draft          bool                  `json:"draft"`
	PatchSetNumber int                   `json:"_number"`
	Created        TimeStamp             `json:"created"`
	Uploader       *AccountInfo          `json:"uploader"`
	Ref            string                `json:"ref"`
	Fetch          map[string]*FetchInfo `json:"fetch"`
	Commit         *CommitInfo           `json:"commit"`
	Files          map[string]*FileInfo  `json:"files"`
	// TODO: more
}

type CommitInfo struct {
	Author    GitPersonInfo `json:"author"`
	Committer GitPersonInfo `json:"committer"`
	CommitID  string        `json:"commit"`
	Subject   string        `json:"subject"`
	Message   string        `json:"message"`
	Parents   []CommitInfo  `json:"parents"`
}

type GitPersonInfo struct {
	Name     string    `json:"name"`
	Email    string    `json:"Email"`
	Date     TimeStamp `json:"date"`
	TZOffset int       `json:"tz"`
}

func (gpi *GitPersonInfo) Equal(v *GitPersonInfo) bool {
	if gpi == nil {
		if gpi != v {
			return false
		}
		return true
	}
	return gpi.Name == v.Name && gpi.Email == v.Email && gpi.Date.Equal(v.Date) &&
		gpi.TZOffset == v.TZOffset
}

// Possible values for the FileInfo Status field.
const (
	FileInfoAdded     = "A"
	FileInfoDeleted   = "D"
	FileInfoRenamed   = "R"
	FileInfoCopied    = "C"
	FileInfoRewritten = "W"
)

type FileInfo struct {
	Status        string `json:"status"`
	Binary        bool   `json:"binary"`
	OldPath       string `json:"old_path"`
	LinesInserted int    `json:"lines_inserted"`
	LinesDeleted  int    `json:"lines_deleted"`
}

type FetchInfo struct {
	URL      string            `json:"url"`
	Ref      string            `json:"ref"`
	Commands map[string]string `json:"commands"`
}

// QueryChangesOpt are options for QueryChanges.
type QueryChangesOpt struct {
	// N is the number of results to return.
	// If 0, the 'n' parameter is not sent to Gerrit.
	N int

	// Start is the number of results to skip (useful in pagination).
	// To figure out if there are more results, the last ChangeInfo struct
	// in the last call to QueryChanges will have the field MoreAccounts=true.
	// If 0, the 'S' parameter is not sent to Gerrit.
	Start int

	// Fields are optional fields to also return.
	// Example strings include "ALL_REVISIONS", "LABELS", "MESSAGES".
	// For a complete list, see:
	// https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#change-info
	Fields []string
}

func condInt(n int) []string {
	if n != 0 {
		return []string{strconv.Itoa(n)}
	}
	return nil
}

// QueryChanges queries changes. The q parameter is a Gerrit search query.
// For the API call, see https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#list-changes
// For the query syntax, see https://gerrit-review.googlesource.com/Documentation/user-search.html#_search_operators
func (c *Client) QueryChanges(ctx context.Context, q string, opts ...QueryChangesOpt) ([]*ChangeInfo, error) {
	var opt QueryChangesOpt
	switch len(opts) {
	case 0:
	case 1:
		opt = opts[0]
	default:
		return nil, errors.New("only 1 option struct supported")
	}
	var changes []*ChangeInfo
	err := c.do(ctx, &changes, "GET", "/changes/", urlValues{
		"q": {q},
		"n": condInt(opt.N),
		"o": opt.Fields,
		"S": condInt(opt.Start),
	})
	return changes, err
}

// GetChange returns information about a single change.
// For the API call, see https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#get-change
// If the change doesn't exist, the error will be ErrChangeNotExist.
func (c *Client) GetChange(ctx context.Context, changeID string, opts ...QueryChangesOpt) (*ChangeInfo, error) {
	var opt QueryChangesOpt
	switch len(opts) {
	case 0:
	case 1:
		opt = opts[0]
	default:
		return nil, errors.New("only 1 option struct supported")
	}
	change := new(ChangeInfo)
	err := c.do(ctx, change, "GET", "/changes/"+changeID, urlValues{
		"n": condInt(opt.N),
		"o": opt.Fields,
	})
	if he, ok := err.(*HTTPError); ok && he.Res.StatusCode == 404 {
		return nil, ErrChangeNotExist
	}
	return change, err
}

// GetChangeDetail retrieves a change with labels, detailed labels, detailed
// accounts, and messages.
// For the API call, see https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#get-change-detail
func (c *Client) GetChangeDetail(ctx context.Context, changeID string, opts ...QueryChangesOpt) (*ChangeInfo, error) {
	var opt QueryChangesOpt
	switch len(opts) {
	case 0:
	case 1:
		opt = opts[0]
	default:
		return nil, errors.New("only 1 option struct supported")
	}
	var change ChangeInfo
	err := c.do(ctx, &change, "GET", "/changes/"+changeID+"/detail", urlValues{
		"o": opt.Fields,
	})
	if err != nil {
		return nil, err
	}
	return &change, nil
}

// ListFiles retrieves a map of filenames to FileInfo's for the given change ID and revision.
// For the API call, see https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#list-files
func (c *Client) ListFiles(ctx context.Context, changeID, revision string) (map[string]*FileInfo, error) {
	var m map[string]*FileInfo
	if err := c.do(ctx, &m, "GET", "/changes/"+changeID+"/revisions/"+revision+"/files"); err != nil {
		return nil, err
	}
	return m, nil
}

// ReviewInput contains information for adding a review to a revision.
// See https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#review-input
type ReviewInput struct {
	Message string         `json:"message,omitempty"`
	Labels  map[string]int `json:"labels,omitempty"`

	// Comments contains optional per-line comments to post.
	// The map key is a file path (such as "src/foo/bar.go").
	Comments map[string][]CommentInput `json:"comments,omitempty"`

	// Reviewers optionally specifies new reviewers to add to the change.
	Reviewers []ReviewerInput `json:"reviewers,omitempty"`
}

// ReviewerInput contains information for adding a reviewer to a change.
// See https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#reviewer-input
type ReviewerInput struct {
	// Reviewer is the ID of the account to be added as reviewer.
	// See https://gerrit-review.googlesource.com/Documentation/rest-api-accounts.html#account-id
	Reviewer string `json:"reviewer"`
	State    string `json:"state,omitempty"` // REVIEWER or CC (default: REVIEWER)
}

// CommentInput contains information for creating an inline comment.
// See https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#comment-input
type CommentInput struct {
	Line    int    `json:"line"`
	Message string `json:"message"`

	// TODO(haya14busa): more, as needed.
}

type reviewInfo struct {
	Labels map[string]int `json:"labels,omitempty"`
}

// SetReview leaves a message on a change and/or modifies labels.
// For the API call, see https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#set-review
// The changeID is https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#change-id
// The revision is https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#revision-id
func (c *Client) SetReview(ctx context.Context, changeID, revision string, review ReviewInput) error {
	var res reviewInfo
	return c.do(ctx, &res, "POST", fmt.Sprintf("/changes/%s/revisions/%s/review", changeID, revision),
		reqBody{review})
}

// ReviewerInfo contains information about reviewers of a change.
// See https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#reviewer-info
type ReviewerInfo struct {
	AccountInfo
	Approvals map[string]string `json:"approvals"`
}

// ListReviewers returns all reviewers on a change.
// For the API call, see https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#list-reviewers
// The changeID is https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#change-id
func (c *Client) ListReviewers(ctx context.Context, changeID string) ([]ReviewerInfo, error) {
	var res []ReviewerInfo
	if err := c.do(ctx, &res, "GET", fmt.Sprintf("/changes/%s/reviewers", changeID)); err != nil {
		return nil, err
	}
	return res, nil
}

// HashtagsInput is the request body used when modifying a CL's hashtags.
//
// See https://gerrit-documentation.storage.googleapis.com/Documentation/2.15.1/rest-api-changes.html#hashtags-input
type HashtagsInput struct {
	Add    []string `json:"add"`
	Remove []string `json:"remove"`
}

// SetHashtags modifies the hashtags for a CL, supporting both adding
// and removing hashtags in one request. On success it returns the new
// set of hashtags.
//
// See https://gerrit-documentation.storage.googleapis.com/Documentation/2.15.1/rest-api-changes.html#set-hashtags
func (c *Client) SetHashtags(ctx context.Context, changeID string, hashtags HashtagsInput) ([]string, error) {
	var res []string
	err := c.do(ctx, &res, "POST", fmt.Sprintf("/changes/%s/hashtags", changeID), reqBody{hashtags})
	return res, err
}

// AddHashtags is a wrapper around SetHashtags that only supports adding tags.
func (c *Client) AddHashtags(ctx context.Context, changeID string, tags ...string) ([]string, error) {
	return c.SetHashtags(ctx, changeID, HashtagsInput{Add: tags})
}

// RemoveHashtags is a wrapper around SetHashtags that only supports removing tags.
func (c *Client) RemoveHashtags(ctx context.Context, changeID string, tags ...string) ([]string, error) {
	return c.SetHashtags(ctx, changeID, HashtagsInput{Remove: tags})
}

// GetHashtags returns a CL's current hashtags.
//
// See https://gerrit-documentation.storage.googleapis.com/Documentation/2.15.1/rest-api-changes.html#get-hashtags
func (c *Client) GetHashtags(ctx context.Context, changeID string) ([]string, error) {
	var res []string
	err := c.do(ctx, &res, "GET", fmt.Sprintf("/changes/%s/hashtags", changeID))
	return res, err
}

// AbandonChange abandons the given change.
// For the API call, see https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#abandon-change
// The changeID is https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#change-id
// The input for the call is https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#abandon-input
func (c *Client) AbandonChange(ctx context.Context, changeID string, message ...string) error {
	var msg string
	if len(message) > 1 {
		panic("invalid use of multiple message inputs")
	}
	if len(message) == 1 {
		msg = message[0]
	}
	b := struct {
		Message string `json:"message,omitempty"`
	}{msg}
	var change ChangeInfo
	return c.do(ctx, &change, "POST", "/changes/"+changeID+"/abandon", reqBody{&b})
}

// ProjectInput contains the options for creating a new project.
// See https://gerrit-review.googlesource.com/Documentation/rest-api-projects.html#project-input
type ProjectInput struct {
	Parent      string `json:"parent,omitempty"`
	Description string `json:"description,omitempty"`
	SubmitType  string `json:"submit_type,omitempty"`

	CreateNewChangeForAllNotInTarget string `json:"create_new_change_for_all_not_in_target,omitempty"`

	// TODO(bradfitz): more, as needed.
}

// ProjectInfo is information about a Gerrit project.
// See https://gerrit-review.googlesource.com/Documentation/rest-api-projects.html#project-info
type ProjectInfo struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Parent      string            `json:"parent"`
	CloneURL    string            `json:"clone_url"`
	Description string            `json:"description"`
	State       string            `json:"state"`
	Branches    map[string]string `json:"branches"`
}

// ListProjects returns the server's active projects.
//
// The returned slice is sorted by project ID and excludes the "All-Projects" project.
//
// See https://gerrit-review.googlesource.com/Documentation/rest-api-projects.html#list-projects
func (c *Client) ListProjects(ctx context.Context) ([]ProjectInfo, error) {
	var res map[string]ProjectInfo
	err := c.do(ctx, &res, "GET", fmt.Sprintf("/projects/"))
	if err != nil {
		return nil, err
	}
	var ret []ProjectInfo
	for name, pi := range res {
		if name == "All-Projects" {
			continue
		}
		if pi.State != "ACTIVE" {
			continue
		}
		ret = append(ret, pi)
	}
	sort.Slice(ret, func(i, j int) bool { return ret[i].ID < ret[j].ID })
	return ret, nil
}

// CreateProject creates a new project.
func (c *Client) CreateProject(ctx context.Context, name string, p ...ProjectInput) (ProjectInfo, error) {
	var pi ProjectInput
	if len(p) > 1 {
		panic("invalid use of multiple project inputs")
	}
	if len(p) == 1 {
		pi = p[0]
	}
	var res ProjectInfo
	err := c.do(ctx, &res, "PUT", fmt.Sprintf("/projects/%s", name), reqBody{&pi}, wantResStatus(http.StatusCreated))
	return res, err
}

// ErrProjectNotExist is returned when a project doesn't exist.
// It is not necessarily returned unless a method is documented as
// returning it.
var ErrProjectNotExist = errors.New("gerrit: requested project does not exist")

// ErrChangeNotExist is returned when a change doesn't exist.
// It is not necessarily returned unless a method is documented as
// returning it.
var ErrChangeNotExist = errors.New("gerrit: requested change does not exist")

// GetProjectInfo returns info about a project.
// If the project doesn't exist, the error will be ErrProjectNotExist.
func (c *Client) GetProjectInfo(ctx context.Context, name string) (ProjectInfo, error) {
	var res ProjectInfo
	err := c.do(ctx, &res, "GET", fmt.Sprintf("/projects/%s", name))
	if he, ok := err.(*HTTPError); ok && he.Res.StatusCode == 404 {
		return res, ErrProjectNotExist
	}
	return res, err
}

// BranchInfo is information about a branch.
// See https://gerrit-review.googlesource.com/Documentation/rest-api-projects.html#branch-info
type BranchInfo struct {
	Ref       string `json:"ref"`
	Revision  string `json:"revision"`
	CanDelete bool   `json:"can_delete"`
}

// GetProjectBranches returns the branches for the project name. The branches are stored in a map
// keyed by reference.
func (c *Client) GetProjectBranches(ctx context.Context, name string) (map[string]BranchInfo, error) {
	var res []BranchInfo
	err := c.do(ctx, &res, "GET", fmt.Sprintf("/projects/%s/branches/", name))
	if err != nil {
		return nil, err
	}
	m := map[string]BranchInfo{}
	for _, bi := range res {
		m[bi.Ref] = bi
	}
	return m, nil
}

// WebLinkInfo is information about a web link.
// See https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#web-link-info
type WebLinkInfo struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	ImageURL string `json:"image_url"`
}

func (wli *WebLinkInfo) Equal(v *WebLinkInfo) bool {
	if wli == nil || v == nil {
		return false
	}
	return wli.Name == v.Name && wli.URL == v.URL && wli.ImageURL == v.ImageURL
}

// TagInfo is information about a tag.
// See https://gerrit-review.googlesource.com/Documentation/rest-api-projects.html#tag-info
type TagInfo struct {
	Ref       string         `json:"ref"`
	Revision  string         `json:"revision"`
	Object    string         `json:"object,omitempty"`
	Message   string         `json:"message,omitempty"`
	Tagger    *GitPersonInfo `json:"tagger,omitempty"`
	Created   TimeStamp      `json:"created,omitempty"`
	CanDelete bool           `json:"can_delete"`
	WebLinks  []WebLinkInfo  `json:"web_links,omitempty"`
}

func (ti *TagInfo) Equal(v *TagInfo) bool {
	if ti == nil || v == nil {
		return false
	}
	if ti.Ref != v.Ref || ti.Revision != v.Revision || ti.Object != v.Object ||
		ti.Message != v.Message || !ti.Created.Equal(v.Created) || ti.CanDelete != v.CanDelete {
		return false
	}
	if !ti.Tagger.Equal(v.Tagger) {
		return false
	}
	if len(ti.WebLinks) != len(v.WebLinks) {
		return false
	}
	for i := range ti.WebLinks {
		if !ti.WebLinks[i].Equal(&v.WebLinks[i]) {
			return false
		}
	}
	return true
}

// GetProjectTags returns the tags for the project name. The tags are stored in a map keyed by
// reference.
func (c *Client) GetProjectTags(ctx context.Context, name string) (map[string]TagInfo, error) {
	var res []TagInfo
	err := c.do(ctx, &res, "GET", fmt.Sprintf("/projects/%s/tags/", name))
	if err != nil {
		return nil, err
	}
	m := map[string]TagInfo{}
	for _, ti := range res {
		m[ti.Ref] = ti
	}
	return m, nil
}

// GetAccountInfo gets the specified account's information from Gerrit.
// For the API call, see https://gerrit-review.googlesource.com/Documentation/rest-api-accounts.html#get-account
// The accountID is https://gerrit-review.googlesource.com/Documentation/rest-api-accounts.html#account-id
//
// Note that getting "self" is a good way to validate host access, since it only requires peeker
// access to the host, not to any particular repository.
func (c *Client) GetAccountInfo(ctx context.Context, accountID string) (AccountInfo, error) {
	var res AccountInfo
	err := c.do(ctx, &res, "GET", fmt.Sprintf("/accounts/%s", accountID))
	return res, err
}

// QueryAccountsOpt are options for QueryAccounts.
type QueryAccountsOpt struct {
	// N is the number of results to return.
	// If 0, the 'n' parameter is not sent to Gerrit.
	N int

	// Start is the number of results to skip (useful in pagination).
	// To figure out if there are more results, the last AccountInfo struct
	// in the last call to QueryAccounts will have the field MoreAccounts=true.
	// If 0, the 'S' parameter is not sent to Gerrit.
	Start int

	// Fields are optional fields to also return.
	// Example strings include "DETAILS", "ALL_EMAILS".
	// By default, only the account IDs are returned.
	// For a complete list, see:
	// https://gerrit-review.googlesource.com/Documentation/rest-api-accounts.html#query-account
	Fields []string
}

// QueryAccounts queries accounts. The q parameter is a Gerrit search query.
// For the API call and query syntax, see https://gerrit-review.googlesource.com/Documentation/rest-api-accounts.html#query-account
func (c *Client) QueryAccounts(ctx context.Context, q string, opts ...QueryAccountsOpt) ([]*AccountInfo, error) {
	var opt QueryAccountsOpt
	switch len(opts) {
	case 0:
	case 1:
		opt = opts[0]
	default:
		return nil, errors.New("only 1 option struct supported")
	}
	var changes []*AccountInfo
	err := c.do(ctx, &changes, "GET", "/accounts/", urlValues{
		"q": {q},
		"n": condInt(opt.N),
		"o": opt.Fields,
		"S": condInt(opt.Start),
	})
	return changes, err
}

// GetProjects returns a map of all projects on the Gerrit server.
func (c *Client) GetProjects(ctx context.Context, branch string) (map[string]*ProjectInfo, error) {
	mp := make(map[string]*ProjectInfo)
	err := c.do(ctx, &mp, "GET", fmt.Sprintf("?b=%s&format=JSON", branch))
	return mp, err
}

type TimeStamp time.Time

func (ts TimeStamp) Equal(v TimeStamp) bool {
	return ts.Time().Equal(v.Time())
}

// Gerrit's timestamp layout is like time.RFC3339Nano, but with a space instead of the "T",
// and without a timezone (it's always in UTC).
const timeStampLayout = "2006-01-02 15:04:05.999999999"

func (ts TimeStamp) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, ts.Time().UTC().Format(timeStampLayout))), nil
}

func (ts *TimeStamp) UnmarshalJSON(p []byte) error {
	if len(p) < 2 {
		return errors.New("Timestamp too short")
	}
	if p[0] != '"' || p[len(p)-1] != '"' {
		return errors.New("not double-quoted")
	}
	s := strings.Trim(string(p), "\"")
	t, err := time.Parse(timeStampLayout, s)
	if err != nil {
		return err
	}
	*ts = TimeStamp(t)
	return nil
}

func (ts TimeStamp) Time() time.Time { return time.Time(ts) }

// GroupInfo contains information about a group.
//
// See https://gerrit-review.googlesource.com/Documentation/rest-api-groups.html#group-info.
type GroupInfo struct {
	ID      string           `json:"id"`
	URL     string           `json:"url"`
	Name    string           `json:"name"`
	GroupID int64            `json:"group_id"`
	Options GroupOptionsInfo `json:"options"`
	Owner   string           `json:"owner"`
	OwnerID string           `json:"owner_id"`
}

type GroupOptionsInfo struct {
	VisibleToAll bool `json:"visible_to_all"`
}

func (c *Client) GetGroups(ctx context.Context) (map[string]*GroupInfo, error) {
	res := make(map[string]*GroupInfo)
	err := c.do(ctx, &res, "GET", "/groups/")
	for k, gi := range res {
		if gi != nil && gi.Name == "" {
			gi.Name = k
		}
	}
	return res, err
}

func (c *Client) GetGroupMembers(ctx context.Context, groupID string) ([]AccountInfo, error) {
	var ais []AccountInfo
	err := c.do(ctx, &ais, "GET", "/groups/"+groupID+"/members")
	return ais, err
}
