// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Logic for the /gophercon endpoint which shows a semi-realtime dashboard of
// contribution activity. Users add their Gerrit user IDs to a hard-coded GitHub
// issue to provide a mapping of Gerrit ID to GitHub user, which allows any
// activity from the user across GitHub and Gerrit to be associated with a
// single GitHub user object. Points are awarded depending on the type of
// activity performed and an aggregated total for all participants is displayed.

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"time"

	"golang.org/x/build/maintner"
)

const issueNumGerritUserMapping = 27373 // Special sign-up issue.

// intFromStr returns the first integer within s, allowing for non-numeric
// characters to be present.
func intFromStr(s string) (int, bool) {
	var (
		foundNum bool
		r        int
	)
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			foundNum = true
			r = r*10 + int(s[i]-'0')
		} else if foundNum {
			return r, true
		}
	}
	if foundNum {
		return r, true
	}
	return 0, false
}

// Keep these in sync with the frontend JS.
const (
	activityTypeRegister     = "REGISTER"
	activityTypeCreateChange = "CREATE_CHANGE"
	activityTypeAmendChange  = "AMEND_CHANGE"
	activityTypeMergeChange  = "MERGE_CHANGE"
)

var pointsPerActivity = map[string]int{
	activityTypeRegister:     1,
	activityTypeCreateChange: 2,
	activityTypeAmendChange:  2,
	activityTypeMergeChange:  3,
}

// An activity represents something a contributor has done. e.g. register on
// the GitHub issue, create a change, amend a change, etc.
type activity struct {
	Type    string    `json:"type"`
	Created time.Time `json:"created"`
	User    string    `json:"gitHubUser"`
	Points  int       `json:"points"`
}

func (s *server) updateActivities() {
	s.cMu.Lock()
	defer s.cMu.Unlock()
	repo := s.corpus.GitHub().Repo("golang", "go")
	if repo == nil {
		log.Println(`s.corpus.GitHub().Repo("golang", "go") = nil`)
		return
	}
	issue := repo.Issue(issueNumGerritUserMapping)
	if issue == nil {
		log.Printf("repo.Issue(%d) = nil", issueNumGerritUserMapping)
		return
	}
	latest := issue.Created
	if len(s.activities) > 0 {
		latest = s.activities[len(s.activities)-1].Created
	}

	var newActivities []activity
	issue.ForeachComment(func(c *maintner.GitHubComment) error {
		if !c.Created.After(latest) {
			return nil
		}
		id, ok := intFromStr(c.Body)
		if !ok {
			return fmt.Errorf("intFromStr(%q) = %v", c.Body, ok)
		}
		s.userMapping[id] = c.User

		newActivities = append(newActivities, activity{
			Type:    activityTypeRegister,
			Created: c.Created,
			User:    c.User.Login,
			Points:  pointsPerActivity[activityTypeRegister],
		})
		s.totalPoints += pointsPerActivity[activityTypeRegister]
		return nil
	})

	s.corpus.Gerrit().ForeachProjectUnsorted(func(p *maintner.GerritProject) error {
		p.ForeachCLUnsorted(func(cl *maintner.GerritCL) error {
			// TODO(golang.org/issue/21984)
			if cl.Commit == nil {
				log.Printf("Got CL with nil Commit field: %+v", cl)
				return nil
			}
			if !cl.Commit.CommitTime.After(latest) {
				return nil
			}
			user := s.userMapping[cl.OwnerID()]
			if user == nil {
				return nil
			}

			newActivities = append(newActivities, activity{
				Type:    activityTypeCreateChange,
				Created: cl.Created,
				User:    user.Login,
				Points:  pointsPerActivity[activityTypeCreateChange],
			})
			s.totalPoints += pointsPerActivity[activityTypeCreateChange]
			if cl.Version > 1 {
				newActivities = append(newActivities, activity{
					Type:    activityTypeAmendChange,
					Created: cl.Commit.CommitTime,
					User:    user.Login,
					Points:  pointsPerActivity[activityTypeAmendChange],
				})
				s.totalPoints += pointsPerActivity[activityTypeAmendChange]
			}
			if cl.Status == "merged" {
				newActivities = append(newActivities, activity{
					Type:    activityTypeMergeChange,
					Created: cl.Commit.CommitTime,
					User:    user.Login,
					Points:  pointsPerActivity[activityTypeMergeChange],
				})
				s.totalPoints += pointsPerActivity[activityTypeMergeChange]
			}
			return nil
		})
		return nil
	})

	sort.Sort(byCreated(newActivities))
	s.activities = append(s.activities, newActivities...)
}

type byCreated []activity

func (a byCreated) Len() int           { return len(a) }
func (a byCreated) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byCreated) Less(i, j int) bool { return a[i].Created.Before(a[j].Created) }

func (s *server) handleActivities(w http.ResponseWriter, r *http.Request) {
	i, _ := strconv.Atoi(r.FormValue("since"))
	since := time.Unix(int64(i)/1000, 0)

	recentActivity := []activity{}
	for _, a := range s.activities {
		if a.Created.After(since) {
			recentActivity = append(recentActivity, a)
		}
	}

	s.cMu.RLock()
	defer s.cMu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	result := struct {
		Activities  []activity `json:"activities"`
		TotalPoints int        `json:"totalPoints"`
	}{
		Activities:  recentActivity,
		TotalPoints: s.totalPoints,
	}
	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Printf("Encode(%+v) = %v", result, err)
		return
	}
}
