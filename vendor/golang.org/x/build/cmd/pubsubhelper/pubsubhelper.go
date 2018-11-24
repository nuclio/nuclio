// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// The pubsubhelper is an SMTP server for Gerrit updates and an HTTP
// server for Github webhook updates. It then lets other clients subscribe
// to those changes.
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/textproto"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bradfitz/go-smtpd/smtpd"
	"github.com/jellevandenhooff/dkim"
	"go4.org/types"
	"golang.org/x/build/cmd/pubsubhelper/pubsubtypes"
	"golang.org/x/crypto/acme/autocert"
)

var (
	botEmail   = flag.String("rcpt", "\x67\x6f\x70\x68\x65\x72\x62\x6f\x74@pubsubhelper.golang.org", "email address of bot. incoming emails must be to this address.")
	httpListen = flag.String("http", ":80", "HTTP listen address")
	acmeDomain = flag.String("autocert", "pubsubhelper.golang.org", "If non-empty, listen on port 443 and serve HTTPS with a LetsEncrypt cert for this domain.")
	smtpListen = flag.String("smtp", ":25", "SMTP listen address")
)

func main() {
	flag.Parse()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	go func() {
		sig := <-ch
		log.Printf("Signal %v received; exiting with status 0.", sig)
		os.Exit(0)
	}()

	http.HandleFunc("/", handleRoot)
	http.HandleFunc("/waitevent", handleWaitEvent)
	http.HandleFunc("/recent", handleRecent)
	http.HandleFunc("/github-webhook", handleGithubWebhook)

	errc := make(chan error)
	go func() {
		log.Printf("running pubsubhelper on %s", *smtpListen)
		s := &smtpd.Server{
			Addr:            *smtpListen,
			OnNewMail:       onNewMail,
			OnNewConnection: onNewConnection,
			ReadTimeout:     time.Minute,
		}
		err := s.ListenAndServe()
		errc <- fmt.Errorf("SMTP ListenAndServe: %v", err)
	}()
	m := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(*acmeDomain),
	}
	go func() {
		if *acmeDomain == "" {
			return
		}
		if _, err := os.Stat("/autocert-cache"); err == nil {
			m.Cache = autocert.DirCache("/autocert-cache")
		} else {
			log.Printf("Warning: running acme/autocert without cache")
		}
		log.Printf("running pubsubhelper HTTPS on :443 for %s", *acmeDomain)
		s := &http.Server{
			Addr:              ":https",
			TLSConfig:         &tls.Config{GetCertificate: m.GetCertificate},
			ReadHeaderTimeout: 10 * time.Second,
			WriteTimeout:      5 * time.Minute,
			IdleTimeout:       5 * time.Minute,
		}
		err := s.ListenAndServeTLS("", "")
		errc <- fmt.Errorf("HTTPS ListenAndServeTLS: %v", err)
	}()
	go func() {
		log.Printf("running pubsubhelper HTTP on %s", *httpListen)
		s := &http.Server{
			Addr:              *httpListen,
			ReadHeaderTimeout: 10 * time.Second,
			WriteTimeout:      5 * time.Minute,
			IdleTimeout:       5 * time.Minute,
			Handler:           m.HTTPHandler(http.DefaultServeMux),
		}
		err := s.ListenAndServe()
		errc <- fmt.Errorf("HTTP ListenAndServe: %v", err)
	}()

	log.Fatal(<-errc)
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	io.WriteString(w, `<html>
<body>
  This is <a href="https://godoc.org/golang.org/x/build/cmd/pubsubhelper">pubsubhelper</a>.

<ul>
   <li><b><a href="/waitevent">/waitevent</a></b>: long-poll wait 30s for next event (use ?after=[RFC3339Nano] to resume at point)</li>
   <li><b><a href="/recent">/recent</a></b>: recent events, without long-polling.</li>
</ul>

</body>
</html>
`)
}

func handleWaitEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "requires GET", http.StatusBadRequest)
		return
	}

	ch := make(chan *eventAndJSON, 1)

	var after time.Time
	if v := r.FormValue("after"); v != "" {
		var err error
		after, err = time.Parse(time.RFC3339Nano, v)
		if err != nil {
			http.Error(w, "'after' parameter is not in time.RFC3339Nano format", http.StatusBadRequest)
			return
		}
	} else {
		after = time.Now()
	}

	register(ch, after)
	defer unregister(ch)
	ctx := r.Context()

	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()

	var e *eventAndJSON
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
		e = newEventAndJSON(&pubsubtypes.Event{
			LongPollTimeout: true,
		})
	case e = <-ch:
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	io.WriteString(w, e.json)
}

func handleRecent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	var after time.Time
	if v := r.FormValue("after"); v != "" {
		var err error
		after, err = time.Parse(time.RFC3339Nano, v)
		if err != nil {
			http.Error(w, "'after' parameter is not in time.RFC3339Nano format", http.StatusBadRequest)
			return
		}
	}

	var buf bytes.Buffer
	mu.Lock()
	buf.WriteString("[\n")
	n := 0
	for i := len(recent) - 1; i >= 0; i-- {
		ev := recent[i]
		if ev.Time.Time().Before(after) {
			continue
		}
		if n > 0 {
			buf.WriteString(",\n")
		}
		n++
		buf.WriteString(ev.json)
	}
	buf.WriteString("\n]\n")
	mu.Unlock()
	w.Write(buf.Bytes())
}

type env struct {
	from    smtpd.MailAddress
	body    bytes.Buffer
	conn    smtpd.Connection
	tooBig  bool
	hasRcpt bool
}

func (e *env) BeginData() error {
	if !e.hasRcpt {
		return smtpd.SMTPError("554 5.5.1 Error: no valid recipients")
	}
	return nil
}

func (e *env) AddRecipient(rcpt smtpd.MailAddress) error {
	if e.hasRcpt {
		return smtpd.SMTPError("554 5.5.1 Error: dup recipients")
	}
	to := rcpt.Email()
	if to != *botEmail {
		return errors.New("bogus recipient")
	}
	e.hasRcpt = true
	return nil
}

func (e *env) Write(line []byte) error {
	const maxSize = 5 << 20
	if e.body.Len() > maxSize {
		e.tooBig = true
		return nil
	}
	e.body.Write(line)
	return nil
}

var (
	headerSep           = []byte("\r\n\r\n")
	dkimSignatureHeader = []byte("\nDKIM-Signature:")
)

func (e *env) Close() error {
	if e.tooBig {
		log.Printf("Ignoring too-large email from %q", e.from)
		return nil
	}
	from := e.from.Email()
	bodyBytes := e.body.Bytes()
	if !bytes.Contains(bodyBytes, dkimSignatureHeader) {
		log.Printf("Ignoring unsigned (~spam) email from %q", from)
		return nil
	}

	headerBytes := bodyBytes
	if i := bytes.Index(headerBytes, headerSep); i == -1 {
		log.Printf("Ignoring email without header separator from %q", from)
		return nil
	} else {
		headerBytes = headerBytes[:i+len(headerSep)]
	}

	ve, err := dkim.ParseAndVerify(string(headerBytes), dkim.HeadersOnly, dnsClient{})
	if err != nil {
		log.Printf("Email from %q didn't pass DKIM verifications: %v", from, err)
		return nil
	}
	if !strings.HasSuffix(ve.Signature.Domain, "google.com") {
		log.Printf("Ignoring DKIM-verified Gerrit email from non-Google domain %q", ve.Signature.Domain)
		return nil
	}
	tp := textproto.NewReader(bufio.NewReader(bytes.NewReader(headerBytes)))
	hdr, err := tp.ReadMIMEHeader()
	if err != nil {
		log.Printf("Ignoring ReadMIMEHeader error: %v    from email:\n%s", err, headerBytes)
		return nil
	}
	if e.from.Hostname() != "gerritcodereview.bounces.google.com" {
		log.Printf("Ignoring signed, DKIM-verified, non-Gerrit email from %q:\n%s", from, bodyBytes)
		return nil
	}
	changeNum, _ := strconv.Atoi(hdr.Get("X-Gerrit-Change-Number"))

	// Extract gerrit project "oauth2" from List-Id header like:
	// List-Id: <gerrit-oauth2.go-review.googlesource.com>
	project := strings.TrimPrefix(hdr.Get("List-Id"), "<gerrit-")
	if i := strings.IndexByte(project, '.'); i == -1 {
		project = ""
	} else {
		project = project[:i]
	}
	publish(&pubsubtypes.Event{
		Gerrit: &pubsubtypes.GerritEvent{
			URL:          strings.Trim(hdr.Get("X-Gerrit-ChangeURL"), "<>"),
			Project:      project,
			CommitHash:   hdr.Get("X-Gerrit-Commit"),
			ChangeNumber: changeNum,
		},
	})
	return nil
}

type eventAndJSON struct {
	*pubsubtypes.Event
	json string // JSON MarshalIndent of Event
}

var (
	mu      sync.Mutex      // guards following
	recent  []*eventAndJSON // newest at end
	waiting = map[chan *eventAndJSON]struct{}{}
)

const (
	keepMin = 50
	maxAge  = 1 * time.Hour
)

func register(ch chan *eventAndJSON, after time.Time) {
	mu.Lock()
	defer mu.Unlock()
	for _, e := range recent {
		if e.Time.Time().After(after) {
			ch <- e
			return
		}
	}
	waiting[ch] = struct{}{}
}

func unregister(ch chan *eventAndJSON) {
	mu.Lock()
	defer mu.Unlock()
	delete(waiting, ch)
}

// numOldInRecentLocked returns how many leading items of recent are
// too old.
func numOldInRecentLocked() int {
	if len(recent) <= keepMin {
		return 0
	}
	n := 0
	tooOld := time.Now().Add(-maxAge)
	for _, e := range recent {
		if e.Time.Time().After(tooOld) {
			break
		}
		n++
	}
	return n
}

func newEventAndJSON(e *pubsubtypes.Event) *eventAndJSON {
	e.Time = types.Time3339(time.Now())
	j, err := json.MarshalIndent(e, "", "\t")
	if err != nil {
		log.Printf("JSON error: %v", err)
	}
	return &eventAndJSON{
		Event: e,
		json:  string(j),
	}
}

func publish(e *pubsubtypes.Event) {
	ej := newEventAndJSON(e)
	log.Printf("Event: %s", ej.json)

	mu.Lock()
	defer mu.Unlock()

	recent = append(recent, ej)
	// Trim old ones off the front of recent
	if n := numOldInRecentLocked(); n > 0 {
		copy(recent, recent[n:])
		recent = recent[:len(recent)-n]
	}

	for ch := range waiting {
		ch <- ej
		delete(waiting, ch)
	}
}

type dnsClient struct{}

var resolver = &net.Resolver{PreferGo: true}

func (dnsClient) LookupTxt(hostname string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return resolver.LookupTXT(ctx, hostname)
}

func onNewMail(c smtpd.Connection, from smtpd.MailAddress) (smtpd.Envelope, error) {
	return &env{
		from: from,
		conn: c,
	}, nil
}

func onNewConnection(c smtpd.Connection) error {
	log.Printf("smtpd: new connection from %v", c.Addr())
	return nil
}
