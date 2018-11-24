// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/build/maintner"
)

const (
	labelProposal = "Proposal"

	prefixProposal = "proposal:"
	prefixDev      = "[dev."

	// The title of the current release milestone in GitHub.
	curMilestoneTitle = "Go1.12"
)

// The start date of the current release milestone.
var curMilestoneStart = time.Date(2018, 8, 20, 0, 0, 0, 0, time.UTC)

// titleDirs returns a slice of prefix directories contained in a title. For
// devapp,maintner: my cool new change, it will return ["devapp", "maintner"].
// If there is no dir prefix, it will return nil.
func titleDirs(title string) []string {
	if i := strings.Index(title, "\n"); i >= 0 {
		title = title[:i]
	}
	title = strings.TrimSpace(title)
	i := strings.Index(title, ":")
	if i < 0 {
		return nil
	}
	var (
		b bytes.Buffer
		r []string
	)
	for j := 0; j < i; j++ {
		switch title[j] {
		case ' ':
			continue
		case ',':
			r = append(r, b.String())
			b.Reset()
			continue
		default:
			b.WriteByte(title[j])
		}
	}
	if b.Len() > 0 {
		r = append(r, b.String())
	}
	return r
}

type releaseData struct {
	LastUpdated  string
	Sections     []section
	BurndownJSON template.JS

	// dirty is set if this data needs to be updated due to a corpus change.
	dirty bool
}

type section struct {
	Title  string
	Count  int
	Groups []group
}

type group struct {
	Dir   string
	Items []item
}

type item struct {
	Issue            *maintner.GitHubIssue
	CLs              []*gerritCL
	FirstPerformance bool // set if this item is the first item which is labeled "performance"
}

func (i *item) ReleaseBlocker() bool {
	if i.Issue == nil {
		return false
	}
	return i.Issue.HasLabel("release-blocker")
}

type itemsBySummary []item

func (x itemsBySummary) Len() int      { return len(x) }
func (x itemsBySummary) Swap(i, j int) { x[i], x[j] = x[j], x[i] }
func (x itemsBySummary) Less(i, j int) bool {
	// Sort performance issues to the end.
	pi := x[i].Issue != nil && x[i].Issue.HasLabel("Performance")
	pj := x[j].Issue != nil && x[j].Issue.HasLabel("Performance")
	if pi != pj {
		return !pi
	}
	// Otherwise sort by the item summary.
	return itemSummary(x[i]) < itemSummary(x[j])
}

func itemSummary(it item) string {
	if it.Issue != nil {
		return it.Issue.Title
	}
	for _, cl := range it.CLs {
		return cl.Subject()
	}
	return ""
}

var milestoneRE = regexp.MustCompile(`^Go1\.(\d+)(|\.(\d+))(|[A-Z].*)$`)

type milestone struct {
	title        string
	major, minor int
}

type milestonesByGoVersion []milestone

func (x milestonesByGoVersion) Len() int      { return len(x) }
func (x milestonesByGoVersion) Swap(i, j int) { x[i], x[j] = x[j], x[i] }
func (x milestonesByGoVersion) Less(i, j int) bool {
	a, b := x[i], x[j]
	if a.major != b.major {
		return a.major < b.major
	}
	if a.minor != b.minor {
		return a.minor < b.minor
	}
	return a.title < b.title
}

var annotationRE = regexp.MustCompile(`(?m)^R=(.+)\b`)

type gerritCL struct {
	*maintner.GerritCL
	Closed    bool
	Milestone string
}

// ReviewURL returns the code review address of cl.
func (cl *gerritCL) ReviewURL() string {
	s := cl.Project.Server()
	if s == "go.googlesource.com" {
		return fmt.Sprintf("https://golang.org/cl/%d", cl.Number)
	}
	subd := strings.TrimSuffix(s, ".googlesource.com")
	if subd == s {
		return ""
	}
	return fmt.Sprintf("https://%s-review.googlesource.com/%d", subd, cl.Number)
}

// burndownData is encoded to JSON and embedded in the page for use when
// rendering a burndown chart using JavaScript.
type burndownData struct {
	Milestone string          `json:"milestone"`
	Entries   []burndownEntry `json:"entries"`
}

type burndownEntry struct {
	DateStr  string `json:"dateStr"` // "12-25"
	Open     int    `json:"open"`
	Blockers int    `json:"blockers"`
}

func (s *server) updateReleaseData() {
	log.Println("Updating release data ...")
	s.cMu.Lock()
	defer s.cMu.Unlock()

	dirToCLs := map[string][]*gerritCL{}
	issueToCLs := map[int32][]*gerritCL{}
	s.corpus.Gerrit().ForeachProjectUnsorted(func(p *maintner.GerritProject) error {
		p.ForeachOpenCL(func(cl *maintner.GerritCL) error {
			if strings.HasPrefix(cl.Subject(), prefixDev) {
				return nil
			}

			var (
				closed        bool
				closedVersion int32
				milestone     string
			)
			for _, m := range cl.Messages {
				if closed && closedVersion < m.Version {
					closed = false
				}
				sm := annotationRE.FindStringSubmatch(m.Message)
				if sm == nil {
					continue
				}
				val := sm[1]
				if val == "close" || val == "closed" {
					closedVersion = m.Version
					closed = true
				} else if milestoneRE.MatchString(val) {
					milestone = val
				}
			}
			gcl := &gerritCL{
				GerritCL:  cl,
				Closed:    closed,
				Milestone: milestone,
			}

			for _, r := range cl.GitHubIssueRefs {
				issueToCLs[r.Number] = append(issueToCLs[r.Number], gcl)
			}
			dirs := titleDirs(cl.Subject())
			if len(dirs) == 0 {
				dirToCLs[""] = append(dirToCLs[""], gcl)
			} else {
				for _, d := range dirs {
					dirToCLs[d] = append(dirToCLs[d], gcl)
				}
			}
			return nil
		})
		return nil
	})

	dirToIssues := map[string][]*maintner.GitHubIssue{}
	var curMilestoneIssues []*maintner.GitHubIssue
	s.repo.ForeachIssue(func(issue *maintner.GitHubIssue) error {
		// Only include issues in active milestones.
		if issue.Milestone.IsUnknown() || issue.Milestone.Closed || issue.Milestone.IsNone() {
			return nil
		}

		if issue.Milestone.Title == curMilestoneTitle {
			curMilestoneIssues = append(curMilestoneIssues, issue)
		}

		// Only open issues are displayed on the page using dirToIssues.
		if issue.Closed {
			return nil
		}

		dirs := titleDirs(issue.Title)
		if len(dirs) == 0 {
			dirToIssues[""] = append(dirToIssues[""], issue)
		} else {
			for _, d := range dirs {
				dirToIssues[d] = append(dirToIssues[d], issue)
			}
		}
		return nil
	})

	bd := burndownData{Milestone: curMilestoneTitle}
	for t, now := curMilestoneStart, time.Now(); t.Before(now); t = t.Add(24 * time.Hour) {
		var e burndownEntry
		for _, issue := range curMilestoneIssues {
			if issue.Created.After(t) || (issue.Closed && issue.ClosedAt.Before(t)) {
				continue
			}
			if issue.HasLabel("release-blocker") {
				e.Blockers++
			}
			e.Open++
		}
		e.DateStr = t.Format("01-02")
		bd.Entries = append(bd.Entries, e)
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(bd); err != nil {
		log.Printf("json.Encode: %v", err)
	}
	s.data.release.BurndownJSON = template.JS(buf.String())
	s.data.release.Sections = nil
	s.appendOpenIssues(dirToIssues, issueToCLs)
	s.appendPendingCLs(dirToCLs)
	s.appendPendingProposals(issueToCLs)
	s.appendClosedIssues()
	s.data.release.LastUpdated = time.Now().UTC().Format(time.UnixDate)
	s.data.release.dirty = false
}

// requires s.cMu be locked.
func (s *server) appendOpenIssues(dirToIssues map[string][]*maintner.GitHubIssue, issueToCLs map[int32][]*gerritCL) {
	var issueDirs []string
	for d := range dirToIssues {
		issueDirs = append(issueDirs, d)
	}
	sort.Strings(issueDirs)
	ms := s.allMilestones()
	for _, m := range ms {
		var (
			issueGroups []group
			issueCount  int
		)
		for _, d := range issueDirs {
			issues, ok := dirToIssues[d]
			if !ok {
				continue
			}
			var items []item
			for _, i := range issues {
				if i.Milestone.Title != m.title {
					continue
				}

				items = append(items, item{
					Issue: i,
					CLs:   issueToCLs[i.Number],
				})
				issueCount++
			}
			if len(items) == 0 {
				continue
			}
			sort.Sort(itemsBySummary(items))
			for idx := range items {
				if items[idx].Issue.HasLabel("Performance") {
					items[idx].FirstPerformance = true
					break
				}
			}
			issueGroups = append(issueGroups, group{
				Dir:   d,
				Items: items,
			})
		}
		s.data.release.Sections = append(s.data.release.Sections, section{
			Title:  m.title,
			Count:  issueCount,
			Groups: issueGroups,
		})
	}
}

// requires s.cMu be locked.
func (s *server) appendPendingCLs(dirToCLs map[string][]*gerritCL) {
	var clDirs []string
	for d := range dirToCLs {
		clDirs = append(clDirs, d)
	}
	sort.Strings(clDirs)
	var (
		clGroups []group
		clCount  int
	)
	for _, d := range clDirs {
		if cls, ok := dirToCLs[d]; ok {
			clCount += len(cls)
			g := group{Dir: d}
			g.Items = append(g.Items, item{CLs: cls})
			sort.Sort(itemsBySummary(g.Items))
			clGroups = append(clGroups, g)
		}
	}
	s.data.release.Sections = append(s.data.release.Sections, section{
		Title:  "Pending CLs",
		Count:  clCount,
		Groups: clGroups,
	})
}

// requires s.cMu be locked.
func (s *server) appendPendingProposals(issueToCLs map[int32][]*gerritCL) {
	var proposals group
	s.repo.ForeachIssue(func(issue *maintner.GitHubIssue) error {
		if issue.Closed {
			return nil
		}
		if issue.HasLabel(labelProposal) || strings.HasPrefix(issue.Title, prefixProposal) {
			proposals.Items = append(proposals.Items, item{
				Issue: issue,
				CLs:   issueToCLs[issue.Number],
			})
		}
		return nil
	})
	sort.Sort(itemsBySummary(proposals.Items))
	s.data.release.Sections = append(s.data.release.Sections, section{
		Title:  "Pending Proposals",
		Count:  len(proposals.Items),
		Groups: []group{proposals},
	})
}

// requires s.cMu be locked.
func (s *server) appendClosedIssues() {
	var (
		closed   group
		lastWeek = time.Now().Add(-(7*24 + 12) * time.Hour)
	)
	s.repo.ForeachIssue(func(issue *maintner.GitHubIssue) error {
		if !issue.Closed {
			return nil
		}
		if issue.Updated.After(lastWeek) {
			closed.Items = append(closed.Items, item{Issue: issue})
		}
		return nil
	})
	sort.Sort(itemsBySummary(closed.Items))
	s.data.release.Sections = append(s.data.release.Sections, section{
		Title:  "Closed Last Week",
		Count:  len(closed.Items),
		Groups: []group{closed},
	})
}

// requires s.cMu be read locked.
func (s *server) allMilestones() []milestone {
	var ms []milestone
	s.repo.ForeachMilestone(func(m *maintner.GitHubMilestone) error {
		if m.Closed {
			return nil
		}
		sm := milestoneRE.FindStringSubmatch(m.Title)
		if sm == nil {
			return nil
		}
		major, _ := strconv.Atoi(sm[1])
		minor, _ := strconv.Atoi(sm[3])
		ms = append(ms, milestone{
			title: m.Title,
			major: major,
			minor: minor,
		})
		return nil
	})
	sort.Sort(milestonesByGoVersion(ms))
	return ms
}

// handleRelease serves dev.golang.org/release.
func (s *server) handleRelease(t *template.Template, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	s.cMu.RLock()
	dirty := s.data.release.dirty
	s.cMu.RUnlock()
	if dirty {
		s.updateReleaseData()
	}

	s.cMu.RLock()
	defer s.cMu.RUnlock()
	if err := t.Execute(w, s.data.release); err != nil {
		log.Printf("t.Execute(w, nil) = %v", err)
		return
	}
}
