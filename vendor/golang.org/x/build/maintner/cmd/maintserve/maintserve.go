// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// maintserve is a program that serves Go issues and CLs over HTTP,
// so they can be viewed in a browser. It uses x/build/maintner/godata
// as its backing source of data.
//
// It statically embeds all the resources it uses, so it's possible to use
// it when offline. During that time, the corpus will not be able to update,
// and GitHub user profile pictures won't load.
//
// maintserve displays partial Gerrit CL data that is available within the
// maintner corpus. Code diffs and inline review comments are not included.
package main

import (
	"context"
	"flag"
	"fmt"
	"html/template"
	"log"
	"mime"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"dmitri.shuralyov.com/app/changes"
	maintnerchange "dmitri.shuralyov.com/service/change/maintner"
	"github.com/shurcooL/gofontwoff"
	"github.com/shurcooL/httpgzip"
	"github.com/shurcooL/issues"
	maintnerissues "github.com/shurcooL/issues/maintner"
	"github.com/shurcooL/issuesapp"
	"golang.org/x/build/maintner"
	"golang.org/x/build/maintner/godata"
)

var httpFlag = flag.String("http", ":8080", "Listen for HTTP connections on this address.")

func main() {
	flag.Parse()

	err := run()
	if err != nil {
		log.Fatalln(err)
	}
}

func run() error {
	if err := mime.AddExtensionType(".woff2", "font/woff2"); err != nil {
		return err
	}

	corpus, err := godata.Get(context.Background())
	if err != nil {
		return err
	}

	issuesService := maintnerissues.NewService(corpus)
	issuesApp := issuesapp.New(issuesService, nil, issuesapp.Options{
		HeadPre: `<meta name="viewport" content="width=device-width">
<link href="/assets/fonts/fonts.css" rel="stylesheet" type="text/css">
<link href="/assets/style.css" rel="stylesheet" type="text/css">`,
		HeadPost: `<style type="text/css">
	.markdown-body { font-family: Go; }
	tt, code, pre  { font-family: "Go Mono"; }
</style>`,
		BodyPre: `<div style="max-width: 800px; margin: 0 auto 100px auto;">

{{/* Override new comment component to link to original issue for leaving comments. */}}
{{define "new-comment"}}<div class="event" style="margin-top: 20px; margin-bottom: 100px;">
	View <a href="https://github.com/{{.RepoSpec}}/issues/{{.Issue.ID}}#new_comment_field">original issue</a> to comment.
</div>{{end}}`,
		DisableReactions: true,
	})

	changeService := maintnerchange.NewService(corpus)
	changesApp := changes.New(changeService, nil, changes.Options{
		HeadPre: `<meta name="viewport" content="width=device-width">
<link href="/assets/fonts/fonts.css" rel="stylesheet" type="text/css">
<link href="/assets/style.css" rel="stylesheet" type="text/css">`,
		HeadPost: `<style type="text/css">
	.markdown-body { font-family: Go; }
	tt, code, pre  { font-family: "Go Mono"; }
</style>`,
		BodyPre:          `<div style="max-width: 800px; margin: 0 auto 100px auto;">`,
		DisableReactions: true,
	})

	// TODO: Implement background updates for corpus while the application is running.
	//       Right now, it only updates at startup.
	//       See gido source code for an example of how to do this.

	printServingAt(*httpFlag)
	err = http.ListenAndServe(*httpFlag, &handler{
		c:              corpus,
		fontsHandler:   httpgzip.FileServer(gofontwoff.Assets, httpgzip.FileServerOptions{}),
		issuesHandler:  issuesApp,
		changesHandler: changesApp,
	})
	return err
}

func printServingAt(addr string) {
	hostPort := addr
	if strings.HasPrefix(hostPort, ":") {
		hostPort = "localhost" + hostPort
	}
	fmt.Printf("serving at http://%s/\n", hostPort)
}

// handler handles all requests to maintserve. It acts like a request multiplexer,
// choosing from various endpoints and parsing the repository ID from URL.
type handler struct {
	c              *maintner.Corpus
	fontsHandler   http.Handler
	issuesHandler  http.Handler
	changesHandler http.Handler
}

func (h *handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Handle "/".
	if req.URL.Path == "/" {
		h.serveIndex(w, req)
		return
	}

	// Handle "/assets/fonts/...".
	if strings.HasPrefix(req.URL.Path, "/assets/fonts") {
		req = stripPrefix(req, len("/assets/fonts"))
		h.fontsHandler.ServeHTTP(w, req)
		return
	}

	// Handle "/assets/style.css".
	if req.URL.Path == "/assets/style.css" {
		http.ServeContent(w, req, "style.css", time.Time{}, strings.NewReader(styleCSS))
		return
	}

	elems := strings.SplitN(req.URL.Path[1:], "/", 3)
	if len(elems) < 2 {
		http.Error(w, "404 Not Found", http.StatusNotFound)
		return
	}
	switch strings.HasSuffix(elems[0], ".googlesource.com") {
	case false:
		// Handle "/owner/repo/..." GitHub repository URLs.
		owner, repo := elems[0], elems[1]
		baseURLLen := 1 + len(owner) + 1 + len(repo) // Base URL is "/owner/repo".
		if baseURL := req.URL.Path[:baseURLLen]; req.URL.Path == baseURL+"/" {
			// Redirect "/owner/repo/" to "/owner/repo".
			if req.URL.RawQuery != "" {
				baseURL += "?" + req.URL.RawQuery
			}
			http.Redirect(w, req, baseURL, http.StatusFound)
			return
		}
		req = stripPrefix(req, baseURLLen)
		h.serveIssues(w, req, maintner.GitHubRepoID{Owner: owner, Repo: repo})

	case true:
		// Handle "/server/project/..." Gerrit project URLs.
		server, project := elems[0], elems[1]
		baseURLLen := 1 + len(server) + 1 + len(project) // Base URL is "/server/project".
		if baseURL := req.URL.Path[:baseURLLen]; req.URL.Path == baseURL+"/" {
			// Redirect "/server/project/" to "/server/project".
			if req.URL.RawQuery != "" {
				baseURL += "?" + req.URL.RawQuery
			}
			http.Redirect(w, req, baseURL, http.StatusFound)
			return
		}
		req = stripPrefix(req, baseURLLen)
		h.serveChanges(w, req, server, project)
	}
}

var indexHTML = template.Must(template.New("").Parse(`<html>
	<head>
		<title>maintserve</title>
		<meta name="viewport" content="width=device-width">
		<link href="/assets/fonts/fonts.css" rel="stylesheet" type="text/css">
		<link href="/assets/style.css" rel="stylesheet" type="text/css">
	</head>
	<body>
		<div style="max-width: 800px; margin: 0 auto 100px auto;">
			<h2>maintserve</h2>
			<div style="display: inline-block; width: 50%;">
				<h3>GitHub Repos</h3>
				<ul>{{range .Repos}}
					<li><a href="/{{.RepoID}}">{{.RepoID}}</a> ({{.Count}} issues)</li>
					{{- end}}
				</ul>
			</div><div style="display: inline-block; width: 50%; vertical-align: top;">
				<h3>Gerrit Projects</h3>
				<ul>{{range .Projects}}
					<li><a href="/{{.ServProj}}">{{.ServProj}}</a> ({{.Count}} changes)</li>
					{{- end}}
				</ul>
			</div>
		</div>
	<body>
</html>`))

// serveIndex serves the index page, which lists all available
// GitHub repositories and Gerrit projects.
func (h *handler) serveIndex(w http.ResponseWriter, req *http.Request) {
	// Enumerate all GitHub repositories.
	type repo struct {
		RepoID maintner.GitHubRepoID
		Count  uint64 // Issues count.
	}
	var repos []repo
	err := h.c.GitHub().ForeachRepo(func(r *maintner.GitHubRepo) error {
		issues, err := countIssues(r)
		if err != nil {
			return err
		}
		repos = append(repos, repo{
			RepoID: r.ID(),
			Count:  issues,
		})
		return nil
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sort.Slice(repos, func(i, j int) bool {
		return repos[i].RepoID.String() < repos[j].RepoID.String()
	})

	// Enumerate all Gerrit projects.
	type project struct {
		ServProj string
		Count    uint64 // Changes count.
	}
	var projects []project
	err = h.c.Gerrit().ForeachProjectUnsorted(func(r *maintner.GerritProject) error {
		changes, err := countChanges(r)
		if err != nil {
			return err
		}
		projects = append(projects, project{
			ServProj: r.ServerSlashProject(),
			Count:    changes,
		})
		return nil
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].ServProj < projects[j].ServProj
	})

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = indexHTML.Execute(w, map[string]interface{}{
		"Repos":    repos,
		"Projects": projects,
	})
	if err != nil {
		log.Println(err)
	}
}

// countIssues reports the number of issues in a GitHubRepo r.
func countIssues(r *maintner.GitHubRepo) (uint64, error) {
	var issues uint64
	err := r.ForeachIssue(func(i *maintner.GitHubIssue) error {
		if i.NotExist || i.PullRequest {
			return nil
		}
		issues++
		return nil
	})
	return issues, err
}

// countChanges reports the number of changes in a GerritProject p.
func countChanges(p *maintner.GerritProject) (uint64, error) {
	var changes uint64
	err := p.ForeachCLUnsorted(func(cl *maintner.GerritCL) error {
		if cl.Private {
			return nil
		}
		changes++
		return nil
	})
	return changes, err
}

// serveIssues serves issues for GitHub repository id.
func (h *handler) serveIssues(w http.ResponseWriter, req *http.Request, id maintner.GitHubRepoID) {
	if h.c.GitHub().Repo(id.Owner, id.Repo) == nil {
		http.Error(w, fmt.Sprintf("404 Not Found\n\nGitHub repository %q not found", id), http.StatusNotFound)
		return
	}

	req = req.WithContext(context.WithValue(req.Context(),
		issuesapp.RepoSpecContextKey, issues.RepoSpec{URI: fmt.Sprintf("%s/%s", id.Owner, id.Repo)}))
	req = req.WithContext(context.WithValue(req.Context(),
		issuesapp.BaseURIContextKey, fmt.Sprintf("/%s/%s", id.Owner, id.Repo)))
	h.issuesHandler.ServeHTTP(w, req)
}

// serveChanges serves changes for Gerrit project server/project.
func (h *handler) serveChanges(w http.ResponseWriter, req *http.Request, server, project string) {
	if h.c.Gerrit().Project(server, project) == nil {
		http.Error(w, fmt.Sprintf("404 Not Found\n\nGerrit project %s/%s not found", server, project), http.StatusNotFound)
		return
	}

	req = req.WithContext(context.WithValue(req.Context(),
		changes.RepoSpecContextKey, fmt.Sprintf("%s/%s", server, project)))
	req = req.WithContext(context.WithValue(req.Context(),
		changes.BaseURIContextKey, fmt.Sprintf("/%s/%s", server, project)))
	h.changesHandler.ServeHTTP(w, req)
}

// stripPrefix returns request r with prefix of length prefixLen stripped from r.URL.Path.
// prefixLen must not be longer than len(r.URL.Path), otherwise stripPrefix panics.
// If r.URL.Path is empty after the prefix is stripped, the path is changed to "/".
func stripPrefix(r *http.Request, prefixLen int) *http.Request {
	r2 := new(http.Request)
	*r2 = *r
	r2.URL = new(url.URL)
	*r2.URL = *r.URL
	r2.URL.Path = r.URL.Path[prefixLen:]
	if r2.URL.Path == "" {
		r2.URL.Path = "/"
	}
	return r2
}

const styleCSS = `body {
	margin: 20px;
	font-family: Go;
	font-size: 14px;
	line-height: initial;
	color: #373a3c;
}
a {
	color: #0275d8;
	text-decoration: none;
}
a:focus, a:hover {
	color: #014c8c;
	text-decoration: underline;
}
.btn {
	font-family: inherit;
	font-size: 11px;
	line-height: 11px;
	height: 18px;
	border-radius: 4px;
	border: solid #d2d2d2 1px;
	background-color: #fff;
	box-shadow: 0 1px 1px rgba(0, 0, 0, .05);
}`
