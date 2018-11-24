// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

func handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	round := func(t time.Duration) time.Duration {
		return t / time.Second * time.Second
	}
	df := diskFree()

	statusMu.Lock()
	data := statusData{
		Total:        len(status),
		Uptime:       round(time.Now().Sub(processStartTime)),
		Recent:       append([]*buildStatus{}, statusDone...),
		DiskFree:     df,
		Version:      Version,
		NumFD:        fdCount(),
		NumGoroutine: runtime.NumGoroutine(),
	}
	for _, st := range status {
		if atomic.LoadInt32(&st.hasBuildlet) != 0 {
			data.ActiveBuilds++
			data.Active = append(data.Active, st)
		} else {
			data.Pending = append(data.Pending, st)
		}
	}
	// TODO: make this prettier.
	var buf bytes.Buffer
	for _, key := range tryList {
		if ts := tries[key]; ts != nil {
			state := ts.state()
			fmt.Fprintf(&buf, "Change-ID: %v Commit: %v (<a href='/try?commit=%v'>status</a>)\n",
				key.ChangeTriple(), key.Commit, key.Commit[:8])
			fmt.Fprintf(&buf, "   Remain: %d, fails: %v\n", state.remain, state.failed)
			for _, bs := range ts.builds {
				fmt.Fprintf(&buf, "  %s: running=%v\n", bs.Name, bs.isRunning())
			}
		}
	}
	statusMu.Unlock()

	data.RemoteBuildlets = template.HTML(remoteBuildletStatus())

	sort.Sort(byAge(data.Active))
	sort.Sort(byAge(data.Pending))
	sort.Sort(sort.Reverse(byAge(data.Recent)))
	if errTryDeps != nil {
		data.TrybotsErr = errTryDeps.Error()
	} else {
		if buf.Len() == 0 {
			data.Trybots = template.HTML("<i>(none)</i>")
		} else {
			data.Trybots = template.HTML("<pre>" + buf.String() + "</pre>")
		}
	}

	buf.Reset()
	gcePool.WriteHTMLStatus(&buf)
	data.GCEPoolStatus = template.HTML(buf.String())
	buf.Reset()

	kubePool.WriteHTMLStatus(&buf)
	data.KubePoolStatus = template.HTML(buf.String())
	buf.Reset()

	reversePool.WriteHTMLStatus(&buf)
	data.ReversePoolStatus = template.HTML(buf.String())

	buf.Reset()
	if err := statusTmpl.Execute(&buf, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	buf.WriteTo(w)
}

func fdCount() int {
	f, err := os.Open("/proc/self/fd")
	if err != nil {
		return -1
	}
	defer f.Close()
	n := 0
	for {
		names, err := f.Readdirnames(1000)
		n += len(names)
		if err == io.EOF {
			return n
		}
		if err != nil {
			return -1
		}
	}
}

func friendlyDuration(d time.Duration) string {
	if d > 10*time.Second {
		d2 := ((d + 50*time.Millisecond) / (100 * time.Millisecond)) * (100 * time.Millisecond)
		return d2.String()
	}
	if d > time.Second {
		d2 := ((d + 5*time.Millisecond) / (10 * time.Millisecond)) * (10 * time.Millisecond)
		return d2.String()
	}
	d2 := ((d + 50*time.Microsecond) / (100 * time.Microsecond)) * (100 * time.Microsecond)
	return d2.String()
}

func diskFree() string {
	out, _ := exec.Command("df", "-h").Output()
	return string(out)
}

// statusData is the data that fills out statusTmpl.
type statusData struct {
	Total             int // number of total builds (including those waiting for a buildlet)
	ActiveBuilds      int // number of running builds (subset of Total with a buildlet)
	NumFD             int
	NumGoroutine      int
	Uptime            time.Duration
	Active            []*buildStatus // have a buildlet
	Pending           []*buildStatus // waiting on a buildlet
	Recent            []*buildStatus
	TrybotsErr        string
	Trybots           template.HTML
	GCEPoolStatus     template.HTML // TODO: embed template
	KubePoolStatus    template.HTML // TODO: embed template
	ReversePoolStatus template.HTML // TODO: embed template
	RemoteBuildlets   template.HTML
	DiskFree          string
	Version           string
}

var statusTmpl = template.Must(template.New("status").Parse(`
<!DOCTYPE html>
<html>
<head><link rel="stylesheet" href="/style.css"/><title>Go Farmer</title></head>
<body>
<header>
	<h1>Go Build Coordinator</h1>
	<nav>
		<a href="https://build.golang.org">Dashboard</a>
		<a href="/builders">Builders</a>
	</nav>
	<div class="clear"></div>
</header>

<h2>Running</h2>
<p>{{printf "%d" .Total}} total builds; {{printf "%d" .ActiveBuilds}} active. Uptime {{printf "%s" .Uptime}}. Version {{.Version}}.

<h2 id=trybots>Active Trybot Runs <a href='#trybots'>¶</a></h2>
{{- if .TrybotsErr}}
<b>trybots disabled:</b>: {{.TrybotsErr}}
{{else}}
{{.Trybots}}
{{end}}

<h2 id=remote>Remote buildlets <a href='#remote'>¶</a></h3>
{{.RemoteBuildlets}}

<h2 id=pools>Buildlet pools <a href='#pools'>¶</a></h2>
<ul>
	<li>{{.GCEPoolStatus}}</li>
	<li>{{.KubePoolStatus}}</li>
	<li>{{.ReversePoolStatus}}</li>
</ul>

<h2 id=active>Active builds <a href='#active'>¶</a></h2>
<ul>
	{{range .Active}}
	<li><pre>{{.HTMLStatusLine}}</pre></li>
	{{end}}
</ul>

<h2 id=pending>Pending builds <a href='#pending'>¶</a></h2>
<ul>
	{{range .Pending}}
	<li><pre>{{.HTMLStatusLine}}</pre></li>
	{{end}}
</ul>

<h2 id=completed>Recently completed <a href='#completed'>¶</a></h2>
<ul>
	{{range .Recent}}
	<li><span>{{.HTMLStatusLine_done}}</span></li>
	{{end}}
</ul>

<h2 id=disk>Disk Space <a href='#disk'>¶</a></h2>
<pre>{{.DiskFree}}</pre>

<h2 id=fd>File Descriptors <a href='#fd'>¶</a></h2>
<p>{{.NumFD}}</p>

<h2 id=goroutines>Goroutines <a href='#goroutines'>¶</a></h2>
<p>{{.NumGoroutine}} <a href='/debug/goroutines'>goroutines</a></p>

</body>
</html>
`))

func handleStyleCSS(w http.ResponseWriter, r *http.Request) {
	src := strings.NewReader(styleCSS)
	http.ServeContent(w, r, "style.css", processStartTime, src)
}

const styleCSS = `
body {
	font-family: sans-serif;
	color: #222;
	padding: 10px;
	margin: 0;
}

h1, h2, h1 > a, h2 > a, h1 > a:visited, h2 > a:visited { 
	color: #375EAB; 
}
h1 { font-size: 24px; }
h2 { font-size: 20px; }

h1 > a, h2 > a {
	display: none;
	text-decoration: none;
}

h1:hover > a, h2:hover > a {
	display: inline;
}

h1 > a:hover, h2 > a:hover {
	text-decoration: underline;
}

pre {
	font-family: monospace;
	font-size: 9pt;
}

header {
	margin: -10px -10px 0 -10px;
	padding: 10px 10px;
	background: #E0EBF5;
}
header a { color: #222; }
header h1 {
	display: inline;
	margin: 0;
	padding-top: 5px;
}
header nav {
	display: inline-block;
	margin-left: 20px;
}
header nav a {
	display: inline-block;
	padding: 10px;
	margin: 0;
	margin-right: 5px;
	color: white;
	background: #375EAB;
	text-decoration: none;
	font-size: 16px;
	border: 1px solid #375EAB;
	border-radius: 5px;
}

table {
	border-collapse: collapse;
	font-size: 9pt;
}

table td, table th, table td, table th {
	text-align: left;
	vertical-align: top;
	padding: 2px 6px;
}

table thead tr {
	background: #fff !important;
}
`
