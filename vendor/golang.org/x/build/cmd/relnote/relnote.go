// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The relnote command summarizes the Go changes in Gerrit marked with
// RELNOTE annotations for the release notes.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"html"
	"io/ioutil"
	"log"
	"regexp"
	"sort"
	"strings"
	"time"

	"golang.org/x/build/maintner"
	"golang.org/x/build/maintner/godata"
)

var (
	htmlMode = flag.Bool("html", false, "write HTML output")
	exclFile = flag.String("exclude-from", "", "optional path to release notes HTML file. If specified, any 'CL NNNN' occurence in the content will cause that CL to be excluded from this tool's output.")
)

// change is a change that was noted via a RELNOTE= comment.
type change struct {
	CL   *maintner.GerritCL
	Note string // the part after RELNOTE=
}

func (c change) TextLine() string {
	subj := clSubject(c.CL)
	if c.Note != "yes" && c.Note != "y" {
		subj = c.Note + ": " + subj
	}
	return fmt.Sprintf("https://golang.org/cl/%d: %s", c.CL.Number, subj)
}

func main() {
	flag.Parse()

	// Releases are every 6 months. Walk forward by 6 month increments to next release.
	cutoff := time.Date(2016, time.August, 1, 00, 00, 00, 0, time.UTC)
	now := time.Now()
	for cutoff.Before(now) {
		cutoff = cutoff.AddDate(0, 6, 0)
	}

	// Previous release was 6 months earlier.
	cutoff = cutoff.AddDate(0, -6, 0)

	var existingHTML []byte
	if *exclFile != "" {
		var err error
		existingHTML, err = ioutil.ReadFile(*exclFile)
		if err != nil {
			log.Fatal(err)
		}
	}

	corpus, err := godata.Get(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	ger := corpus.Gerrit()
	changes := map[string][]change{} // keyed by pkg
	ger.ForeachProjectUnsorted(func(gp *maintner.GerritProject) error {
		if gp.Server() != "go.googlesource.com" {
			return nil
		}
		gp.ForeachCLUnsorted(func(cl *maintner.GerritCL) error {
			if cl.Status != "merged" {
				return nil
			}
			if cl.Commit.CommitTime.Before(cutoff) {
				// Was in a previous release; not for this one.
				return nil
			}
			relnote := clRelNote(cl)
			if relnote == "" ||
				bytes.Contains(existingHTML, []byte(fmt.Sprintf("CL %d", cl.Number))) {
				return nil
			}
			pkg := clPackage(cl)
			changes[pkg] = append(changes[pkg], change{
				Note: relnote,
				CL:   cl,
			})
			return nil
		})
		return nil
	})

	var pkgs []string
	for pkg, changes := range changes {
		pkgs = append(pkgs, pkg)
		sort.Slice(changes, func(i, j int) bool {
			return changes[i].CL.Number < changes[j].CL.Number
		})
	}
	sort.Strings(pkgs)

	if *htmlMode {
		for _, pkg := range pkgs {
			if !strings.HasPrefix(pkg, "cmd/") {
				continue
			}
			for _, change := range changes[pkg] {
				fmt.Printf("<!-- CL %d: %s -->\n", change.CL.Number, change.TextLine())
			}
		}
		for _, pkg := range pkgs {
			if strings.HasPrefix(pkg, "cmd/") {
				continue
			}
			fmt.Printf("<dl id=%q><dt><a href=%q>%s</a></dt>\n  <dd>\n",
				pkg, "/pkg/"+pkg+"/", pkg)
			for _, change := range changes[pkg] {
				changeURL := fmt.Sprintf("https://golang.org/cl/%d", change.CL.Number)
				subj := clSubject(change.CL)
				subj = strings.TrimPrefix(subj, pkg+": ")
				fmt.Printf("    <p><!-- CL %d -->\n      TODO: <a href=%q>%s</a>: %s\n    </p>\n\n",
					change.CL.Number, changeURL, changeURL, html.EscapeString(subj))
			}
			fmt.Printf("</dl><!-- %s -->\n\n", pkg)
		}

	} else {
		for _, pkg := range pkgs {
			fmt.Printf("%s\n", pkg)
			for _, change := range changes[pkg] {
				fmt.Printf("  %s\n", change.TextLine())
			}
		}
	}
}

// clSubject returns the first line of the CL's commit message,
// without the trailing newline.
func clSubject(cl *maintner.GerritCL) string {
	subj := cl.Commit.Msg
	if i := strings.Index(subj, "\n"); i != -1 {
		return subj[:i]
	}
	return subj
}

// clPackage returns the package name from the CL's commit message,
// or "??" if it's formatted unconventionally.
func clPackage(cl *maintner.GerritCL) string {
	subj := clSubject(cl)
	if i := strings.Index(subj, ":"); i != -1 {
		return subj[:i]
	}
	return "??"
}

var relNoteRx = regexp.MustCompile(`RELNOTES?=(.+)`)

func parseRelNote(s string) string {
	if m := relNoteRx.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

func clRelNote(cl *maintner.GerritCL) string {
	msg := cl.Commit.Msg
	if strings.Contains(msg, "RELNOTE") {
		return parseRelNote(msg)
	}
	for _, comment := range cl.Messages {
		if strings.Contains(comment.Message, "RELNOTE") {
			return parseRelNote(comment.Message)
		}
	}
	return ""
}
