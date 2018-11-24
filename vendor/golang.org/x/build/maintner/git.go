// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package maintner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/build/maintner/maintpb"
)

// GitHash is a git commit in binary form (NOT hex form).
// They are currently always 20 bytes long. (for SHA-1 refs)
// That may change in the future.
type GitHash string

func (h GitHash) String() string { return fmt.Sprintf("%x", string(h)) }

// requires c.mu be held for writing
func (c *Corpus) gitHashFromHexStr(s string) GitHash {
	if len(s) != 40 {
		panic(fmt.Sprintf("bogus git hash %q", s))
	}
	var buf [40]byte
	copy(buf[:], s)
	_, err := hex.Decode(buf[:20], buf[:]) // aliasing is safe
	if err != nil {
		panic(fmt.Sprintf("bogus git hash %q: %v", s, err))
	}
	return GitHash(c.strb(buf[:20]))
}

// requires c.mu be held for writing
func (c *Corpus) gitHashFromHex(s []byte) GitHash {
	if len(s) != 40 {
		panic(fmt.Sprintf("bogus git hash %q", s))
	}
	var buf [20]byte
	_, err := hex.Decode(buf[:], s)
	if err != nil {
		panic(fmt.Sprintf("bogus git hash %q: %v", s, err))
	}
	return GitHash(c.strb(buf[:20]))
}

// placeholderCommitter is a sentinel value for GitCommit.Committer to
// mean that the GitCommit is a placeholder. It's used for commits we
// know should exist (because they're referenced as parents) but we
// haven't yet seen in the log.
var placeholderCommitter = new(GitPerson)

// GitCommit represents a single commit in a git repository.
type GitCommit struct {
	Hash       GitHash
	Tree       GitHash
	Parents    []*GitCommit
	Author     *GitPerson
	AuthorTime time.Time
	Committer  *GitPerson
	Reviewer   *GitPerson
	CommitTime time.Time
	Msg        string // Commit message subject and body
	Files      []*maintpb.GitDiffTreeFile
	GerritMeta *GerritMeta // non-nil if it's a Gerrit NoteDB meta commit
}

func (gc *GitCommit) String() string {
	if gc == nil {
		return "<nil *GitCommit>"
	}
	return fmt.Sprintf("{GitCommit %s}", gc.Hash)
}

// HasAncestor reports whether gc contains the provided ancestor
// commit in gc's history.
func (gc *GitCommit) HasAncestor(ancestor *GitCommit) bool {
	return gc.hasAncestor(ancestor, make(map[*GitCommit]bool))
}

func (gc *GitCommit) hasAncestor(ancestor *GitCommit, checked map[*GitCommit]bool) bool {
	if v, ok := checked[gc]; ok {
		return v
	}
	checked[gc] = false
	for _, pc := range gc.Parents {
		if pc == nil {
			panic("nil parent")
		}
		if pc.Committer == placeholderCommitter {
			log.Printf("WARNING: hasAncestor(%q, %q) found parent %q with placeholder parent", gc.Hash, ancestor.Hash, pc.Hash)
		}
		if pc.Hash == ancestor.Hash || pc.hasAncestor(ancestor, checked) {
			checked[gc] = true
			return true
		}
	}
	return false
}

// Summary returns the first line of the commit message.
func (gc *GitCommit) Summary() string {
	s := gc.Msg
	if i := strings.IndexByte(s, '\n'); i != -1 {
		s = s[:i]
	}
	s = strings.TrimSpace(s)
	return s
}

// SameDiffStat reports whether gc has the same diff stat numbers as b.
// If either is unknown, false is returned.
func (gc *GitCommit) SameDiffStat(b *GitCommit) bool {
	if len(gc.Files) != len(b.Files) {
		return false
	}
	for i, af := range gc.Files {
		bf := b.Files[i]
		if af == nil || bf == nil {
			return false
		}
		if *af != *bf {
			return false
		}
	}
	return true
}

// GitPerson is a person in a git commit.
type GitPerson struct {
	Str string // "Foo Bar <foo@bar.com>"
}

// Email returns the GitPerson's email address only, without the name
// or angle brackets.
func (p *GitPerson) Email() string {
	lt := strings.IndexByte(p.Str, '<')
	gt := strings.IndexByte(p.Str, '>')
	if lt < 0 || gt < lt {
		return ""
	}
	return p.Str[lt+1 : gt]
}

func (p *GitPerson) Name() string {
	i := strings.IndexByte(p.Str, '<')
	if i < 0 {
		return p.Str
	}
	return strings.TrimSpace(p.Str[:i])
}

// requires c.mu be held for writing.
func (c *Corpus) enqueueCommitLocked(h GitHash) {
	if _, ok := c.gitCommit[h]; ok {
		return
	}
	if c.gitCommitTodo == nil {
		c.gitCommitTodo = map[GitHash]bool{}
	}
	c.gitCommitTodo[h] = true
}

// syncGitCommits polls for git commits in a directory.
func (c *Corpus) syncGitCommits(ctx context.Context, conf polledGitCommits, loop bool) error {
	cmd := exec.CommandContext(ctx, "git", "show-ref", "refs/remotes/origin/master")
	cmd.Dir = conf.dir
	out, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	outs := strings.TrimSpace(string(out))
	if outs == "" {
		return fmt.Errorf("no remote found for refs/remotes/origin/master")
	}
	ref := strings.Fields(outs)[0]
	c.mu.Lock()
	refHash := c.gitHashFromHexStr(ref)
	c.enqueueCommitLocked(refHash)
	c.mu.Unlock()

	idle := false
	for {
		hash := c.gitCommitToIndex()
		if hash == "" {
			if !loop {
				return nil
			}
			if !idle {
				log.Printf("All git commits index for %v; idle.", conf.repo)
				idle = true
			}
			time.Sleep(5 * time.Second)
			continue
		}
		if err := c.indexCommit(conf, hash); err != nil {
			log.Printf("Error indexing %v: %v", hash, err)
			select {
			case <-ctx.Done():
				return ctx.Err()
				// TODO: temporary vs permanent failure? reschedule? fail hard?
				// For now just loop with a sleep.
			case <-time.After(5 * time.Second):
			}
		}
	}
}

// returns nil if no work.
func (c *Corpus) gitCommitToIndex() GitHash {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for hash := range c.gitCommitTodo {
		if _, ok := c.gitCommit[hash]; !ok {
			return hash
		}
		log.Printf("Warning: git commit %v in todo map, but already known; ignoring", hash)
	}
	return ""
}

var (
	nlnl           = []byte("\n\n")
	parentSpace    = []byte("parent ")
	authorSpace    = []byte("author ")
	committerSpace = []byte("committer ")
	treeSpace      = []byte("tree ")
	golangHgSpace  = []byte("golang-hg ")
	gpgSigSpace    = []byte("gpgsig ")
	encodingSpace  = []byte("encoding ")
	space          = []byte(" ")
)

func parseCommitFromGit(dir string, hash GitHash) (*maintpb.GitCommit, error) {
	cmd := exec.Command("git", "cat-file", "commit", hash.String())
	cmd.Dir = dir
	catFile, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git cat-file -p %v: %v", hash, err)
	}
	cmd = exec.Command("git", "diff-tree", "--numstat", hash.String())
	cmd.Dir = dir
	diffTreeOut, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff-tree --numstat %v: %v", hash, err)
	}

	diffTree := &maintpb.GitDiffTree{}
	bs := bufio.NewScanner(bytes.NewReader(diffTreeOut))
	lineNum := 0
	for bs.Scan() {
		line := strings.TrimSpace(bs.Text())
		lineNum++
		if lineNum == 1 && line == hash.String() {
			continue
		}
		f := strings.Fields(line)
		// A line is like: <added> WS+ <deleted> WS+ <filename>
		// Where <added> or <deleted> can be '-' to mean binary.
		// The filename could contain spaces.
		// 49      8       maintner/maintner.go
		// Or:
		// 49      8       some/name with spaces.txt
		if len(f) < 3 {
			continue
		}
		binary := f[0] == "-" || f[1] == "-"
		added, _ := strconv.ParseInt(f[0], 10, 64)
		deleted, _ := strconv.ParseInt(f[1], 10, 64)
		file := strings.TrimPrefix(line, f[0])
		file = strings.TrimSpace(file)
		file = strings.TrimPrefix(file, f[1])
		file = strings.TrimSpace(file)

		diffTree.File = append(diffTree.File, &maintpb.GitDiffTreeFile{
			File:    file,
			Added:   added,
			Deleted: deleted,
			Binary:  binary,
		})
	}
	if err := bs.Err(); err != nil {
		return nil, err
	}
	commit := &maintpb.GitCommit{
		Raw:      catFile,
		DiffTree: diffTree,
	}
	switch len(hash) {
	case 20:
		commit.Sha1 = hash.String()
	default:
		return nil, fmt.Errorf("unsupported git hash %q", hash.String())
	}
	return commit, nil
}

func (c *Corpus) indexCommit(conf polledGitCommits, hash GitHash) error {
	if conf.repo == nil {
		panic("bogus config; nil repo")
	}
	commit, err := parseCommitFromGit(conf.dir, hash)
	if err != nil {
		return err
	}
	m := &maintpb.Mutation{
		Git: &maintpb.GitMutation{
			Repo:   conf.repo,
			Commit: commit,
		},
	}
	c.addMutation(m)
	return nil
}

// c.mu is held for writing.
func (c *Corpus) processGitMutation(m *maintpb.GitMutation) {
	commit := m.Commit
	if commit == nil {
		return
	}
	// TODO: care about m.Repo?
	c.processGitCommit(commit)
}

// c.mu is held for writing.
func (c *Corpus) processGitCommit(commit *maintpb.GitCommit) (*GitCommit, error) {
	if c.gitCommit == nil {
		c.gitCommit = map[GitHash]*GitCommit{}
	}
	if len(commit.Sha1) != 40 {
		return nil, fmt.Errorf("bogus git sha1 %q", commit.Sha1)
	}
	hash := c.gitHashFromHexStr(commit.Sha1)

	catFile := commit.Raw
	i := bytes.Index(catFile, nlnl)
	if i == 0 {
		return nil, fmt.Errorf("commit %v lacks double newline", hash)
	}
	hdr, msg := catFile[:i], catFile[i+2:]
	gc := &GitCommit{
		Hash:    hash,
		Parents: make([]*GitCommit, 0, bytes.Count(hdr, parentSpace)),
		Msg:     c.strb(msg),
	}

	// The commit message contains the reviewer email address. Sample commit message:
	// Update patch set 1
	//
	// Patch Set 1: Code-Review+2
	//
	// Patch-set: 1
	// Reviewer: Ian Lance Taylor <5206@62eb7196-b449-3ce5-99f1-c037f21e1705>
	// Label: Code-Review=+2
	if reviewer, _ := lineValue(c.strb(msg), "Reviewer: "); reviewer != "" {
		gc.Reviewer = &GitPerson{Str: reviewer}
	}

	if commit.DiffTree != nil {
		gc.Files = commit.DiffTree.File
	}
	for _, f := range gc.Files {
		f.File = c.str(f.File) // intern the string
	}
	sort.Slice(gc.Files, func(i, j int) bool { return gc.Files[i].File < gc.Files[j].File })
	parents := 0
	err := ForeachLine(hdr, func(ln []byte) error {
		if bytes.HasPrefix(ln, parentSpace) {
			parents++
			parentHash := c.gitHashFromHex(ln[len(parentSpace):])
			parent := c.gitCommit[parentHash]
			if parent == nil {
				// Install a placeholder to be filled in later.
				parent = &GitCommit{
					Hash:      parentHash,
					Committer: placeholderCommitter,
				}
				c.gitCommit[parentHash] = parent
			}
			gc.Parents = append(gc.Parents, parent)
			c.enqueueCommitLocked(parentHash)
			return nil
		}
		if bytes.HasPrefix(ln, authorSpace) {
			p, t, err := c.parsePerson(ln[len(authorSpace):])
			if err != nil {
				return fmt.Errorf("unrecognized author line %q: %v", ln, err)
			}
			gc.Author = p
			gc.AuthorTime = t
			return nil
		}
		if bytes.HasPrefix(ln, committerSpace) {
			p, t, err := c.parsePerson(ln[len(committerSpace):])
			if err != nil {
				return fmt.Errorf("unrecognized committer line %q: %v", ln, err)
			}
			gc.Committer = p
			gc.CommitTime = t
			return nil
		}
		if bytes.HasPrefix(ln, treeSpace) {
			gc.Tree = c.gitHashFromHex(ln[len(treeSpace):])
			return nil
		}
		if bytes.HasPrefix(ln, golangHgSpace) {
			if c.gitOfHg == nil {
				c.gitOfHg = map[string]GitHash{}
			}
			c.gitOfHg[string(ln[len(golangHgSpace):])] = hash
			return nil
		}
		if bytes.HasPrefix(ln, gpgSigSpace) || bytes.HasPrefix(ln, space) {
			// Jessie Frazelle is a unique butterfly.
			return nil
		}
		if bytes.HasPrefix(ln, encodingSpace) {
			// Also ignore this. In practice this has only
			// been seen to declare that a commit's
			// metadata is utf-8 when the author name has
			// non-ASCII.
			return nil
		}
		log.Printf("in commit %s, unrecognized line %q", hash, ln)
		return nil
	})
	if err != nil {
		log.Printf("Unparseable commit %q: %v", hash, err)
		return nil, fmt.Errorf("Unparseable commit %q: %v", hash, err)
	}
	if ph, ok := c.gitCommit[hash]; ok {
		// Update placeholder.
		*ph = *gc
	} else {
		c.gitCommit[hash] = gc
	}
	if c.gitCommitTodo != nil {
		delete(c.gitCommitTodo, hash)
	}
	if c.verbose {
		now := time.Now()
		if now.After(c.lastGitCount.Add(time.Second)) {
			c.lastGitCount = now
			log.Printf("Num git commits = %v", len(c.gitCommit))
		}
	}
	return gc, nil
}

// calls f on each non-empty line in v, without the trailing \n. the
// final line need not include a trailing \n. Returns first non-nil
// error returned by f.
// TODO: this is too generalized to be in the maintner package. Move it out.
func ForeachLine(v []byte, f func([]byte) error) error {
	for len(v) > 0 {
		i := bytes.IndexByte(v, '\n')
		if i < 0 {
			return f(v)
		}
		if err := f(v[:i]); err != nil {
			return err
		}
		v = v[i+1:]
	}
	return nil
}

// string variant of ForeachLine.
// calls f on each non-empty line in s, without the trailing \n. the
// final line need not include a trailing \n. Returns first non-nil
// error returned by f.
// TODO: this is too generalized to be in the maintner package. Move it out.
func ForeachLineStr(s string, f func(string) error) error {
	for len(s) > 0 {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			return f(s)
		}
		if err := f(s[:i]); err != nil {
			return err
		}
		s = s[i+1:]
	}
	return nil
}

// parsePerson parses an "author" or "committer" value from "git cat-file -p COMMIT"
// The values are like:
//    Foo Bar <foobar@gmail.com> 1488624439 +0900
// c.mu must be held for writing.
func (c *Corpus) parsePerson(v []byte) (*GitPerson, time.Time, error) {
	v = bytes.TrimSpace(v)

	lastSpace := bytes.LastIndexByte(v, ' ')
	if lastSpace < 0 {
		return nil, time.Time{}, errors.New("failed to match person")
	}
	tz := v[lastSpace+1:] // "+0800"
	v = v[:lastSpace]     // now v is "Foo Bar <foobar@gmail.com> 1488624439"

	lastSpace = bytes.LastIndexByte(v, ' ')
	if lastSpace < 0 {
		return nil, time.Time{}, errors.New("failed to match person")
	}
	unixTime := v[lastSpace+1:]
	nameEmail := v[:lastSpace] // now v is "Foo Bar <foobar@gmail.com>"

	ut, err := strconv.ParseInt(string(unixTime), 10, 64)
	if err != nil {
		return nil, time.Time{}, err
	}
	t := time.Unix(ut, 0).In(c.gitLocation(tz))

	p, ok := c.gitPeople[string(nameEmail)]
	if !ok {
		p = &GitPerson{Str: string(nameEmail)}
		if c.gitPeople == nil {
			c.gitPeople = map[string]*GitPerson{}
		}
		c.gitPeople[p.Str] = p
	}
	return p, t, nil

}

// GitCommit returns the provided git commit, or nil if it's unknown.
func (c *Corpus) GitCommit(hash string) *GitCommit {
	if len(hash) != 40 {
		// TODO: support prefix lookups. build a trie. But
		// for now just avoid panicking in gitHashFromHexStr.
		return nil
	}
	var buf [20]byte
	_, err := decodeHexStr(buf[:], hash)
	if err != nil {
		return nil
	}
	return c.gitCommit[GitHash(buf[:])]
}

// v is like '[+-]hhmm'
// c.mu must be held for writing.
func (c *Corpus) gitLocation(v []byte) *time.Location {
	if loc, ok := c.zoneCache[string(v)]; ok {
		return loc
	}
	s := string(v)
	h, _ := strconv.Atoi(s[1:3])
	m, _ := strconv.Atoi(s[3:5])
	east := 1
	if v[0] == '-' {
		east = -1
	}
	loc := time.FixedZone(s, east*(h*3600+m*60))
	if c.zoneCache == nil {
		c.zoneCache = map[string]*time.Location{}
	}
	c.zoneCache[s] = loc
	return loc
}

func decodeHexStr(dst []byte, src string) (int, error) {
	if len(src)%2 == 1 {
		return 0, hex.ErrLength
	}

	for i := 0; i < len(src)/2; i++ {
		a, ok := fromHexChar(src[i*2])
		if !ok {
			return 0, hex.InvalidByteError(src[i*2])
		}
		b, ok := fromHexChar(src[i*2+1])
		if !ok {
			return 0, hex.InvalidByteError(src[i*2+1])
		}
		dst[i] = (a << 4) | b
	}

	return len(src) / 2, nil
}

// fromHexChar converts a hex character into its value and a success flag.
func fromHexChar(c byte) (byte, bool) {
	switch {
	case '0' <= c && c <= '9':
		return c - '0', true
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10, true
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10, true
	}

	return 0, false
}
