package ndr

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadCommonHeader(t *testing.T) {
	var tests = []struct {
		EncodedHex string
		ExpectFail bool
	}{
		{"01100800cccccccc", false}, // Little Endian
		{"01000008cccccccc", false}, // Big Endian have to change the bytes for the header size? This test vector was artificially created. Need proper test vector
		//{"01100800cccccccc1802000000000000", false},
		//{"01100800cccccccc0002000000000000", false},
		//{"01100800cccccccc0001000000000000", false},
		//{"01100800cccccccce000000000000000", false},
		//{"01100800ccccccccf000000000000000", false},
		//{"01100800cccccccc7801000000000000", false},
		//{"01100800cccccccc4801000000000000", false},
		//{"01100800ccccccccd001000000000000", false},
		{"02100800cccccccc", true}, // Incorrect version
		{"02100900cccccccc", true}, // Incorrect length

	}

	for i, test := range tests {
		b, _ := hex.DecodeString(test.EncodedHex)
		dec := NewDecoder(bytes.NewReader(b))
		err := dec.readCommonHeader()
		if err != nil && !test.ExpectFail {
			t.Errorf("error reading common header of test %d: %v", i, err)
		}
		if err == nil && test.ExpectFail {
			t.Errorf("expected failure on reading common header of test %d: %v", i, err)
		}
	}
}

func TestReadPrivateHeader(t *testing.T) {
	var tests = []struct {
		EncodedHex string
		ExpectFail bool
		Length     int
	}{
		{"01100800cccccccc1802000000000000", false, 536},
		{"01100800cccccccc0002000000000000", false, 512},
		{"01100800cccccccc0001000000000000", false, 256},
		{"01100800ccccccccFF00000000000000", true, 255}, // Length not multiple of 8
		{"01100800cccccccc00010000000000", true, 256},   // Too short

	}

	for i, test := range tests {
		b, _ := hex.DecodeString(test.EncodedHex)
		dec := NewDecoder(bytes.NewReader(b))
		err := dec.readCommonHeader()
		if err != nil {
			t.Errorf("error reading common header of test %d: %v", i, err)
		}
		err = dec.readPrivateHeader()
		if err != nil && !test.ExpectFail {
			t.Errorf("error reading private header of test %d: %v", i, err)
		}
		if err == nil && test.ExpectFail {
			t.Errorf("expected failure on reading private header of test %d: %v", i, err)
		}
		if dec.ph.ObjectBufferLength != uint32(test.Length) {
			t.Errorf("Objectbuffer length expected %d actual %d", test.Length, dec.ph.ObjectBufferLength)
		}
	}
}

type SimpleTest struct {
	A uint32
	B uint32
}

func TestBasicDecode(t *testing.T) {
	hexStr := "01100800cccccccca00400000000000000000200d186660f656ac601"
	b, _ := hex.DecodeString(hexStr)
	ft := new(SimpleTest)
	dec := NewDecoder(bytes.NewReader(b))
	err := dec.Decode(ft)
	if err != nil {
		t.Fatalf("error decoding: %v", err)
	}
	assert.Equal(t, uint32(258377425), ft.A, "Value of field A not as expected")
	assert.Equal(t, uint32(29780581), ft.B, "Value of field B not as expected %d")
}

func TestBasicDecodeOverRun(t *testing.T) {
	hexStr := "01100800cccccccca00400000000000000000200d186660f"
	b, _ := hex.DecodeString(hexStr)
	ft := new(SimpleTest)
	dec := NewDecoder(bytes.NewReader(b))
	err := dec.Decode(ft)
	if err == nil {
		t.Errorf("Expected error for trying to read more than the bytes we have")
	}
}

type testEmbeddingPointer struct {
	A testEmbeddedPointer `ndr:"pointer"`
	B uint32              // 1
}

type testEmbeddedPointer struct {
	C testEmbeddedPointer2 `ndr:"pointer"`
	D uint32               `ndr:"pointer"` // 2
	E uint32               // 3
}

type testEmbeddedPointer2 struct {
	F uint32 `ndr:"pointer"` // 4
	G uint32 // 5
}

func Test_EmbeddedPointers(t *testing.T) {
	hexStr := TestHeader + "00040002" + "01000000" + "00040002" + "00040002" + "03000000" + "00040002" + "05000000" + "04000000" + "02000000"
	b, _ := hex.DecodeString(hexStr)
	ft := new(testEmbeddingPointer)
	dec := NewDecoder(bytes.NewReader(b))
	err := dec.Decode(ft)
	if err != nil {
		t.Fatalf("error decoding: %v", err)
	}
	assert.Equal(t, uint32(1), ft.B)
	assert.Equal(t, uint32(2), ft.A.D)
	assert.Equal(t, uint32(3), ft.A.E)
	assert.Equal(t, uint32(4), ft.A.C.F)
	assert.Equal(t, uint32(5), ft.A.C.G)
}
