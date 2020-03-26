// +build go1.10

package yaml

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestUnmarshalWithTags(t *testing.T) {
	type WithTaggedField struct {
		Field string `json:"field"`
	}

	t.Run("Known tagged field", func(t *testing.T) {
		y := []byte(`field: "hello"`)
		v := WithTaggedField{}
		if err := Unmarshal(y, &v, DisallowUnknownFields); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if v.Field != "hello" {
			t.Errorf("v.Field=%v, want 'hello'", v.Field)
		}

	})
	t.Run("With unknown tagged field", func(t *testing.T) {
		y := []byte(`unknown: "hello"`)
		v := WithTaggedField{}
		err := Unmarshal(y, &v, DisallowUnknownFields)
		if err == nil {
			t.Errorf("want error because of unknown field, got <nil>: v=%#v", v)
		}
	})

}

// TestUnmarshalStrictWithJSONOpts tests that we return an error if there are
// duplicate fields in the YAML input.
func TestUnmarshalStrictWithJSONOpts(t *testing.T) {
	for _, tc := range []struct {
		yaml        []byte
		opts        []JSONOpt
		want        UnmarshalString
		wantErr     string
	}{
		{
			// By default, unknown field is ignored.
			yaml: []byte("a: 1\nunknownField: 2"),
			want: UnmarshalString{A: "1"},
		},
		{
			// Unknown field produces an error with `DisallowUnknownFields` option.
			yaml:        []byte("a: 1\nunknownField: 2"),
			opts:        []JSONOpt{DisallowUnknownFields},
			wantErr:     `unknown field "unknownField"`,
		},
	} {
		po := prettyFunctionName(tc.opts)
		s := UnmarshalString{}
		err := UnmarshalStrict(tc.yaml, &s, tc.opts...)
		if tc.wantErr != "" && err == nil {
			t.Errorf("UnmarshalStrict(%#q, &s, %v) = nil; want error", string(tc.yaml), po)
			continue
		}
		if tc.wantErr == "" && err != nil {
			t.Errorf("UnmarshalStrict(%#q, &s, %#v) = %v; want no error", string(tc.yaml), po, err)
			continue
		}
		// We expect that duplicate fields are discovered during JSON unmarshalling.
		if want := "error unmarshaling JSON"; tc.wantErr != "" && !strings.Contains(err.Error(), want) {
			t.Errorf("UnmarshalStrict(%#q, &s, %#v) = %v; want err contains %#q", string(tc.yaml), po, err, want)
		}
		if tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr) {
			t.Errorf("UnmarshalStrict(%#q, &s, %#v) = %v; want err contains %#q", string(tc.yaml), po, err, tc.wantErr)
		}

		// Only test content of `s` if parsing indicated no error.
		// If we got an error, `s` may be partially parsed and contain some data.
		if err != nil {
			continue
		}

		if !reflect.DeepEqual(s, tc.want) {
			t.Errorf("UnmarshalStrict(%#q, &s, %#v) = %+#v; want %+#v", string(tc.yaml), po, s, tc.want)
		}
	}
}

func ExampleUnknown() {
	type WithTaggedField struct {
		Field string `json:"field"`
	}
	y := []byte(`unknown: "hello"`)
	v := WithTaggedField{}
	fmt.Printf("%v\n", Unmarshal(y, &v, DisallowUnknownFields))
	// Ouptut:
	// unmarshaling JSON: while decoding JSON: json: unknown field "unknown"
}
