package mstypes

import (
	"bytes"
	"encoding/hex"
	"github.com/stretchr/testify/assert"
	"gopkg.in/jcmturner/rpc.v1/ndr"
	"testing"
	"time"
)

const TestNDRHeader = "01100800cccccccca00400000000000000000200"

func TestFileTime(t *testing.T) {
	t.Parallel()
	//2007-02-22 17:00:01.6382155
	tt := time.Date(2007, 2, 22, 17, 0, 1, 638215500, time.UTC)
	ft := GetFileTime(tt)
	assert.Equal(t, tt.Unix(), ft.Unix(), "Unix epoch time not as expected")
	assert.Equal(t, int64(128166372016382155), ft.MSEpoch(), "MSEpoch not as expected")
	assert.Equal(t, tt, ft.Time(), "Golang time object returned from FileTime not as expected")
}

func TestDecodeFileTime(t *testing.T) {
	var tests = []struct {
		Hex      string
		UnixNano int64
	}{
		{"d186660f656ac601", 1146188570925640100},
		{"17d439fe784ac601", 1142678694837147900},
		{"1794a328424bc601", 1142765094837147900},
		{"175424977a81c601", 1148726694837147900},
		{"058e4fdd80c6d201", 1494085991825766900},
		{"cc27969c39c6d201", 1494055388968750000},
		{"cce7ffc602c7d201", 1494141788968750000},
		{"c30bcc79e444d301", 1507982621052409900},
		{"c764125a0842d301", 1507668176220282300},
		{"c7247c84d142d301", 1507754576220282300},
	}

	for i, test := range tests {
		a := new(FileTime)
		hexStr := TestNDRHeader + test.Hex
		b, _ := hex.DecodeString(hexStr)
		dec := ndr.NewDecoder(bytes.NewReader(b))
		err := dec.Decode(a)
		if err != nil {
			t.Fatalf("test %d: %v", i+1, err)
		}
		assert.Equal(t, test.UnixNano, a.Time().UnixNano(), "Time value not as expected for test: %d", i+1)
	}
}
