package ndr

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type Test struct {
	A int `ndr:"value"`
	B int `ndr:"key:value"`
	C int `ndr:"value1,key:value2"`
	D int `dr:"value"`
}

func TestParseTags(t *testing.T) {
	var test Test
	tag0 := reflect.TypeOf(test).Field(0).Tag
	tag1 := reflect.TypeOf(test).Field(1).Tag
	tag2 := reflect.TypeOf(test).Field(2).Tag
	tag3 := reflect.TypeOf(test).Field(3).Tag

	tg0 := parseTags(tag0)
	tg1 := parseTags(tag1)
	tg2 := parseTags(tag2)
	tg3 := parseTags(tag3)

	assert.Equal(t, []string{"value"}, tg0.Values, "Values not as expected for test %d", 0)
	assert.Equal(t, make(map[string]string), tg0.Map, "Map not as expected for test %d", 0)
	assert.Equal(t, []string{}, tg1.Values, "Values not as expected for test %d", 1)
	assert.Equal(t, map[string]string{"key": "value"}, tg1.Map, "Map not as expected for test %d", 1)
	assert.Equal(t, []string{"value1"}, tg2.Values, "Values not as expected for test %d", 2)
	assert.Equal(t, map[string]string{"key": "value2"}, tg2.Map, "Map not as expected for test %d", 2)
	assert.Equal(t, []string{}, tg3.Values, "Values not as expected for test %d", 3)
	assert.Equal(t, make(map[string]string), tg3.Map, "Map not as expected for test %d", 3)
}
