// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gerrit

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Auth is a Gerrit authentication mode.
// The most common ones are NoAuth or BasicAuth.
type Auth interface {
	setAuth(*Client, *http.Request)
}

// BasicAuth sends a username and password.
func BasicAuth(username, password string) Auth {
	return basicAuth{username, password}
}

type basicAuth struct {
	username, password string
}

func (ba basicAuth) setAuth(c *Client, r *http.Request) {
	r.SetBasicAuth(ba.username, ba.password)
}

// GitCookiesAuth derives the Gerrit authentication token from
// gitcookies based on the URL of the Gerrit request.
// The cookie file used is determined by running "git config
// http.cookiefile" in the current directory.
// To use a specific file, see GitCookieFileAuth.
func GitCookiesAuth() Auth {
	return gitCookiesAuth{}
}

// GitCookieFileAuth derives the Gerrit authentication token from the
// provided gitcookies file. It is equivalent to GitCookiesAuth,
// except that "git config http.cookiefile" is not used to find which
// cookie file to use.
func GitCookieFileAuth(file string) Auth {
	return &gitCookieFileAuth{file: file}
}

func netrcPath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("USERPROFILE"), "_netrc")
	}
	return filepath.Join(os.Getenv("HOME"), ".netrc")
}

type gitCookiesAuth struct{}

func (gitCookiesAuth) setAuth(c *Client, r *http.Request) {
	// First look in Git's http.cookiefile, which is where Gerrit
	// now tells users to store this information.
	git := exec.Command("git", "config", "http.cookiefile")
	git.Stderr = os.Stderr

	// Ignore a failure here, git will exit(1) if no cookies are
	// present and prevent the netrc from being read below.
	gitOut, _ := git.Output()

	cookieFile := strings.TrimSpace(string(gitOut))
	if len(cookieFile) != 0 {
		auth := &gitCookieFileAuth{file: cookieFile}
		auth.setAuth(c, r)
		if len(r.Header["Cookie"]) > 0 {
			return
		}
	}

	url, err := url.Parse(c.url)
	if err != nil {
		// Something else will complain about this.
		return
	}

	// If not there, then look in $HOME/.netrc, which is where Gerrit
	// used to tell users to store the information, until the passwords
	// got so long that old versions of curl couldn't handle them.
	host := url.Host
	netrc := netrcPath()
	data, _ := ioutil.ReadFile(netrc)
	for _, line := range strings.Split(string(data), "\n") {
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		f := strings.Fields(line)
		if len(f) >= 6 && f[0] == "machine" && f[1] == host && f[2] == "login" && f[4] == "password" {
			r.SetBasicAuth(f[3], f[5])
			return
		}
	}
	log.Printf("no authentication configured for Gerrit; tried both git config http.cookiefile and %s", netrc)
}

type gitCookieFileAuth struct {
	file string

	once sync.Once
	jar  *cookiejar.Jar
	err  error
}

func (a *gitCookieFileAuth) loadCookieFileOnce() {
	data, err := ioutil.ReadFile(a.file)
	if err != nil {
		a.err = fmt.Errorf("Error loading cookie file: %v", err)
		return
	}
	a.jar = parseGitCookies(string(data))
}

func (a *gitCookieFileAuth) setAuth(c *Client, r *http.Request) {
	a.once.Do(a.loadCookieFileOnce)
	if a.err != nil {
		log.Print(a.err)
		return
	}

	url, err := url.Parse(c.url)
	if err != nil {
		// Something else will complain about this.
		return
	}

	for _, cookie := range a.jar.Cookies(url) {
		r.AddCookie(cookie)
	}
}

func parseGitCookies(data string) *cookiejar.Jar {
	jar, _ := cookiejar.New(nil)
	for _, line := range strings.Split(data, "\n") {
		f := strings.Split(line, "\t")
		if len(f) < 7 {
			continue
		}
		expires, err := strconv.ParseInt(f[4], 10, 64)
		if err != nil {
			continue
		}
		c := http.Cookie{
			Domain:  f[0],
			Path:    f[2],
			Secure:  f[3] == "TRUE",
			Expires: time.Unix(expires, 0),
			Name:    f[5],
			Value:   f[6],
		}
		// Construct a fake URL to add c to the jar.
		url := url.URL{
			Scheme: "http",
			Host:   c.Domain,
			Path:   c.Path,
		}
		jar.SetCookies(&url, []*http.Cookie{&c})
	}
	return jar
}

// NoAuth makes requests unauthenticated.
var NoAuth = noAuth{}

type noAuth struct{}

func (noAuth) setAuth(c *Client, r *http.Request) {}

type digestAuth struct {
	Username, Password, Realm, NONCE, QOP, Opaque, Algorithm string
}

func getDigestAuth(username, password string, resp *http.Response) *digestAuth {
	header := resp.Header.Get("www-authenticate")
	parts := strings.SplitN(header, " ", 2)
	parts = strings.Split(parts[1], ", ")
	opts := make(map[string]string)

	for _, part := range parts {
		vals := strings.SplitN(part, "=", 2)
		key := vals[0]
		val := strings.Trim(vals[1], "\",")
		opts[key] = val
	}

	auth := digestAuth{
		username, password,
		opts["realm"], opts["nonce"], opts["qop"], opts["opaque"], opts["algorithm"],
	}
	return &auth
}

func setDigestAuth(r *http.Request, username, password string, resp *http.Response, nc int) {
	auth := getDigestAuth(username, password, resp)
	authStr := getDigestAuthString(auth, r.URL, r.Method, nc)
	r.Header.Add("Authorization", authStr)
}

func getDigestAuthString(auth *digestAuth, url *url.URL, method string, nc int) string {
	var buf bytes.Buffer
	h := md5.New()
	fmt.Fprintf(&buf, "%s:%s:%s", auth.Username, auth.Realm, auth.Password)
	buf.WriteTo(h)
	ha1 := hex.EncodeToString(h.Sum(nil))

	h = md5.New()
	fmt.Fprintf(&buf, "%s:%s", method, url.Path)
	buf.WriteTo(h)
	ha2 := hex.EncodeToString(h.Sum(nil))

	ncStr := fmt.Sprintf("%08x", nc)
	hnc := "MTM3MDgw"

	h = md5.New()
	fmt.Fprintf(&buf, "%s:%s:%s:%s:%s:%s", ha1, auth.NONCE, ncStr, hnc, auth.QOP, ha2)
	buf.WriteTo(h)
	respdig := hex.EncodeToString(h.Sum(nil))

	buf.Write([]byte("Digest "))
	fmt.Fprintf(&buf,
		`username="%s", realm="%s", nonce="%s", uri="%s", response="%s"`,
		auth.Username, auth.Realm, auth.NONCE, url.Path, respdig,
	)

	if auth.Opaque != "" {
		fmt.Fprintf(&buf, `, opaque="%s"`, auth.Opaque)
	}
	if auth.QOP != "" {
		fmt.Fprintf(&buf, `, qop="%s", nc=%s, cnonce="%s"`, auth.QOP, ncStr, hnc)
	}
	if auth.Algorithm != "" {
		fmt.Fprintf(&buf, `, algorithm="%s"`, auth.Algorithm)
	}

	return buf.String()
}

func (a digestAuth) setAuth(c *Client, r *http.Request) {
	resp, err := http.Get(r.URL.String())
	if err != nil {
		return
	}
	setDigestAuth(r, a.Username, a.Password, resp, 1)
}

// DigestAuth returns an Auth implementation which sends
// the provided username and password using HTTP Digest Authentication
// (RFC 2617)
func DigestAuth(username, password string) Auth {
	return digestAuth{
		Username: username,
		Password: password,
	}
}
