package mstypes

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/jcmturner/rpc.v1/ndr"
)

const (
	TestRPCUnicodeStringBytes = "1200120004000200" + "01000000" + "0900000000000000090000007400650073007400750073006500720031000000"
	TestRPCUnicodeStringValue = "testuser1"
)

type TestRPCUnicodeString struct {
	RPCStr     RPCUnicodeString
	OtherValue uint32
}

func Test_RPCUnicodeString(t *testing.T) {
	a := new(TestRPCUnicodeString)
	hexStr := TestNDRHeader + TestRPCUnicodeStringBytes
	b, _ := hex.DecodeString(hexStr)
	dec := ndr.NewDecoder(bytes.NewReader(b))
	err := dec.Decode(a)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, TestRPCUnicodeStringValue, a.RPCStr.Value, "String value not as expected")
}
