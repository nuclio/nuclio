package main

import (
	"encoding/hex"
	"log"
	"os"
	"time"

	"gopkg.in/jcmturner/gokrb5.v7/client"
	"gopkg.in/jcmturner/gokrb5.v7/config"
	"gopkg.in/jcmturner/gokrb5.v7/keytab"
	"gopkg.in/jcmturner/gokrb5.v7/test/testdata"
)

const (
	kRB5CONF = `[libdefaults]
  default_realm = TEST.GOKRB5
  dns_lookup_realm = false
  dns_lookup_kdc = false
  ticket_lifetime = 24h
  forwardable = yes
  default_tkt_enctypes = aes256-cts-hmac-sha1-96
  default_tgs_enctypes = aes256-cts-hmac-sha1-96

[realms]
 TEST.GOKRB5 = {
  kdc = 10.80.88.88:88
  admin_server = 10.80.88.88:749
  default_domain = test.gokrb5
 }

[domain_realm]
 .test.gokrb5 = TEST.GOKRB5
 test.gokrb5 = TEST.GOKRB5
 `
)

func main() {
	l := log.New(os.Stderr, "GOKRB5 Client: ", log.LstdFlags)

	//defer profile.Start(profile.TraceProfile).Stop()
	// Load the keytab
	kb, _ := hex.DecodeString(testdata.TESTUSER2_KEYTAB)
	kt := keytab.New()
	err := kt.Unmarshal(kb)
	if err != nil {
		l.Fatalf("could not load client keytab: %v", err)
	}

	// Load the client krb5 config
	conf, err := config.NewConfigFromString(kRB5CONF)
	if err != nil {
		l.Fatalf("could not load krb5.conf: %v", err)
	}
	addr := os.Getenv("TEST_KDC_ADDR")
	if addr != "" {
		conf.Realms[0].KDC = []string{addr + ":88"}
	}

	// Create the client with the keytab
	cl := client.NewClientWithKeytab("testuser2", "TEST.GOKRB5", kt, conf, client.Logger(l), client.DisablePAFXFAST(true))

	// Log in the client
	err = cl.Login()
	if err != nil {
		l.Fatalf("could not login client: %v", err)
	}

	for {
		_, _, err := cl.GetServiceTicket("HTTP/host.test.gokrb5")
		if err != nil {
			l.Printf("failed to get service ticket: %v\n", err)
		}
		time.Sleep(time.Minute * 5)
	}
}
