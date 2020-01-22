# gokrb5
[![Version](https://img.shields.io/github/release/jcmturner/gokrb5.svg)](https://github.com/jcmturner/gokrb5/releases)

[![GoDoc](https://godoc.org/gopkg.in/jcmturner/gokrb5.v7?status.svg)](https://godoc.org/gopkg.in/jcmturner/gokrb5.v7) [![Go Report Card](https://goreportcard.com/badge/gopkg.in/jcmturner/gokrb5.v7)](https://goreportcard.com/report/gopkg.in/jcmturner/gokrb5.v7) 

[![Build Status](https://github.com/jcmturner/gokrb5/workflows/gokrb5/badge.svg)](https://github.com/jcmturner/gokrb5/actions)

#### Go Version Support
![Go version](https://img.shields.io/badge/Go-1.13-brightgreen.svg)
![Go version](https://img.shields.io/badge/Go-1.12-brightgreen.svg)
![Go version](https://img.shields.io/badge/Go-1.11-brightgreen.svg)

gokrb5 may work with other versions of Go but they are not tested.

### Go Get
To get the package, execute:
```
go get -d gopkg.in/jcmturner/gokrb5.v7/...
```
To import this package, add the following line to your code:
```go
import "gopkg.in/jcmturner/gokrb5.v7/<sub package>"
```

## Features
* **Pure Go** - no dependency on external libraries 
* No platform specific code
* Server Side
  * HTTP handler wrapper implements SPNEGO Kerberos authentication
  * HTTP handler wrapper decodes Microsoft AD PAC authorization data
* Client Side
  * Client that can authenticate to an SPNEGO Kerberos authenticated web service
  * Ability to change client's password
* General
  * Kerberos libraries for custom integration
  * Parsing Keytab files
  * Parsing krb5.conf files
  * Parsing client credentials cache files such as `/tmp/krb5cc_$(id -u $(whoami))`

#### Implemented Encryption & Checksum Types

| Implementation | Encryption ID | Checksum ID | RFC |
|-------|-------------|------------|------|
| des3-cbc-sha1-kd | 16 | 12 | 3961 |
| aes128-cts-hmac-sha1-96 | 17 | 15 | 3962 |
| aes256-cts-hmac-sha1-96 | 18 | 16 | 3962 |
| aes128-cts-hmac-sha256-128 | 19 | 19 | 8009 |
| aes256-cts-hmac-sha384-192 | 20 | 20 | 8009 |
| rc4-hmac | 23 | -138 | 4757 |


The following is working/tested:
* Tested against MIT KDC (1.6.3 is the oldest version tested against) and Microsoft Active Directory (Windows 2008 R2)
* Tested against a KDC that supports PA-FX-FAST.
* Tested against users that have pre-authentication required using PA-ENC-TIMESTAMP.
* Microsoft PAC Authorization Data is processed and exposed in the HTTP request context. Available if Microsoft Active Directory is used as the KDC.

## Contributing
If you are interested in contributing to gokrb5, great! Please read the [contribution guidelines](https://github.com/jcmturner/gokrb5/blob/master/CONTRIBUTING.md).

## Usage

---

### Configuration
The gokrb5 libraries use the same krb5.conf configuration file format as MIT Kerberos, described [here](https://web.mit.edu/kerberos/krb5-latest/doc/admin/conf_files/krb5_conf.html).
Config instances can be created by loading from a file path or by passing a string, io.Reader or bufio.Scanner to the relevant method:
```go
import "gopkg.in/jcmturner/gokrb5.v7/config"
cfg, err := config.Load("/path/to/config/file")
cfg, err := config.NewConfigFromString(krb5Str) //String must have appropriate newline separations
cfg, err := config.NewConfigFromReader(reader)
cfg, err := config.NewConfigFromScanner(scanner)
```
### Keytab files
Standard keytab files can be read from a file or from a slice of bytes:
```go
import 	"gopkg.in/jcmturner/gokrb5.v7/keytab"
ktFromFile, err := keytab.Load("/path/to/file.keytab")
ktFromBytes, err := keytab.Parse(b)

```

---

### Kerberos Client
**Create** a client instance with either a password or a keytab.
A configuration must also be passed. Additionally optional additional settings can be provided.
```go
import 	"gopkg.in/jcmturner/gokrb5.v7/client"
cl := client.NewClientWithPassword("username", "REALM.COM", "password", cfg)
cl := client.NewClientWithKeytab("username", "REALM.COM", kt, cfg)
```
Optional settings are provided using the functions defined in the ``client/settings.go`` source file.

**Login**:
```go
err := cl.Login()
```
Kerberos Ticket Granting Tickets (TGT) will be automatically renewed unless the client was created from a CCache.

A client can be **destroyed** with the following method:
```go
cl.Destroy()
```

#### Active Directory KDC and FAST negotiation
Active Directory does not commonly support FAST negotiation so you will need to disable this on the client.
If this is the case you will see this error:
```KDC did not respond appropriately to FAST negotiation```
To resolve this disable PA-FX-Fast on the client before performing Login().
This is done with one of the optional client settings as shown below:
```go
cl := client.NewClientWithPassword("username", "REALM.COM", "password", cfg, client.DisablePAFXFAST(true))
```

#### Authenticate to a Service

##### HTTP SPNEGO
Create the HTTP request object and then create an SPNEGO client and use this to process the request with methods that 
are the same as on a HTTP client.
If nil is passed as the HTTP client when creating the SPNEGO client the http.DefaultClient is used.
When creating the SPNEGO client pass the Service Principal Name (SPN) or auto generate the SPN from the request 
object by passing a null string "".
```go
r, _ := http.NewRequest("GET", "http://host.test.gokrb5/index.html", nil)
spnegoCl := spnego.NewClient(cl, nil, "")
resp, err := spnegoCl.Do(r)
```

##### Generic Kerberos Client
To authenticate to a service a client will need to request a service ticket for a Service Principal Name (SPN) and form into an AP_REQ message along with an authenticator encrypted with the session key that was delivered from the KDC along with the service ticket.

The steps below outline how to do this.
* Get the service ticket and session key for the service the client is authenticating to.
The following method will use the client's cache either returning a valid cached ticket, renewing a cached ticket with the KDC or requesting a new ticket from the KDC.
Therefore the GetServiceTicket method can be continually used for the most efficient interaction with the KDC.
```go
tkt, key, err := cl.GetServiceTicket("HTTP/host.test.gokrb5")
```

The steps after this will be specific to the application protocol but it will likely involve a client/server Authentication Protocol exchange (AP exchange).
This will involve these steps:

* Generate a new Authenticator and generate a sequence number and subkey:
```go
auth, _ := types.NewAuthenticator(cl.Credentials.Realm, cl.Credentials.CName)
etype, _ := crypto.GetEtype(key.KeyType)
auth.GenerateSeqNumberAndSubKey(key.KeyType, etype.GetKeyByteSize())
```
* Set the checksum on the authenticator
The checksum is an application specific value. Set as follows:
```go
auth.Cksum = types.Checksum{
		CksumType: checksumIDint,
		Checksum:  checksumBytesSlice,
	}
```
* Create the AP_REQ:
```go
APReq, err := messages.NewAPReq(tkt, key, auth)
```

Now send the AP_REQ to the service. How this is done will be specific to the application use case.

#### Changing a Client Password
This feature uses the Microsoft Kerberos Password Change protocol (RFC 3244). 
This is implemented in Microsoft Active Directory and in MIT krb5kdc as of version 1.7.
Typically the kpasswd server listens on port 464.

Below is example code for how to use this feature:
```go
cfg, err := config.Load("/path/to/config/file")
if err != nil {
	panic(err.Error())
}
kt, err := keytab.Load("/path/to/file.keytab")
if err != nil {
	panic(err.Error())
}
cl := client.NewClientWithKeytab("username", "REALM.COM", kt)
cl.WithConfig(cfg)

ok, err := cl.ChangePasswd("newpassword")
if err != nil {
	panic(err.Error())
}
if !ok {
	panic("failed to change password")
}
```

The client kerberos config (krb5.conf) will need to have either the kpassd_server or admin_server defined in the relevant [realms] section.
For example:
```
REALM.COM = {
  kdc = 127.0.0.1:88
  kpasswd_server = 127.0.0.1:464
  default_domain = realm.com
 }
```
See https://web.mit.edu/kerberos/krb5-latest/doc/admin/conf_files/krb5_conf.html#realms for more information.

---

### Kerberised Service

#### SPNEGO/Kerberos HTTP Service
A HTTP handler wrapper can be used to implement Kerberos SPNEGO authentication for web services.
To configure the wrapper the keytab for the SPN and a Logger are required:
```go
kt, err := keytab.Load("/path/to/file.keytab")
l := log.New(os.Stderr, "GOKRB5 Service: ", log.Ldate|log.Ltime|log.Lshortfile)
```
Create a handler function of the application's handling method (apphandler in the example below):
```go
h := http.HandlerFunc(apphandler)
```
Configure the HTTP handler:
```go
http.Handler("/", spnego.SPNEGOKRB5Authenticate(h, &kt, service.Logger(l)))
```
The handler to be wrapped and the keytab are required arguments. 
Additional optional settings can be provided, such as the logger shown above.

Another example of optional settings may be that when using Active Directory where the SPN is mapped to a user account 
the keytab may contain an entry for this user account. In this case this should be specified as below with the ``KeytabPrincipal``:
```go
http.Handler("/", spnego.SPNEGOKRB5Authenticate(h, &kt, service.Logger(l), service.KeytabPrincipal(pn)))
```

If authentication succeeds then the request's context will have the following values added so they can be accessed within the application's handler:
* spnego.CTXKeyAuthenticated - Boolean indicating if the user is authenticated. Use of this value should also handle that this value may not be set and should assume "false" in that case.
* spnego.CTXKeyCredentials - The authenticated user's credentials.
If Microsoft Active Directory is used as the KDC then additional ADCredentials are available in the credentials.Attributes map under the key credentials.AttributeKeyADCredentials. For example the SIDs of the users group membership are available and can be used by your application for authorization.

Access the credentials within your application:
```go
ctx := r.Context()
if validuser, ok := ctx.Value(spnego.CTXKeyAuthenticated).(bool); ok && validuser {
        if creds, ok := ctx.Value(spnego.CTXKeyCredentials).(goidentity.Identity); ok {
                if ADCreds, ok := creds.Attributes()[credentials.AttributeKeyADCredentials].(credentials.ADCredentials); ok {
                        // Now access the fields of the ADCredentials struct. For example:
                        groupSids := ADCreds.GroupMembershipSIDs
                }
        } 

}
```

#### Generic Kerberised Service - Validating Client Details
To validate the AP_REQ sent by the client on the service side call this method:
```go
import 	"gopkg.in/jcmturner/gokrb5.v7/service"
s := service.NewSettings(&kt) // kt is a keytab and optional settings can also be provided.
if ok, creds, err := service.VerifyAPREQ(APReq, s); ok {
        // Perform application specific actions
        // creds object has details about the client identity
}
```

---

## References
* [RFC 3244 Microsoft Windows 2000 Kerberos Change Password and Set Password Protocols](https://tools.ietf.org/html/rfc3244)
* [RFC 4120 The Kerberos Network Authentication Service (V5)](https://tools.ietf.org/html/rfc4120)
* [RFC 3961 Encryption and Checksum Specifications for Kerberos 5](https://tools.ietf.org/html/rfc3961)
* [RFC 3962 Advanced Encryption Standard (AES) Encryption for Kerberos 5](https://tools.ietf.org/html/rfc3962)
* [RFC 4121 The Kerberos Version 5 GSS-API Mechanism](https://tools.ietf.org/html/rfc4121)
* [RFC 4178 The Simple and Protected Generic Security Service Application Program Interface (GSS-API) Negotiation Mechanism](https://tools.ietf.org/html/rfc4178.html)
* [RFC 4559 SPNEGO-based Kerberos and NTLM HTTP Authentication in Microsoft Windows](https://tools.ietf.org/html/rfc4559.html)
* [RFC 4757 The RC4-HMAC Kerberos Encryption Types Used by Microsoft Windows](https://tools.ietf.org/html/rfc4757)
* [RFC 6806 Kerberos Principal Name Canonicalization and Cross-Realm Referrals](https://tools.ietf.org/html/rfc6806.html)
* [RFC 6113 A Generalized Framework for Kerberos Pre-Authentication](https://tools.ietf.org/html/rfc6113.html)
* [RFC 8009 AES Encryption with HMAC-SHA2 for Kerberos 5](https://tools.ietf.org/html/rfc8009)
* [IANA Assigned Kerberos Numbers](http://www.iana.org/assignments/kerberos-parameters/kerberos-parameters.xhtml)
* [HTTP-Based Cross-Platform Authentication by Using the Negotiate Protocol - Part 1](https://msdn.microsoft.com/en-us/library/ms995329.aspx)
* [HTTP-Based Cross-Platform Authentication by Using the Negotiate Protocol - Part 2](https://msdn.microsoft.com/en-us/library/ms995330.aspx)
* [Microsoft PAC Validation](https://blogs.msdn.microsoft.com/openspecification/2009/04/24/understanding-microsoft-kerberos-pac-validation/)
* [Microsoft Kerberos Protocol Extensions](https://msdn.microsoft.com/en-us/library/cc233855.aspx)
* [Windows Data Types](https://msdn.microsoft.com/en-us/library/cc230273.aspx)

### Useful Links
* https://en.wikipedia.org/wiki/Ciphertext_stealing#CBC_ciphertext_stealing

## Thanks
* Greg Hudson from the MIT Consortium for Kerberos and Internet Trust for providing useful advice.

## Contributing
Thank you for your interest in contributing to gokrb5 please read the 
[contribution guide](https://github.com/jcmturner/gokrb5/blob/master/CONTRIBUTING.md) as it should help you get started.

## Known Issues
| Issue | Worked around? | References |
|-------|-------------|------------|
| The Go standard library's encoding/asn1 package cannot unmarshal into slice of asn1.RawValue | Yes | https://github.com/golang/go/issues/17321 |
| The Go standard library's encoding/asn1 package cannot marshal into a GeneralString | Yes - using https://github.com/jcmturner/gofork/tree/master/encoding/asn1 | https://github.com/golang/go/issues/18832 |
| The Go standard library's encoding/asn1 package cannot marshal into slice of strings and pass stringtype parameter tags to members | Yes - using https://github.com/jcmturner/gofork/tree/master/encoding/asn1 | https://github.com/golang/go/issues/18834 |
| The Go standard library's encoding/asn1 package cannot marshal with application tags | Yes | |
| The Go standard library's x/crypto/pbkdf2.Key function uses the int type for iteraction count limiting meaning the 4294967296 count specified in https://tools.ietf.org/html/rfc3962 section 4 cannot be met on 32bit systems | Yes - using https://github.com/jcmturner/gofork/tree/master/x/crypto/pbkdf2 | https://go-review.googlesource.com/c/crypto/+/85535 |
