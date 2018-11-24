// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package revdial

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"net"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/nettest"
)

func TestDialer(t *testing.T) {
	pr, pw := io.Pipe()
	var out bytes.Buffer
	d := NewDialer(bufio.NewReadWriter(
		bufio.NewReader(pr),
		bufio.NewWriter(&out),
	), ioutil.NopCloser(nil))

	c, err := d.Dial()
	if err != nil {
		t.Fatal(err)
	}
	if c.(*conn).id != 1 {
		t.Fatalf("first id = %d; want 1", c.(*conn).id)
	}
	c.Close() // to verify incoming write frames don't block

	c, err = d.Dial()
	if err != nil {
		t.Fatal(err)
	}
	if c.(*conn).id != 2 {
		t.Fatalf("second id = %d; want 2", c.(*conn).id)
	}

	if g, w := len(d.conns), 1; g != w {
		t.Errorf("size of conns map after dial+close+dial = %v; want %v", g, w)
	}

	go func() {
		// Write "b" and then "ar", and read it as "bar"
		pw.Write([]byte{byte(frameWrite), 0, 0, 0, 2, 0, 1, 'b'})
		pw.Write([]byte{byte(frameWrite), 0, 0, 0, 1, 0, 1, 'x'}) // verify doesn't block first conn
		pw.Write([]byte{byte(frameWrite), 0, 0, 0, 2, 0, 2, 'a', 'r'})
	}()
	buf := make([]byte, 3)
	if n, err := io.ReadFull(c, buf); err != nil {
		t.Fatalf("ReadFul = %v (%q), %v", n, buf[:n], err)
	}
	if string(buf) != "bar" {
		t.Fatalf("read = %q; want bar", buf)
	}
	if _, err := io.WriteString(c, "hello, world"); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	want := "N\x00\x00\x00\x01\x00\x00" +
		"C\x00\x00\x00\x01\x00\x00" +
		"N\x00\x00\x00\x02\x00\x00" +
		"W\x00\x00\x00\x02\x00\fhello, world"
	if got != want {
		t.Errorf("Written on wire differs.\nWrote: %q\n Want: %q", got, want)
	}
}

func TestListener(t *testing.T) {
	pr, pw := io.Pipe()
	var out bytes.Buffer
	ln := NewListener(bufio.NewReadWriter(
		bufio.NewReader(pr),
		bufio.NewWriter(&out),
	))
	go io.WriteString(pw, "N\x00\x00\x00\x42\x00\x00")
	c, err := ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	if g, w := c.(*conn).id, uint32(0x42); g != w {
		t.Errorf("conn id = %d; want %d", g, w)
	}
	go func() {
		io.WriteString(pw, "W\x00\x00\x00\x42\x00\x03"+"foo")
		io.WriteString(pw, "W\x00\x00\x00\x42\x00\x03"+"bar")
		io.WriteString(pw, "C\x00\x00\x00\x42\x00\x00")
	}()
	slurp, err := ioutil.ReadAll(c)
	if g, w := string(slurp), "foobar"; g != w {
		t.Errorf("Read %q; want %q", g, w)
	}
	if err != nil {
		t.Errorf("Read = %v", err)
	}
	ln.Close()
	if _, err := ln.Accept(); err == nil {
		t.Fatalf("Accept after Closed returned nil error; want error")
	}

	io.WriteString(c, "first write")
	io.WriteString(c, "second write")
	c.Close()
	got := out.String()
	want := "W\x00\x00\x00B\x00\vfirst write" +
		"W\x00\x00\x00B\x00\fsecond write" +
		"C\x00\x00\x00B\x00\x00"
	if got != want {
		t.Errorf("Wrote: %q\n Want: %q", got, want)
	}
}

func TestInterop(t *testing.T) {
	var na, nb net.Conn
	if true {
		tln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer tln.Close()
		na, err = net.Dial("tcp", tln.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		nb, err = tln.Accept()
		if err != nil {
			t.Fatal(err)
		}
	} else {
		// TODO(bradfitz): also run this way
		na, nb = net.Pipe()
	}
	defer na.Close()
	defer nb.Close()
	ln := NewListener(bufio.NewReadWriter(
		bufio.NewReader(na),
		bufio.NewWriter(na),
	))
	defer ln.Close()
	d := NewDialer(bufio.NewReadWriter(
		bufio.NewReader(nb),
		bufio.NewWriter(nb),
	), ioutil.NopCloser(nil))
	defer d.Close()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := d.Dial()
			if err != nil {
				t.Errorf("Dial: %v", err)
				return
			}
			defer c.Close()
			sc, err := ln.Accept()
			if err != nil {
				t.Errorf("Accept: %v", err)
				return
			}
			defer sc.Close()
			const cmsg = "Some client message"
			const smsg = "Some server message"
			io.WriteString(c, cmsg) // TODO(bradfitz): why the 3/500 failure rate when these are "go io.WriteString"?
			io.WriteString(sc, smsg)
			buf := make([]byte, len(cmsg))
			if n, err := io.ReadFull(c, buf); err != nil {
				t.Errorf("reading from client conn: (%d %q, %v)", n, buf[:n], err)
				return
			}
			if string(buf) != smsg {
				t.Errorf("client read %q; want %q", buf, smsg)
			}
			if _, err := io.ReadFull(sc, buf); err != nil {
				t.Errorf("reading from server conn: %v", err)
				return
			}
			if string(buf) != cmsg {
				t.Errorf("server read %q; want %q", buf, cmsg)
			}
		}()
	}
	wg.Wait()
}

// Verify that the server (e.g. the buildlet dialing the coordinator)
// going away unblocks all connections active back to it.
func TestServerEOFKillsConns(t *testing.T) {
	pr, pw := io.Pipe()
	var out bytes.Buffer
	d := NewDialer(bufio.NewReadWriter(
		bufio.NewReader(pr),
		bufio.NewWriter(&out),
	), ioutil.NopCloser(nil))

	c, err := d.Dial()
	if err != nil {
		t.Fatal(err)
	}

	readErr := make(chan error, 1)
	go func() {
		_, err := c.Read([]byte{0})
		readErr <- err
	}()
	pw.Close()

	select {
	case err := <-readErr:
		if err == nil {
			t.Fatal("got nil read error; want non-nil")
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for Read")
	}

	select {
	case <-d.Done():
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for Done channel")
	}
}

func TestConnAgainstNetTest(t *testing.T) {
	if testing.Short() {
		t.Skipf("testing in short mode")
	}
	t.Logf("warning: the revdial's SetWriteDeadline support is not complete; some tests involving write deadlines known to be flaky")
	nettest.TestConn(t, func() (c1, c2 net.Conn, stop func(), err error) {
		tln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
			return
		}
		cc, err := net.Dial("tcp", tln.Addr().String())
		if err != nil {
			t.Fatal(err)
			return
		}
		sc, err := tln.Accept()
		if err != nil {
			t.Fatal(err)
			return
		}

		rd := NewDialer(bufio.NewReadWriter(
			bufio.NewReader(sc),
			bufio.NewWriter(sc),
		), ioutil.NopCloser(nil))

		rl := NewListener(bufio.NewReadWriter(
			bufio.NewReader(cc),
			bufio.NewWriter(cc),
		))

		c1c := make(chan interface{}, 1)
		c2c := make(chan interface{}, 1)
		go func() {
			c, err := rd.Dial()
			if err != nil {
				c1c <- err
				return
			}
			c1c <- c
		}()
		go func() {
			c, err := rl.Accept()
			if err != nil {
				c2c <- err
				return
			}
			c2c <- c
		}()
		switch v := (<-c1c).(type) {
		case net.Conn:
			c1 = v
		case error:
			t.Fatalf("revdial.Dial: %v", v)
		}
		switch v := (<-c2c).(type) {
		case net.Conn:
			c2 = v
		case error:
			t.Fatalf("revdial.Accept: %v", v)
		}

		stop = func() {
			tln.Close()
			cc.Close()
			sc.Close()
		}
		return
	})
}
