// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gerrit

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// taken from https://go-review.googlesource.com/projects/go
var exampleProjectResponse = []byte(`)]}'
{
  "id": "go",
  "name": "go",
  "parent": "All-Projects",
  "description": "The Go Programming Language",
  "state": "ACTIVE",
  "web_links": [
    {
      "name": "gitiles",
      "url": "https://go.googlesource.com/go/",
      "target": "_blank"
    }
  ]
}
`)

func TestGetProjectInfo(t *testing.T) {
	hitServer := false
	path := ""
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitServer = true
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(200)
		w.Write(exampleProjectResponse)
	}))
	defer s.Close()
	c := NewClient(s.URL, NoAuth)
	info, err := c.GetProjectInfo(context.Background(), "go")
	if err != nil {
		t.Fatal(err)
	}
	if !hitServer {
		t.Errorf("expected to hit test server, didn't")
	}
	if path != "/projects/go" {
		t.Errorf("expected Path to be '/projects/go', got %s", path)
	}
	if info.Name != "go" {
		t.Errorf("expected Name to be 'go', got %s", info.Name)
	}
}

func TestProjectNotFound(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		w.WriteHeader(404)
		w.Write([]byte("Not found: unknown"))
	}))
	defer s.Close()
	c := NewClient(s.URL, NoAuth)
	_, err := c.GetProjectInfo(context.Background(), "unknown")
	if err != ErrProjectNotExist {
		t.Errorf("expected to get ErrProjectNotExist, got %v", err)
	}
}

func TestContextError(t *testing.T) {
	c := NewClient("http://localhost", NoAuth)
	yearsAgo, _ := time.Parse("2006", "2006")
	ctx, cancel := context.WithDeadline(context.Background(), yearsAgo)
	defer cancel()
	_, err := c.GetProjectInfo(ctx, "unknown")
	if err == nil {
		t.Errorf("expected non-nil error, got nil")
	}
	uerr, ok := err.(*url.Error)
	if !ok {
		t.Errorf("expected url.Error, got %#v", err)
	}
	if uerr.Err != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded error, got %v", uerr.Err)
	}
}

var getChangeResponse = []byte(`)]}'
{
  "id": "build~master~I92989e0231299ed305ddfbbe6034d293f1c87470",
  "project": "build",
  "branch": "master",
  "hashtags": [],
  "change_id": "I92989e0231299ed305ddfbbe6034d293f1c87470",
  "subject": "devapp: fix tests",
  "status": "ABANDONED",
  "created": "2017-07-13 06:09:10.000000000",
  "updated": "2017-07-14 16:31:32.000000000",
  "insertions": 1,
  "deletions": 1,
  "unresolved_comment_count": 0,
  "has_review_started": true,
  "_number": 48330,
  "owner": {
    "_account_id": 13437
  },
  "messages": [
    {
      "id": "f9fcf0ff9eb58fc8edd989f8bbb3500ff73f9b11",
      "author": {
        "_account_id": 22285
      },
      "real_author": {
        "_account_id": 22285
      },
      "date": "2017-07-13 06:14:48.000000000",
      "message": "Patch Set 1:\n\nCheck out https://go-review.googlesource.com/c/48350/ :)",
      "_revision_number": 1
    }
  ]
}`)

func TestGetChange(t *testing.T) {
	hitServer := false
	uri := ""
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitServer = true
		uri = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(200)
		w.Write(getChangeResponse)
	}))
	defer s.Close()
	c := NewClient(s.URL, NoAuth)
	info, err := c.GetChange(context.Background(), "48330", QueryChangesOpt{
		Fields: []string{"MESSAGES"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !hitServer {
		t.Errorf("expected to hit test server, didn't")
	}
	if want := "/changes/48330?o=MESSAGES"; uri != want {
		t.Errorf("expected RequestURI to be %q, got %q", want, uri)
	}
	if len(info.Messages) != 1 {
		t.Errorf("expected message length to be 1, got %d", len(info.Messages))
	}
	msg := info.Messages[0].Message
	if !strings.Contains(msg, "Check out") {
		t.Errorf("expected to find string in Message, got %s", msg)
	}
}

func TestGetChangeError(t *testing.T) {
	hitServer := false
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitServer = true
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		w.WriteHeader(404)
		io.WriteString(w, "Not found: 99999")
	}))
	defer s.Close()
	c := NewClient(s.URL, NoAuth)
	_, err := c.GetChange(context.Background(), "99999", QueryChangesOpt{
		Fields: []string{"MESSAGES"},
	})
	if !hitServer {
		t.Errorf("expected to hit test server, didn't")
	}
	if err != ErrChangeNotExist {
		t.Errorf("expected ErrChangeNotExist, got %v", err)
	}
}

var queryAccountsResponse = []byte(`)]}'
[
  {
    "_account_id": 1,
    "name": "John Doe",
    "email": "john@doe.com"
  },
  {
    "_account_id": 2,
    "name": "Jane Doe",
    "email": "jane@doe.com",
    "_more_accounts": true
  }
]`)

func TestQueryAccounts(t *testing.T) {
	hitServer := false
	uri := ""

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitServer = true
		uri = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(200)
		w.Write(queryAccountsResponse)
	}))
	defer s.Close()
	c := NewClient(s.URL, NoAuth)
	info, err := c.QueryAccounts(context.Background(), "is:active", QueryAccountsOpt{
		Fields: []string{"DETAILS"},
		N:      2,
	})
	if err != nil {
		t.Fatal(err.Error())
	}
	if !hitServer {
		t.Errorf("expected to hit test server, didn't")
	}
	if want := "/accounts/?n=2&o=DETAILS&q=is%3Aactive"; uri != want {
		t.Errorf("expected RequestURI to be %q, got %q", want, uri)
	}
	if len(info) != 2 {
		t.Errorf("expected accounts length to be 2, got %d", len(info))
	}
	if info[0].NumericID != 1 || info[0].Name != "John Doe" || info[0].Email != "john@doe.com" {
		t.Errorf("expected to match John Doe in account, got %v", info[0])
	}
	if info[1].NumericID != 2 || info[1].Name != "Jane Doe" || info[1].Email != "jane@doe.com" {
		t.Errorf("expected to match Jane Doe in account, got %v", info[1])
	}
	if info[0].MoreAccounts {
		t.Errorf("expected to MoreAccounts to be false for John Doe")
	}
	if !info[1].MoreAccounts {
		t.Errorf("expected to MoreAccounts to be true for Jane Doe")
	}
}

func TestTimeStampMarshalJson(t *testing.T) {
	ts := TimeStamp(time.Date(1888, 6, 24, 6, 8, 30, 123456789, time.FixedZone("+1", 3600)))
	b, err := ts.MarshalJSON()
	if err != nil {
		t.Errorf("unexpected err %v", err)
	}
	expected := `"1888-06-24 05:08:30.123456789"`
	if string(b) != expected {
		t.Errorf("expected %q, got %q", expected, b)
	}
}

func TestTimeStampUnmarshalJson(t *testing.T) {
	var ts TimeStamp
	err := ts.UnmarshalJSON([]byte(`"1888-06-24 05:08:30.123456789"`))
	if err != nil {
		t.Errorf("unexpected err %v", err)
	}
	expected := time.Date(1888, 6, 24, 5, 8, 30, 123456789, time.UTC)
	if !ts.Time().Equal(expected) {
		t.Errorf("expected %v, got %v", expected, ts.Time())
	}
}

// taken from https://gerrit-review.googlesource.com/Documentation/rest-api-projects.html#list-tags
var exampleProjectTagsResponse = []byte(`  )]}'
  [
    {
      "ref": "refs/tags/v1.0",
      "revision": "49ce77fdcfd3398dc0dedbe016d1a425fd52d666",
      "object": "1624f5af8ae89148d1a3730df8c290413e3dcf30",
      "message": "Annotated tag",
      "tagger": {
        "name": "David Pursehouse",
        "email": "david.pursehouse@sonymobile.com",
        "date": "2014-10-06 07:35:03.000000000",
        "tz": 540
      }
    },
    {
      "ref": "refs/tags/v2.0",
      "revision": "1624f5af8ae89148d1a3730df8c290413e3dcf30"
    }
  ]
`)

func TestGetProjectTags(t *testing.T) {
	hitServer := false
	path := ""
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitServer = true
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(200)
		w.Write(exampleProjectTagsResponse)
	}))
	defer s.Close()
	c := NewClient(s.URL, NoAuth)
	tags, err := c.GetProjectTags(context.Background(), "go")
	if err != nil {
		t.Fatal(err)
	}
	if !hitServer {
		t.Errorf("expected to hit test server, didn't")
	}
	if path != "/projects/go/tags/" {
		t.Errorf("expected Path to be '/projects/go/tags/', got %s", path)
	}
	expectedTags := map[string]TagInfo{
		"refs/tags/v1.0": TagInfo{
			Ref:      "refs/tags/v1.0",
			Revision: "49ce77fdcfd3398dc0dedbe016d1a425fd52d666",
			Object:   "1624f5af8ae89148d1a3730df8c290413e3dcf30",
			Message:  "Annotated tag",
			Tagger: &GitPersonInfo{
				Name:     "David Pursehouse",
				Email:    "david.pursehouse@sonymobile.com",
				Date:     TimeStamp(time.Date(2014, 10, 6, 7, 35, 3, 0, time.UTC)),
				TZOffset: 540,
			},
		},
		"refs/tags/v2.0": TagInfo{
			Ref:      "refs/tags/v2.0",
			Revision: "1624f5af8ae89148d1a3730df8c290413e3dcf30",
		},
	}
	if len(tags) != len(expectedTags) {
		t.Errorf("expected %d tags, got %d", len(expectedTags), len(tags))
	}
	for ref, tag := range tags {
		expectedTag, found := expectedTags[ref]
		if !found {
			t.Errorf("unexpected tag %q", ref)
		}
		if !tag.Equal(&expectedTag) {
			t.Errorf("tags don't match (expected %#v and got %#v)", expectedTag, tag)
		}
	}
}
