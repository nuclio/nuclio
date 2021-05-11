package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// StringMapToString converts a map of a: x, b: y to a string in the form of "a=x,b=y"
func StringMapToString(source map[string]string) string {
	list := []string{}

	for k, v := range source {
		list = append(list, k+"="+v)
	}

	return strings.Join(list, ",")
}

// StringToStringMap converts a string in the form of a{separator}x,b{separator}y to a map of a: x, b: y,
// inputs source-string & string-separator
func StringToStringMap(source string, separator string) map[string]string {
	separatedString := strings.Split(source, ",")
	result := map[string]string{}

	for _, keyAndValue := range separatedString {
		kv := strings.Split(keyAndValue, separator)

		if len(kv) > 1 {
			result[kv[0]] = kv[1]
		}
	}

	return result
}

// StructureToMap converts a strcuture to a map, flattening all members
func StructureToMap(input interface{}) map[string]interface{} {
	var decodedInput interface{}

	// TODO: find a more elegent mechanism than JSON encode/decode
	encodedInput, _ := json.Marshal(input)
	if err := json.Unmarshal(encodedInput, &decodedInput); err != nil {
		return map[string]interface{}{}
	}

	return decodedInput.(map[string]interface{})
}

// MapInterfaceInterfaceToMapStringInterface recursively converts map[interface{}]interface{} to map[string]interface{}
func MapInterfaceInterfaceToMapStringInterface(mapInterfaceInterface map[interface{}]interface{}) map[string]interface{} {
	stringInterfaceMap := map[string]interface{}{}

	for key, value := range mapInterfaceInterface {

		switch typedValue := value.(type) {
		case map[interface{}]interface{}:
			stringInterfaceMap[key.(string)] = MapInterfaceInterfaceToMapStringInterface(typedValue)
		default:
			stringInterfaceMap[key.(string)] = value
		}
	}

	return stringInterfaceMap
}

// MapToSlice converts {key1: val1, key2: val2 ...} to [key1, val1, key2, val2 ...]
func MapToSlice(m map[string]interface{}) []interface{} {
	out := make([]interface{}, 0, len(m)*2)
	for key, value := range m {
		out = append(out, key)
		out = append(out, value)
	}

	return out
}

// MapStringInterfaceGetIntOrDefault will return the key as an integer or return a default
func MapStringInterfaceGetOrDefault(mapStringInterface map[string]interface{}, key string, defaultValue interface{}) interface{} {

	value, found := mapStringInterface[key]

	// if the key wasn't found, return the default value
	if !found {
		return defaultValue
	}

	// if the default value isn't the same type of the key, return the default
	if reflect.TypeOf(value) != reflect.TypeOf(defaultValue) {
		return defaultValue
	}

	return value
}

func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// Create a string from the given strings map by key and value (unordered)
// If the given map is nil return an empty string
// For example:
// CreateKeyValuePairs(map[string]string{"a_key": "a_val", "b_key": "b_val"}) will return one of these:
// a_key="a_val" || b_key="b_val"
// b_key="b_val" || a_key="a_val"
func CreateKeyValuePairs(m map[string]string) string {
	b := new(bytes.Buffer)
	delimiter := " || "
	for key, value := range m {
		fmt.Fprintf(b, "%s=\"%s\"%s", key, value, delimiter) // nolint: errcheck
	}

	generatedString := b.String()

	if len(generatedString) != 0 {

		// remove last delimiter
		generatedString = generatedString[:len(generatedString)-len(delimiter)]
	}
	return generatedString
}

// returns string to string map if it's not nil. otherwise, return an empty one
func GetStringToStringMapOrEmpty(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}

	return m
}
