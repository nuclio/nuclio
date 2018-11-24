// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Releasebot manages the process of defining, packaging, and publishing Go
// releases. It is a work in progress; right now it only handles beta, rc and
// minor (point) releases, but eventually we want it to handle major releases too.
package main

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/build/buildenv"
	"golang.org/x/build/maintner"
)

var releaseTargets = []string{
	"src",
	"linux-386",
	"linux-armv6l",
	"linux-amd64",
	"linux-arm64",
	"freebsd-386",
	"freebsd-amd64",
	"windows-386",
	"windows-amd64",
	"darwin-amd64",
	"linux-s390x",
	"linux-ppc64le",
}

var releaseModes = map[string]bool{
	"prepare": true,
	"release": true,
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: releasebot -mode <release mode> [-dry-run] {go1.8.5|go1.10beta2|go1.11rc1}")
	fmt.Fprintln(os.Stderr, "Release modes:")
	fmt.Fprintln(os.Stderr)
	for m := range releaseModes {
		fmt.Fprintln(os.Stderr, m)
	}
	os.Exit(2)
}

var dryRun bool // only perform pre-flight checks, only log to terminal

func main() {
	modeFlag := flag.String("mode", "", "release mode (prepare, release)")
	flag.BoolVar(&dryRun, "dry-run", false, "only perform pre-flight checks, only log to terminal")
	flag.Usage = usage
	flag.Parse()
	if *modeFlag == "" || !releaseModes[*modeFlag] || flag.NArg() == 0 {
		usage()
	}

	http.DefaultTransport = newLogger(http.DefaultTransport)

	buildenv.CheckUserCredentials()
	checkForGitCodereview()
	loadMaintner()
	loadGomoteUser()
	loadGithubAuth()
	loadGCSAuth()

	var wg sync.WaitGroup
	for _, release := range flag.Args() {
		if strings.Contains(release, "beta") || strings.Contains(release, "rc") {
			w := &Work{
				Prepare:     *modeFlag == "prepare",
				Version:     release,
				BetaRelease: strings.Contains(release, "beta"),
				RCRelease:   strings.Contains(release, "rc"),
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				w.doRelease()
			}()
			continue
		}

		errFoundMilestone := errors.New("found milestone")
		err := goRepo.ForeachMilestone(func(m *maintner.GitHubMilestone) error {
			if strings.ToLower(m.Title) == release {
				nextM, err := nextMilestone(m)
				if err != nil {
					return err
				}
				w := &Work{
					Milestone:     m,
					NextMilestone: nextM,
					Prepare:       *modeFlag == "prepare",
					Version:       release,
				}
				wg.Add(1)
				go func() {
					defer wg.Done()
					w.doRelease()
				}()
				return errFoundMilestone
			}
			return nil
		})
		if err != nil && err != errFoundMilestone {
			log.Printf("error looking for release %s: %v", release, err)
		}
		if err == nil {
			log.Printf("cannot find release %s", release)
		}
	}
	wg.Wait()
}

func nextMilestone(m *maintner.GitHubMilestone) (*maintner.GitHubMilestone, error) {
	titleParts := strings.Split(m.Title, ".")
	n, err := strconv.Atoi(titleParts[len(titleParts)-1])
	if err != nil {
		return nil, err
	}
	titleParts[len(titleParts)-1] = strconv.Itoa(n + 1)
	newTitle := strings.Join(titleParts, ".")
	var res *maintner.GitHubMilestone
	err = goRepo.ForeachMilestone(func(m *maintner.GitHubMilestone) error {
		if m.Title == newTitle {
			res = m
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return res, fmt.Errorf("no next milestone found with title %q", newTitle)
	}
	return res, nil
}

// checkForGitCodereview exits the program if git-codereview is not installed
// in the user's path.
func checkForGitCodereview() {
	cmd := exec.Command("which", "git-codereview")
	if err := cmd.Run(); err != nil {
		log.Fatal("could not find git-codereivew: ", cmd.Args, ": ", err, "\n\n"+
			"Please install it via go get golang.org/x/review/git-codereview\n"+
			"to use this program.")
	}
}

var gomoteUser string

func loadGomoteUser() {
	tokenPath := filepath.Join(os.Getenv("HOME"), ".config/gomote")
	files, _ := ioutil.ReadDir(tokenPath)
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		if strings.HasSuffix(name, ".token") && strings.HasPrefix(name, "user-") {
			gomoteUser = strings.TrimPrefix(strings.TrimSuffix(name, ".token"), "user-")
			return
		}
	}
	log.Fatal("missing gomote token - cannot build releases.\n**FIX**: Download https://build-dot-golang-org.appspot.com/key?builder=user-YOURNAME\nand store in ~/.config/gomote/user-YOURNAME.token")
}

// Work collects all the work state for managing a particular release.
// The intent is that the code could be used in a setting where one program
// is managing multiple releases, although the current releasebot command line
// only accepts a single release.
type Work struct {
	logBuf *bytes.Buffer
	log    *log.Logger

	Prepare     bool // create the release commit and submit it for review
	BetaRelease bool
	RCRelease   bool

	ReleaseIssue  int    // Release status issue number
	ReleaseBranch string // "master" for beta releases
	Dir           string // work directory ($HOME/go-releasebot-work/<release>)
	Errors        []string
	ReleaseBinary string
	Version       string
	VersionCommit string

	releaseMu   sync.Mutex
	ReleaseInfo map[string]*ReleaseInfo // map and info protected by releaseMu

	// Properties set for minor releases only.
	Milestone     *maintner.GitHubMilestone
	NextMilestone *maintner.GitHubMilestone // Next minor milestone
}

// ReleaseInfo describes a release build for a specific target.
type ReleaseInfo struct {
	Outputs []*ReleaseOutput
	Msg     string
}

// ReleaseOutput describes a single release file.
type ReleaseOutput struct {
	File   string
	Suffix string
	Link   string
	Error  string
}

// logError records an error.
// The error is always shown in the "PROBLEMS WITH RELEASE"
// section at the top of the status page.
// If cl is not nil, the error is also shown in that CL's summary.
func (w *Work) logError(msg string, a ...interface{}) {
	w.Errors = append(w.Errors, fmt.Sprintf(msg, a...))
}

// finally should be deferred at the top of each goroutine using a Work
// (as in "defer w.finally()"). It catches and logs panics and posts
// the log.
func (w *Work) finally() {
	if err := recover(); err != nil {
		w.log.Printf("\n\nPANIC: %v\n\n%s", err, debug.Stack())
	}
	w.postSummary()
}

type runner struct {
	w        *Work
	dir      string
	extraEnv []string
}

func (w *Work) runner(dir string, env ...string) *runner {
	return &runner{
		w:        w,
		dir:      dir,
		extraEnv: env,
	}
}

// run runs the command and requires that it succeeds.
// If not, it logs the failure and aborts the work.
// It logs the command line.
func (r *runner) run(args ...string) {
	out, err := r.runErr(args...)
	if err != nil {
		r.w.log.Printf("command failed: %s\n%s", err, out)
		panic("command failed")
	}
}

// runOut runs the command, requires that it succeeds,
// and returns the command's output.
// It does not log the command line except in case of failure.
// Not logging these commands avoids filling the log with
// runs of side-effect-free commands like "git cat-file commit HEAD".
func (r *runner) runOut(args ...string) []byte {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = r.dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.w.log.Printf("$ %s\n", strings.Join(args, " "))
		r.w.log.Printf("command failed: %s\n%s", err, out)
		panic("command failed")
	}
	return out
}

// runErr runs the given command and returns the output and status (error).
// It logs the command line.
func (r *runner) runErr(args ...string) ([]byte, error) {
	r.w.log.Printf("$ %s\n", strings.Join(args, " "))
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = r.dir
	if len(r.extraEnv) > 0 {
		cmd.Env = append(os.Environ(), r.extraEnv...)
	}
	return cmd.CombinedOutput()
}

func (w *Work) doRelease() {
	w.logBuf = new(bytes.Buffer)
	w.log = log.New(io.MultiWriter(os.Stdout, w.logBuf), "", log.LstdFlags)
	defer w.finally()

	w.log.Printf("starting")

	if w.BetaRelease {
		w.ReleaseBranch = "master"
	} else if w.RCRelease {
		shortRel := strings.Split(w.Version, "rc")[0]
		w.ReleaseBranch = "release-branch." + shortRel
	} else {
		shortRel := w.Version[:strings.LastIndex(w.Version, ".")]
		w.ReleaseBranch = "release-branch." + shortRel
	}

	w.checkSpelling()
	w.gitCheckout()
	// In release mode we carry on even if the tag exists, in case we
	// need to resume a failed build.
	if w.Prepare && w.gitTagExists() {
		w.logError("%s tag already exists in Go repository!", w.Version)
		w.logError("**Found errors during release. Stopping!**")
		return
	}
	if w.BetaRelease || w.RCRelease {
		// TODO: go tool api -allow_new=false
	} else {
		w.checkReleaseBlockers()
		w.checkDocs()
	}
	w.findOrCreateReleaseIssue()
	if len(w.Errors) > 0 && !dryRun {
		w.logError("**Found errors during release. Stopping!**")
		return
	}

	if w.Prepare {
		if w.BetaRelease {
			w.nextStepsBeta()
		} else {
			changeID := w.writeVersion()
			w.nextStepsPrepare(changeID)
		}
	} else {
		// TODO: check build.golang.org
		if !w.BetaRelease {
			w.checkVersion()
		}
		if len(w.Errors) > 0 {
			w.logError("**Found errors during release. Stopping!**")
			return
		}
		w.gitTagVersion()
		w.buildReleases()
		if !w.BetaRelease && !w.RCRelease {
			w.pushIssues()
			w.closeMilestone()
		}
		w.nextStepsRelease()
	}
}

func (w *Work) checkSpelling() {
	if w.Version != strings.ToLower(w.Version) {
		w.logError("release name should be lowercase: %q", w.Version)
	}
	if strings.Contains(w.Version, " ") {
		w.logError("release name should not contain any spaces: %q", w.Version)
	}
	if !strings.HasPrefix(w.Version, "go") {
		w.logError("release name should have 'go' prefix: %q", w.Version)
	}
}

func (w *Work) checkReleaseBlockers() {
	if err := goRepo.ForeachIssue(func(gi *maintner.GitHubIssue) error {
		if gi.Milestone == nil || gi.Milestone.Title != w.Milestone.Title {
			return nil
		}
		if !gi.Closed && gi.HasLabel("release-blocker") {
			w.logError("open issue #%d is tagged release-blocker", gi.Number)
		}
		return nil
	}); err != nil {
		w.logError("error checking release-blockers: %v", err.Error())
		return
	}
}

func (w *Work) nextStepsPrepare(changeID string) {
	w.log.Printf(`

The release is ready.

Please review and submit https://go-review.googlesource.com/q/%s
after making sure its TryBot pass and then run the release stage.

`, changeID)
}

func (w *Work) nextStepsBeta() {
	w.log.Printf(`

The release is ready. Run with mode=release to execute it.

`)
}

func (w *Work) nextStepsRelease() {
	w.log.Printf(`

The release run is complete! Refer to the playbook for the next steps.

Thanks for riding with releasebot today.

`)
}

func (w *Work) postSummary() {
	var md bytes.Buffer

	if len(w.Errors) > 0 {
		fmt.Fprintf(&md, "## PROBLEMS WITH RELEASE\n\n")
		for _, e := range w.Errors {
			fmt.Fprintf(&md, "  - ")
			fmt.Fprintf(&md, "%s\n", strings.Replace(strings.TrimRight(e, "\n"), "\n", "\n    ", -1))
		}
	}

	if !w.Prepare {
		fmt.Fprintf(&md, "\n## Latest build: %s\n\n", mdEscape(w.Version))
		w.printReleaseTable(&md)
	}

	fmt.Fprintf(&md, "\n## Log\n\n    ")
	md.WriteString(strings.Replace(w.logBuf.String(), "\n", "\n    ", -1))
	fmt.Fprintf(&md, "\n\n")

	body := md.String()
	fmt.Printf("%s", body)
	if dryRun {
		return
	}
	err := postGithubComment(w.ReleaseIssue, body)
	if err != nil {
		fmt.Printf("error posting update comment: %v\n", err)
	}
}

func (w *Work) printReleaseTable(md *bytes.Buffer) {
	// TODO: print sha256
	w.releaseMu.Lock()
	defer w.releaseMu.Unlock()
	for _, target := range releaseTargets {
		fmt.Fprintf(md, "%s", mdEscape(target))
		info := w.ReleaseInfo[target]
		if info == nil {
			fmt.Fprintf(md, " not started\n")
			continue
		}
		for _, out := range info.Outputs {
			if out.Link == "" {
				fmt.Fprintf(md, " (~~%s~~)", mdEscape(out.Suffix))
			} else {
				fmt.Fprintf(md, " ([%s](%s))", mdEscape(out.Suffix), out.Link)
			}
		}
		if len(info.Outputs) == 0 {
			fmt.Fprintf(md, " not built")
		}
		fmt.Fprintf(md, "\n")
		if info.Msg != "" {
			fmt.Fprintf(md, "  - %s\n", strings.Replace(strings.TrimRight(info.Msg, "\n"), "\n", "\n    ", -1))
		}
	}
}

func (w *Work) checkDocs() {
	// Check that we've documented the release.
	data, err := ioutil.ReadFile(filepath.Join(w.Dir, "gitwork", "doc/devel/release.html"))
	if err != nil {
		w.log.Panic(err)
	}
	if !strings.Contains(string(data), "\n<p>\n"+w.Version+" (released ") {
		w.logError("doc/devel/release.html does not document " + w.Version)
	}
}

func (w *Work) writeVersion() (changeID string) {
	changeID = fmt.Sprintf("I%x", sha1.Sum([]byte(fmt.Sprintf("cmd/release-version-%s", w.Version))))

	err := ioutil.WriteFile(filepath.Join(w.Dir, "gitwork", "VERSION"), []byte(w.Version), 0666)
	if err != nil {
		w.log.Panic(err)
	}

	desc := w.Version + "\n\n"
	desc += "Change-Id: " + changeID + "\n"

	r := w.runner(filepath.Join(w.Dir, "gitwork"))
	r.run("git", "add", "VERSION")
	r.run("git", "commit", "-m", desc, "VERSION")
	if dryRun {
		fmt.Printf("\n### VERSION commit\n\n%s\n", r.runOut("git", "show", "HEAD"))
	} else {
		r.run("git", "codereview", "mail", "-trybot", "HEAD")
	}
	return
}

// checkVersion makes sure that the version commit has been submitted.
func (w *Work) checkVersion() {
	ver, err := ioutil.ReadFile(filepath.Join(w.Dir, "gitwork", "VERSION"))
	if err != nil {
		w.log.Panic(err)
	}
	if string(ver) != w.Version {
		w.logError("VERSION is %q; want %q. Did you run prepare and submit the CL?", string(ver), w.Version)
	}
}

func (w *Work) buildReleaseBinary() {
	gopath := filepath.Join(w.Dir, "gopath")
	if err := os.RemoveAll(gopath); err != nil {
		w.log.Panic(err)
	}
	if err := os.MkdirAll(gopath, 0777); err != nil {
		w.log.Panic(err)
	}
	r := w.runner(w.Dir, "GOPATH="+gopath, "GOBIN="+filepath.Join(gopath, "bin"))
	r.run("go", "get", "golang.org/x/build/cmd/release")
	w.ReleaseBinary = filepath.Join(gopath, "bin/release")
}

func (w *Work) buildReleases() {
	w.buildReleaseBinary()
	if err := os.MkdirAll(filepath.Join(w.Dir, "release"), 0777); err != nil {
		w.log.Panic(err)
	}
	w.ReleaseInfo = make(map[string]*ReleaseInfo)

	var wg sync.WaitGroup
	for _, target := range releaseTargets {
		func() {
			w.releaseMu.Lock()
			defer w.releaseMu.Unlock()
			w.ReleaseInfo[target] = new(ReleaseInfo)
		}()

		wg.Add(1)
		target := target
		go func() {
			defer wg.Done()
			defer func() {
				if err := recover(); err != nil {
					stk := strings.TrimSpace(string(debug.Stack()))
					msg := fmt.Sprintf("PANIC: %v\n\n    %s\n", mdEscape(fmt.Sprint(err)), strings.Replace(stk, "\n", "\n    ", -1))
					w.logError(msg)
					w.log.Printf("\n\nBuilding %s: PANIC: %v\n\n%s", target, err, debug.Stack())
					w.releaseMu.Lock()
					w.ReleaseInfo[target].Msg = msg
					w.releaseMu.Unlock()
				}
			}()
			w.buildRelease(target)
		}()
	}
	wg.Wait()

	// Check for release errors and stop if any.
	w.releaseMu.Lock()
	for _, target := range releaseTargets {
		for _, out := range w.ReleaseInfo[target].Outputs {
			if out.Error != "" || len(w.Errors) > 0 {
				w.logError("RELEASE BUILD FAILED\n")
				w.releaseMu.Unlock()
				return
			}
		}
	}
	w.releaseMu.Unlock()
}

// buildRelease builds the release packaging for a given target.
// Because the "release" program can be flaky, it tries up to five times.
// The release files are written to the current release directory
// ($HOME/go-releasebot-work/go1.2.3/release).
// If files for the current version are already present in that
// directory, they are reused instead of being rebuilt.
// buildRelease then uploads the release packaging to the
// gs://golang-release-staging bucket, along with
// files containing the SHA256 hash of the releases,
// for eventual use by the download page.
func (w *Work) buildRelease(target string) {
	log.Printf("BUILDRELEASE %s %s\n", w.Version, target)
	defer log.Printf("DONE BUILDRELEASE %s\n", target)
	prefix := fmt.Sprintf("%s.%s.", w.Version, target)
	var files []string
	switch {
	case strings.HasPrefix(target, "windows-"):
		files = []string{prefix + "zip", prefix + "msi"}
	case strings.HasPrefix(target, "darwin-"):
		files = []string{prefix + "tar.gz", prefix + "pkg"}
	default:
		files = []string{prefix + "tar.gz"}
	}
	var outs []*ReleaseOutput
	haveFiles := true
	for _, file := range files {
		out := &ReleaseOutput{
			File:   file,
			Suffix: strings.TrimPrefix(file, prefix),
		}
		outs = append(outs, out)
		_, err := os.Stat(filepath.Join(w.Dir, "release", file))
		if err != nil {
			haveFiles = false
		}
	}
	w.releaseMu.Lock()
	w.ReleaseInfo[target].Outputs = outs
	w.releaseMu.Unlock()

	if haveFiles {
		w.log.Printf("release %s: already have %v; not rebuilding files", target, files)
	} else {
		failures := 0
		for {
			out, err := w.runner(filepath.Join(w.Dir, "release"), "GOPATH="+filepath.Join(w.Dir, "gopath")).runErr(
				w.ReleaseBinary, "-target", target, "-user", gomoteUser, "-version", w.Version, "-rev", w.VersionCommit,
				"-tools", w.ReleaseBranch, "-net", w.ReleaseBranch)
			// Exit code from release binary is apparently unreliable.
			// Look to see if the files we expected were created instead.
			failed := false
			w.releaseMu.Lock()
			for _, out := range outs {
				if _, err := os.Stat(filepath.Join(w.Dir, "release", out.File)); err != nil {
					failed = true
				}
			}
			w.releaseMu.Unlock()
			if !failed {
				break
			}
			w.log.Printf("release %s: %s\n%s", target, err, out)
			if failures++; failures >= 3 {
				w.log.Printf("release %s: too many failures\n", target)
				for _, out := range outs {
					w.releaseMu.Lock()
					out.Error = fmt.Sprintf("release %s: build failed", target)
					w.releaseMu.Unlock()
				}
				return
			}
			time.Sleep(1 * time.Minute)
		}
	}

	if dryRun {
		return
	}

	for _, out := range outs {
		if err := w.uploadStagingRelease(target, out); err != nil {
			w.log.Printf("release %s: %s", target, err)
			w.releaseMu.Lock()
			out.Error = err.Error()
			w.releaseMu.Unlock()
		}
	}
}

// uploadStagingRelease uploads target to the release staging bucket.
// If successful, it records the corresponding URL in out.Link.
// In addition to uploading target, it creates and uploads a file
// named "<target>.sha256" containing the hex sha256 hash
// of the target file. This is needed for the release signing process
// and also displayed on the eventual download page.
func (w *Work) uploadStagingRelease(target string, out *ReleaseOutput) error {
	if dryRun {
		return errors.New("attempted write operation in dry-run mode")
	}

	src := filepath.Join(w.Dir, "release", out.File)
	h := sha256.New()
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	_, err = io.Copy(h, f)
	f.Close()
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(src+".sha256", []byte(fmt.Sprintf("%x", h.Sum(nil))), 0666); err != nil {
		return err
	}

	dst := w.Version + "/" + out.File
	if err := gcsUpload(src, dst); err != nil {
		return err
	}
	if err := gcsUpload(src+".sha256", dst+".sha256"); err != nil {
		return err
	}

	w.releaseMu.Lock()
	out.Link = "https://" + releaseBucket + ".storage.googleapis.com/" + dst
	w.releaseMu.Unlock()
	return nil
}
