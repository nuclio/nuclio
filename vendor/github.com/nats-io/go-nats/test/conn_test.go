package test

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/gnatsd/server"
	"github.com/nats-io/gnatsd/test"
	"github.com/nats-io/go-nats"
)

func TestDefaultConnection(t *testing.T) {
	s := RunDefaultServer()
	defer s.Shutdown()

	nc := NewDefaultConnection(t)
	nc.Close()
}

func TestConnectionStatus(t *testing.T) {
	s := RunDefaultServer()
	defer s.Shutdown()

	nc := NewDefaultConnection(t)
	defer nc.Close()

	if nc.Status() != nats.CONNECTED {
		t.Fatal("Should have status set to CONNECTED")
	}
	if !nc.IsConnected() {
		t.Fatal("Should have status set to CONNECTED")
	}
	nc.Close()
	if nc.Status() != nats.CLOSED {
		t.Fatal("Should have status set to CLOSED")
	}
	if !nc.IsClosed() {
		t.Fatal("Should have status set to CLOSED")
	}
}

func TestConnClosedCB(t *testing.T) {
	s := RunDefaultServer()
	defer s.Shutdown()

	ch := make(chan bool)
	o := nats.GetDefaultOptions()
	o.Url = nats.DefaultURL
	o.ClosedCB = func(_ *nats.Conn) {
		ch <- true
	}
	nc, err := o.Connect()
	if err != nil {
		t.Fatalf("Should have connected ok: %v", err)
	}
	nc.Close()
	if e := Wait(ch); e != nil {
		t.Fatalf("Closed callback not triggered\n")
	}
}

func TestCloseDisconnectedCB(t *testing.T) {
	s := RunDefaultServer()
	defer s.Shutdown()

	ch := make(chan bool)
	o := nats.GetDefaultOptions()
	o.Url = nats.DefaultURL
	o.AllowReconnect = false
	o.DisconnectedCB = func(_ *nats.Conn) {
		ch <- true
	}
	nc, err := o.Connect()
	if err != nil {
		t.Fatalf("Should have connected ok: %v", err)
	}
	nc.Close()
	if e := Wait(ch); e != nil {
		t.Fatal("Disconnected callback not triggered")
	}
}

func TestServerStopDisconnectedCB(t *testing.T) {
	s := RunDefaultServer()
	defer s.Shutdown()

	ch := make(chan bool)
	o := nats.GetDefaultOptions()
	o.Url = nats.DefaultURL
	o.AllowReconnect = false
	o.DisconnectedCB = func(nc *nats.Conn) {
		ch <- true
	}
	nc, err := o.Connect()
	if err != nil {
		t.Fatalf("Should have connected ok: %v", err)
	}
	defer nc.Close()

	s.Shutdown()
	if e := Wait(ch); e != nil {
		t.Fatalf("Disconnected callback not triggered\n")
	}
}

func TestServerSecureConnections(t *testing.T) {
	s, opts := RunServerWithConfig("./configs/tls.conf")
	defer s.Shutdown()

	endpoint := fmt.Sprintf("%s:%d", opts.Host, opts.Port)
	secureURL := fmt.Sprintf("nats://%s:%s@%s/", opts.Username, opts.Password, endpoint)

	// Make sure this succeeds
	nc, err := nats.Connect(secureURL, nats.Secure())
	if err != nil {
		t.Fatalf("Failed to create secure (TLS) connection: %v", err)
	}
	defer nc.Close()

	omsg := []byte("Hello World")
	checkRecv := make(chan bool)

	received := 0
	nc.Subscribe("foo", func(m *nats.Msg) {
		received++
		if !bytes.Equal(m.Data, omsg) {
			t.Fatal("Message received does not match")
		}
		checkRecv <- true
	})
	err = nc.Publish("foo", omsg)
	if err != nil {
		t.Fatalf("Failed to publish on secure (TLS) connection: %v", err)
	}
	nc.Flush()

	if err := Wait(checkRecv); err != nil {
		t.Fatal("Failed receiving message")
	}

	nc.Close()

	// Server required, but not requested.
	nc, err = nats.Connect(secureURL)
	if err == nil || nc != nil || err != nats.ErrSecureConnRequired {
		if nc != nil {
			nc.Close()
		}
		t.Fatal("Should have failed to create secure (TLS) connection")
	}

	// Test flag mismatch
	// Wanted but not available..
	ds := RunDefaultServer()
	defer ds.Shutdown()

	nc, err = nats.Connect(nats.DefaultURL, nats.Secure())
	if err == nil || nc != nil || err != nats.ErrSecureConnWanted {
		if nc != nil {
			nc.Close()
		}
		t.Fatalf("Should have failed to create connection: %v", err)
	}

	// Let's be more TLS correct and verify servername, endpoint etc.
	// Now do more advanced checking, verifying servername and using rootCA.
	// Setup our own TLSConfig using RootCA from our self signed cert.
	rootPEM, err := ioutil.ReadFile("./configs/certs/ca.pem")
	if err != nil || rootPEM == nil {
		t.Fatalf("failed to read root certificate")
	}
	pool := x509.NewCertPool()
	ok := pool.AppendCertsFromPEM([]byte(rootPEM))
	if !ok {
		t.Fatal("failed to parse root certificate")
	}

	tls1 := &tls.Config{
		ServerName: opts.Host,
		RootCAs:    pool,
		MinVersion: tls.VersionTLS12,
	}

	nc, err = nats.Connect(secureURL, nats.Secure(tls1))
	if err != nil {
		t.Fatalf("Got an error on Connect with Secure Options: %+v\n", err)
	}
	defer nc.Close()

	tls2 := &tls.Config{
		ServerName: "OtherHostName",
		RootCAs:    pool,
		MinVersion: tls.VersionTLS12,
	}

	nc2, err := nats.Connect(secureURL, nats.Secure(tls1, tls2))
	if err == nil {
		nc2.Close()
		t.Fatal("Was expecting an error!")
	}
}

func TestClientCertificate(t *testing.T) {

	s, opts := RunServerWithConfig("./configs/tlsverify.conf")
	defer s.Shutdown()

	endpoint := fmt.Sprintf("%s:%d", opts.Host, opts.Port)
	secureURL := fmt.Sprintf("nats://%s", endpoint)

	// Make sure this fails
	nc, err := nats.Connect(secureURL, nats.Secure())
	if err == nil {
		nc.Close()
		t.Fatal("Should have failed (TLS) connection without client certificate")
	}

	// Check parameters validity
	nc, err = nats.Connect(secureURL, nats.ClientCert("", ""))
	if err == nil {
		nc.Close()
		t.Fatal("Should have failed due to invalid parameters")
	}

	// Should fail because wrong key
	nc, err = nats.Connect(secureURL,
		nats.ClientCert("./configs/certs/client-cert.pem", "./configs/certs/key.pem"))
	if err == nil {
		nc.Close()
		t.Fatal("Should have failed due to invalid key")
	}

	// Should fail because no CA
	nc, err = nats.Connect(secureURL,
		nats.ClientCert("./configs/certs/client-cert.pem", "./configs/certs/client-key.pem"))
	if err == nil {
		nc.Close()
		t.Fatal("Should have failed due to missing ca")
	}

	nc, err = nats.Connect(secureURL,
		nats.RootCAs("./configs/certs/ca.pem"),
		nats.ClientCert("./configs/certs/client-cert.pem", "./configs/certs/client-key.pem"))
	if err != nil {
		t.Fatalf("Failed to create (TLS) connection: %v", err)
	}
	defer nc.Close()

	omsg := []byte("Hello!")
	checkRecv := make(chan bool)

	received := 0
	nc.Subscribe("foo", func(m *nats.Msg) {
		received++
		if !bytes.Equal(m.Data, omsg) {
			t.Fatal("Message received does not match")
		}
		checkRecv <- true
	})
	err = nc.Publish("foo", omsg)
	if err != nil {
		t.Fatalf("Failed to publish on secure (TLS) connection: %v", err)
	}
	nc.Flush()

	if err := Wait(checkRecv); err != nil {
		t.Fatal("Failed to receive message")
	}
}

func TestServerTLSHintConnections(t *testing.T) {
	s, opts := RunServerWithConfig("./configs/tls.conf")
	defer s.Shutdown()

	endpoint := fmt.Sprintf("%s:%d", opts.Host, opts.Port)
	secureURL := fmt.Sprintf("tls://%s:%s@%s/", opts.Username, opts.Password, endpoint)

	nc, err := nats.Connect(secureURL, nats.RootCAs("./configs/certs/badca.pem"))
	if err == nil {
		nc.Close()
		t.Fatal("Expected an error from bad RootCA file")
	}

	nc, err = nats.Connect(secureURL, nats.RootCAs("./configs/certs/ca.pem"))
	if err != nil {
		t.Fatalf("Failed to create secure (TLS) connection: %v", err)
	}
	defer nc.Close()
}

func TestClosedConnections(t *testing.T) {
	s := RunDefaultServer()
	defer s.Shutdown()

	nc := NewDefaultConnection(t)
	defer nc.Close()

	sub, _ := nc.SubscribeSync("foo")
	if sub == nil {
		t.Fatal("Failed to create valid subscription")
	}

	// Test all API endpoints do the right thing with a closed connection.
	nc.Close()
	if err := nc.Publish("foo", nil); err != nats.ErrConnectionClosed {
		t.Fatalf("Publish on closed conn did not fail properly: %v\n", err)
	}
	if err := nc.PublishMsg(&nats.Msg{Subject: "foo"}); err != nats.ErrConnectionClosed {
		t.Fatalf("PublishMsg on closed conn did not fail properly: %v\n", err)
	}
	if err := nc.Flush(); err != nats.ErrConnectionClosed {
		t.Fatalf("Flush on closed conn did not fail properly: %v\n", err)
	}
	_, err := nc.Subscribe("foo", nil)
	if err != nats.ErrConnectionClosed {
		t.Fatalf("Subscribe on closed conn did not fail properly: %v\n", err)
	}
	_, err = nc.SubscribeSync("foo")
	if err != nats.ErrConnectionClosed {
		t.Fatalf("SubscribeSync on closed conn did not fail properly: %v\n", err)
	}
	_, err = nc.QueueSubscribe("foo", "bar", nil)
	if err != nats.ErrConnectionClosed {
		t.Fatalf("QueueSubscribe on closed conn did not fail properly: %v\n", err)
	}
	_, err = nc.Request("foo", []byte("help"), 10*time.Millisecond)
	if err != nats.ErrConnectionClosed {
		t.Fatalf("Request on closed conn did not fail properly: %v\n", err)
	}
	if _, err = sub.NextMsg(10); err != nats.ErrConnectionClosed {
		t.Fatalf("NextMessage on closed conn did not fail properly: %v\n", err)
	}
	if err = sub.Unsubscribe(); err != nats.ErrConnectionClosed {
		t.Fatalf("Unsubscribe on closed conn did not fail properly: %v\n", err)
	}
}

func TestErrOnConnectAndDeadlock(t *testing.T) {
	// We will hand run a fake server that will timeout and not return a proper
	// INFO proto. This is to test that we do not deadlock. Issue #18

	l, e := net.Listen("tcp", ":0")
	if e != nil {
		t.Fatal("Could not listen on an ephemeral port")
	}
	tl := l.(*net.TCPListener)
	defer tl.Close()

	addr := tl.Addr().(*net.TCPAddr)

	go func() {
		conn, err := l.Accept()
		if err != nil {
			t.Fatalf("Error accepting client connection: %v\n", err)
		}
		defer conn.Close()
		// Send back a mal-formed INFO.
		conn.Write([]byte("INFOZ \r\n"))
	}()

	// Used to synchronize
	ch := make(chan bool)

	go func() {
		natsURL := fmt.Sprintf("nats://localhost:%d/", addr.Port)
		nc, err := nats.Connect(natsURL)
		if err == nil {
			nc.Close()
			t.Fatal("Expected bad INFO err, got none")
		}
		ch <- true
	}()

	// Setup a timer to watch for deadlock
	select {
	case <-ch:
		break
	case <-time.After(time.Second):
		t.Fatalf("Connect took too long, deadlock?")
	}
}

func TestMoreErrOnConnect(t *testing.T) {
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal("Could not listen on an ephemeral port")
	}
	tl := l.(*net.TCPListener)
	defer tl.Close()

	addr := tl.Addr().(*net.TCPAddr)

	done := make(chan bool)
	case1 := make(chan bool)
	case2 := make(chan bool)
	case3 := make(chan bool)
	case4 := make(chan bool)

	go func() {
		for i := 0; i < 5; i++ {
			conn, err := l.Accept()
			if err != nil {
				t.Fatalf("Error accepting client connection: %v\n", err)
			}
			switch i {
			case 0:
				// Send back a partial INFO and close the connection.
				conn.Write([]byte("INFO"))
			case 1:
				// Send just INFO
				conn.Write([]byte("INFO\r\n"))
				// Stick around a bit
				<-case1
			case 2:
				info := fmt.Sprintf("INFO {\"server_id\":\"foobar\",\"version\":\"0.7.3\",\"go\":\"go1.5.1\",\"host\":\"%s\",\"port\":%d,\"auth_required\":false,\"ssl_required\":false,\"max_payload\":1048576}\r\n", addr.IP, addr.Port)
				// Send complete INFO
				conn.Write([]byte(info))
				// Read connect and ping commands sent from the client
				br := bufio.NewReaderSize(conn, 1024)
				if _, err := br.ReadString('\n'); err != nil {
					t.Fatalf("Expected CONNECT from client, got: %s", err)
				}
				if _, err := br.ReadString('\n'); err != nil {
					t.Fatalf("Expected PING from client, got: %s", err)
				}
				// Client expect +OK, send it but then something else than PONG
				conn.Write([]byte("+OK\r\n"))
				// Stick around a bit
				<-case2
			case 3:
				info := fmt.Sprintf("INFO {\"server_id\":\"foobar\",\"version\":\"0.7.3\",\"go\":\"go1.5.1\",\"host\":\"%s\",\"port\":%d,\"auth_required\":false,\"ssl_required\":false,\"max_payload\":1048576}\r\n", addr.IP, addr.Port)
				// Send complete INFO
				conn.Write([]byte(info))
				// Read connect and ping commands sent from the client
				br := bufio.NewReaderSize(conn, 1024)
				if _, err := br.ReadString('\n'); err != nil {
					t.Fatalf("Expected CONNECT from client, got: %s", err)
				}
				if _, err := br.ReadString('\n'); err != nil {
					t.Fatalf("Expected PING from client, got: %s", err)
				}
				// Client expect +OK, send it but then something else than PONG
				conn.Write([]byte("+OK\r\nXXX\r\n"))
				// Stick around a bit
				<-case3
			case 4:
				info := fmt.Sprintf("INFO {'x'}\r\n")
				// Send INFO with JSON marshall error
				conn.Write([]byte(info))
				// Stick around a bit
				<-case4
			}

			conn.Close()
		}

		// Hang around until asked to quit
		<-done
	}()

	natsURL := fmt.Sprintf("nats://localhost:%d", addr.Port)

	if nc, err := nats.Connect(natsURL, nats.Timeout(20*time.Millisecond)); err == nil {
		nc.Close()
		t.Fatal("Expected error, got none")
	}

	if nc, err := nats.Connect(natsURL, nats.Timeout(20*time.Millisecond)); err == nil {
		close(case1)
		nc.Close()
		t.Fatal("Expected error, got none")
	}

	close(case1)

	opts := nats.GetDefaultOptions()
	opts.Servers = []string{natsURL}
	opts.Timeout = 20 * time.Millisecond
	opts.Verbose = true

	if nc, err := opts.Connect(); err == nil {
		close(case2)
		nc.Close()
		t.Fatal("Expected error, got none")
	}

	close(case2)

	if nc, err := opts.Connect(); err == nil {
		close(case3)
		nc.Close()
		t.Fatal("Expected error, got none")
	}

	close(case3)

	if nc, err := opts.Connect(); err == nil {
		close(case4)
		nc.Close()
		t.Fatal("Expected error, got none")
	}

	close(case4)

	close(done)
}

func TestErrOnMaxPayloadLimit(t *testing.T) {
	expectedMaxPayload := int64(10)
	serverInfo := "INFO {\"server_id\":\"foobar\",\"version\":\"0.6.6\",\"go\":\"go1.5.1\",\"host\":\"%s\",\"port\":%d,\"auth_required\":false,\"ssl_required\":false,\"max_payload\":%d}\r\n"

	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal("Could not listen on an ephemeral port")
	}
	tl := l.(*net.TCPListener)
	defer tl.Close()

	addr := tl.Addr().(*net.TCPAddr)

	// Send back an INFO message with custom max payload size on connect.
	var conn net.Conn
	var err error

	go func() {
		conn, err = l.Accept()
		if err != nil {
			t.Fatalf("Error accepting client connection: %v\n", err)
		}
		defer conn.Close()
		info := fmt.Sprintf(serverInfo, addr.IP, addr.Port, expectedMaxPayload)
		conn.Write([]byte(info))

		// Read connect and ping commands sent from the client
		line := make([]byte, 111)
		_, err := conn.Read(line)
		if err != nil {
			t.Fatalf("Expected CONNECT and PING from client, got: %s", err)
		}
		conn.Write([]byte("PONG\r\n"))
		// Hang around a bit to not err on EOF in client.
		time.Sleep(250 * time.Millisecond)
	}()

	// Wait for server mock to start
	time.Sleep(100 * time.Millisecond)

	natsURL := fmt.Sprintf("nats://%s:%d", addr.IP, addr.Port)
	opts := nats.GetDefaultOptions()
	opts.Servers = []string{natsURL}
	nc, err := opts.Connect()
	if err != nil {
		t.Fatalf("Expected INFO message with custom max payload, got: %s", err)
	}
	defer nc.Close()

	got := nc.MaxPayload()
	if got != expectedMaxPayload {
		t.Fatalf("Expected MaxPayload to be %d, got: %d", expectedMaxPayload, got)
	}
	err = nc.Publish("hello", []byte("hello world"))
	if err != nats.ErrMaxPayload {
		t.Fatalf("Expected to fail trying to send more than max payload, got: %s", err)
	}
	err = nc.Publish("hello", []byte("a"))
	if err != nil {
		t.Fatalf("Expected to succeed trying to send less than max payload, got: %s", err)
	}
}

func TestConnectVerbose(t *testing.T) {
	s := RunDefaultServer()
	defer s.Shutdown()

	o := nats.GetDefaultOptions()
	o.Verbose = true

	nc, err := o.Connect()
	if err != nil {
		t.Fatalf("Should have connected ok: %v", err)
	}
	nc.Close()
}

func isRunningInAsyncCBDispatcher() error {
	var stacks []byte

	stacksSize := 10000

	for {
		stacks = make([]byte, stacksSize)
		n := runtime.Stack(stacks, false)
		if n == stacksSize {
			stacksSize *= stacksSize
			continue
		}
		break
	}

	strStacks := string(stacks)

	if strings.Contains(strStacks, "asyncDispatch") {
		return nil
	}

	return fmt.Errorf("callback not executed from dispatcher:\n %s", strStacks)
}

func TestCallbacksOrder(t *testing.T) {
	authS, authSOpts := RunServerWithConfig("./configs/tls.conf")
	defer authS.Shutdown()

	s := RunDefaultServer()
	defer s.Shutdown()

	firstDisconnect := true
	dtime1 := time.Time{}
	dtime2 := time.Time{}
	rtime := time.Time{}
	atime1 := time.Time{}
	atime2 := time.Time{}
	ctime := time.Time{}

	cbErrors := make(chan error, 20)

	reconnected := make(chan bool)
	closed := make(chan bool)
	asyncErr := make(chan bool, 2)
	recvCh := make(chan bool, 2)
	recvCh1 := make(chan bool)
	recvCh2 := make(chan bool)

	dch := func(nc *nats.Conn) {
		if err := isRunningInAsyncCBDispatcher(); err != nil {
			cbErrors <- err
			return
		}
		time.Sleep(100 * time.Millisecond)
		if firstDisconnect {
			firstDisconnect = false
			dtime1 = time.Now()
		} else {
			dtime2 = time.Now()
		}
	}

	rch := func(nc *nats.Conn) {
		if err := isRunningInAsyncCBDispatcher(); err != nil {
			cbErrors <- err
			reconnected <- true
			return
		}
		time.Sleep(50 * time.Millisecond)
		rtime = time.Now()
		reconnected <- true
	}

	ech := func(nc *nats.Conn, sub *nats.Subscription, err error) {
		if err := isRunningInAsyncCBDispatcher(); err != nil {
			cbErrors <- err
			asyncErr <- true
			return
		}
		if sub.Subject == "foo" {
			time.Sleep(20 * time.Millisecond)
			atime1 = time.Now()
		} else {
			atime2 = time.Now()
		}
		asyncErr <- true
	}

	cch := func(nc *nats.Conn) {
		if err := isRunningInAsyncCBDispatcher(); err != nil {
			cbErrors <- err
			closed <- true
			return
		}
		ctime = time.Now()
		closed <- true
	}

	url := net.JoinHostPort(authSOpts.Host, strconv.Itoa(authSOpts.Port))
	url = "nats://" + url + "," + nats.DefaultURL

	nc, err := nats.Connect(url,
		nats.DisconnectHandler(dch),
		nats.ReconnectHandler(rch),
		nats.ClosedHandler(cch),
		nats.ErrorHandler(ech),
		nats.ReconnectWait(50*time.Millisecond),
		nats.DontRandomize())
	if err != nil {
		t.Fatalf("Unable to connect: %v\n", err)
	}
	defer nc.Close()

	ncp, err := nats.Connect(nats.DefaultURL,
		nats.ReconnectWait(50*time.Millisecond))
	if err != nil {
		t.Fatalf("Unable to connect: %v\n", err)
	}
	defer ncp.Close()

	// Wait to make sure that if we have closed (incorrectly) the
	// asyncCBDispatcher during the connect process, this is caught here.
	time.Sleep(time.Second)

	s.Shutdown()

	s = RunDefaultServer()
	defer s.Shutdown()

	if err := Wait(reconnected); err != nil {
		t.Fatal("Did not get the reconnected callback")
	}

	var sub1 *nats.Subscription
	var sub2 *nats.Subscription

	recv := func(m *nats.Msg) {
		// Signal that one message is received
		recvCh <- true

		// We will now block
		if m.Subject == "foo" {
			<-recvCh1
		} else {
			<-recvCh2
		}
		m.Sub.Unsubscribe()
	}

	sub1, err = nc.Subscribe("foo", recv)
	if err != nil {
		t.Fatalf("Unable to create subscription: %v\n", err)
	}
	sub1.SetPendingLimits(1, 100000)

	sub2, err = nc.Subscribe("bar", recv)
	if err != nil {
		t.Fatalf("Unable to create subscription: %v\n", err)
	}
	sub2.SetPendingLimits(1, 100000)

	nc.Flush()

	ncp.Publish("foo", []byte("test"))
	ncp.Publish("bar", []byte("test"))
	ncp.Flush()

	// Wait notification that message were received
	err = Wait(recvCh)
	if err == nil {
		err = Wait(recvCh)
	}
	if err != nil {
		t.Fatal("Did not receive message")
	}

	for i := 0; i < 2; i++ {
		ncp.Publish("foo", []byte("test"))
		ncp.Publish("bar", []byte("test"))
	}
	ncp.Flush()

	if err := Wait(asyncErr); err != nil {
		t.Fatal("Did not get the async callback")
	}
	if err := Wait(asyncErr); err != nil {
		t.Fatal("Did not get the async callback")
	}

	close(recvCh1)
	close(recvCh2)

	nc.Close()

	if err := Wait(closed); err != nil {
		t.Fatal("Did not get the close callback")
	}

	if len(cbErrors) > 0 {
		t.Fatalf("%v", <-cbErrors)
	}

	if (dtime1 == time.Time{}) || (dtime2 == time.Time{}) || (rtime == time.Time{}) || (atime1 == time.Time{}) || (atime2 == time.Time{}) || (ctime == time.Time{}) {
		t.Fatalf("Some callbacks did not fire:\n%v\n%v\n%v\n%v\n%v\n%v", dtime1, rtime, atime1, atime2, dtime2, ctime)
	}

	if rtime.Before(dtime1) || dtime2.Before(rtime) || atime2.Before(atime1) || ctime.Before(atime2) {
		t.Fatalf("Wrong callback order:\n%v\n%v\n%v\n%v\n%v\n%v", dtime1, rtime, atime1, atime2, dtime2, ctime)
	}
}

func TestFlushReleaseOnClose(t *testing.T) {
	serverInfo := "INFO {\"server_id\":\"foobar\",\"version\":\"0.7.3\",\"go\":\"go1.5.1\",\"host\":\"%s\",\"port\":%d,\"auth_required\":false,\"ssl_required\":false,\"max_payload\":1048576}\r\n"

	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal("Could not listen on an ephemeral port")
	}
	tl := l.(*net.TCPListener)
	defer tl.Close()

	addr := tl.Addr().(*net.TCPAddr)
	done := make(chan bool)

	go func() {
		conn, err := l.Accept()
		if err != nil {
			t.Fatalf("Error accepting client connection: %v\n", err)
		}
		defer conn.Close()
		info := fmt.Sprintf(serverInfo, addr.IP, addr.Port)
		conn.Write([]byte(info))

		// Read connect and ping commands sent from the client
		br := bufio.NewReaderSize(conn, 1024)
		if _, err := br.ReadString('\n'); err != nil {
			t.Fatalf("Expected CONNECT from client, got: %s", err)
		}
		if _, err := br.ReadString('\n'); err != nil {
			t.Fatalf("Expected PING from client, got: %s", err)
		}
		conn.Write([]byte("PONG\r\n"))

		// Hang around until asked to quit
		<-done
	}()

	// Wait for server mock to start
	time.Sleep(100 * time.Millisecond)

	natsURL := fmt.Sprintf("nats://%s:%d", addr.IP, addr.Port)
	opts := nats.GetDefaultOptions()
	opts.AllowReconnect = false
	opts.Servers = []string{natsURL}
	nc, err := opts.Connect()
	if err != nil {
		t.Fatalf("Expected INFO message with custom max payload, got: %s", err)
	}
	defer nc.Close()

	// First try a FlushTimeout() and make sure we timeout
	if err := nc.FlushTimeout(50 * time.Millisecond); err == nil || err != nats.ErrTimeout {
		t.Fatalf("Expected a timeout error, got: %v", err)
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		nc.Close()
	}()

	if err := nc.Flush(); err == nil {
		t.Fatal("Expected error on Flush() released by Close()")
	}

	close(done)
}

func TestMaxPendingOut(t *testing.T) {
	serverInfo := "INFO {\"server_id\":\"foobar\",\"version\":\"0.7.3\",\"go\":\"go1.5.1\",\"host\":\"%s\",\"port\":%d,\"auth_required\":false,\"ssl_required\":false,\"max_payload\":1048576}\r\n"

	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal("Could not listen on an ephemeral port")
	}
	tl := l.(*net.TCPListener)
	defer tl.Close()

	addr := tl.Addr().(*net.TCPAddr)
	done := make(chan bool)
	cch := make(chan bool)

	go func() {
		conn, err := l.Accept()
		if err != nil {
			t.Fatalf("Error accepting client connection: %v\n", err)
		}
		defer conn.Close()
		info := fmt.Sprintf(serverInfo, addr.IP, addr.Port)
		conn.Write([]byte(info))

		// Read connect and ping commands sent from the client
		br := bufio.NewReaderSize(conn, 1024)
		if _, err := br.ReadString('\n'); err != nil {
			t.Fatalf("Expected CONNECT from client, got: %s", err)
		}
		if _, err := br.ReadString('\n'); err != nil {
			t.Fatalf("Expected PING from client, got: %s", err)
		}
		conn.Write([]byte("PONG\r\n"))

		// Hang around until asked to quit
		<-done
	}()

	// Wait for server mock to start
	time.Sleep(100 * time.Millisecond)

	natsURL := fmt.Sprintf("nats://%s:%d", addr.IP, addr.Port)
	opts := nats.GetDefaultOptions()
	opts.PingInterval = 20 * time.Millisecond
	opts.MaxPingsOut = 2
	opts.AllowReconnect = false
	opts.ClosedCB = func(_ *nats.Conn) { cch <- true }
	opts.Servers = []string{natsURL}
	nc, err := opts.Connect()
	if err != nil {
		t.Fatalf("Expected INFO message with custom max payload, got: %s", err)
	}
	defer nc.Close()

	// After 60 ms, we should have closed the connection
	time.Sleep(100 * time.Millisecond)

	if err := Wait(cch); err != nil {
		t.Fatal("Failed to get ClosedCB")
	}
	if nc.LastError() != nats.ErrStaleConnection {
		t.Fatalf("Expected to get %v, got %v", nats.ErrStaleConnection, nc.LastError())
	}

	close(done)
}

func TestErrInReadLoop(t *testing.T) {
	serverInfo := "INFO {\"server_id\":\"foobar\",\"version\":\"0.7.3\",\"go\":\"go1.5.1\",\"host\":\"%s\",\"port\":%d,\"auth_required\":false,\"ssl_required\":false,\"max_payload\":1048576}\r\n"

	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal("Could not listen on an ephemeral port")
	}
	tl := l.(*net.TCPListener)
	defer tl.Close()

	addr := tl.Addr().(*net.TCPAddr)
	done := make(chan bool)
	cch := make(chan bool)

	go func() {
		conn, err := l.Accept()
		if err != nil {
			t.Fatalf("Error accepting client connection: %v\n", err)
		}
		defer conn.Close()
		info := fmt.Sprintf(serverInfo, addr.IP, addr.Port)
		conn.Write([]byte(info))

		// Read connect and ping commands sent from the client
		br := bufio.NewReaderSize(conn, 1024)
		if _, err := br.ReadString('\n'); err != nil {
			t.Fatalf("Expected CONNECT from client, got: %s", err)
		}
		if _, err := br.ReadString('\n'); err != nil {
			t.Fatalf("Expected PING from client, got: %s", err)
		}
		conn.Write([]byte("PONG\r\n"))

		// Read (and ignore) the SUB from the client
		if _, err := br.ReadString('\n'); err != nil {
			t.Fatalf("Expected SUB from client, got: %s", err)
		}

		// Send something that should make the subscriber fail.
		conn.Write([]byte("Ivan"))

		// Hang around until asked to quit
		<-done
	}()

	// Wait for server mock to start
	time.Sleep(100 * time.Millisecond)

	natsURL := fmt.Sprintf("nats://%s:%d", addr.IP, addr.Port)
	opts := nats.GetDefaultOptions()
	opts.AllowReconnect = false
	opts.ClosedCB = func(_ *nats.Conn) { cch <- true }
	opts.Servers = []string{natsURL}
	nc, err := opts.Connect()
	if err != nil {
		t.Fatalf("Expected INFO message with custom max payload, got: %s", err)
	}
	defer nc.Close()

	received := int64(0)

	nc.Subscribe("foo", func(_ *nats.Msg) {
		atomic.AddInt64(&received, 1)
	})

	if err := Wait(cch); err != nil {
		t.Fatal("Failed to get ClosedCB")
	}

	recv := int(atomic.LoadInt64(&received))
	if recv != 0 {
		t.Fatalf("Should not have received messages, got: %d", recv)
	}

	close(done)
}

func TestErrStaleConnection(t *testing.T) {
	serverInfo := "INFO {\"server_id\":\"foobar\",\"version\":\"0.7.3\",\"go\":\"go1.5.1\",\"host\":\"%s\",\"port\":%d,\"auth_required\":false,\"ssl_required\":false,\"max_payload\":1048576}\r\n"

	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal("Could not listen on an ephemeral port")
	}
	tl := l.(*net.TCPListener)
	defer tl.Close()

	addr := tl.Addr().(*net.TCPAddr)
	done := make(chan bool)
	dch := make(chan bool)
	rch := make(chan bool)
	cch := make(chan bool)
	sch := make(chan bool)

	firstDisconnect := true

	go func() {
		for i := 0; i < 2; i++ {
			conn, err := l.Accept()
			if err != nil {
				t.Fatalf("Error accepting client connection: %v\n", err)
			}
			defer conn.Close()
			info := fmt.Sprintf(serverInfo, addr.IP, addr.Port)
			conn.Write([]byte(info))

			// Read connect and ping commands sent from the client
			br := bufio.NewReaderSize(conn, 1024)
			if _, err := br.ReadString('\n'); err != nil {
				t.Fatalf("Expected CONNECT from client, got: %s", err)
			}
			if _, err := br.ReadString('\n'); err != nil {
				t.Fatalf("Expected PING from client, got: %s", err)
			}
			conn.Write([]byte("PONG\r\n"))

			if i == 0 {
				// Wait a tiny, and simulate a Stale Connection
				time.Sleep(50 * time.Millisecond)
				conn.Write([]byte("-ERR 'Stale Connection'\r\n"))

				// The client should try to reconnect. When getting the
				// disconnected callback, it will close this channel.
				<-sch

				// Close the connection and go back to accept the new
				// connection.
				conn.Close()
			} else {
				// Hang around a bit
				<-done
			}
		}
	}()

	// Wait for server mock to start
	time.Sleep(100 * time.Millisecond)

	natsURL := fmt.Sprintf("nats://%s:%d", addr.IP, addr.Port)
	opts := nats.GetDefaultOptions()
	opts.AllowReconnect = true
	opts.DisconnectedCB = func(_ *nats.Conn) {
		// Interested only in the first disconnect cb
		if firstDisconnect {
			firstDisconnect = false
			close(sch)
			dch <- true
		}
	}
	opts.ReconnectedCB = func(_ *nats.Conn) { rch <- true }
	opts.ClosedCB = func(_ *nats.Conn) { cch <- true }
	opts.ReconnectWait = 20 * time.Millisecond
	opts.MaxReconnect = 100
	opts.Servers = []string{natsURL}
	nc, err := opts.Connect()
	if err != nil {
		t.Fatalf("Expected INFO message with custom max payload, got: %s", err)
	}
	defer nc.Close()

	// We should first gets disconnected
	if err := Wait(dch); err != nil {
		t.Fatal("Failed to get DisconnectedCB")
	}

	// Then reconneted..
	if err := Wait(rch); err != nil {
		t.Fatal("Failed to get ReconnectedCB")
	}

	// Now close the connection
	nc.Close()

	// We should get the closed cb
	if err := Wait(cch); err != nil {
		t.Fatal("Failed to get ClosedCB")
	}

	close(done)
}

func TestServerErrorClosesConnection(t *testing.T) {
	serverInfo := "INFO {\"server_id\":\"foobar\",\"version\":\"0.7.3\",\"go\":\"go1.5.1\",\"host\":\"%s\",\"port\":%d,\"auth_required\":false,\"ssl_required\":false,\"max_payload\":1048576}\r\n"

	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal("Could not listen on an ephemeral port")
	}
	tl := l.(*net.TCPListener)
	defer tl.Close()

	addr := tl.Addr().(*net.TCPAddr)
	done := make(chan bool)
	dch := make(chan bool)
	cch := make(chan bool)

	serverSentError := "Any Error"
	reconnected := int64(0)

	go func() {
		conn, err := l.Accept()
		if err != nil {
			t.Fatalf("Error accepting client connection: %v\n", err)
		}
		defer conn.Close()
		info := fmt.Sprintf(serverInfo, addr.IP, addr.Port)
		conn.Write([]byte(info))

		// Read connect and ping commands sent from the client
		br := bufio.NewReaderSize(conn, 1024)
		if _, err := br.ReadString('\n'); err != nil {
			t.Fatalf("Expected CONNECT from client, got: %s", err)
		}
		if _, err := br.ReadString('\n'); err != nil {
			t.Fatalf("Expected PING from client, got: %s", err)
		}
		conn.Write([]byte("PONG\r\n"))

		// Wait a tiny, and simulate a Stale Connection
		time.Sleep(50 * time.Millisecond)
		conn.Write([]byte("-ERR '" + serverSentError + "'\r\n"))

		// Hang around a bit
		<-done
	}()

	// Wait for server mock to start
	time.Sleep(100 * time.Millisecond)

	natsURL := fmt.Sprintf("nats://%s:%d", addr.IP, addr.Port)
	opts := nats.GetDefaultOptions()
	opts.AllowReconnect = true
	opts.DisconnectedCB = func(_ *nats.Conn) { dch <- true }
	opts.ReconnectedCB = func(_ *nats.Conn) { atomic.AddInt64(&reconnected, 1) }
	opts.ClosedCB = func(_ *nats.Conn) { cch <- true }
	opts.ReconnectWait = 20 * time.Millisecond
	opts.MaxReconnect = 100
	opts.Servers = []string{natsURL}
	nc, err := opts.Connect()
	if err != nil {
		t.Fatalf("Expected INFO message with custom max payload, got: %s", err)
	}
	defer nc.Close()

	// The server sends an error that should cause the client to simply close
	// the connection.

	// We should first gets disconnected
	if err := Wait(dch); err != nil {
		t.Fatal("Failed to get DisconnectedCB")
	}

	// We should get the closed cb
	if err := Wait(cch); err != nil {
		t.Fatal("Failed to get ClosedCB")
	}

	// We should not have been reconnected
	if atomic.LoadInt64(&reconnected) != 0 {
		t.Fatal("ReconnectedCB should not have been invoked")
	}

	// Check LastError(), it should be "nats: <server error in lower case>"
	lastErr := nc.LastError().Error()
	expectedErr := "nats: " + strings.ToLower(serverSentError)
	if lastErr != expectedErr {
		t.Fatalf("Expected error: '%v', got '%v'", expectedErr, lastErr)
	}

	close(done)
}

func TestUseDefaultTimeout(t *testing.T) {
	s := RunDefaultServer()
	defer s.Shutdown()

	opts := &nats.Options{
		Servers: []string{nats.DefaultURL},
	}
	nc, err := opts.Connect()
	if err != nil {
		t.Fatalf("Unexpected error on connect: %v", err)
	}
	defer nc.Close()
	if nc.Opts.Timeout != nats.DefaultTimeout {
		t.Fatalf("Expected Timeout to be set to %v, got %v", nats.DefaultTimeout, nc.Opts.Timeout)
	}
}

func TestNoRaceOnLastError(t *testing.T) {
	s := RunDefaultServer()
	defer s.Shutdown()

	// Access LastError in disconnection and closed handlers to make sure
	// that there is no race. It is possible in some cases that
	// nc.LastError() returns a non nil error. We don't care here about the
	// returned value.
	dch := func(c *nats.Conn) {
		c.LastError()
	}
	closedCh := make(chan struct{})
	cch := func(c *nats.Conn) {
		c.LastError()
		closedCh <- struct{}{}
	}
	nc, err := nats.Connect(nats.DefaultURL,
		nats.DisconnectHandler(dch),
		nats.ClosedHandler(cch),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(5*time.Millisecond))
	if err != nil {
		t.Fatalf("Unable to connect: %v\n", err)
	}
	defer nc.Close()

	// Restart the server several times to trigger a reconnection.
	for i := 0; i < 10; i++ {
		s.Shutdown()
		time.Sleep(10 * time.Millisecond)
		s = RunDefaultServer()
	}
	nc.Close()
	s.Shutdown()
	select {
	case <-closedCh:
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for the closed callback")
	}
}

func TestUseCustomDialer(t *testing.T) {
	s := RunDefaultServer()
	defer s.Shutdown()

	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		DualStack: true,
	}
	opts := &nats.Options{
		Servers: []string{nats.DefaultURL},
		Dialer:  dialer,
	}
	nc, err := opts.Connect()
	if err != nil {
		t.Fatalf("Unexpected error on connect: %v", err)
	}
	defer nc.Close()
	if nc.Opts.Dialer != dialer {
		t.Fatalf("Expected Dialer to be set to %v, got %v", dialer, nc.Opts.Dialer)
	}

	// Should be possible to set via variadic func based Option setter
	dialer2 := &net.Dialer{
		Timeout:   5 * time.Second,
		DualStack: true,
	}
	nc2, err := nats.Connect(nats.DefaultURL, nats.Dialer(dialer2))
	if err != nil {
		t.Fatalf("Unexpected error on connect: %v", err)
	}
	defer nc2.Close()
	if !nc2.Opts.Dialer.DualStack {
		t.Fatalf("Expected for dialer to be customized to use dual stack support")
	}

	// By default, dialer still uses the DefaultTimeout
	nc3, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		t.Fatalf("Unexpected error on connect: %v", err)
	}
	defer nc3.Close()
	if nc3.Opts.Dialer.Timeout != nats.DefaultTimeout {
		t.Fatalf("Expected DialTimeout to be set to %v, got %v", nats.DefaultTimeout, nc.Opts.Dialer.Timeout)
	}
}

func TestDefaultOptionsDialer(t *testing.T) {
	s := RunDefaultServer()
	defer s.Shutdown()

	opts1 := nats.DefaultOptions
	opts2 := nats.DefaultOptions

	nc1, err := opts1.Connect()
	if err != nil {
		t.Fatalf("Unexpected error on connect: %v", err)
	}
	defer nc1.Close()

	nc2, err := opts2.Connect()
	if err != nil {
		t.Fatalf("Unexpected error on connect: %v", err)
	}
	defer nc2.Close()

	if nc1.Opts.Dialer == nc2.Opts.Dialer {
		t.Fatalf("Expected each connection to have its own dialer")
	}
}

func TestCustomFlusherTimeout(t *testing.T) {
	s := RunDefaultServer()
	defer s.Shutdown()

	opts := &nats.Options{
		Servers: []string{nats.DefaultURL},

		// Reasonably large flusher timeout will not induce errors
		// when we can flush fast
		FlusherTimeout: 10 * time.Second,
	}
	nc1, err := opts.Connect()
	if err != nil {
		t.Fatalf("Expected to be able to connect, got: %s", err)
	}
	doneCh := make(chan struct{})
	payload := ""
	for i := 0; i < 8192; i++ {
		payload += "A"
	}
	payloadBytes := []byte(payload)

	go func() {
		for {
			select {
			case <-time.After(200 * time.Millisecond):
				err := nc1.Publish("hello", payloadBytes)
				if err != nil {
					t.Errorf("Error during publish: %s", err)
				}
			case <-time.After(5 * time.Second):
				t.Errorf("Timeout publishing messages")
				return
			case <-doneCh:
				return
			}
		}
	}()
	defer nc1.Close()

	opts = &nats.Options{
		Servers: []string{nats.DefaultURL},

		// Use short flusher timeout to trigger the error
		FlusherTimeout: 1 * time.Microsecond,

		// Upon failure to be able to exercice ping pong interval
		// then we will hit this timeout and disconnect
		PingInterval: 500 * time.Millisecond,
	}

	opts.DisconnectedCB = func(nc *nats.Conn) {
		// Ping loops that test is done
		doneCh <- struct{}{}
	}

	nc2, err := opts.Connect()
	if err != nil {
		t.Fatalf("Expected to be able to connect, got: %s", err)
	}
	defer nc2.Close()

	// Consume messages to make the reading loop work
	_, err = nc2.Subscribe(">", func(_ *nats.Msg) {})
	if err != nil {
		t.Fatalf("Expected to be able to create subscription, got: %s", err)
	}

	for {
		select {
		case <-time.After(100 * time.Millisecond):
			// Some of the publishes will succeed and others fail with i/o timeout error
			// but eventually ping interval will fail and close the connection.
			err = nc2.Publish("world", payloadBytes)
			if err == nats.ErrConnectionClosed {
				return
			}
		case <-time.After(5 * time.Second):
			t.Errorf("Timeout publishing messages")
			return
		}
	}
}

func TestNewServers(t *testing.T) {
	s1Opts := test.DefaultTestOptions
	s1Opts.Cluster.Host = "localhost"
	s1Opts.Cluster.Port = 6222
	s1 := test.RunServer(&s1Opts)
	defer s1.Shutdown()

	s2Opts := test.DefaultTestOptions
	s2Opts.Port = s1Opts.Port + 1
	s2Opts.Cluster.Host = "localhost"
	s2Opts.Cluster.Port = 6223
	s2Opts.Routes = server.RoutesFromStr("nats://localhost:6222")
	s2 := test.RunServer(&s2Opts)
	defer s2.Shutdown()

	ch := make(chan bool)
	cb := func(_ *nats.Conn) {
		ch <- true
	}
	url := fmt.Sprintf("nats://%s:%d", s1Opts.Host, s1Opts.Port)
	nc1, err := nats.Connect(url, nats.DiscoveredServersHandler(cb))
	if err != nil {
		t.Fatalf("Error on connect: %v", err)
	}
	defer nc1.Close()

	nc2, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("Error on connect: %v", err)
	}
	defer nc2.Close()
	nc2.SetDiscoveredServersHandler(cb)

	opts := nats.GetDefaultOptions()
	opts.Url = nats.DefaultURL
	opts.DiscoveredServersCB = cb
	nc3, err := opts.Connect()
	if err != nil {
		t.Fatalf("Error on connect: %v", err)
	}
	defer nc3.Close()

	// Make sure that handler is not invoked on initial connect.
	select {
	case <-ch:
		t.Fatalf("Handler should not have been invoked")
	case <-time.After(500 * time.Millisecond):
	}

	// Start a new server.
	s3Opts := test.DefaultTestOptions
	s3Opts.Port = s2Opts.Port + 1
	s3Opts.Cluster.Host = "localhost"
	s3Opts.Cluster.Port = 6224
	s3Opts.Routes = server.RoutesFromStr("nats://localhost:6222")
	s3 := test.RunServer(&s3Opts)
	defer s3.Shutdown()

	// The callbacks should have been invoked
	if err := Wait(ch); err != nil {
		t.Fatal("Did not get our callback")
	}
	if err := Wait(ch); err != nil {
		t.Fatal("Did not get our callback")
	}
	if err := Wait(ch); err != nil {
		t.Fatal("Did not get our callback")
	}
}
