package dyno

import (
	"encoding/json"
	"reflect"
	"testing"
)

var (
	s = []interface{}{
		1, "a", 3.3, []interface{}{"inner", "inner2"},
	}
	mi = map[interface{}]interface{}{
		"x": 1,
		"y": 2,
		"z": map[interface{}]interface{}{
			3: "three",
		},
	}
	ms = map[string]interface{}{
		"a": 1,
		"p": map[string]interface{}{
			"x": 1,
			"y": 2,
		},
		"pi":  mi,
		"ns":  []interface{}{1.1, 2.2, 3.3},
		"b":   2,
		"s":   s,
		"str": "abc",
		"flt": 3.14,
	}
)

func TestGet(t *testing.T) {
	cases := []struct {
		title string        // Title of the test case
		v     interface{}   // Input dynamic object
		path  []interface{} // path whose value to get
		value interface{}   // Expected value
		isErr bool          // Tells if error is expected
	}{
		// Test success:
		{
			title: "nil path on map",
			v:     ms,
			path:  nil,
			value: ms,
		},
		{
			title: "nil path on slice",
			v:     s,
			path:  nil,
			value: s,
		},
		{
			title: "simple map element",
			v:     ms,
			path:  []interface{}{"a"},
			value: 1,
		},
		{
			title: "simple map element #2",
			v:     ms,
			path:  []interface{}{"s"},
			value: s,
		},
		{
			title: "nested map element",
			v:     ms,
			path:  []interface{}{"p", "x"},
			value: 1,
		},
		{
			title: "nested map (mi) element",
			v:     ms,
			path:  []interface{}{"pi", "x"},
			value: 1,
		},
		{
			title: "nested map (mi) element #2",
			v:     ms,
			path:  []interface{}{"pi", "z", 3},
			value: "three",
		},
		{
			title: "nested slice element",
			v:     s,
			path:  []interface{}{3, 1},
			value: "inner2",
		},
		{
			title: "map element and slice element",
			v:     ms,
			path:  []interface{}{"ns", 1},
			value: 2.2,
		},
		{
			title: "map element and slice element #2",
			v:     ms,
			path:  []interface{}{"s", 1},
			value: "a",
		},

		// Test errors:
		{
			title: "expected map or slice node error",
			v:     1,
			path:  []interface{}{"x"},
			isErr: true,
		},
		{
			title: "expected string path element error",
			v:     ms,
			path:  []interface{}{1},
			isErr: true,
		},
		{
			title: "missing key (ms) error",
			v:     ms,
			path:  []interface{}{"x"},
			isErr: true,
		},
		{
			title: "missing key (mi) error",
			v:     mi,
			path:  []interface{}{"a"},
			isErr: true,
		},
		{
			title: "expected int path element error",
			v:     ms,
			path:  []interface{}{"ns", "x"},
			isErr: true,
		},
		{
			title: "index out of range error (negative)",
			v:     ms,
			path:  []interface{}{"ns", -1},
			isErr: true,
		},
		{
			title: "index out of range error (too big)",
			v:     ms,
			path:  []interface{}{"ns", 11},
			isErr: true,
		},
	}

	for _, c := range cases {
		value, err := Get(c.v, c.path...)
		if !reflect.DeepEqual(value, c.value) {
			t.Errorf("[title: %s] Expected value: %v, got: %v", c.title, c.value, value)
		}
		if c.isErr != (err != nil) {
			t.Errorf("[title: %s] Expected error: %v, got: %v, err value: %v", c.title, c.isErr, err != nil, err)
		}
	}
}

func TestGetInt(t *testing.T) {
	cases := []struct {
		title string        // Title of the test case
		v     interface{}   // Input dynamic object
		path  []interface{} // path whose value to get
		value int           // Expected value
		isErr bool          // Tells if error is expected
	}{
		// Test success:
		{
			title: "empty path on int",
			v:     1,
			path:  []interface{}{},
			value: 1,
		},
		{
			title: "success",
			v:     ms,
			path:  []interface{}{"a"},
			value: 1,
		},

		// Test errors:
		{
			title: "internal Get call returns error",
			v:     ms,
			path:  []interface{}{"x"},
			value: 0,
			isErr: true,
		},
		{
			title: "expected int error",
			v:     ms,
			path:  []interface{}{"s"},
			value: 0,
			isErr: true,
		},
	}

	for _, c := range cases {
		value, err := GetInt(c.v, c.path...)
		if value != c.value {
			t.Errorf("[title: %s] Expected value: %v, got: %v", c.title, c.value, value)
		}
		if c.isErr != (err != nil) {
			t.Errorf("[title: %s] Expected error: %v, got: %v, err value: %v", c.title, c.isErr, err != nil, err)
		}
	}
}

func TestGetSlice(t *testing.T) {
	cases := []struct {
		title string        // Title of the test case
		v     interface{}   // Input dynamic object
		path  []interface{} // path whose value to get
		value []interface{} // Expected value
		isErr bool          // Tells if error is expected
	}{
		// Test success:
		{
			title: "empty path on slice",
			v:     []interface{}{1},
			path:  []interface{}{},
			value: []interface{}{1},
		},
		{
			title: "success",
			v:     ms,
			path:  []interface{}{"ns"},
			value: ms["ns"].([]interface{}),
		},
		{
			title: "success #2",
			v:     s,
			path:  []interface{}{3},
			value: s[3].([]interface{}),
		},

		// Test errors:
		{
			title: "internal Get call returns error",
			v:     ms,
			path:  []interface{}{"x"},
			value: nil,
			isErr: true,
		},
		{
			title: "expected slice error",
			v:     ms,
			path:  []interface{}{"a"},
			value: nil,
			isErr: true,
		},
	}

	for _, c := range cases {
		value, err := GetSlice(c.v, c.path...)
		if !reflect.DeepEqual(value, c.value) {
			t.Errorf("[title: %s] Expected value: %v, got: %v", c.title, c.value, value)
		}
		if c.isErr != (err != nil) {
			t.Errorf("[title: %s] Expected error: %v, got: %v, err value: %v", c.title, c.isErr, err != nil, err)
		}
	}
}

func TestGetMapI(t *testing.T) {
	cases := []struct {
		title string                      // Title of the test case
		v     interface{}                 // Input dynamic object
		path  []interface{}               // path whose value to get
		value map[interface{}]interface{} // Expected value
		isErr bool                        // Tells if error is expected
	}{
		// Test success:
		{
			title: "empty path on map",
			v:     map[interface{}]interface{}{1: "one"},
			path:  []interface{}{},
			value: map[interface{}]interface{}{1: "one"},
		},
		{
			title: "success",
			v:     mi,
			path:  []interface{}{"z"},
			value: mi["z"].(map[interface{}]interface{}),
		},

		// Test errors:
		{
			title: "internal Get call returns error",
			v:     ms,
			path:  []interface{}{"x"},
			value: nil,
			isErr: true,
		},
		{
			title: "expected map error",
			v:     ms,
			path:  []interface{}{"a"},
			value: nil,
			isErr: true,
		},
	}

	for _, c := range cases {
		value, err := GetMapI(c.v, c.path...)
		if !reflect.DeepEqual(value, c.value) {
			t.Errorf("[title: %s] Expected value: %v, got: %v", c.title, c.value, value)
		}
		if c.isErr != (err != nil) {
			t.Errorf("[title: %s] Expected error: %v, got: %v, err value: %v", c.title, c.isErr, err != nil, err)
		}
	}
}

func TestGetMapS(t *testing.T) {
	cases := []struct {
		title string                 // Title of the test case
		v     interface{}            // Input dynamic object
		path  []interface{}          // path whose value to get
		value map[string]interface{} // Expected value
		isErr bool                   // Tells if error is expected
	}{
		// Test success:
		{
			title: "empty path on map",
			v:     map[string]interface{}{"one": 1},
			path:  []interface{}{},
			value: map[string]interface{}{"one": 1},
		},
		{
			title: "success",
			v:     ms,
			path:  []interface{}{"p"},
			value: ms["p"].(map[string]interface{}),
		},

		// Test errors:
		{
			title: "internal Get call returns error",
			v:     ms,
			path:  []interface{}{"x"},
			value: nil,
			isErr: true,
		},
		{
			title: "expected map error",
			v:     ms,
			path:  []interface{}{"a"},
			value: nil,
			isErr: true,
		},
	}

	for _, c := range cases {
		value, err := GetMapS(c.v, c.path...)
		if !reflect.DeepEqual(value, c.value) {
			t.Errorf("[title: %s] Expected value: %v, got: %v", c.title, c.value, value)
		}
		if c.isErr != (err != nil) {
			t.Errorf("[title: %s] Expected error: %v, got: %v, err value: %v", c.title, c.isErr, err != nil, err)
		}
	}
}

func TestGetInteger(t *testing.T) {
	cases := []struct {
		title string        // Title of the test case
		v     interface{}   // Input dynamic object
		path  []interface{} // path whose value to get
		value int64         // Expected value
		isErr bool          // Tells if error is expected
	}{
		// Test success:
		{
			title: "empty path on int",
			v:     1,
			path:  []interface{}{},
			value: 1,
		},
		{
			title: "success",
			v:     ms,
			path:  []interface{}{"a"},
			value: 1,
		},
		{
			title: "success from int64",
			v:     int64(1),
			path:  []interface{}{},
			value: 1,
		},
		{
			title: "success from int",
			v:     int(1),
			path:  []interface{}{},
			value: 1,
		},
		{
			title: "success from int32",
			v:     int32(1),
			path:  []interface{}{},
			value: 1,
		},
		{
			title: "success from int16",
			v:     int16(1),
			path:  []interface{}{},
			value: 1,
		},
		{
			title: "success from int8",
			v:     int8(1),
			path:  []interface{}{},
			value: 1,
		},
		{
			title: "success from uint",
			v:     uint(1),
			path:  []interface{}{},
			value: 1,
		},
		{
			title: "success from uint64",
			v:     uint64(1),
			path:  []interface{}{},
			value: 1,
		},
		{
			title: "success from uint32",
			v:     uint32(1),
			path:  []interface{}{},
			value: 1,
		},
		{
			title: "success from uint16",
			v:     uint16(1),
			path:  []interface{}{},
			value: 1,
		},
		{
			title: "success from uint8",
			v:     uint8(1),
			path:  []interface{}{},
			value: 1,
		},
		{
			title: "success from float64",
			v:     float64(1.1),
			path:  []interface{}{},
			value: 1,
		},
		{
			title: "success from float32",
			v:     float32(1.1),
			path:  []interface{}{},
			value: 1,
		},
		{
			title: "success from string",
			v:     "1",
			path:  []interface{}{},
			value: 1,
		},
		{
			title: "success from Int64() method",
			v:     json.Number("1"),
			path:  []interface{}{},
			value: 1,
		},

		// Test errors:
		{
			title: "internal Get call returns error",
			v:     ms,
			path:  []interface{}{"x"},
			value: 0,
			isErr: true,
		},
		{
			title: "expected some form of integer error",
			v:     ms,
			path:  []interface{}{"s"},
			value: 0,
			isErr: true,
		},
	}

	for _, c := range cases {
		value, err := GetInteger(c.v, c.path...)
		if value != c.value {
			t.Errorf("[title: %s] Expected value: %v, got: %v", c.title, c.value, value)
		}
		if c.isErr != (err != nil) {
			t.Errorf("[title: %s] Expected error: %v, got: %v, err value: %v", c.title, c.isErr, err != nil, err)
		}
	}
}

func TestGetFloat64(t *testing.T) {
	cases := []struct {
		title string        // Title of the test case
		v     interface{}   // Input dynamic object
		path  []interface{} // path whose value to get
		value float64       // Expected value
		isErr bool          // Tells if error is expected
	}{
		// Test success:
		{
			title: "empty path on float64",
			v:     3.14,
			path:  []interface{}{},
			value: 3.14,
		},
		{
			title: "success",
			v:     ms,
			path:  []interface{}{"flt"},
			value: 3.14,
		},

		// Test errors:
		{
			title: "internal Get call returns error",
			v:     ms,
			path:  []interface{}{"x"},
			value: 0,
			isErr: true,
		},
		{
			title: "expected float64 error",
			v:     ms,
			path:  []interface{}{"s"},
			value: 0,
			isErr: true,
		},
	}

	for _, c := range cases {
		value, err := GetFloat64(c.v, c.path...)
		if value != c.value {
			t.Errorf("[title: %s] Expected value: %v, got: %v", c.title, c.value, value)
		}
		if c.isErr != (err != nil) {
			t.Errorf("[title: %s] Expected error: %v, got: %v, err value: %v", c.title, c.isErr, err != nil, err)
		}
	}
}

func TestGetFloating(t *testing.T) {
	cases := []struct {
		title string        // Title of the test case
		v     interface{}   // Input dynamic object
		path  []interface{} // path whose value to get
		value float64       // Expected value
		isErr bool          // Tells if error is expected
	}{
		// Test success:
		{
			title: "empty path on int",
			v:     1,
			path:  []interface{}{},
			value: 1,
		},
		{
			title: "success",
			v:     ms,
			path:  []interface{}{"a"},
			value: 1,
		},
		{
			title: "success from float64",
			v:     float64(1.1),
			path:  []interface{}{},
			value: 1.1,
		},
		{
			title: "success from float32",
			v:     float32(1.1),
			path:  []interface{}{},
			value: float64(float32(1.1)),
		},
		{
			title: "success from int64",
			v:     int64(1),
			path:  []interface{}{},
			value: 1,
		},
		{
			title: "success from int",
			v:     int(1),
			path:  []interface{}{},
			value: 1,
		},
		{
			title: "success from int32",
			v:     int32(1),
			path:  []interface{}{},
			value: 1,
		},
		{
			title: "success from int16",
			v:     int16(1),
			path:  []interface{}{},
			value: 1,
		},
		{
			title: "success from int8",
			v:     int8(1),
			path:  []interface{}{},
			value: 1,
		},
		{
			title: "success from uint",
			v:     uint(1),
			path:  []interface{}{},
			value: 1,
		},
		{
			title: "success from uint64",
			v:     uint64(1),
			path:  []interface{}{},
			value: 1,
		},
		{
			title: "success from uint32",
			v:     uint32(1),
			path:  []interface{}{},
			value: 1,
		},
		{
			title: "success from uint16",
			v:     uint16(1),
			path:  []interface{}{},
			value: 1,
		},
		{
			title: "success from uint8",
			v:     uint8(1),
			path:  []interface{}{},
			value: 1,
		},
		{
			title: "success from string",
			v:     "1.1",
			path:  []interface{}{},
			value: 1.1,
		},
		{
			title: "success from Float64() method",
			v:     json.Number("1.1"),
			path:  []interface{}{},
			value: 1.1,
		},

		// Test errors:
		{
			title: "internal Get call returns error",
			v:     ms,
			path:  []interface{}{"x"},
			value: 0,
			isErr: true,
		},
		{
			title: "expected some form of floating point error",
			v:     ms,
			path:  []interface{}{"s"},
			value: 0,
			isErr: true,
		},
	}

	for _, c := range cases {
		value, err := GetFloating(c.v, c.path...)
		if value != c.value {
			t.Errorf("[title: %s] Expected value: %v, got: %v", c.title, c.value, value)
		}
		if c.isErr != (err != nil) {
			t.Errorf("[title: %s] Expected error: %v, got: %v, err value: %v", c.title, c.isErr, err != nil, err)
		}
	}
}

func TestGetString(t *testing.T) {
	cases := []struct {
		title string        // Title of the test case
		v     interface{}   // Input dynamic object
		path  []interface{} // path whose value to get
		value string        // Expected value
		isErr bool          // Tells if error is expected
	}{
		// Test success:
		{
			title: "empty path on string",
			v:     "a",
			path:  []interface{}{},
			value: "a",
		},
		{
			title: "success",
			v:     ms,
			path:  []interface{}{"str"},
			value: "abc",
		},

		// Test errors:
		{
			title: "internal Get call returns error",
			v:     ms,
			path:  []interface{}{"x"},
			value: "",
			isErr: true,
		},
		{
			title: "expected string error",
			v:     ms,
			path:  []interface{}{"a"},
			value: "",
			isErr: true,
		},
	}

	for _, c := range cases {
		value, err := GetString(c.v, c.path...)
		if value != c.value {
			t.Errorf("[title: %s] Expected value: %v, got: %v", c.title, c.value, value)
		}
		if c.isErr != (err != nil) {
			t.Errorf("[title: %s] Expected error: %v, got: %v, err value: %v", c.title, c.isErr, err != nil, err)
		}
	}
}

func TestGetBoolean(t *testing.T) {
	cases := []struct {
		title string        // Title of the test case
		v     interface{}   // Input dynamic object
		path  []interface{} // path whose value to get
		value bool          // Expected value
		isErr bool          // Tells if error is expected
	}{
		// Test success:
		{
			title: "empty path on int",
			v:     1,
			path:  []interface{}{},
			value: true,
		},
		{
			title: "success",
			v:     ms,
			path:  []interface{}{"a"},
			value: true,
		},
		{
			title: "success from bool",
			v:     bool(true),
			path:  []interface{}{},
			value: true,
		},
		{
			title: "success from float64",
			v:     float64(1.1),
			path:  []interface{}{},
			value: true,
		},
		{
			title: "success from float32",
			v:     float32(1.1),
			path:  []interface{}{},
			value: true,
		},
		{
			title: "success from int64",
			v:     int64(1),
			path:  []interface{}{},
			value: true,
		},
		{
			title: "success from int",
			v:     int(1),
			path:  []interface{}{},
			value: true,
		},
		{
			title: "success from int32",
			v:     int32(1),
			path:  []interface{}{},
			value: true,
		},
		{
			title: "success from int16",
			v:     int16(1),
			path:  []interface{}{},
			value: true,
		},
		{
			title: "success from int8",
			v:     int8(1),
			path:  []interface{}{},
			value: true,
		},
		{
			title: "success from uint",
			v:     uint(1),
			path:  []interface{}{},
			value: true,
		},
		{
			title: "success from uint64",
			v:     uint64(1),
			path:  []interface{}{},
			value: true,
		},
		{
			title: "success from uint32",
			v:     uint32(1),
			path:  []interface{}{},
			value: true,
		},
		{
			title: "success from uint16",
			v:     uint16(1),
			path:  []interface{}{},
			value: true,
		},
		{
			title: "success from uint8",
			v:     uint8(1),
			path:  []interface{}{},
			value: true,
		},
		{
			title: "success from string (true)",
			v:     "true",
			path:  []interface{}{},
			value: true,
		},
		{
			title: "success from string (TRUE)",
			v:     "TRUE",
			path:  []interface{}{},
			value: true,
		},
		{
			title: "success from string (False)",
			v:     "False",
			path:  []interface{}{},
			value: false,
		},
		{
			title: "success from string (1)",
			v:     "1",
			path:  []interface{}{},
			value: true,
		},
		{
			title: "success from string (false)",
			v:     "0",
			path:  []interface{}{},
			value: false,
		},
		{
			title: "success from string 1.1",
			v:     string("1.1"),
			path:  []interface{}{},
			value: true,
		},
		{
			title: "success from string 0",
			v:     string("0"),
			path:  []interface{}{},
			value: false,
		},
		{
			title: "success from json.Number method",
			v:     json.Number("1.1"),
			path:  []interface{}{},
			value: true,
		},

		// Test errors:
		{
			title: "internal Get call returns error",
			v:     ms,
			path:  []interface{}{"x"},
			value: false,
			isErr: true,
		},
		{
			title: "expected some form of floating point error",
			v:     ms,
			path:  []interface{}{"s"},
			value: false,
			isErr: true,
		},
		{
			title: "some decimal number as string",
			v:     json.Number("0xff"),
			path:  []interface{}{},
			value: false,
			isErr: true,
		},
	}

	for _, c := range cases {
		value, err := GetBoolean(c.v, c.path...)
		if value != c.value {
			t.Errorf("[title: %s] Expected value: %v, got: %v", c.title, c.value, value)
		}
		if c.isErr != (err != nil) {
			t.Errorf("[title: %s] Expected error: %v, got: %v, err value: %v", c.title, c.isErr, err != nil, err)
		}
	}
}

func TestSGet(t *testing.T) {
	cases := []struct {
		title string                 // Title of the test case
		v     map[string]interface{} // Input map
		path  []string               // path whose value to get
		value interface{}            // Expected value
		isErr bool                   // Tells if error is expected
	}{
		// Test success:
		{
			title: "nil path on map",
			v:     ms,
			path:  nil,
			value: ms,
		},
		{
			title: "empty path on map",
			v:     ms,
			path:  []string{},
			value: ms,
		},
		{
			title: "simple map element",
			v:     ms,
			path:  []string{"a"},
			value: 1,
		},
		{
			title: "simple map element #2",
			v:     ms,
			path:  []string{"s"},
			value: s,
		},
		{
			title: "nested map element",
			v:     ms,
			path:  []string{"p", "x"},
			value: 1,
		},

		// Test errors:
		{
			title: "missing key error",
			v:     ms,
			path:  []string{"x"},
			isErr: true,
		},
		{
			title: "expected map with string keys node error",
			v:     ms,
			path:  []string{"pi", "x"},
			isErr: true,
		},
		{
			title: "expected map with string keys node error #2",
			v:     ms,
			path:  []string{"ns", "1"},
			isErr: true,
		},
	}

	for _, c := range cases {
		value, err := SGet(c.v, c.path...)
		if !reflect.DeepEqual(value, c.value) {
			t.Errorf("[title: %s] Expected value: %v, got: %v", c.title, c.value, value)
		}
		if c.isErr != (err != nil) {
			t.Errorf("[title: %s] Expected error: %v, got: %v, err value: %v", c.title, c.isErr, err != nil, err)
		}
	}
}

func TestSet(t *testing.T) {
	cases := []struct {
		title string        // Title of the test case
		v     interface{}   // Input dynamic object
		value interface{}   // Value to set
		path  []interface{} // path whose value to set
		exp   interface{}   // Expected result
		isErr bool          // Tells if error is expected
	}{
		// Test success:
		{
			title: "add new map element",
			v:     map[string]interface{}{},
			value: 1,
			path:  []interface{}{"a"},
			exp:   map[string]interface{}{"a": 1},
		},
		{
			title: "change existing map element",
			v:     map[string]interface{}{"a": 1},
			value: 2,
			path:  []interface{}{"a"},
			exp:   map[string]interface{}{"a": 2},
		},
		{
			title: "change existing slice element",
			v:     []interface{}{"a", 1},
			value: 2,
			path:  []interface{}{1},
			exp:   []interface{}{"a", 2},
		},
		{
			title: "change existing map (mi) element",
			v:     map[interface{}]interface{}{1: "one"},
			value: "two",
			path:  []interface{}{1},
			exp:   map[interface{}]interface{}{1: "two"},
		},
		{
			title: "change existing nested map element",
			v: map[string]interface{}{
				"a": map[string]interface{}{"b": 1},
			},
			value: 2,
			path:  []interface{}{"a", "b"},
			exp: map[string]interface{}{
				"a": map[string]interface{}{"b": 2},
			},
		},
		{
			title: "replace existing element with a value of different type",
			v: map[string]interface{}{
				"a": map[string]interface{}{"b": 1},
			},
			value: 2,
			path:  []interface{}{"a"},
			exp:   map[string]interface{}{"a": 2},
		},
		{
			title: "change existing element in map-slice-map",
			v: map[string]interface{}{
				"a": []interface{}{
					map[string]interface{}{"b": 1},
				},
			},
			value: 2,
			path:  []interface{}{"a", 0, "b"},
			exp: map[string]interface{}{
				"a": []interface{}{
					map[string]interface{}{"b": 2},
				},
			},
		},

		// Test errors:
		{
			title: "path cannot be empty error",
			v:     map[string]interface{}{"a": 1},
			value: 2,
			path:  []interface{}{},
			exp:   map[string]interface{}{"a": 1},
			isErr: true,
		},
		{
			title: "internal Get call returns error",
			v:     map[string]interface{}{"a": 1},
			value: 2,
			path:  []interface{}{"b", "c"},
			exp:   map[string]interface{}{"a": 1},
			isErr: true,
		},
		{
			title: "expected string path element error",
			v:     map[string]interface{}{"a": 1},
			value: 2,
			path:  []interface{}{1},
			exp:   map[string]interface{}{"a": 1},
			isErr: true,
		},
		{
			title: "expected int path element error",
			v:     []interface{}{"a", 1},
			value: 2,
			path:  []interface{}{"a"},
			exp:   []interface{}{"a", 1},
			isErr: true,
		},
		{
			title: "index out of range error (negative)",
			v:     []interface{}{"a", 1},
			value: 2,
			path:  []interface{}{-1},
			exp:   []interface{}{"a", 1},
			isErr: true,
		},
		{
			title: "index out of range error (too big)",
			v:     []interface{}{"a", 1},
			value: 2,
			path:  []interface{}{11},
			exp:   []interface{}{"a", 1},
			isErr: true,
		},
		{
			title: "expected map or slice node error",
			v:     1,
			value: 2,
			path:  []interface{}{"x"},
			exp:   1,
			isErr: true,
		},
	}

	for _, c := range cases {
		err := Set(c.v, c.value, c.path...)
		if !reflect.DeepEqual(c.v, c.exp) {
			t.Errorf("[title: %s] Expected value: %v, got: %v", c.title, c.exp, c.v)
		}
		if c.isErr != (err != nil) {
			t.Errorf("[title: %s] Expected error: %v, got: %v, err value: %v", c.title, c.isErr, err != nil, err)
		}
	}
}

func TestSSet(t *testing.T) {
	cases := []struct {
		title string                 // Title of the test case
		m     map[string]interface{} // Input map
		value interface{}            // Value to set
		path  []string               // path whose value to set
		exp   map[string]interface{} // Expected result
		isErr bool                   // Tells if error is expected
	}{
		// Test success:
		{
			title: "add new map element",
			m:     map[string]interface{}{},
			value: 1,
			path:  []string{"a"},
			exp:   map[string]interface{}{"a": 1},
		},
		{
			title: "change existing map element",
			m:     map[string]interface{}{"a": 1},
			value: 2,
			path:  []string{"a"},
			exp:   map[string]interface{}{"a": 2},
		},
		{
			title: "change existing nested map element",
			m: map[string]interface{}{
				"a": map[string]interface{}{"b": 1},
			},
			value: 2,
			path:  []string{"a", "b"},
			exp: map[string]interface{}{
				"a": map[string]interface{}{"b": 2},
			},
		},
		{
			title: "replace existing element with a value of different type",
			m: map[string]interface{}{
				"a": map[string]interface{}{"b": 1},
			},
			value: 2,
			path:  []string{"a"},
			exp:   map[string]interface{}{"a": 2},
		},

		// Test errors:
		{
			title: "path cannot be empty error",
			m:     map[string]interface{}{"a": 1},
			value: 2,
			path:  []string{},
			exp:   map[string]interface{}{"a": 1},
			isErr: true,
		},
		{
			title: "internal SGet call returns error",
			m:     map[string]interface{}{"a": 1},
			value: 2,
			path:  []string{"b", "c"},
			exp:   map[string]interface{}{"a": 1},
			isErr: true,
		},
		{
			title: "expected map with string keys node error",
			m: map[string]interface{}{
				"a": 1,
			},
			value: 2,
			path:  []string{"a", "b"},
			exp: map[string]interface{}{
				"a": 1,
			},
			isErr: true,
		},
	}

	for _, c := range cases {
		err := SSet(c.m, c.value, c.path...)
		if !reflect.DeepEqual(c.m, c.exp) {
			t.Errorf("[title: %s] Expected value: %v, got: %v", c.title, c.exp, c.m)
		}
		if c.isErr != (err != nil) {
			t.Errorf("[title: %s] Expected error: %v, got: %v, err value: %v", c.title, c.isErr, err != nil, err)
		}
	}
}

func TestAppend(t *testing.T) {
	cases := []struct {
		title string        // Title of the test case
		v     interface{}   // Input dynamic object
		value interface{}   // Value to append
		path  []interface{} // path of slice to append to
		exp   interface{}   // Expected result
		isErr bool          // Tells if error is expected
	}{
		// Test success:
		{
			title: "append to nil slice",
			v: map[string]interface{}{
				"a": []interface{}(nil),
			},
			value: 1,
			path:  []interface{}{"a"},
			exp: map[string]interface{}{
				"a": []interface{}{1},
			},
		},
		{
			title: "append to slice",
			v: map[string]interface{}{
				"a": []interface{}{"b", 3.3},
			},
			value: 1,
			path:  []interface{}{"a"},
			exp: map[string]interface{}{
				"a": []interface{}{"b", 3.3, 1},
			},
		},

		// Test errors:
		{
			title: "path cannot be empty error",
			v: map[string]interface{}{
				"a": []interface{}{"b", 3.3},
			},
			value: 1,
			path:  []interface{}{},
			exp: map[string]interface{}{
				"a": []interface{}{"b", 3.3},
			},
			isErr: true,
		},
		{
			title: "internal Get call returns error",
			v: map[string]interface{}{
				"a": []interface{}{"b", 3.3},
			},
			value: 1,
			path:  []interface{}{"b"},
			exp: map[string]interface{}{
				"a": []interface{}{"b", 3.3},
			},
			isErr: true,
		},
		{
			title: "expected slice node error",
			v: map[string]interface{}{
				"a": []interface{}{"b", 3.3},
				"b": 1,
			},
			value: 1,
			path:  []interface{}{"b"},
			exp: map[string]interface{}{
				"a": []interface{}{"b", 3.3},
				"b": 1,
			},
			isErr: true,
		},
	}

	for _, c := range cases {
		err := Append(c.v, c.value, c.path...)
		if !reflect.DeepEqual(c.v, c.exp) {
			t.Errorf("[title: %s] Expected value: %v, got: %v", c.title, c.exp, c.v)
		}
		if c.isErr != (err != nil) {
			t.Errorf("[title: %s] Expected error: %v, got: %v, err value: %v", c.title, c.isErr, err != nil, err)
		}
	}
}

func TestAppendMore(t *testing.T) {
	cases := []struct {
		title  string        // Title of the test case
		v      interface{}   // Input dynamic object
		values []interface{} // Values to append
		path   []interface{} // path of slice to append to
		exp    interface{}   // Expected result
		isErr  bool          // Tells if error is expected
	}{
		// Test success:
		{
			title: "append to nil slice",
			v: map[string]interface{}{
				"a": []interface{}(nil),
			},
			values: []interface{}{1},
			path:   []interface{}{"a"},
			exp: map[string]interface{}{
				"a": []interface{}{1},
			},
		},
		{
			title: "append nil slice to slice",
			v: map[string]interface{}{
				"a": []interface{}{"b", 3.3},
			},
			values: nil,
			path:   []interface{}{"a"},
			exp: map[string]interface{}{
				"a": []interface{}{"b", 3.3},
			},
		},
		{
			title: "append to slice",
			v: map[string]interface{}{
				"a": []interface{}{"b", 3.3},
			},
			values: []interface{}{1},
			path:   []interface{}{"a"},
			exp: map[string]interface{}{
				"a": []interface{}{"b", 3.3, 1},
			},
		},
		{
			title: "append multiple values to slice",
			v: map[string]interface{}{
				"a": []interface{}{"b", 3.3},
			},
			values: []interface{}{1, []interface{}{2}},
			path:   []interface{}{"a"},
			exp: map[string]interface{}{
				"a": []interface{}{"b", 3.3, 1, []interface{}{2}},
			},
		},

		// Test errors:
		{
			title: "path cannot be empty error",
			v: map[string]interface{}{
				"a": []interface{}{"b", 3.3},
			},
			values: []interface{}{1},
			path:   []interface{}{},
			exp: map[string]interface{}{
				"a": []interface{}{"b", 3.3},
			},
			isErr: true,
		},
		{
			title: "internal Get call returns error",
			v: map[string]interface{}{
				"a": []interface{}{"b", 3.3},
			},
			values: []interface{}{1},
			path:   []interface{}{"b"},
			exp: map[string]interface{}{
				"a": []interface{}{"b", 3.3},
			},
			isErr: true,
		},
		{
			title: "expected slice node error",
			v: map[string]interface{}{
				"a": []interface{}{"b", 3.3},
				"b": 1,
			},
			values: []interface{}{1},
			path:   []interface{}{"b"},
			exp: map[string]interface{}{
				"a": []interface{}{"b", 3.3},
				"b": 1,
			},
			isErr: true,
		},
	}

	for _, c := range cases {
		err := AppendMore(c.v, c.values, c.path...)
		if !reflect.DeepEqual(c.v, c.exp) {
			t.Errorf("[title: %s] Expected value: %v, got: %v", c.title, c.exp, c.v)
		}
		if c.isErr != (err != nil) {
			t.Errorf("[title: %s] Expected error: %v, got: %v, err value: %v", c.title, c.isErr, err != nil, err)
		}
	}
}

func TestDelete(t *testing.T) {
	cases := []struct {
		title string        // Title of the test case
		v     interface{}   // Input dynamic object
		key   interface{}   // Key (or index) to delete
		path  []interface{} // path whose key or element to remove
		exp   interface{}   // Expected result
		isErr bool          // Tells if error is expected
	}{
		// Test success:
		{
			title: "delete non-existing map key with empty path",
			v:     map[string]interface{}{"a": 1},
			key:   "x",
			path:  []interface{}{},
			exp:   map[string]interface{}{"a": 1},
		},
		{
			title: "delete non-existing map key",
			v:     map[string]interface{}{"a": map[string]interface{}{"b": 1}},
			key:   "x",
			path:  []interface{}{"a"},
			exp:   map[string]interface{}{"a": map[string]interface{}{"b": 1}},
		},
		{
			title: "delete existing map element",
			v:     map[string]interface{}{"a": 1, "b": 2},
			key:   "a",
			path:  []interface{}{},
			exp:   map[string]interface{}{"b": 2},
		},
		{
			title: "delete slice element",
			v:     map[string]interface{}{"a": []interface{}{"b", 1}},
			key:   0,
			path:  []interface{}{"a"},
			exp:   map[string]interface{}{"a": []interface{}{1}},
		},
		{
			title: "delete nested slice element",
			v:     []interface{}{"a", []interface{}{"b", 1}},
			key:   0,
			path:  []interface{}{1},
			exp:   []interface{}{"a", []interface{}{1}},
		},
		{
			title: "delete non-existing map (mi) key with empty path",
			v:     map[interface{}]interface{}{1: "a"},
			key:   "x",
			path:  []interface{}{},
			exp:   map[interface{}]interface{}{1: "a"},
		},
		{
			title: "delete non-existing map (mi) key",
			v:     map[interface{}]interface{}{1: map[interface{}]interface{}{2: "b"}},
			key:   "x",
			path:  []interface{}{1},
			exp:   map[interface{}]interface{}{1: map[interface{}]interface{}{2: "b"}},
		},
		{
			title: "delete existing map (mi) element",
			v:     map[interface{}]interface{}{1: "a", 2: "b"},
			key:   1,
			path:  []interface{}{},
			exp:   map[interface{}]interface{}{2: "b"},
		},

		// Test errors:
		{
			title: "path cannot be empty if v is a slice",
			v:     []interface{}{1, "a"},
			key:   0,
			path:  []interface{}{},
			exp:   []interface{}{1, "a"},
			isErr: true,
		},
		{
			title: "internal Get call returns error",
			v:     map[string]interface{}{"a": 1},
			key:   2,
			path:  []interface{}{"b", "c"},
			exp:   map[string]interface{}{"a": 1},
			isErr: true,
		},
		{
			title: "expected string key error",
			v:     map[string]interface{}{"a": 1},
			key:   2,
			path:  []interface{}{},
			exp:   map[string]interface{}{"a": 1},
			isErr: true,
		},
		{
			title: "expected int key error",
			v:     map[string]interface{}{"a": []interface{}{"b", 1}},
			key:   "b",
			path:  []interface{}{"a"},
			exp:   map[string]interface{}{"a": []interface{}{"b", 1}},
			isErr: true,
		},
		{
			title: "index out of range error (negative)",
			v:     map[string]interface{}{"a": []interface{}{"b", 1}},
			key:   -1,
			path:  []interface{}{"a"},
			exp:   map[string]interface{}{"a": []interface{}{"b", 1}},
			isErr: true,
		},
		{
			title: "index out of range error (too big)",
			v:     map[string]interface{}{"a": []interface{}{"b", 1}},
			key:   2,
			path:  []interface{}{"a"},
			exp:   map[string]interface{}{"a": []interface{}{"b", 1}},
			isErr: true,
		},
		{
			title: "expected map or slice node error",
			v:     map[string]interface{}{"a": 1},
			key:   2,
			path:  []interface{}{"a"},
			exp:   map[string]interface{}{"a": 1},
			isErr: true,
		},
	}

	for _, c := range cases {
		err := Delete(c.v, c.key, c.path...)
		if !reflect.DeepEqual(c.v, c.exp) {
			t.Errorf("[title: %s] Expected value: %v, got: %v", c.title, c.exp, c.v)
		}
		if c.isErr != (err != nil) {
			t.Errorf("[title: %s] Expected error: %v, got: %v, err value: %v", c.title, c.isErr, err != nil, err)
		}
	}
}

func TestConvertMapI2MapS(t *testing.T) {
	cases := []struct {
		title string      // Title of the test case
		v     interface{} // Input dynamic object
		exp   interface{} // Expected result
	}{
		{
			title: "nil value",
			v:     nil,
			exp:   nil,
		},
		{
			title: "string value",
			v:     "a",
			exp:   "a",
		},
		{
			title: "map[interfac{}]interface{} value",
			v: map[interface{}]interface{}{
				"s": "s",
				1:   1,
			},
			exp: map[string]interface{}{
				"s": "s",
				"1": 1,
			},
		},
		{
			title: "nested maps and slices",
			v: map[interface{}]interface{}{
				"s": "s",
				1:   1,
				float64(0): []interface{}{
					1,
					"x",
					map[interface{}]interface{}{
						"s": "s",
						2.0: 2,
					},
					map[string]interface{}{
						"s": "s",
						"1": 1,
					},
				},
			},
			exp: map[string]interface{}{
				"s": "s",
				"1": 1,
				"0": []interface{}{
					1,
					"x",
					map[string]interface{}{
						"s": "s",
						"2": 2,
					},
					map[string]interface{}{
						"s": "s",
						"1": 1,
					},
				},
			},
		},
	}

	for _, c := range cases {
		v := ConvertMapI2MapS(c.v)
		if !reflect.DeepEqual(v, c.exp) {
			t.Errorf("[title: %s] Expected value: %v, got: %v", c.title, c.exp, c.v)
		}
	}
}
