// +build examples

// Package examples provides simple examples of gokrb5 use.
package main

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"

	"gopkg.in/jcmturner/goidentity.v3"
	"gopkg.in/jcmturner/gokrb5.v7/client"
	"gopkg.in/jcmturner/gokrb5.v7/config"
	"gopkg.in/jcmturner/gokrb5.v7/keytab"
	"gopkg.in/jcmturner/gokrb5.v7/service"
	"gopkg.in/jcmturner/gokrb5.v7/spnego"
	"gopkg.in/jcmturner/gokrb5.v7/test/testdata"
)

func main() {
	s := httpServer()
	defer s.Close()

	b, _ := hex.DecodeString(testdata.TESTUSER1_KEYTAB)
	kt := keytab.New()
	kt.Unmarshal(b)
	c, _ := config.NewConfigFromString(testdata.TEST_KRB5CONF)
	c.LibDefaults.NoAddresses = true
	cl := client.NewClientWithKeytab("testuser1", "TEST.GOKRB5", kt, c)
	httpRequest(s.URL, cl)

	b, _ = hex.DecodeString(testdata.TESTUSER2_KEYTAB)
	kt = keytab.New()
	kt.Unmarshal(b)
	c, _ = config.NewConfigFromString(testdata.TEST_KRB5CONF)
	c.LibDefaults.NoAddresses = true
	cl = client.NewClientWithKeytab("testuser2", "TEST.GOKRB5", kt, c)
	httpRequest(s.URL, cl)
}

func httpRequest(url string, cl *client.Client) {
	l := log.New(os.Stderr, "GOKRB5 Client: ", log.Ldate|log.Ltime|log.Lshortfile)

	err := cl.Login()
	if err != nil {
		l.Printf("Error on AS_REQ: %v\n", err)
	}
	r, _ := http.NewRequest("GET", url, nil)
	err = spnego.SetSPNEGOHeader(cl, r, "HTTP/host.test.gokrb5")
	if err != nil {
		l.Printf("Error setting client SPNEGO header: %v", err)
	}
	httpResp, err := http.DefaultClient.Do(r)
	if err != nil {
		l.Printf("Request error: %v\n", err)
	}
	fmt.Fprintf(os.Stdout, "Response Code: %v\n", httpResp.StatusCode)
	content, _ := ioutil.ReadAll(httpResp.Body)
	fmt.Fprintf(os.Stdout, "Response Body:\n%s\n", content)
}

func httpServer() *httptest.Server {
	l := log.New(os.Stderr, "GOKRB5 Service Tests: ", log.Ldate|log.Ltime|log.Lshortfile)
	b, _ := hex.DecodeString(testdata.HTTP_KEYTAB)
	kt := keytab.New()
	kt.Unmarshal(b)
	th := http.HandlerFunc(testAppHandler)
	s := httptest.NewServer(spnego.SPNEGOKRB5Authenticate(th, kt, service.Logger(l)))
	return s
}

func testAppHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	fmt.Fprint(w, "<html>\n<p><h1>TEST.GOKRB5 Handler</h1></p>\n")
	if validuser, ok := ctx.Value(spnego.CTXKeyAuthenticated).(bool); ok && validuser {
		if creds, ok := ctx.Value(spnego.CTXKeyCredentials).(goidentity.Identity); ok {
			fmt.Fprintf(w, "<ul><li>Authenticed user: %s</li>\n", creds.UserName())
			fmt.Fprintf(w, "<li>User's realm: %s</li></ul>\n", creds.Domain())
		}

	} else {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, "Authentication failed")
	}
	fmt.Fprint(w, "</html>")
	return
}
