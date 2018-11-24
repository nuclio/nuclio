// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package maintner

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/golang/protobuf/ptypes"
	google_protobuf "github.com/golang/protobuf/ptypes/timestamp"
	"github.com/google/go-github/github"
	"golang.org/x/build/maintner/maintpb"
)

var u1 = &GitHubUser{
	Login: "gopherbot",
	ID:    100,
}
var u2 = &GitHubUser{
	Login: "kevinburke",
	ID:    101,
}

type dummyMutationLogger struct {
	Mutations []*maintpb.Mutation
}

func (d *dummyMutationLogger) Log(m *maintpb.Mutation) error {
	if d.Mutations == nil {
		d.Mutations = []*maintpb.Mutation{}
	}
	d.Mutations = append(d.Mutations, m)
	return nil
}

type mutationTest struct {
	corpus *Corpus
	want   *Corpus
}

func (mt mutationTest) test(t *testing.T, muts ...*maintpb.Mutation) {
	c := mt.corpus
	if c == nil {
		c = new(Corpus)
	}
	for _, m := range muts {
		c.processMutationLocked(m)
	}
	c.github.c = nil
	mt.want.github.c = nil
	if !reflect.DeepEqual(c.github, mt.want.github) {
		t.Errorf("corpus mismatch:\n got: %s\n\nwant: %s\n\ndiff: %v",
			spew.Sdump(c.github),
			spew.Sdump(mt.want.github),
			diffPath(reflect.ValueOf(c.github), reflect.ValueOf(mt.want.github)))
	}
}

var t1, t2 time.Time
var tp1, tp2 *google_protobuf.Timestamp

func init() {
	t1, _ = time.Parse(time.RFC3339, "2016-01-02T15:04:00Z")
	t2, _ = time.Parse(time.RFC3339, "2016-01-02T15:30:00Z")
	tp1, _ = ptypes.TimestampProto(t1)
	tp2, _ = ptypes.TimestampProto(t2)
}

func TestProcessMutation_Github_NewIssue(t *testing.T) {
	c := new(Corpus)
	github := &GitHub{c: c}
	c.github = github
	github.users = map[int64]*GitHubUser{
		u1.ID: u1,
	}
	github.repos = map[GitHubRepoID]*GitHubRepo{
		GitHubRepoID{"golang", "go"}: &GitHubRepo{
			github: github,
			id:     GitHubRepoID{"golang", "go"},
			issues: map[int32]*GitHubIssue{
				3: &GitHubIssue{
					Number:    3,
					User:      u1,
					Title:     "some title",
					Body:      "some body",
					Created:   t1,
					Assignees: nil,
				},
			},
		},
	}
	mutationTest{want: c}.test(t, &maintpb.Mutation{
		GithubIssue: &maintpb.GithubIssueMutation{
			Owner:  "golang",
			Repo:   "go",
			Number: 3,
			User: &maintpb.GithubUser{
				Login: "gopherbot",
				Id:    100,
			},
			Title:   "some title",
			Body:    "some body",
			Created: tp1,
		},
	})
}

func TestProcessMutation_Github(t *testing.T) {
	c := new(Corpus)
	github := &GitHub{c: c}
	c.github = github
	github.repos = map[GitHubRepoID]*GitHubRepo{
		GitHubRepoID{"golang", "go"}: &GitHubRepo{
			github: github,
			id:     GitHubRepoID{"golang", "go"},
			issues: make(map[int32]*GitHubIssue),
		},
	}
	mutationTest{want: c}.test(t, &maintpb.Mutation{
		Github: &maintpb.GithubMutation{
			Owner: "golang",
			Repo:  "go",
		},
	})
}

func TestNewMutationsFromIssue(t *testing.T) {
	gh := &github.Issue{
		Number:    github.Int(5),
		CreatedAt: &t1,
		UpdatedAt: &t2,
		Body:      github.String("body of the issue"),
		State:     github.String("closed"),
	}
	gr := &GitHubRepo{
		id: GitHubRepoID{"golang", "go"},
	}
	is := gr.newMutationFromIssue(nil, gh)
	want := &maintpb.Mutation{GithubIssue: &maintpb.GithubIssueMutation{
		Owner:       "golang",
		Repo:        "go",
		Number:      5,
		Body:        "body of the issue",
		Created:     tp1,
		Updated:     tp2,
		Assignees:   []*maintpb.GithubUser{},
		NoMilestone: true,
		Closed:      &maintpb.BoolChange{Val: true},
	}}
	if !reflect.DeepEqual(is, want) {
		t.Errorf("issue mismatch\n got: %v\nwant: %v\ndiff path: %v", spew.Sdump(is), spew.Sdump(want),
			diffPath(reflect.ValueOf(is), reflect.ValueOf(want)))
	}
}

func TestNewAssigneesHandlesNil(t *testing.T) {
	users := []*github.User{
		&github.User{Login: github.String("foo"), ID: github.Int64(3)},
	}
	got := newAssignees(nil, users)
	want := []*maintpb.GithubUser{&maintpb.GithubUser{
		Id:    3,
		Login: "foo",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("assignee mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestAssigneesDeleted(t *testing.T) {
	c := new(Corpus)
	assignees := []*GitHubUser{u1, u2}
	issue := &GitHubIssue{
		Number:    3,
		User:      u1,
		Body:      "some body",
		Created:   t2,
		Updated:   t2,
		Assignees: assignees,
	}
	gr := &GitHubRepo{
		id: GitHubRepoID{"golang", "go"},
		issues: map[int32]*GitHubIssue{
			3: issue,
		},
	}
	c.github = &GitHub{
		users: map[int64]*GitHubUser{
			u1.ID: u1,
		},
		repos: map[GitHubRepoID]*GitHubRepo{
			GitHubRepoID{"golang", "go"}: gr,
		},
	}

	mutation := gr.newMutationFromIssue(issue, &github.Issue{
		Number:    github.Int(3),
		Assignees: []*github.User{&github.User{ID: github.Int64(u2.ID)}},
	})
	c.addMutation(mutation)
	gi := gr.issues[3]
	if len(gi.Assignees) != 1 || gi.Assignees[0].ID != u2.ID {
		t.Errorf("expected u1 to be deleted, got %v", gi.Assignees)
	}
}

func DeepDiff(got, want interface{}) error {
	return diffPath(reflect.ValueOf(got), reflect.ValueOf(want))
}

func diffPath(got, want reflect.Value) error {
	if !got.IsValid() {
		return errors.New("'got' value invalid")
	}
	if !want.IsValid() {
		return errors.New("'want' value invalid")
	}

	t := got.Type()
	if t != want.Type() {
		return fmt.Errorf("got=%s, want=%s", got.Type(), want.Type())
	}

	switch t.Kind() {
	case reflect.Ptr, reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Slice:
		if got.IsNil() != want.IsNil() {
			if got.IsNil() {
				return fmt.Errorf("got = (%s)(nil), want = non-nil", t)
			}
			return fmt.Errorf("got = (%s)(non-nil), want = nil", t)
		}
	}

	switch t.Kind() {
	case reflect.Ptr:
		if got.IsNil() {
			return nil
		}
		return diffPath(got.Elem(), want.Elem())

	case reflect.Struct:
		nf := t.NumField()
		for i := 0; i < nf; i++ {
			sf := t.Field(i)
			if err := diffPath(got.Field(i), want.Field(i)); err != nil {
				inner := err.Error()
				sep := "."
				if strings.HasPrefix(inner, "field ") {
					inner = strings.TrimPrefix(inner, "field ")
				} else {
					sep = ": "
				}
				return fmt.Errorf("field %s%s%v", sf.Name, sep, inner)
			}
		}
		return nil
	case reflect.String:
		if got.String() != want.String() {
			return fmt.Errorf("got = %q; want = %q", got.String(), want.String())
		}
		return nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if got.Int() != want.Int() {
			return fmt.Errorf("got = %v; want = %v", got.Int(), want.Int())
		}
		return nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if got.Uint() != want.Uint() {
			return fmt.Errorf("got = %v; want = %v", got.Uint(), want.Uint())
		}
		return nil

	case reflect.Bool:
		if got.Bool() != want.Bool() {
			return fmt.Errorf("got = %v; want = %v", got.Bool(), want.Bool())
		}
		return nil

	case reflect.Slice:
		gl, wl := got.Len(), want.Len()
		if gl != wl {
			return fmt.Errorf("slice len %v; want %v", gl, wl)
		}
		for i := 0; i < gl; i++ {
			if err := diffPath(got.Index(i), want.Index(i)); err != nil {
				return fmt.Errorf("index[%d] differs: %v", i, err)
			}
		}
		return nil

	default:
		return fmt.Errorf("unhandled kind %v", t.Kind())
	}
}
