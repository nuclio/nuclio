package ndr

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testUnionSelected1Enc    = "0100000001"
	testUnionSelected2Enc    = "020000000200"
	testUnionSelected1NonEnc = "010000000100000001"
	testUnionSelected2NonEnc = "02000000020000000200"
)

type testUnionEncapsulated struct {
	Tag    uint32 `ndr:"unionTag,encapsulated"`
	Value1 uint8  `ndr:"unionField"`
	Value2 uint16 `ndr:"unionField"`
}

type testUnionNonEncapsulated struct {
	Tag    uint32 `ndr:"unionTag"`
	Value1 uint8  `ndr:"unionField"`
	Value2 uint16 `ndr:"unionField"`
}

func (u testUnionEncapsulated) SwitchFunc(tag interface{}) string {
	t := tag.(uint32)
	switch t {
	case 1:
		return "Value1"
	case 2:
		return "Value2"
	}
	return ""
}

func (u testUnionNonEncapsulated) SwitchFunc(tag interface{}) string {
	t := tag.(uint32)
	switch t {
	case 1:
		return "Value1"
	case 2:
		return "Value2"
	}
	return ""
}

func Test_readUnionEncapsulated(t *testing.T) {
	var tests = []struct {
		Hex string
		Tag uint32
		V1  uint8
		V2  uint16
	}{
		{testUnionSelected1Enc, uint32(1), uint8(1), uint16(0)},
		{testUnionSelected2Enc, uint32(2), uint8(0), uint16(2)},
	}

	for i, test := range tests {
		a := new(testUnionEncapsulated)
		hexStr := TestHeader + test.Hex
		b, _ := hex.DecodeString(hexStr)
		dec := NewDecoder(bytes.NewReader(b))
		err := dec.Decode(a)
		if err != nil {
			t.Fatalf("test %d: %v", i+1, err)
		}
		assert.Equal(t, test.Tag, a.Tag, "Tag value not as expected for test: %d", i+1)
		assert.Equal(t, test.V1, a.Value1, "Value1 not as expected for test: %d", i+1)
		assert.Equal(t, test.V2, a.Value2, "Value2 value not as expected for test: %d", i+1)

	}
}

func Test_readUnionNonEncapsulated(t *testing.T) {
	var tests = []struct {
		Hex string
		Tag uint32
		V1  uint8
		V2  uint16
	}{
		{testUnionSelected1NonEnc, uint32(1), uint8(1), uint16(0)},
		{testUnionSelected2NonEnc, uint32(2), uint8(0), uint16(2)},
	}

	for i, test := range tests {
		a := new(testUnionNonEncapsulated)
		hexStr := TestHeader + test.Hex
		b, _ := hex.DecodeString(hexStr)
		dec := NewDecoder(bytes.NewReader(b))
		err := dec.Decode(a)
		if err != nil {
			t.Fatalf("test %d: %v", i+1, err)
		}
		assert.Equal(t, test.Tag, a.Tag, "Tag value not as expected for test: %d", i+1)
		assert.Equal(t, test.V1, a.Value1, "Value1 not as expected for test: %d", i+1)
		assert.Equal(t, test.V2, a.Value2, "Value2 value not as expected for test: %d", i+1)

	}
}
