package msgpack_test

import (
	"bytes"
	"encoding/hex"
	"reflect"
	"testing"
	"time"

	"github.com/vmihailenco/msgpack"
	"github.com/vmihailenco/msgpack/codes"
)

func init() {
	msgpack.RegisterExt(9, (*ExtTest)(nil))
}

func TestRegisterExtPanic(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("panic expected")
		}
		got := r.(error).Error()
		wanted := "msgpack: ext with id=9 is already registered"
		if got != wanted {
			t.Fatalf("got %q, wanted %q", got, wanted)
		}
	}()
	msgpack.RegisterExt(9, (*ExtTest)(nil))
}

type ExtTest struct {
	S string
}

var _ msgpack.CustomEncoder = (*ExtTest)(nil)
var _ msgpack.CustomDecoder = (*ExtTest)(nil)

func (ext ExtTest) EncodeMsgpack(e *msgpack.Encoder) error {
	return e.EncodeString("hello " + ext.S)
}

func (ext *ExtTest) DecodeMsgpack(d *msgpack.Decoder) error {
	var err error
	ext.S, err = d.DecodeString()
	return err
}

func TestEncodeDecodeExtHeader(t *testing.T) {
	v := &ExtTest{"world"}

	// Marshal using EncodeExtHeader
	var b bytes.Buffer
	enc := msgpack.NewEncoder(&b)
	err := v.EncodeMsgpack(enc)
	if err != nil {
		t.Fatal(err)
	}

	payload := make([]byte, len(b.Bytes()))
	copy(payload, b.Bytes())

	b.Reset()
	enc = msgpack.NewEncoder(&b)
	err = enc.EncodeExtHeader(9, len(payload))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Write(payload); err != nil {
		t.Fatal(err)
	}

	// Unmarshal using generic function
	var dst interface{}
	err = msgpack.Unmarshal(b.Bytes(), &dst)
	if err != nil {
		t.Fatal(err)
	}

	v, ok := dst.(*ExtTest)
	if !ok {
		t.Fatalf("got %#v, wanted ExtTest", dst)
	}

	wanted := "hello world"
	if v.S != wanted {
		t.Fatalf("got %q, wanted %q", v.S, wanted)
	}

	// Unmarshal using DecodeExtHeader
	d := msgpack.NewDecoder(&b)
	typeId, length, err := d.DecodeExtHeader()
	if err != nil {
		t.Fatal(err)
	}

	if typeId != 9 {
		t.Fatalf("got %d, wanted 9", 9)
	}
	if length != len(payload) {
		t.Fatalf("got %d, wanted %d", length, len(payload))
	}

	v = &ExtTest{}
	err = v.DecodeMsgpack(d)
	if err != nil {
		t.Fatal(err)
	}

	if v.S != wanted {
		t.Fatalf("got %q, wanted %q", v.S, wanted)
	}
}

func TestExt(t *testing.T) {
	for _, v := range []interface{}{ExtTest{"world"}, &ExtTest{"world"}} {
		b, err := msgpack.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}

		var dst interface{}
		err = msgpack.Unmarshal(b, &dst)
		if err != nil {
			t.Fatal(err)
		}

		v, ok := dst.(*ExtTest)
		if !ok {
			t.Fatalf("got %#v, wanted ExtTest", dst)
		}

		wanted := "hello world"
		if v.S != wanted {
			t.Fatalf("got %q, wanted %q", v.S, wanted)
		}

		ext := new(ExtTest)
		err = msgpack.Unmarshal(b, ext)
		if err != nil {
			t.Fatal(err)
		}
		if ext.S != wanted {
			t.Fatalf("got %q, wanted %q", ext.S, wanted)
		}
	}
}

func TestUnknownExt(t *testing.T) {
	b := []byte{byte(codes.FixExt1), 2, 0}

	var dst interface{}
	err := msgpack.Unmarshal(b, &dst)
	if err == nil {
		t.Fatalf("got nil, wanted error")
	}
	got := err.Error()
	wanted := "msgpack: unknown ext id=2"
	if got != wanted {
		t.Fatalf("got %q, wanted %q", got, wanted)
	}
}

func TestDecodeExtWithMap(t *testing.T) {
	type S struct {
		I int
	}
	msgpack.RegisterExt(2, S{})

	b, err := msgpack.Marshal(&S{I: 42})
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]interface{}
	if err := msgpack.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}

	wanted := map[string]interface{}{"I": int64(42)}
	if !reflect.DeepEqual(got, wanted) {
		t.Fatalf("got %#v, but wanted %#v", got, wanted)
	}
}

func TestSliceOfTime(t *testing.T) {
	in := []interface{}{time.Now()}
	b, err := msgpack.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}

	var out []interface{}
	err = msgpack.Unmarshal(b, &out)
	if err != nil {
		t.Fatal(err)
	}

	outTime := *out[0].(*time.Time)
	inTime := in[0].(time.Time)
	if outTime.Unix() != inTime.Unix() {
		t.Fatalf("got %v, wanted %v", outTime, inTime)
	}
}

type customPayload struct {
	payload []byte
}

func (cp *customPayload) UnmarshalMsgpack(b []byte) error {
	cp.payload = b
	return nil
}

func TestDecodeCustomPayload(t *testing.T) {
	b, err := hex.DecodeString("c70500c09eec3100")
	if err != nil {
		t.Fatal(err)
	}

	msgpack.RegisterExt(0, (*customPayload)(nil))

	var cp *customPayload
	err = msgpack.Unmarshal(b, &cp)
	if err != nil {
		t.Fatal(err)
	}

	payload := hex.EncodeToString(cp.payload)
	wanted := "c09eec3100"
	if payload != wanted {
		t.Fatalf("got %q, wanted %q", payload, wanted)
	}
}
