package ndr

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadFloat32(t *testing.T) {
	tests := []struct {
		hexStr string
		value  float32
		order  binary.ByteOrder
	}{
		{"3E200000", 0.15625, binary.BigEndian},
		{"00000000", 0.0, binary.BigEndian},
		{"3F800000", 1.0, binary.BigEndian},
		{"BF800000", -1.0, binary.BigEndian},
		{"00000001", 1.4e-45, binary.BigEndian},
		{"00400000", 5.877472e-39, binary.BigEndian},
		{"007FFFFF", 1.1754942e-38, binary.BigEndian},
		{"00800000", 1.1754944e-38, binary.BigEndian},
		{"7F7FFFFF", 3.4028235e38, binary.BigEndian},
		//TODO need some littleendian test vectors
	}
	for i, test := range tests {
		b, _ := hex.DecodeString(test.hexStr)
		//t.Logf("%s %08b\n", test.hexStr,b)
		r := bufio.NewReader(bytes.NewReader(b))
		dec := Decoder{
			r:  r,
			ch: CommonHeader{Endianness: test.order},
		}
		f, err := dec.readFloat32()
		if err != nil {
			t.Errorf("could not read float32 test %d: %v", i, err)
		}
		assert.Equal(t, test.value, f, "float32 not as expect for test %d: %s", i, test.hexStr)
	}
}
