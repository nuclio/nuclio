package md2man

import (
	"bytes"
	"testing"
)

func TestBlockCode(t *testing.T) {
	r := &roffRenderer{}
	buf := bytes.NewBuffer(nil)

	code := []byte("$ echo hello world\nhello world\n")
	r.BlockCode(buf, code, "")

	expected := `
.PP
.RS

.nf
$ echo hello world
hello world

.fi
.RE
`
	result := buf.String()
	if expected != result {
		t.Fatalf("got incorrect output:\nexpected:\n%v\n\ngot:\n%v", expected, result)
	}
}

func TestTableCell(t *testing.T) {
	r := &roffRenderer{}
	buf := bytes.NewBuffer(nil)
	cell := []byte{}

	r.TableCell(buf, cell, 0)
	expected := " "
	if buf.String() != expected {
		t.Fatal("expected %q, got %q", expected, buf.String())
	}

	r.TableCell(buf, cell, 0)
	expected += "\t "
	if buf.String() != expected {
		t.Fatalf("expected %q, got %q", expected, buf.String())
	}

	cell = []byte("*")
	r.TableCell(buf, cell, 0)
	expected += "\t*"
	if buf.String() != expected {
		t.Fatalf("expected %q, got %q", expected, buf.String())
	}

	cell = []byte("this is a test with some really long string")
	r.TableCell(buf, cell, 0)
	expected += "\tT{\nthis is a test with some really long string\nT}"
	if buf.String() != expected {
		t.Fatalf("expected %q, got %q", expected, buf.String())
	}

	cell = []byte("some short string")
	r.TableCell(buf, cell, 0)
	expected += "\tsome short string"
	if buf.String() != expected {
		t.Fatalf("expected %q, got %q", expected, buf.String())
	}

	cell = []byte{}
	r.TableCell(buf, cell, 0)
	expected += "\t "
	if buf.String() != expected {
		t.Fatalf("expected %q, got %q", expected, buf.String())
	}

	cell = []byte("*")
	r.TableCell(buf, cell, 0)
	expected += "\t*"
	if buf.String() != expected {
		t.Fatalf("expected %q, got %q", expected, buf.String())
	}
}
