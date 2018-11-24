// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package maintner

import (
	"reflect"
	"testing"
	"time"
)

func TestParsePerson(t *testing.T) {
	var c Corpus

	p, ct, err := c.parsePerson([]byte(" Foo Bar <foo@bar.com> 1257894000 -0800"))
	if err != nil {
		t.Fatal(err)
	}
	wantp := &GitPerson{Str: "Foo Bar <foo@bar.com>"}
	if !reflect.DeepEqual(p, wantp) {
		t.Errorf("person = %+v; want %+v", p, wantp)
	}
	wantct := time.Unix(1257894000, 0)
	if !ct.Equal(wantct) {
		t.Errorf("commit time = %v; want %v", ct, wantct)
	}
	zoneName, off := ct.Zone()
	if want := "-0800"; zoneName != want {
		t.Errorf("zone name = %q; want %q", zoneName, want)
	}
	if want := -28800; off != want {
		t.Errorf("offset = %v; want %v", off, want)
	}

	p2, ct2, err := c.parsePerson([]byte("Foo Bar <foo@bar.com> 1257894001 -0800"))
	if err != nil {
		t.Fatal(err)
	}
	if p != p2 {
		t.Errorf("gitPerson pointer values differ; not sharing memory")
	}
	if !ct2.Equal(ct.Add(time.Second)) {
		t.Errorf("wrong time")
	}
}

func BenchmarkParsePerson(b *testing.B) {
	b.ReportAllocs()
	in := []byte(" Foo Bar <foo@bar.com> 1257894000 -0800")
	var c Corpus
	for i := 0; i < b.N; i++ {
		_, _, err := c.parsePerson(in)
		if err != nil {
			b.Fatal(err)
		}
	}
}
