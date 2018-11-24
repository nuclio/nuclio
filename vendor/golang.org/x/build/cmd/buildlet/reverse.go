// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"crypto/hmac"
	"crypto/md5"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"golang.org/x/build"
	"golang.org/x/build/revdial"
)

// mode is either a BuildConfig or HostConfig name (map key in x/build/dashboard/builders.go)
func keyForMode(mode string) (string, error) {
	if isDevReverseMode() {
		return devBuilderKey(mode), nil
	}
	if os.Getenv("GO_BUILDER_ENV") == "macstadium_vm" {
		infoKey := "guestinfo.key-" + mode
		key := vmwareGetInfo(infoKey)
		if key == "" {
			return "", fmt.Errorf("no build key found for VMWare info-get key %q", infoKey)
		}
		return key, nil
	}
	keyPath := filepath.Join(homedir(), ".gobuildkey-"+mode)
	if v := os.Getenv("GO_BUILD_KEY_PATH"); v != "" {
		keyPath = v
	}
	key, err := ioutil.ReadFile(keyPath)
	if ok, _ := strconv.ParseBool(os.Getenv("GO_BUILD_KEY_DELETE_AFTER_READ")); ok {
		os.Remove(keyPath)
	}
	if err != nil {
		if os.IsNotExist(err) && *reverse != "" && !strings.Contains(*reverse, ",") {
			globalKeyPath := filepath.Join(homedir(), ".gobuildkey")
			key, err = ioutil.ReadFile(globalKeyPath)
			if err != nil {
				return "", fmt.Errorf("cannot read either key file %q or %q: %v", keyPath, globalKeyPath, err)
			}
		}
		if len(key) == 0 || err != nil {
			return "", fmt.Errorf("cannot read key file %q: %v", keyPath, err)
		}
	}
	return string(key), nil
}

func isDevReverseMode() bool {
	return !strings.HasPrefix(*coordinator, "farmer.golang.org")
}

func dialCoordinator() error {
	devMode := isDevReverseMode()

	if *hostname == "" {
		*hostname, _ = os.Hostname()
	}

	var modes, keys []string
	if *reverse != "" {
		// Old way.
		modes = strings.Split(*reverse, ",")
		for _, m := range modes {
			key, err := keyForMode(m)
			if err != nil {
				log.Fatalf("failed to find key for %s: %v", m, err)
			}
			keys = append(keys, key)
		}
	} else {
		// New way.
		key, err := keyForMode(*reverseType)
		if err != nil {
			log.Fatalf("failed to find key for %s: %v", *reverseType, err)
		}
		keys = append(keys, key)
	}

	caCert := build.ProdCoordinatorCA
	addr := *coordinator
	if addr == "farmer.golang.org" {
		addr = "farmer.golang.org:443"
	}
	if devMode {
		caCert = build.DevCoordinatorCA
	}

	var caPool *x509.CertPool
	if runtime.GOOS == "windows" {
		// No SystemCertPool on Windows. But we don't run
		// Windows in reverse mode anyway.  So just don't set
		// caPool, which will cause tls.Config to use the
		// system verification.
	} else {
		var err error
		caPool, err = x509.SystemCertPool()
		if err != nil {
			return fmt.Errorf("SystemCertPool: %v", err)
		}
		// Temporarily accept our own CA. This predates LetsEncrypt.
		// Our old self-signed cert expires April 4th, 2017.
		// We can remove this after golang.org/issue/16442 is fixed.
		if !caPool.AppendCertsFromPEM([]byte(caCert)) {
			return errors.New("failed to append coordinator CA certificate")
		}
	}

	log.Printf("Dialing coordinator %s ...", addr)
	tcpConn, err := dialCoordinatorTCP(addr)
	if err != nil {
		return err
	}

	serverName := strings.TrimSuffix(addr, ":443")
	log.Printf("Doing TLS handshake with coordinator (verifying hostname %q)...", serverName)
	tcpConn.SetDeadline(time.Now().Add(30 * time.Second))
	config := &tls.Config{
		ServerName:         serverName,
		RootCAs:            caPool,
		InsecureSkipVerify: devMode,
	}
	conn := tls.Client(tcpConn, config)
	if err := conn.Handshake(); err != nil {
		return fmt.Errorf("failed to handshake with coordinator: %v", err)
	}
	tcpConn.SetDeadline(time.Time{})

	bufr := bufio.NewReader(conn)

	log.Printf("Registering reverse mode with coordinator...")
	req, err := http.NewRequest("GET", "/reverse", nil)
	if err != nil {
		log.Fatal(err)
	}
	if *reverse != "" {
		// Old way.
		req.Header["X-Go-Builder-Type"] = modes
	} else {
		req.Header.Set("X-Go-Host-Type", *reverseType)
	}
	req.Header["X-Go-Builder-Key"] = keys
	req.Header.Set("X-Go-Builder-Hostname", *hostname)
	req.Header.Set("X-Go-Builder-Version", strconv.Itoa(buildletVersion))
	if err := req.Write(conn); err != nil {
		return fmt.Errorf("coordinator /reverse request failed: %v", err)
	}
	resp, err := http.ReadResponse(bufr, req)
	if err != nil {
		return fmt.Errorf("coordinator /reverse response failed: %v", err)
	}
	if resp.StatusCode != 101 {
		msg, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("coordinator registration failed; want HTTP status 101; got %v:\n\t%s", resp.Status, msg)
	}

	log.Printf("Connected to coordinator; reverse dialing active")
	srv := &http.Server{}
	ln := revdial.NewListener(bufio.NewReadWriter(
		bufio.NewReader(conn),
		bufio.NewWriter(deadlinePerWriteConn{conn, 60 * time.Second}),
	))
	err = srv.Serve(ln)
	if ln.Closed() {
		return nil
	}
	return fmt.Errorf("http.Serve on reverse connection complete: %v", err)
}

var coordDialer = &net.Dialer{
	Timeout:   10 * time.Second,
	KeepAlive: 15 * time.Second,
}

// dialCoordinatorTCP returns a TCP connection to the coordinator, making
// a CONNECT request to a proxy as a fallback.
func dialCoordinatorTCP(addr string) (net.Conn, error) {
	tcpConn, err := coordDialer.Dial("tcp", addr)
	if err != nil {
		// If we had problems connecting to the TCP addr
		// directly, perhaps there's a proxy in the way. See
		// if they have an HTTPS_PROXY environment variable
		// defined and try to do a CONNECT request to it.
		req, _ := http.NewRequest("GET", "https://"+addr, nil)
		proxyURL, _ := http.ProxyFromEnvironment(req)
		if proxyURL != nil {
			return dialCoordinatorViaCONNECT(addr, proxyURL)
		}
		return nil, err
	}
	return tcpConn, nil
}

func dialCoordinatorViaCONNECT(addr string, proxy *url.URL) (net.Conn, error) {
	proxyAddr := proxy.Host
	if proxy.Port() == "" {
		proxyAddr = net.JoinHostPort(proxyAddr, "80")
	}
	log.Printf("dialing proxy %q ...", proxyAddr)
	c, err := net.Dial("tcp", proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("dialing proxy %q failed: %v", proxyAddr, err)
	}
	fmt.Fprintf(c, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", addr, proxy.Hostname())
	br := bufio.NewReader(c)
	res, err := http.ReadResponse(br, nil)
	if err != nil {
		return nil, fmt.Errorf("reading HTTP response from CONNECT to %s via proxy %s failed: %v",
			addr, proxyAddr, err)
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("proxy error from %s while dialing %s: %v", proxyAddr, addr, res.Status)
	}

	// It's safe to discard the bufio.Reader here and return the
	// original TCP conn directly because we only use this for
	// TLS, and in TLS the client speaks first, so we know there's
	// no unbuffered data. But we can double-check.
	if br.Buffered() > 0 {
		return nil, fmt.Errorf("unexpected %d bytes of buffered data from CONNECT proxy %q",
			br.Buffered(), proxyAddr)
	}
	return c, nil
}

type deadlinePerWriteConn struct {
	net.Conn
	writeTimeout time.Duration
}

func (c deadlinePerWriteConn) Write(p []byte) (n int, err error) {
	c.Conn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	defer c.Conn.SetWriteDeadline(time.Time{})
	return c.Conn.Write(p)
}

func devBuilderKey(builder string) string {
	h := hmac.New(md5.New, []byte("gophers rule"))
	io.WriteString(h, builder)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func homedir() string {
	switch runtime.GOOS {
	case "windows":
		return os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
	case "plan9":
		return os.Getenv("home")
	}
	home := os.Getenv("HOME")
	if home != "" {
		return home
	}
	if os.Getuid() == 0 {
		return "/root"
	}
	return "/"
}

// TestDialCoordinator dials the coordinator. Exported for testing.
func TestDialCoordinator() {
	// TODO(crawshaw): move some of this logic out of main to simplify testing hook.
	http.Handle("/status", http.HandlerFunc(handleStatus))
	dialCoordinator()
}
