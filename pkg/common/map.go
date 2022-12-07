/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"encoding/json"
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

// MapStringInterfaceGetOrDefault will return the key as an integer or return a default
func MapStringInterfaceGetOrDefault(mapStringInterface map[string]interface{},
	key string,
	defaultValue interface{}) interface{} {

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

// MapStringStringToMapStringBytesArray converts the string values of a map to byte arrays
// Example: {"a": "b"} -> {"a": []byte("b")}
func MapStringStringToMapStringBytesArray(m map[string]string) map[string][]byte {
	out := map[string][]byte{}

	for key, value := range m {
		out[key] = []byte(value)
	}

	return out
}

func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// GetStringToStringMapOrEmpty returns string to string map if it's not nil. otherwise, return an empty one
func GetStringToStringMapOrEmpty(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}

	return m
}

// GetAttributeRecursivelyFromMapStringInterface iterates over the attributes slice and recursively searches the map,
// returning the last attribute in the slice
func GetAttributeRecursivelyFromMapStringInterface(mapStringInterface map[string]interface{}, attributes []string) map[string]interface{} {

	currentLevel := mapStringInterface
	var ok bool
	for _, attribute := range attributes {
		currentLevel, ok = currentLevel[attribute].(map[string]interface{})
		if !ok {
			return nil
		}
	}

	return currentLevel
}
