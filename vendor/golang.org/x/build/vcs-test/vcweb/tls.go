// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"crypto/tls"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"os"
	"reflect"
	"runtime/debug"
	"strconv"
	"sync"
	"time"
)

func handleTLS(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<html>\n")
	fmt.Fprintf(w, "<title>vcs-test.golang.org/tls</title>\n<pre>\n")
	fmt.Fprintf(w, "<b>vcs-test.golang.org/tls</b>\n\n")
	fmt.Fprintf(w, "This page shows debug information about TLS client connections.\n\n")
	fmt.Fprintf(w, "\n<b><u>Your Request</u></b>\n\n")
	showTLSRequest(w, time.Now(), r, r.TLS, findHello(w, true))
	showTLSLog(w)
	showHelloLog(w)
}

const maxTLSLog = 100

var tlsLog struct {
	mu  sync.Mutex
	w   int64
	buf [maxTLSLog]tlsLogEntry
}

type tlsLogEntry struct {
	time  time.Time
	r     *http.Request
	cs    *tls.ConnectionState
	hello *tls.ClientHelloInfo
}

const maxHelloLog = 100

var recentHello struct {
	mu  sync.Mutex
	w   int64
	buf [maxHelloLog]helloLogEntry
}

type helloLogEntry struct {
	time  time.Time
	hello *tls.ClientHelloInfo
	conn  net.Conn
	used  bool
}

func saveHello(hello *tls.ClientHelloInfo) {
	recentHello.mu.Lock()
	recentHello.buf[recentHello.w%maxHelloLog] = helloLogEntry{time.Now(), hello, hello.Conn, false}
	recentHello.w++
	recentHello.mu.Unlock()
}

func findHello(w http.ResponseWriter, dbg bool) *tls.ClientHelloInfo {
	defer func() {
		if err := recover(); err != nil && dbg {
			fmt.Fprintf(w, "PANIC: %s\n%s\n", err, debug.Stack())
		}
	}()
	rw := reflect.ValueOf(w) // *loggingResponseWriter
	rw = rw.Elem().FieldByName("ResponseWriter").Elem()
	switch rw.Type().String() {
	case "*http.http2responseWriter":
		rw = rw.Elem().FieldByName("rws").Elem().FieldByName("conn").Elem().FieldByName("conn")
	case "*http.response":
		rw = rw.Elem().FieldByName("conn").Elem().FieldByName("rwc")
	}
	// now a net.Conn implemented by *tls.Conn
	rw = rw.Elem().Elem().FieldByName("conn").Elem()
	// now a net.Conn implementation of some kind
	if rw.Kind() != reflect.Ptr || rw.IsNil() {
		if dbg {
			fmt.Fprintf(w, "FINDHELLO STOP: %s\n", rw.Type().String())
		}
		return nil
	}
	ptr := rw.Pointer()
	recentHello.mu.Lock()
	defer recentHello.mu.Unlock()
	for i, e := range recentHello.buf {
		if e.conn == nil || e.hello == nil {
			continue
		}
		rc := reflect.ValueOf(e.conn)
		if rc.Kind() != reflect.Ptr || rc.IsNil() || rc.Type() != rw.Type() {
			if dbg {
				fmt.Fprintf(w, "- findhello wrong %s vs %s\n", rc.Type().String(), rw.Type())
			}
			continue
		}
		if rc.Pointer() == ptr {
			recentHello.buf[i].used = true
			return e.hello
		}
	}
	if dbg {
		fmt.Fprintf(w, "FINDHELLO: %s\n", rw.Type().String())
	}
	fmt.Fprintf(os.Stderr, "FINDHELLO: %s\n", rw.Type().String())
	return nil
}

func addTLSLog(w http.ResponseWriter, r *http.Request) {
	if r.TLS == nil {
		return
	}
	hello := findHello(w, false)
	tlsLog.mu.Lock()
	tlsLog.buf[tlsLog.w%maxTLSLog] = tlsLogEntry{time.Now(), r, r.TLS, hello}
	tlsLog.w++
	tlsLog.mu.Unlock()
}

func showTLSLog(w http.ResponseWriter) {
	all := make([]tlsLogEntry, 0, maxTLSLog)
	tlsLog.mu.Lock()
	i := tlsLog.w - maxTLSLog
	if i < 0 {
		i = 0
	}
	for ; i < tlsLog.w; i++ {
		all = append(all, tlsLog.buf[i%maxTLSLog])
	}
	tlsLog.mu.Unlock()

	fmt.Fprintf(w, "<b><u>Recent Requests</u></b>\n\n")
	for _, e := range all {
		showTLSRequest(w, e.time, e.r, e.cs, e.hello)
	}
}

func showHelloLog(w http.ResponseWriter) {
	all := make([]helloLogEntry, 0, maxHelloLog)
	recentHello.mu.Lock()
	i := recentHello.w - maxHelloLog
	if i < 0 {
		i = 0
	}
	for ; i < recentHello.w; i++ {
		all = append(all, recentHello.buf[i%maxTLSLog])
	}
	recentHello.mu.Unlock()

	fmt.Fprintf(w, "<b><u>Recent unmatched ClientHelloInfo</u></b>\n\n")
	for _, e := range all {
		if e.conn == nil || e.hello == nil || e.used {
			continue
		}
		fmt.Fprintf(w, "<b>%s # %s</b>\n", e.conn.RemoteAddr(), e.time.Format(time.RFC3339))
		printTLSHello(w, e.hello)
		fmt.Fprintf(w, "\n")
	}
}

func showTLSRequest(w http.ResponseWriter, t time.Time, r *http.Request, cs *tls.ConnectionState, hello *tls.ClientHelloInfo) {
	var e = html.EscapeString
	var q = strconv.Quote

	defer fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "<b>%s %s %s %s # %s</b>\n", e(r.RemoteAddr), e(r.Method), e(r.RequestURI), e(r.Proto), t.Format(time.RFC3339))
	fmt.Fprintf(w, "\tUser-Agent: %s\n", e(r.Header.Get("User-Agent")))
	if cs == nil {
		fmt.Fprintf(w, "\tNo TLS\n")
		return
	}
	fmt.Fprintf(w, "\tTLS ConnectionState:\n")
	fmt.Fprintf(w, "\t\tVersion: %s\n", nameOrValue(int(cs.Version), versionNames, "%#x"))
	fmt.Fprintf(w, "\t\tCipherSuite: %s\n", nameOrValue(int(cs.CipherSuite), suiteNames, "%#x"))
	fmt.Fprintf(w, "\t\tNegotiatedProtocol: %s\n", e(q(cs.NegotiatedProtocol)))
	fmt.Fprintf(w, "\t\tServerName: %s\n", e(q(cs.ServerName)))
	fmt.Fprintf(w, "\tTLS ClientHello:\n")
	if hello == nil {
		fmt.Fprintf(w, "\t\tunavailable\n")
		return
	}
	findHello(w, false) // so it doesn't show up as unmatched
	printTLSHello(w, hello)
}

func printTLSHello(w io.Writer, hello *tls.ClientHelloInfo) {
	var e = html.EscapeString
	var q = strconv.Quote

	fmt.Fprintf(w, "\t\tServerName: %s\n", e(q(hello.ServerName)))
	fmt.Fprintf(w, "\t\tCipherSuites:")
	for _, suite := range hello.CipherSuites {
		fmt.Fprintf(w, "\n\t\t\t%s", nameOrValue(int(suite), suiteNames, "%#x"))
	}
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "\t\tSupportedCurves:")
	for _, curve := range hello.SupportedCurves {
		fmt.Fprintf(w, " %s", nameOrValue(int(curve), curveNames, "%d"))
	}
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "\t\tSupportedPoints:")
	for _, pt := range hello.SupportedPoints {
		fmt.Fprintf(w, " %s", nameOrValue(int(pt), pointNames, "%d"))
	}
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "\t\tSignatureSchemes:")
	for _, sig := range hello.SignatureSchemes {
		fmt.Fprintf(w, " %s", nameOrValue(int(sig), sigNames, "%#x"))
	}
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "\t\tSupportedVersions:")
	for _, v := range hello.SupportedVersions {
		fmt.Fprintf(w, " %s", nameOrValue(int(v), versionNames, "%#x"))
	}
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "\t\tSupportedProtos:")
	for _, s := range hello.SupportedProtos {
		fmt.Fprintf(w, " %s", e(q(s)))
	}
	fmt.Fprintf(w, "\n")
}

func nameOrValue(x int, names map[int]string, format string) string {
	name := names[x]
	if name != "" {
		return name
	}
	return fmt.Sprintf(format, x)
}

var suiteNames = map[int]string{
	// https://www.iana.org/assignments/tls-parameters/tls-parameters-4.csv
	// csv2tsv tls-parameters-4.csv  | awk '{print "\t"$1": \"" $2 "\","}'|  sed 's/,0x//'  | egrep -v 'Unassigned|Reserved'
	0x0000: "TLS_NULL_WITH_NULL_NULL",
	0x0001: "TLS_RSA_WITH_NULL_MD5",
	0x0002: "TLS_RSA_WITH_NULL_SHA",
	0x0003: "TLS_RSA_EXPORT_WITH_RC4_40_MD5",
	0x0004: "TLS_RSA_WITH_RC4_128_MD5",
	0x0005: "TLS_RSA_WITH_RC4_128_SHA",
	0x0006: "TLS_RSA_EXPORT_WITH_RC2_CBC_40_MD5",
	0x0007: "TLS_RSA_WITH_IDEA_CBC_SHA",
	0x0008: "TLS_RSA_EXPORT_WITH_DES40_CBC_SHA",
	0x0009: "TLS_RSA_WITH_DES_CBC_SHA",
	0x000A: "TLS_RSA_WITH_3DES_EDE_CBC_SHA",
	0x000B: "TLS_DH_DSS_EXPORT_WITH_DES40_CBC_SHA",
	0x000C: "TLS_DH_DSS_WITH_DES_CBC_SHA",
	0x000D: "TLS_DH_DSS_WITH_3DES_EDE_CBC_SHA",
	0x000E: "TLS_DH_RSA_EXPORT_WITH_DES40_CBC_SHA",
	0x000F: "TLS_DH_RSA_WITH_DES_CBC_SHA",
	0x0010: "TLS_DH_RSA_WITH_3DES_EDE_CBC_SHA",
	0x0011: "TLS_DHE_DSS_EXPORT_WITH_DES40_CBC_SHA",
	0x0012: "TLS_DHE_DSS_WITH_DES_CBC_SHA",
	0x0013: "TLS_DHE_DSS_WITH_3DES_EDE_CBC_SHA",
	0x0014: "TLS_DHE_RSA_EXPORT_WITH_DES40_CBC_SHA",
	0x0015: "TLS_DHE_RSA_WITH_DES_CBC_SHA",
	0x0016: "TLS_DHE_RSA_WITH_3DES_EDE_CBC_SHA",
	0x0017: "TLS_DH_anon_EXPORT_WITH_RC4_40_MD5",
	0x0018: "TLS_DH_anon_WITH_RC4_128_MD5",
	0x0019: "TLS_DH_anon_EXPORT_WITH_DES40_CBC_SHA",
	0x001A: "TLS_DH_anon_WITH_DES_CBC_SHA",
	0x001B: "TLS_DH_anon_WITH_3DES_EDE_CBC_SHA",
	0x001E: "TLS_KRB5_WITH_DES_CBC_SHA",
	0x001F: "TLS_KRB5_WITH_3DES_EDE_CBC_SHA",
	0x0020: "TLS_KRB5_WITH_RC4_128_SHA",
	0x0021: "TLS_KRB5_WITH_IDEA_CBC_SHA",
	0x0022: "TLS_KRB5_WITH_DES_CBC_MD5",
	0x0023: "TLS_KRB5_WITH_3DES_EDE_CBC_MD5",
	0x0024: "TLS_KRB5_WITH_RC4_128_MD5",
	0x0025: "TLS_KRB5_WITH_IDEA_CBC_MD5",
	0x0026: "TLS_KRB5_EXPORT_WITH_DES_CBC_40_SHA",
	0x0027: "TLS_KRB5_EXPORT_WITH_RC2_CBC_40_SHA",
	0x0028: "TLS_KRB5_EXPORT_WITH_RC4_40_SHA",
	0x0029: "TLS_KRB5_EXPORT_WITH_DES_CBC_40_MD5",
	0x002A: "TLS_KRB5_EXPORT_WITH_RC2_CBC_40_MD5",
	0x002B: "TLS_KRB5_EXPORT_WITH_RC4_40_MD5",
	0x002C: "TLS_PSK_WITH_NULL_SHA",
	0x002D: "TLS_DHE_PSK_WITH_NULL_SHA",
	0x002E: "TLS_RSA_PSK_WITH_NULL_SHA",
	0x002F: "TLS_RSA_WITH_AES_128_CBC_SHA",
	0x0030: "TLS_DH_DSS_WITH_AES_128_CBC_SHA",
	0x0031: "TLS_DH_RSA_WITH_AES_128_CBC_SHA",
	0x0032: "TLS_DHE_DSS_WITH_AES_128_CBC_SHA",
	0x0033: "TLS_DHE_RSA_WITH_AES_128_CBC_SHA",
	0x0034: "TLS_DH_anon_WITH_AES_128_CBC_SHA",
	0x0035: "TLS_RSA_WITH_AES_256_CBC_SHA",
	0x0036: "TLS_DH_DSS_WITH_AES_256_CBC_SHA",
	0x0037: "TLS_DH_RSA_WITH_AES_256_CBC_SHA",
	0x0038: "TLS_DHE_DSS_WITH_AES_256_CBC_SHA",
	0x0039: "TLS_DHE_RSA_WITH_AES_256_CBC_SHA",
	0x003A: "TLS_DH_anon_WITH_AES_256_CBC_SHA",
	0x003B: "TLS_RSA_WITH_NULL_SHA256",
	0x003C: "TLS_RSA_WITH_AES_128_CBC_SHA256",
	0x003D: "TLS_RSA_WITH_AES_256_CBC_SHA256",
	0x003E: "TLS_DH_DSS_WITH_AES_128_CBC_SHA256",
	0x003F: "TLS_DH_RSA_WITH_AES_128_CBC_SHA256",
	0x0040: "TLS_DHE_DSS_WITH_AES_128_CBC_SHA256",
	0x0041: "TLS_RSA_WITH_CAMELLIA_128_CBC_SHA",
	0x0042: "TLS_DH_DSS_WITH_CAMELLIA_128_CBC_SHA",
	0x0043: "TLS_DH_RSA_WITH_CAMELLIA_128_CBC_SHA",
	0x0044: "TLS_DHE_DSS_WITH_CAMELLIA_128_CBC_SHA",
	0x0045: "TLS_DHE_RSA_WITH_CAMELLIA_128_CBC_SHA",
	0x0046: "TLS_DH_anon_WITH_CAMELLIA_128_CBC_SHA",
	0x0067: "TLS_DHE_RSA_WITH_AES_128_CBC_SHA256",
	0x0068: "TLS_DH_DSS_WITH_AES_256_CBC_SHA256",
	0x0069: "TLS_DH_RSA_WITH_AES_256_CBC_SHA256",
	0x006A: "TLS_DHE_DSS_WITH_AES_256_CBC_SHA256",
	0x006B: "TLS_DHE_RSA_WITH_AES_256_CBC_SHA256",
	0x006C: "TLS_DH_anon_WITH_AES_128_CBC_SHA256",
	0x006D: "TLS_DH_anon_WITH_AES_256_CBC_SHA256",
	0x0084: "TLS_RSA_WITH_CAMELLIA_256_CBC_SHA",
	0x0085: "TLS_DH_DSS_WITH_CAMELLIA_256_CBC_SHA",
	0x0086: "TLS_DH_RSA_WITH_CAMELLIA_256_CBC_SHA",
	0x0087: "TLS_DHE_DSS_WITH_CAMELLIA_256_CBC_SHA",
	0x0088: "TLS_DHE_RSA_WITH_CAMELLIA_256_CBC_SHA",
	0x0089: "TLS_DH_anon_WITH_CAMELLIA_256_CBC_SHA",
	0x008A: "TLS_PSK_WITH_RC4_128_SHA",
	0x008B: "TLS_PSK_WITH_3DES_EDE_CBC_SHA",
	0x008C: "TLS_PSK_WITH_AES_128_CBC_SHA",
	0x008D: "TLS_PSK_WITH_AES_256_CBC_SHA",
	0x008E: "TLS_DHE_PSK_WITH_RC4_128_SHA",
	0x008F: "TLS_DHE_PSK_WITH_3DES_EDE_CBC_SHA",
	0x0090: "TLS_DHE_PSK_WITH_AES_128_CBC_SHA",
	0x0091: "TLS_DHE_PSK_WITH_AES_256_CBC_SHA",
	0x0092: "TLS_RSA_PSK_WITH_RC4_128_SHA",
	0x0093: "TLS_RSA_PSK_WITH_3DES_EDE_CBC_SHA",
	0x0094: "TLS_RSA_PSK_WITH_AES_128_CBC_SHA",
	0x0095: "TLS_RSA_PSK_WITH_AES_256_CBC_SHA",
	0x0096: "TLS_RSA_WITH_SEED_CBC_SHA",
	0x0097: "TLS_DH_DSS_WITH_SEED_CBC_SHA",
	0x0098: "TLS_DH_RSA_WITH_SEED_CBC_SHA",
	0x0099: "TLS_DHE_DSS_WITH_SEED_CBC_SHA",
	0x009A: "TLS_DHE_RSA_WITH_SEED_CBC_SHA",
	0x009B: "TLS_DH_anon_WITH_SEED_CBC_SHA",
	0x009C: "TLS_RSA_WITH_AES_128_GCM_SHA256",
	0x009D: "TLS_RSA_WITH_AES_256_GCM_SHA384",
	0x009E: "TLS_DHE_RSA_WITH_AES_128_GCM_SHA256",
	0x009F: "TLS_DHE_RSA_WITH_AES_256_GCM_SHA384",
	0x00A0: "TLS_DH_RSA_WITH_AES_128_GCM_SHA256",
	0x00A1: "TLS_DH_RSA_WITH_AES_256_GCM_SHA384",
	0x00A2: "TLS_DHE_DSS_WITH_AES_128_GCM_SHA256",
	0x00A3: "TLS_DHE_DSS_WITH_AES_256_GCM_SHA384",
	0x00A4: "TLS_DH_DSS_WITH_AES_128_GCM_SHA256",
	0x00A5: "TLS_DH_DSS_WITH_AES_256_GCM_SHA384",
	0x00A6: "TLS_DH_anon_WITH_AES_128_GCM_SHA256",
	0x00A7: "TLS_DH_anon_WITH_AES_256_GCM_SHA384",
	0x00A8: "TLS_PSK_WITH_AES_128_GCM_SHA256",
	0x00A9: "TLS_PSK_WITH_AES_256_GCM_SHA384",
	0x00AA: "TLS_DHE_PSK_WITH_AES_128_GCM_SHA256",
	0x00AB: "TLS_DHE_PSK_WITH_AES_256_GCM_SHA384",
	0x00AC: "TLS_RSA_PSK_WITH_AES_128_GCM_SHA256",
	0x00AD: "TLS_RSA_PSK_WITH_AES_256_GCM_SHA384",
	0x00AE: "TLS_PSK_WITH_AES_128_CBC_SHA256",
	0x00AF: "TLS_PSK_WITH_AES_256_CBC_SHA384",
	0x00B0: "TLS_PSK_WITH_NULL_SHA256",
	0x00B1: "TLS_PSK_WITH_NULL_SHA384",
	0x00B2: "TLS_DHE_PSK_WITH_AES_128_CBC_SHA256",
	0x00B3: "TLS_DHE_PSK_WITH_AES_256_CBC_SHA384",
	0x00B4: "TLS_DHE_PSK_WITH_NULL_SHA256",
	0x00B5: "TLS_DHE_PSK_WITH_NULL_SHA384",
	0x00B6: "TLS_RSA_PSK_WITH_AES_128_CBC_SHA256",
	0x00B7: "TLS_RSA_PSK_WITH_AES_256_CBC_SHA384",
	0x00B8: "TLS_RSA_PSK_WITH_NULL_SHA256",
	0x00B9: "TLS_RSA_PSK_WITH_NULL_SHA384",
	0x00BA: "TLS_RSA_WITH_CAMELLIA_128_CBC_SHA256",
	0x00BB: "TLS_DH_DSS_WITH_CAMELLIA_128_CBC_SHA256",
	0x00BC: "TLS_DH_RSA_WITH_CAMELLIA_128_CBC_SHA256",
	0x00BD: "TLS_DHE_DSS_WITH_CAMELLIA_128_CBC_SHA256",
	0x00BE: "TLS_DHE_RSA_WITH_CAMELLIA_128_CBC_SHA256",
	0x00BF: "TLS_DH_anon_WITH_CAMELLIA_128_CBC_SHA256",
	0x00C0: "TLS_RSA_WITH_CAMELLIA_256_CBC_SHA256",
	0x00C1: "TLS_DH_DSS_WITH_CAMELLIA_256_CBC_SHA256",
	0x00C2: "TLS_DH_RSA_WITH_CAMELLIA_256_CBC_SHA256",
	0x00C3: "TLS_DHE_DSS_WITH_CAMELLIA_256_CBC_SHA256",
	0x00C4: "TLS_DHE_RSA_WITH_CAMELLIA_256_CBC_SHA256",
	0x00C5: "TLS_DH_anon_WITH_CAMELLIA_256_CBC_SHA256",
	0x00FF: "TLS_EMPTY_RENEGOTIATION_INFO_SCSV",
	0x5600: "TLS_FALLBACK_SCSV",
	0xC001: "TLS_ECDH_ECDSA_WITH_NULL_SHA",
	0xC002: "TLS_ECDH_ECDSA_WITH_RC4_128_SHA",
	0xC003: "TLS_ECDH_ECDSA_WITH_3DES_EDE_CBC_SHA",
	0xC004: "TLS_ECDH_ECDSA_WITH_AES_128_CBC_SHA",
	0xC005: "TLS_ECDH_ECDSA_WITH_AES_256_CBC_SHA",
	0xC006: "TLS_ECDHE_ECDSA_WITH_NULL_SHA",
	0xC007: "TLS_ECDHE_ECDSA_WITH_RC4_128_SHA",
	0xC008: "TLS_ECDHE_ECDSA_WITH_3DES_EDE_CBC_SHA",
	0xC009: "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA",
	0xC00A: "TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA",
	0xC00B: "TLS_ECDH_RSA_WITH_NULL_SHA",
	0xC00C: "TLS_ECDH_RSA_WITH_RC4_128_SHA",
	0xC00D: "TLS_ECDH_RSA_WITH_3DES_EDE_CBC_SHA",
	0xC00E: "TLS_ECDH_RSA_WITH_AES_128_CBC_SHA",
	0xC00F: "TLS_ECDH_RSA_WITH_AES_256_CBC_SHA",
	0xC010: "TLS_ECDHE_RSA_WITH_NULL_SHA",
	0xC011: "TLS_ECDHE_RSA_WITH_RC4_128_SHA",
	0xC012: "TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA",
	0xC013: "TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA",
	0xC014: "TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA",
	0xC015: "TLS_ECDH_anon_WITH_NULL_SHA",
	0xC016: "TLS_ECDH_anon_WITH_RC4_128_SHA",
	0xC017: "TLS_ECDH_anon_WITH_3DES_EDE_CBC_SHA",
	0xC018: "TLS_ECDH_anon_WITH_AES_128_CBC_SHA",
	0xC019: "TLS_ECDH_anon_WITH_AES_256_CBC_SHA",
	0xC01A: "TLS_SRP_SHA_WITH_3DES_EDE_CBC_SHA",
	0xC01B: "TLS_SRP_SHA_RSA_WITH_3DES_EDE_CBC_SHA",
	0xC01C: "TLS_SRP_SHA_DSS_WITH_3DES_EDE_CBC_SHA",
	0xC01D: "TLS_SRP_SHA_WITH_AES_128_CBC_SHA",
	0xC01E: "TLS_SRP_SHA_RSA_WITH_AES_128_CBC_SHA",
	0xC01F: "TLS_SRP_SHA_DSS_WITH_AES_128_CBC_SHA",
	0xC020: "TLS_SRP_SHA_WITH_AES_256_CBC_SHA",
	0xC021: "TLS_SRP_SHA_RSA_WITH_AES_256_CBC_SHA",
	0xC022: "TLS_SRP_SHA_DSS_WITH_AES_256_CBC_SHA",
	0xC023: "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256",
	0xC024: "TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA384",
	0xC025: "TLS_ECDH_ECDSA_WITH_AES_128_CBC_SHA256",
	0xC026: "TLS_ECDH_ECDSA_WITH_AES_256_CBC_SHA384",
	0xC027: "TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256",
	0xC028: "TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA384",
	0xC029: "TLS_ECDH_RSA_WITH_AES_128_CBC_SHA256",
	0xC02A: "TLS_ECDH_RSA_WITH_AES_256_CBC_SHA384",
	0xC02B: "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
	0xC02C: "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
	0xC02D: "TLS_ECDH_ECDSA_WITH_AES_128_GCM_SHA256",
	0xC02E: "TLS_ECDH_ECDSA_WITH_AES_256_GCM_SHA384",
	0xC02F: "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
	0xC030: "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
	0xC031: "TLS_ECDH_RSA_WITH_AES_128_GCM_SHA256",
	0xC032: "TLS_ECDH_RSA_WITH_AES_256_GCM_SHA384",
	0xC033: "TLS_ECDHE_PSK_WITH_RC4_128_SHA",
	0xC034: "TLS_ECDHE_PSK_WITH_3DES_EDE_CBC_SHA",
	0xC035: "TLS_ECDHE_PSK_WITH_AES_128_CBC_SHA",
	0xC036: "TLS_ECDHE_PSK_WITH_AES_256_CBC_SHA",
	0xC037: "TLS_ECDHE_PSK_WITH_AES_128_CBC_SHA256",
	0xC038: "TLS_ECDHE_PSK_WITH_AES_256_CBC_SHA384",
	0xC039: "TLS_ECDHE_PSK_WITH_NULL_SHA",
	0xC03A: "TLS_ECDHE_PSK_WITH_NULL_SHA256",
	0xC03B: "TLS_ECDHE_PSK_WITH_NULL_SHA384",
	0xC03C: "TLS_RSA_WITH_ARIA_128_CBC_SHA256",
	0xC03D: "TLS_RSA_WITH_ARIA_256_CBC_SHA384",
	0xC03E: "TLS_DH_DSS_WITH_ARIA_128_CBC_SHA256",
	0xC03F: "TLS_DH_DSS_WITH_ARIA_256_CBC_SHA384",
	0xC040: "TLS_DH_RSA_WITH_ARIA_128_CBC_SHA256",
	0xC041: "TLS_DH_RSA_WITH_ARIA_256_CBC_SHA384",
	0xC042: "TLS_DHE_DSS_WITH_ARIA_128_CBC_SHA256",
	0xC043: "TLS_DHE_DSS_WITH_ARIA_256_CBC_SHA384",
	0xC044: "TLS_DHE_RSA_WITH_ARIA_128_CBC_SHA256",
	0xC045: "TLS_DHE_RSA_WITH_ARIA_256_CBC_SHA384",
	0xC046: "TLS_DH_anon_WITH_ARIA_128_CBC_SHA256",
	0xC047: "TLS_DH_anon_WITH_ARIA_256_CBC_SHA384",
	0xC048: "TLS_ECDHE_ECDSA_WITH_ARIA_128_CBC_SHA256",
	0xC049: "TLS_ECDHE_ECDSA_WITH_ARIA_256_CBC_SHA384",
	0xC04A: "TLS_ECDH_ECDSA_WITH_ARIA_128_CBC_SHA256",
	0xC04B: "TLS_ECDH_ECDSA_WITH_ARIA_256_CBC_SHA384",
	0xC04C: "TLS_ECDHE_RSA_WITH_ARIA_128_CBC_SHA256",
	0xC04D: "TLS_ECDHE_RSA_WITH_ARIA_256_CBC_SHA384",
	0xC04E: "TLS_ECDH_RSA_WITH_ARIA_128_CBC_SHA256",
	0xC04F: "TLS_ECDH_RSA_WITH_ARIA_256_CBC_SHA384",
	0xC050: "TLS_RSA_WITH_ARIA_128_GCM_SHA256",
	0xC051: "TLS_RSA_WITH_ARIA_256_GCM_SHA384",
	0xC052: "TLS_DHE_RSA_WITH_ARIA_128_GCM_SHA256",
	0xC053: "TLS_DHE_RSA_WITH_ARIA_256_GCM_SHA384",
	0xC054: "TLS_DH_RSA_WITH_ARIA_128_GCM_SHA256",
	0xC055: "TLS_DH_RSA_WITH_ARIA_256_GCM_SHA384",
	0xC056: "TLS_DHE_DSS_WITH_ARIA_128_GCM_SHA256",
	0xC057: "TLS_DHE_DSS_WITH_ARIA_256_GCM_SHA384",
	0xC058: "TLS_DH_DSS_WITH_ARIA_128_GCM_SHA256",
	0xC059: "TLS_DH_DSS_WITH_ARIA_256_GCM_SHA384",
	0xC05A: "TLS_DH_anon_WITH_ARIA_128_GCM_SHA256",
	0xC05B: "TLS_DH_anon_WITH_ARIA_256_GCM_SHA384",
	0xC05C: "TLS_ECDHE_ECDSA_WITH_ARIA_128_GCM_SHA256",
	0xC05D: "TLS_ECDHE_ECDSA_WITH_ARIA_256_GCM_SHA384",
	0xC05E: "TLS_ECDH_ECDSA_WITH_ARIA_128_GCM_SHA256",
	0xC05F: "TLS_ECDH_ECDSA_WITH_ARIA_256_GCM_SHA384",
	0xC060: "TLS_ECDHE_RSA_WITH_ARIA_128_GCM_SHA256",
	0xC061: "TLS_ECDHE_RSA_WITH_ARIA_256_GCM_SHA384",
	0xC062: "TLS_ECDH_RSA_WITH_ARIA_128_GCM_SHA256",
	0xC063: "TLS_ECDH_RSA_WITH_ARIA_256_GCM_SHA384",
	0xC064: "TLS_PSK_WITH_ARIA_128_CBC_SHA256",
	0xC065: "TLS_PSK_WITH_ARIA_256_CBC_SHA384",
	0xC066: "TLS_DHE_PSK_WITH_ARIA_128_CBC_SHA256",
	0xC067: "TLS_DHE_PSK_WITH_ARIA_256_CBC_SHA384",
	0xC068: "TLS_RSA_PSK_WITH_ARIA_128_CBC_SHA256",
	0xC069: "TLS_RSA_PSK_WITH_ARIA_256_CBC_SHA384",
	0xC06A: "TLS_PSK_WITH_ARIA_128_GCM_SHA256",
	0xC06B: "TLS_PSK_WITH_ARIA_256_GCM_SHA384",
	0xC06C: "TLS_DHE_PSK_WITH_ARIA_128_GCM_SHA256",
	0xC06D: "TLS_DHE_PSK_WITH_ARIA_256_GCM_SHA384",
	0xC06E: "TLS_RSA_PSK_WITH_ARIA_128_GCM_SHA256",
	0xC06F: "TLS_RSA_PSK_WITH_ARIA_256_GCM_SHA384",
	0xC070: "TLS_ECDHE_PSK_WITH_ARIA_128_CBC_SHA256",
	0xC071: "TLS_ECDHE_PSK_WITH_ARIA_256_CBC_SHA384",
	0xC072: "TLS_ECDHE_ECDSA_WITH_CAMELLIA_128_CBC_SHA256",
	0xC073: "TLS_ECDHE_ECDSA_WITH_CAMELLIA_256_CBC_SHA384",
	0xC074: "TLS_ECDH_ECDSA_WITH_CAMELLIA_128_CBC_SHA256",
	0xC075: "TLS_ECDH_ECDSA_WITH_CAMELLIA_256_CBC_SHA384",
	0xC076: "TLS_ECDHE_RSA_WITH_CAMELLIA_128_CBC_SHA256",
	0xC077: "TLS_ECDHE_RSA_WITH_CAMELLIA_256_CBC_SHA384",
	0xC078: "TLS_ECDH_RSA_WITH_CAMELLIA_128_CBC_SHA256",
	0xC079: "TLS_ECDH_RSA_WITH_CAMELLIA_256_CBC_SHA384",
	0xC07A: "TLS_RSA_WITH_CAMELLIA_128_GCM_SHA256",
	0xC07B: "TLS_RSA_WITH_CAMELLIA_256_GCM_SHA384",
	0xC07C: "TLS_DHE_RSA_WITH_CAMELLIA_128_GCM_SHA256",
	0xC07D: "TLS_DHE_RSA_WITH_CAMELLIA_256_GCM_SHA384",
	0xC07E: "TLS_DH_RSA_WITH_CAMELLIA_128_GCM_SHA256",
	0xC07F: "TLS_DH_RSA_WITH_CAMELLIA_256_GCM_SHA384",
	0xC080: "TLS_DHE_DSS_WITH_CAMELLIA_128_GCM_SHA256",
	0xC081: "TLS_DHE_DSS_WITH_CAMELLIA_256_GCM_SHA384",
	0xC082: "TLS_DH_DSS_WITH_CAMELLIA_128_GCM_SHA256",
	0xC083: "TLS_DH_DSS_WITH_CAMELLIA_256_GCM_SHA384",
	0xC084: "TLS_DH_anon_WITH_CAMELLIA_128_GCM_SHA256",
	0xC085: "TLS_DH_anon_WITH_CAMELLIA_256_GCM_SHA384",
	0xC086: "TLS_ECDHE_ECDSA_WITH_CAMELLIA_128_GCM_SHA256",
	0xC087: "TLS_ECDHE_ECDSA_WITH_CAMELLIA_256_GCM_SHA384",
	0xC088: "TLS_ECDH_ECDSA_WITH_CAMELLIA_128_GCM_SHA256",
	0xC089: "TLS_ECDH_ECDSA_WITH_CAMELLIA_256_GCM_SHA384",
	0xC08A: "TLS_ECDHE_RSA_WITH_CAMELLIA_128_GCM_SHA256",
	0xC08B: "TLS_ECDHE_RSA_WITH_CAMELLIA_256_GCM_SHA384",
	0xC08C: "TLS_ECDH_RSA_WITH_CAMELLIA_128_GCM_SHA256",
	0xC08D: "TLS_ECDH_RSA_WITH_CAMELLIA_256_GCM_SHA384",
	0xC08E: "TLS_PSK_WITH_CAMELLIA_128_GCM_SHA256",
	0xC08F: "TLS_PSK_WITH_CAMELLIA_256_GCM_SHA384",
	0xC090: "TLS_DHE_PSK_WITH_CAMELLIA_128_GCM_SHA256",
	0xC091: "TLS_DHE_PSK_WITH_CAMELLIA_256_GCM_SHA384",
	0xC092: "TLS_RSA_PSK_WITH_CAMELLIA_128_GCM_SHA256",
	0xC093: "TLS_RSA_PSK_WITH_CAMELLIA_256_GCM_SHA384",
	0xC094: "TLS_PSK_WITH_CAMELLIA_128_CBC_SHA256",
	0xC095: "TLS_PSK_WITH_CAMELLIA_256_CBC_SHA384",
	0xC096: "TLS_DHE_PSK_WITH_CAMELLIA_128_CBC_SHA256",
	0xC097: "TLS_DHE_PSK_WITH_CAMELLIA_256_CBC_SHA384",
	0xC098: "TLS_RSA_PSK_WITH_CAMELLIA_128_CBC_SHA256",
	0xC099: "TLS_RSA_PSK_WITH_CAMELLIA_256_CBC_SHA384",
	0xC09A: "TLS_ECDHE_PSK_WITH_CAMELLIA_128_CBC_SHA256",
	0xC09B: "TLS_ECDHE_PSK_WITH_CAMELLIA_256_CBC_SHA384",
	0xC09C: "TLS_RSA_WITH_AES_128_CCM",
	0xC09D: "TLS_RSA_WITH_AES_256_CCM",
	0xC09E: "TLS_DHE_RSA_WITH_AES_128_CCM",
	0xC09F: "TLS_DHE_RSA_WITH_AES_256_CCM",
	0xC0A0: "TLS_RSA_WITH_AES_128_CCM_8",
	0xC0A1: "TLS_RSA_WITH_AES_256_CCM_8",
	0xC0A2: "TLS_DHE_RSA_WITH_AES_128_CCM_8",
	0xC0A3: "TLS_DHE_RSA_WITH_AES_256_CCM_8",
	0xC0A4: "TLS_PSK_WITH_AES_128_CCM",
	0xC0A5: "TLS_PSK_WITH_AES_256_CCM",
	0xC0A6: "TLS_DHE_PSK_WITH_AES_128_CCM",
	0xC0A7: "TLS_DHE_PSK_WITH_AES_256_CCM",
	0xC0A8: "TLS_PSK_WITH_AES_128_CCM_8",
	0xC0A9: "TLS_PSK_WITH_AES_256_CCM_8",
	0xC0AA: "TLS_PSK_DHE_WITH_AES_128_CCM_8",
	0xC0AB: "TLS_PSK_DHE_WITH_AES_256_CCM_8",
	0xC0AC: "TLS_ECDHE_ECDSA_WITH_AES_128_CCM",
	0xC0AD: "TLS_ECDHE_ECDSA_WITH_AES_256_CCM",
	0xC0AE: "TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8",
	0xC0AF: "TLS_ECDHE_ECDSA_WITH_AES_256_CCM_8",
	0xCCA8: "TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256",
	0xCCA9: "TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256",
	0xCCAA: "TLS_DHE_RSA_WITH_CHACHA20_POLY1305_SHA256",
	0xCCAB: "TLS_PSK_WITH_CHACHA20_POLY1305_SHA256",
	0xCCAC: "TLS_ECDHE_PSK_WITH_CHACHA20_POLY1305_SHA256",
	0xCCAD: "TLS_DHE_PSK_WITH_CHACHA20_POLY1305_SHA256",
	0xCCAE: "TLS_RSA_PSK_WITH_CHACHA20_POLY1305_SHA256",
	0xD001: "TLS_ECDHE_PSK_WITH_AES_128_GCM_SHA256",
	0xD002: "TLS_ECDHE_PSK_WITH_AES_256_GCM_SHA384",
	0xD003: "TLS_ECDHE_PSK_WITH_AES_128_CCM_8_SHA256",
	0xD005: "TLS_ECDHE_PSK_WITH_AES_128_CCM_SHA256",
}

var versionNames = map[int]string{
	tls.VersionSSL30: "SSL3.0",
	tls.VersionTLS10: "TLS1.0",
	tls.VersionTLS11: "TLS1.1",
	tls.VersionTLS12: "TLS1.2",
}

var pointNames = map[int]string{
	// csv2tsv tls-parameters-9.csv | awk '{print "\t" $1 ": \"" $2 "\","}'| egrep -v 'Unassigned|Reserved'
	0: "uncompressed",
	1: "ansiX962_compressed_prime",
	2: "ansiX962_compressed_char2",
}

var curveNames = map[int]string{
	// csv2tsv tls-parameters-8.csv | awk '{print "\t" $1 ": \"" $2 "\","}'| egrep -v 'Unassigned|Reserved'
	1:     "sect163k1",
	2:     "sect163r1",
	3:     "sect163r2",
	4:     "sect193r1",
	5:     "sect193r2",
	6:     "sect233k1",
	7:     "sect233r1",
	8:     "sect239k1",
	9:     "sect283k1",
	10:    "sect283r1",
	11:    "sect409k1",
	12:    "sect409r1",
	13:    "sect571k1",
	14:    "sect571r1",
	15:    "secp160k1",
	16:    "secp160r1",
	17:    "secp160r2",
	18:    "secp192k1",
	19:    "secp192r1",
	20:    "secp224k1",
	21:    "secp224r1",
	22:    "secp256k1",
	23:    "secp256r1",
	24:    "secp384r1",
	25:    "secp521r1",
	26:    "brainpoolP256r1",
	27:    "brainpoolP384r1",
	28:    "brainpoolP512r1",
	29:    "x25519",
	30:    "x448",
	256:   "ffdhe2048",
	257:   "ffdhe3072",
	258:   "ffdhe4096",
	259:   "ffdhe6144",
	260:   "ffdhe8192",
	65281: "arbitrary_explicit_prime_curves",
	65282: "arbitrary_explicit_char2_curves",
}

var sigNames = map[int]string{
	// https://tools.ietf.org/html/draft-ietf-tls-tls13-18#section-4.2.3
	0x0201: "rsa_pkcs1_sha1",
	0x0401: "rsa_pkcs1_sha256",
	0x0501: "rsa_pkcs1_sha384",
	0x0601: "rsa_pkcs1_sha512",

	0x0403: "ecdsa_secp256r1_sha256",
	0x0503: "ecdsa_secp384r1_sha384",
	0x0603: "ecdsa_secp521r1_sha512",

	0x0804: "rsa_pss_sha256",
	0x0805: "rsa_pss_sha384",
	0x0806: "rsa_pss_sha512",

	0x0807: "ed25519",
	0x0808: "ed448",
}
