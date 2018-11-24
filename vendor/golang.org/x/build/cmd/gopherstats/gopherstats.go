// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"golang.org/x/build/gerrit"
	"golang.org/x/build/internal/gophers"
	"golang.org/x/build/maintner"
	"golang.org/x/build/maintner/godata"
	"golang.org/x/oauth2"
)

var (
	mode      = flag.String("mode", "", "mode to run in. Valid values:\n\n"+modeSummary())
	startTime = newTimeFlag("from", "1900-01-01", "start of time range for the 'range-stats' mode")
	endTime   = newTimeFlag("to", "2100-01-01", "end of time range for the 'range-stats' mode")
	timeZone  = flag.String("tz", "US/Pacific", "timezone to use for time values")
)

func newTimeFlag(name, defVal, desc string) *time.Time {
	var t time.Time
	tf := (*timeFlag)(&t)
	if err := tf.Set(defVal); err != nil {
		panic(err.Error())
	}
	flag.Var(tf, name, desc)
	return &t
}

type timeFlag time.Time

func (t *timeFlag) String() string { return time.Time(*t).String() }
func (t *timeFlag) Set(v string) error {
	loc, err := time.LoadLocation(*timeZone)
	if err != nil {
		return err
	}
	for _, pat := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02",
	} {
		parsedTime, err := time.ParseInLocation(pat, v, loc)
		if err == nil {
			*t = timeFlag(parsedTime)
			return nil
		}
	}
	return fmt.Errorf("unrecognized RFC3339 or prefix %q", v)
}

type handler struct {
	fn   func(*statsClient)
	desc string
}

var modes = map[string]handler{
	"find-github-email":   {(*statsClient).findGithubEmails, "discover mappings between github usernames and emails"},
	"gerrit-groups":       {(*statsClient).gerritGroups, "print stats on gerrit groups"},
	"github-groups":       {(*statsClient).githubGroups, "print stats on github groups"},
	"github-issue-close":  {(*statsClient).githubIssueCloseStats, "print stats on github issues closes by quarter (googler-vs-not, unique numbers)"},
	"gerrit-cls":          {(*statsClient).gerritCLStats, "print stats on opened gerrit CLs by quarter"},
	"workshop-stats":      {(*statsClient).workshopStats, "print stats from contributor workshop"},
	"find-gerrit-gophers": {(*statsClient).findGerritGophers, "discover mappings between internal/gopher entries and Gerrit IDs"},
	"range-stats":         {(*statsClient).rangeStats, "show various summaries of activity in the flag-provided time range"},
}

func modeSummary() string {
	var buf bytes.Buffer
	var sorted []string
	for mode := range modes {
		sorted = append(sorted, mode)
	}
	sort.Strings(sorted)
	for _, mode := range sorted {
		fmt.Fprintf(&buf, "%q: %s\n", mode, modes[mode].desc)
	}
	return buf.String()
}

type statsClient struct {
	lazyGitHub *github.Client
	lazyGerrit *gerrit.Client

	corpusCache *maintner.Corpus
}

func (sc *statsClient) github() *github.Client {
	if sc.lazyGitHub != nil {
		return sc.lazyGitHub
	}
	ghc, err := getGithubClient()
	if err != nil {
		log.Fatal(err)
	}
	sc.lazyGitHub = ghc
	return ghc
}

func (sc *statsClient) gerrit() *gerrit.Client {
	if sc.lazyGerrit != nil {
		return sc.lazyGerrit
	}
	gerrc := gerrit.NewClient("https://go-review.googlesource.com", gerrit.GitCookieFileAuth(filepath.Join(os.Getenv("HOME"), ".gitcookies")))
	sc.lazyGerrit = gerrc
	return gerrc
}

func (sc *statsClient) corpus() *maintner.Corpus {
	if sc.corpusCache == nil {
		var err error
		sc.corpusCache, err = godata.Get(context.Background())
		if err != nil {
			log.Fatalf("Loading maintner corpus: %v", err)
		}
	}
	return sc.corpusCache
}

func main() {
	flag.Parse()

	if *mode == "" {
		fmt.Fprintf(os.Stderr, "Missing required --mode flag.\n")
		flag.Usage()
		os.Exit(1)
	}
	h, ok := modes[*mode]
	if !ok {
		fmt.Fprintf(os.Stderr, "Unknown --mode flag.\n")
		flag.Usage()
		os.Exit(1)
	}

	sc := &statsClient{}
	h.fn(sc)
}

func (sc *statsClient) gerritGroups() {
	ctx := context.Background()
	gerrc := sc.gerrit()

	groups, err := gerrc.GetGroups(ctx)
	if err != nil {
		log.Fatalf("Gerrit.GetGroups: %v", err)
	}
	for name, gi := range groups {
		switch name {
		case "admins", "approvers", "may-start-trybots", "gophers",
			"may-abandon-changes",
			"may-forge-author-identity", "osp-team",
			"release-managers":
			members, err := gerrc.GetGroupMembers(ctx, gi.ID)
			if err != nil {
				log.Fatal(err)
			}
			numGoog, numExt := 0, 0
			for _, member := range members {
				//fmt.Printf("  %s: %+v\n", name, member)
				p := gophers.GetGerritPerson(member)
				if p == nil {
					fmt.Printf("addPerson(%q, %q)\n", member.Name, member.Email)
				} else {
					if p.Googler {
						numGoog++
					} else {
						numExt++
					}
				}
			}
			fmt.Printf("Group %s: %d total (%d googlers, %d external)\n", name, numGoog+numExt, numGoog, numExt)
		}
	}
}

// quarter returns a quarter of a year, in the form "2017q1".
func quarter(t time.Time) string {
	// TODO: do this allocation-free? preculate them in init?
	return fmt.Sprintf("%04dq%v", t.Year(), (int(t.Month()-1)/3)+1)
}

func (sc *statsClient) githubIssueCloseStats() {
	repo := sc.corpus().GitHub().Repo("golang", "go")
	if repo == nil {
		log.Fatal("Failed to find Go repo.")
	}
	commClosed := map[string]map[*gophers.Person]int{}
	googClosed := map[string]map[*gophers.Person]int{}
	quarterSet := map[string]struct{}{}
	repo.ForeachIssue(func(gi *maintner.GitHubIssue) error {
		if !gi.Closed {
			return nil
		}
		gi.ForeachEvent(func(e *maintner.GitHubIssueEvent) error {
			if e.Type != "closed" {
				return nil
			}
			if e.Actor == nil {
				return nil
			}
			q := quarter(e.Created)
			quarterSet[q] = struct{}{}
			if commClosed[q] == nil {
				commClosed[q] = map[*gophers.Person]int{}
			}
			if googClosed[q] == nil {
				googClosed[q] = map[*gophers.Person]int{}
			}
			var p *gophers.Person
			if e.Actor.Login == "gopherbot" {
				gc := sc.corpus().GitCommit(e.CommitID)
				if gc != nil {
					email := gc.Author.Email()
					p = gophers.GetPerson(email)
					if p == nil {
						log.Printf("unknown closer email: %q", email)
					}
				}
			} else {
				p = gophers.GetPerson("@" + e.Actor.Login)
			}
			if p != nil {
				if p.Googler {
					googClosed[q][p]++
				} else {
					commClosed[q][p]++
				}
			}
			return nil
		})
		return nil
	})
	sumPeeps := func(m map[*gophers.Person]int) (sum int) {
		for _, v := range m {
			sum += v
		}
		return
	}
	var quarters []string
	for q := range quarterSet {
		quarters = append(quarters, q)
	}
	sort.Strings(quarters)
	for _, q := range quarters {
		googTotal := sumPeeps(googClosed[q])
		commTotal := sumPeeps(commClosed[q])
		googUniq := len(googClosed[q])
		commUniq := len(commClosed[q])
		tot := googTotal + commTotal
		totUniq := googUniq + commUniq
		percentGoog := 100 * float64(googTotal) / float64(tot)
		fmt.Printf("%s closed issues: %v closes (%.2f%% goog %d; ext %d), %d unique people (%d goog, %d ext)\n",
			q, tot,
			percentGoog, googTotal, commTotal,
			totUniq, googUniq, commUniq,
		)
	}
}

type personSet struct {
	s       map[*gophers.Person]struct{}
	numGoog int
	numExt  int
}

func (s *personSet) sum() int { return len(s.s) }

func (s *personSet) add(p *gophers.Person) {
	if s.s == nil {
		s.s = make(map[*gophers.Person]struct{})
	}
	if _, ok := s.s[p]; !ok {
		s.s[p] = struct{}{}
		if p.Googler {
			s.numGoog++
		} else {
			s.numExt++
		}
	}
}

func (sc *statsClient) githubGroups() {
	ctx := context.Background()
	ghc := sc.github()
	teamList, _, err := ghc.Repositories.ListTeams(ctx, "golang", "go", nil)
	if err != nil {
		log.Fatal(err)
	}

	var teams = map[string]*personSet{}
	for _, t := range teamList {
		teamName := t.GetName()
		switch teamName {
		default:
			continue
		case "go-approvers", "gophers":
		}

		ps := new(personSet)
		teams[teamName] = ps
		users, _, err := ghc.Teams.ListTeamMembers(ctx, t.GetID(), &github.TeamListTeamMembersOptions{
			ListOptions: github.ListOptions{PerPage: 1000},
		})
		if err != nil {
			log.Fatal(err)
		}

		for _, u := range users {
			login := strings.ToLower(u.GetLogin())
			if login == "gopherbot" {
				continue
			}
			p := gophers.GetPerson("@" + login)
			if p == nil {
				panic(fmt.Sprintf("failed to find github person %q", "@"+login))
			}
			ps.add(p)
		}
	}

	cur := teams["go-approvers"]
	prev := parseOldSnapshot(githubGoApprovers20170106)
	log.Printf("Approvers 2016-12-13: %d: %v goog, %v ext", prev.sum(), prev.numGoog, prev.numExt)
	log.Printf("Approvers        cur: %d: %v goog, %v ext", cur.sum(), cur.numGoog, cur.numExt)
}

func parseOldSnapshot(s string) *personSet {
	ps := new(personSet)
	for _, f := range strings.Fields(s) {
		if !strings.HasPrefix(f, "@") {
			continue
		}
		p := gophers.GetPerson(f)
		if p == nil {
			panic(fmt.Sprintf("failed to find github person %q", f))
		}
		ps.add(p)
	}
	return ps
}

// Gerrit 2016-12-13:
// May start trybots, non-Googlers: 11
// Approvers, non-Googlers: 19

const githubGoApprovers20170106 = `
@0intro
0intro
David du Colombier
 
@4ad
4ad
Aram Hăvărneanu
 
@adams-sarah
adams-sarah
Sarah Adams
 
@adg
adg Owner
Andrew Gerrand
 
@alexbrainman
alexbrainman
Alex Brainman
 
@ality
ality
Anthony Martin
 
@campoy
campoy
Francesc Campoy
 
@DanielMorsing
DanielMorsing
Daniel Morsing
 
@davecheney
davecheney
Dave Cheney
 
@davidlazar
davidlazar
David Lazar
 
@dvyukov
dvyukov
Dmitry Vyukov
 
@eliasnaur
eliasnaur
Elias Naur
 
@hanwen
hanwen
Han-Wen Nienhuys
 
@josharian
josharian
Josh Bleecher Snyder
 
@jpoirier
jpoirier
Joseph Poirier
 
@kardianos
kardianos
Daniel Theophanes
 
@martisch
martisch
Martin Möhrmann
 
@matloob
matloob
Michael Matloob
 
@mdempsky
mdempsky
Matthew Dempsky
 
@mikioh
mikioh
Mikio Hara
 
@minux
minux
Minux Ma
 
@mwhudson
mwhudson
Michael Hudson-Doyle
 
@neild
neild
Damien Neil
 
@niemeyer
niemeyer
Gustavo Niemeyer
 
@odeke-em
odeke-em
Emmanuel T Odeke
 
@quentinmit
quentinmit Owner
Quentin Smith
 
@rakyll
rakyll
jbd@
 
@remyoudompheng
remyoudompheng
Rémy Oudompheng
 
@rminnich
rminnich
ron minnich
 
@rogpeppe
rogpeppe
Roger Peppe
 
@rui314
rui314
Rui Ueyama
 
@thanm
thanm
Than McIntosh
`

const githubGoAssignees20170106 = `
@crawshaw
crawshaw Team maintainer
David Crawshaw
 
@0intro
0intro
David du Colombier
 
@4ad
4ad
Aram Hăvărneanu
 
@adams-sarah
adams-sarah
Sarah Adams
 
@alexbrainman
alexbrainman
Alex Brainman
 
@alexcesaro
alexcesaro
Alexandre Cesaro
 
@ality
ality
Anthony Martin
 
@artyom
artyom
Artyom Pervukhin
 
@bcmills
bcmills
Bryan C. Mills
 
@billotosyr
billotosyr
 
@brtzsnr
brtzsnr
Alexandru Moșoi
 
@bsiegert
bsiegert
Benny Siegert
 
@c4milo
c4milo
Camilo Aguilar
 
@carl-mastrangelo
carl-mastrangelo
Carl Mastrangelo
 
@cespare
cespare
Caleb Spare
 
@DanielMorsing
DanielMorsing
Daniel Morsing
 
@davecheney
davecheney
Dave Cheney
 
@dominikh
dominikh
Dominik Honnef
 
@dskinner
dskinner
Daniel Skinner
 
@dsnet
dsnet
Joe Tsai
 
@dspezia
dspezia
Didier Spezia
 
@eliasnaur
eliasnaur
Elias Naur
 
@emergencybutter
emergencybutter
Arnaud
 
@evandbrown
evandbrown
Evan Brown
 
@fatih
fatih
Fatih Arslan
 
@garyburd
garyburd
Gary Burd
 
@hanwen
hanwen
Han-Wen Nienhuys
 
@jeffallen
jeffallen
Jeff R. Allen
 
@johanbrandhorst
johanbrandhorst
Johan Brandhorst
 
@josharian
josharian
Josh Bleecher Snyder
 
@jtsylve
jtsylve
Joe Sylve
 
@kardianos
kardianos
Daniel Theophanes
 
@kytrinyx
kytrinyx
Katrina Owen
 
@marete
marete
Brian Gitonga Marete
 
@martisch
martisch
Martin Möhrmann
 
@mattn
mattn
mattn
 
@mdempsky
mdempsky
Matthew Dempsky
 
@mdlayher
mdlayher
Matt Layher
 
@mikioh
mikioh
Mikio Hara
 
@millerresearch
millerresearch
Richard Miller
 
@minux
minux
Minux Ma
 
@mundaym
mundaym
Michael Munday
 
@mwhudson
mwhudson
Michael Hudson-Doyle
 
@myitcv
myitcv
Paul Jolly
 
@neelance
neelance
Richard Musiol
 
@niemeyer
niemeyer
Gustavo Niemeyer
 
@nodirt
nodirt
Nodir Turakulov
 
@rahulchaudhry
rahulchaudhry
Rahul Chaudhry
 
@rauls5382
rauls5382
Raul Silvera
 
@remyoudompheng
remyoudompheng
Rémy Oudompheng
 
@rhysh
rhysh
Rhys Hiltner
 
@rogpeppe
rogpeppe
Roger Peppe
 
@rsc
rsc Owner
Russ Cox
 
@rui314
rui314
Rui Ueyama
 
@sbinet
sbinet
Sebastien Binet
 
@shawnps
shawnps
Shawn Smith
 
@thanm
thanm
Than McIntosh
 
@titanous
titanous
Jonathan Rudenberg
 
@tombergan
tombergan
 
@tzneal
tzneal
Todd
 
@vstefanovic
vstefanovic
 
@wathiede
wathiede
Bill
 
@x1ddos
x1ddos
alex
 
@zombiezen
zombiezen
Ross Light
`

var discoverGoRepo = flag.String("discovery-go-repo", "go", "github.com/golang repo to discovery email addreses from")

func (sc *statsClient) findGerritGophers() {
	gerrc := sc.gerrit()
	log.Printf("find gerrit gophers")
	gerritEmails := map[string]int{}

	const suffix = "@62eb7196-b449-3ce5-99f1-c037f21e1705"

	sc.corpus().Gerrit().ForeachProjectUnsorted(func(gp *maintner.GerritProject) error {
		return gp.ForeachCLUnsorted(func(cl *maintner.GerritCL) error {
			for _, meta := range cl.Metas {
				who := meta.Commit.Author.Email()
				if strings.HasSuffix(who, suffix) {
					gerritEmails[who]++
				}
			}
			return nil
		})
	})

	var emails []string
	for k := range gerritEmails {
		emails = append(emails, k)
	}
	sort.Slice(emails, func(i, j int) bool {
		return gerritEmails[emails[j]] < gerritEmails[emails[i]]
	})
	for _, email := range emails {
		p := gophers.GetPerson(email)
		if p == nil {
			ai, err := gerrc.GetAccountInfo(context.Background(), strings.TrimSuffix(email, suffix))
			if err != nil {
				log.Printf("Looking up %s: %v", email, err)
				continue
			}
			fmt.Printf("addPerson(%q, %q, %q)\n", ai.Name, ai.Email, email)
		}
	}

}

func (sc *statsClient) findGithubEmails() {
	ghc := sc.github()
	seen := map[string]bool{}
	for page := 1; page < 500; page++ {
		commits, _, err := ghc.Repositories.ListCommits(context.Background(), "golang", *discoverGoRepo, &github.CommitsListOptions{
			ListOptions: github.ListOptions{Page: page, PerPage: 1000},
		})
		if err != nil {
			log.Fatalf("page %d: %v", page, err)
		}
		for _, com := range commits {
			ghUser := com.Author.GetLogin()
			if ghUser == "" {
				continue
			}
			if seen[ghUser] {
				continue
			}
			seen[ghUser] = true
			ca := com.Commit.Author

			p := gophers.GetPerson("@" + ghUser)
			if p != nil && gophers.GetPerson(ca.GetEmail()) == p {
				// Nothing new.
				continue
			}
			fmt.Printf("addPerson(%q, %q, %q)\n", ca.GetName(), ca.GetEmail(), "@"+ghUser)
		}
	}
}

func (sc *statsClient) gerritCLStats() {
	perQuarter := map[string]int{}
	perQuarterGoog := map[string]int{}
	perQuarterExt := map[string]int{}
	printedUnknown := map[string]bool{}
	perQuarterUniq := map[string]*personSet{}

	sc.corpus().Gerrit().ForeachProjectUnsorted(func(gp *maintner.GerritProject) error {
		gp.ForeachCLUnsorted(func(cl *maintner.GerritCL) error {
			q := quarter(cl.Created)
			perQuarter[q]++
			email := cl.Commit.Author.Email()
			p := gophers.GetPerson(email)
			var isGoog bool
			if p != nil {
				isGoog = p.Googler
				if _, ok := perQuarterUniq[q]; !ok {
					perQuarterUniq[q] = new(personSet)
				}
				perQuarterUniq[q].add(p)
			} else {
				isGoog = strings.HasSuffix(email, "@google.com")
				if !printedUnknown[email] {
					printedUnknown[email] = true
					fmt.Printf("addPerson(%q, %q)\n", cl.Commit.Author.Name(), email)

				}
			}
			if isGoog {
				perQuarterGoog[q]++
			} else {
				perQuarterExt[q]++
			}
			return nil
		})
		return nil
	})
	for _, q := range sortedStrMapKeys(perQuarter) {
		goog := perQuarterGoog[q]
		ext := perQuarterExt[q]
		tot := goog + ext
		fmt.Printf("%s: %d commits (%0.2f%% %d goog, %d ext)\n", q, perQuarter[q], 100*float64(goog)/float64(tot), goog, ext)
	}
	for _, q := range sortedStrMapKeys(perQuarter) {
		ps := perQuarterUniq[q]
		fmt.Printf("%s: %d unique users (%0.2f%% %d goog, %d ext)\n", q, len(ps.s), 100*float64(ps.numGoog)/float64(len(ps.s)), ps.numGoog, ps.numExt)
	}
}

func sortedStrMapKeys(m map[string]int) []string {
	ret := make([]string, 0, len(m))
	for k := range m {
		ret = append(ret, k)
	}
	sort.Strings(ret)
	return ret
}

func (sc *statsClient) workshopStats() {
	const workshopIssue = 21017
	loc, err := time.LoadLocation("America/Denver")
	if err != nil {
		fmt.Fprintf(os.Stderr, "loading location failed: %v", err)
		os.Exit(2)
	}
	workshopStartDate := time.Date(2017, time.July, 15, 0, 0, 0, 0, loc)

	// The key is the string representation of the gerrit ID.
	// The value is the string for the GitHub login.
	contributors := map[string]string{}

	// Get all the contributors from comments on the issue.
	sc.corpus().GitHub().Repo("golang", "go").Issue(workshopIssue).ForeachComment(func(c *maintner.GitHubComment) error {
		contributors[strings.TrimSpace(c.Body)] = c.User.Login
		return nil
	})
	fmt.Printf("Number of registrations: %d\n", len(contributors))

	// Store the already known contributors before the workshop.
	knownContributors := map[string]struct{}{}

	type projectStats struct {
		name      string
		openedCLs []string // Gerrit IDs of owners of opened CLs
		mergedCLs []string // Gerrit IDs of owners of merged CLs
	}
	ps := []projectStats{}

	// Get all the CLs during the time of the workshop and after.
	sc.corpus().Gerrit().ForeachProjectUnsorted(func(gp *maintner.GerritProject) error {
		p := projectStats{
			name: gp.Project(),
		}
		gp.ForeachCLUnsorted(func(cl *maintner.GerritCL) error {
			ownerID := fmt.Sprintf("%d", cl.OwnerID())
			// Make sure it was made after the workshop started
			// otherwise save as a known contributor.
			if cl.Created.After(workshopStartDate) {
				if _, ok := contributors[ownerID]; ok {
					p.openedCLs = append(p.openedCLs, ownerID)
					if cl.Status == "merged" {
						p.mergedCLs = append(p.mergedCLs, ownerID)
					}
				}
			} else {
				knownContributors[ownerID] = struct{}{}
			}
			return nil
		})

		// Return early if no one contributed to that project.
		if len(p.openedCLs) == 0 && len(p.mergedCLs) == 0 {
			return nil
		}

		ps = append(ps, p)
		return nil
	})

	sort.Slice(ps, func(i, j int) bool { return ps[i].name < ps[j].name })
	for _, p := range ps {
		var newOpened, newMerged int

		// Determine the first time contributors.
		for _, id := range p.openedCLs {
			if _, ok := knownContributors[id]; !ok {
				newOpened++
			}
		}
		for _, id := range p.mergedCLs {
			if _, ok := knownContributors[id]; !ok {
				newMerged++
			}
		}

		// Ignore repos where only past contributors had patches merged.
		if newOpened != 0 || newMerged != 0 {
			fmt.Printf(`%s:
	Total Opened CLs: %d
	Total Merged CLs: %d
	New Contributors Opened CLs: %d
	New Contributors Merged CLs: %d`+"\n", p.name, len(p.openedCLs), len(p.mergedCLs), newOpened, newMerged)
		}
	}
}

func (sc *statsClient) rangeStats() {
	var (
		newCLs                    = map[*gophers.Person]int{}
		commentsOnOtherCLs        = map[*gophers.Person]int{}
		githubIssuesCreated       = map[*gophers.Person]int{}
		githubUniqueIssueComments = map[*gophers.Person]int{} // non-owner
		githubUniqueIssueEvents   = map[*gophers.Person]int{} // non-owner
		uniqueFilesEdited         = map[*gophers.Person]int{}
		uniqueDirsEdited          = map[*gophers.Person]int{}
	)

	t1 := *startTime
	t2 := *endTime

	sc.corpus().GitHub().ForeachRepo(func(r *maintner.GitHubRepo) error {
		if r.ID().Owner != "golang" {
			return nil
		}
		return r.ForeachIssue(func(gi *maintner.GitHubIssue) error {
			if gi.User == nil {
				return nil
			}
			owner := gophers.GetPerson("@" + gi.User.Login)
			if gi.Created.After(t1) && gi.Created.Before(t2) {
				if owner == nil {
					log.Printf("No owner for golang.org/issue/%d (%q)", gi.Number, gi.User.Login)
				} else if !owner.Bot {
					githubIssuesCreated[owner]++
				}
			}

			sawCommenter := map[*gophers.Person]bool{}
			gi.ForeachComment(func(gc *maintner.GitHubComment) error {
				if gc.User == nil || gc.User.ID == gi.User.ID {
					return nil
				}
				if gc.Created.After(t1) && gc.Created.Before(t2) {
					commenter := gophers.GetPerson("@" + gc.User.Login)
					if commenter == nil || sawCommenter[commenter] || commenter.Bot {
						return nil
					}
					sawCommenter[commenter] = true
					githubUniqueIssueComments[commenter]++
				}
				return nil
			})

			sawEventer := map[*gophers.Person]bool{}
			gi.ForeachEvent(func(gc *maintner.GitHubIssueEvent) error {
				if gc.Actor == nil || gc.Actor.ID == gi.User.ID {
					return nil
				}
				if gc.Created.After(t1) && gc.Created.Before(t2) {
					eventer := gophers.GetPerson("@" + gc.Actor.Login)
					if eventer == nil || sawEventer[eventer] || eventer.Bot {
						return nil
					}
					sawEventer[eventer] = true
					githubUniqueIssueEvents[eventer]++
				}
				return nil
			})

			return nil
		})
	})

	type projectFile struct {
		gp   *maintner.GerritProject
		file string
	}
	var fileTouched = map[*gophers.Person]map[projectFile]bool{}
	var dirTouched = map[*gophers.Person]map[projectFile]bool{}

	sc.corpus().Gerrit().ForeachProjectUnsorted(func(gp *maintner.GerritProject) error {
		if gp.Server() != "go.googlesource.com" {
			return nil
		}
		return gp.ForeachCLUnsorted(func(cl *maintner.GerritCL) error {
			owner := gophers.GetPerson(cl.Commit.Author.Email())
			if cl.Created.After(t1) && cl.Created.Before(t2) {
				newCLs[owner]++
			}

			if ct := cl.Commit.CommitTime; ct.After(t1) && ct.Before(t2) && cl.Status == "merged" {
				email := cl.Commit.Author.Email() // gerrit-y email
				who := gophers.GetPerson(email)
				if who != nil {
					if fileTouched[who] == nil {
						fileTouched[who] = map[projectFile]bool{}
					}
					if dirTouched[who] == nil {
						dirTouched[who] = map[projectFile]bool{}
					}
					for _, diff := range cl.Commit.Files {
						if strings.Contains(diff.File, "vendor/") {
							continue
						}
						fileTouched[who][projectFile{gp, diff.File}] = true
						dirTouched[who][projectFile{gp, path.Dir(diff.File)}] = true
					}
				}
			}

			saw := map[*gophers.Person]bool{}
			for _, meta := range cl.Metas {
				t := meta.Commit.CommitTime
				if t.Before(t1) || t.After(t2) {
					continue
				}
				email := meta.Commit.Author.Email() // gerrit-y email
				who := gophers.GetPerson(email)
				if who == owner || who == nil || saw[who] || who.Bot {
					continue
				}
				saw[who] = true
				commentsOnOtherCLs[who]++
			}
			return nil
		})
	})

	for p, m := range fileTouched {
		uniqueFilesEdited[p] = len(m)
	}
	for p, m := range dirTouched {
		uniqueDirsEdited[p] = len(m)
	}

	top(newCLs, "CLs created:", 20)
	top(commentsOnOtherCLs, "Unique non-self CLs commented on:", 20)

	top(githubIssuesCreated, "GitHub issues created:", 20)
	top(githubUniqueIssueComments, "Unique GitHub issues commented on:", 20)
	top(githubUniqueIssueEvents, "Unique GitHub issues acted on:", 20)

	top(uniqueFilesEdited, "Unique files edited:", 20)
	top(uniqueDirsEdited, "Unique directories edited:", 20)
}

func top(m map[*gophers.Person]int, title string, n int) {
	var kk []*gophers.Person
	for k := range m {
		if k == nil {
			continue
		}
		kk = append(kk, k)
	}
	sort.Slice(kk, func(i, j int) bool { return m[kk[j]] < m[kk[i]] })
	fmt.Println(title)
	for i, k := range kk {
		if i == n {
			break
		}
		fmt.Printf(" %5d %s\n", m[k], k.Name)
	}
	fmt.Println()
}

func getGithubToken() (string, error) {
	// TODO: get from GCE metadata, etc.
	tokenFile := filepath.Join(os.Getenv("HOME"), "keys", "github-read-org")
	slurp, err := ioutil.ReadFile(tokenFile)
	if err != nil {
		return "", err
	}
	f := strings.SplitN(strings.TrimSpace(string(slurp)), ":", 2)
	if len(f) != 2 || f[0] == "" || f[1] == "" {
		return "", fmt.Errorf("Expected token file %s to be of form <username>:<token>", tokenFile)
	}
	return f[1], nil
}

func getGithubClient() (*github.Client, error) {
	token, err := getGithubToken()
	if err != nil {
		return nil, err
	}
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)
	return github.NewClient(tc), nil
}
