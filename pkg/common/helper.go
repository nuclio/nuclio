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
	"os"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/spf13/viper"
)

// ByteArrayToString converts a byte array to a string
func ByteArrayToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// StringToStringMap converts a map of a: x, b: y to a string in the form of "a=x,b=y"
func StringMapToString(source map[string]string) string {
	list := []string{}

	for k, v := range source {
		list = append(list, k+"="+v)
	}

	return strings.Join(list, ",")
}

// StringToStringMap converts a string in the form of a=x,b=y to a map of a: x, b: y
func StringToStringMap(source string) map[string]string {
	separatedString := strings.Split(source, ",")
	result := map[string]string{}

	for _, keyAndValie := range separatedString {
		kv := strings.Split(keyAndValie, "=")

		if len(kv) > 1 {
			result[kv[0]] = kv[1]
		}
	}

	return result
}

// GetObjectSlice extracts a list of objects from a viper instance. there may be a better way to do this with viper
// but i've yet to find it (TODO: post issue?)
func GetObjectSlice(configuration *viper.Viper, key string) []map[string]interface{} {
	objectsAsMapStringInterface := []map[string]interface{}{}

	keyValue := configuration.Get(key)
	if keyValue == nil {
		return []map[string]interface{}{}
	}

	// get as slice of interfaces
	objectsAsInterfaces := keyValue.([]interface{})

	// iterate over objects as interfaces
	for _, objectAsInterface := range objectsAsInterfaces {
		objectAsMapStringInterface := map[string]interface{}{}

		// convert each object to a map of its fields (interface/interface)
		objectFieldsAsMapInterfaceInterface := objectAsInterface.(map[interface{}]interface{})

		// iterate over fields, convert key to string and keep value as interface, shove to
		// objectAsMapStringInterface
		for objectFieldKey, objectFieldValue := range objectFieldsAsMapInterfaceInterface {
			objectAsMapStringInterface[objectFieldKey.(string)] = objectFieldValue
		}

		// add object to map
		objectsAsMapStringInterface = append(objectsAsMapStringInterface, objectAsMapStringInterface)
	}

	return objectsAsMapStringInterface
}

// StructureToMap converts a strcuture to a map, flattening all members
func StructureToMap(input interface{}) map[string]interface{} {
	var decodedInput interface{}

	// TODO: find a more elegent mechanism than JSON encode/decode
	encodedInput, _ := json.Marshal(input)
	json.Unmarshal(encodedInput, &decodedInput)

	return decodedInput.(map[string]interface{})
}

// IsFile returns true if the object @ path is a file
func IsFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}

// IsDir returns true if the object @ path is a dir
func IsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return info.IsDir()
}

// FileExists returns true if the file @ path exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// StringSliceToIntSlice converts slices of strings to slices of int. e.g. ["1", "3"] -> [1, 3]
func StringSliceToIntSlice(stringSlice []string) ([]int, error) {
	result := []int{}

	for _, stringValue := range stringSlice {
		var intValue int
		var err error

		if intValue, err = strconv.Atoi(stringValue); err != nil {
			return nil, err
		}

		result = append(result, intValue)
	}

	return result, nil
}

// RetryUntilSuccessful calls callback every interval for duration until it returns true
func RetryUntilSuccessful(duration time.Duration, interval time.Duration, callback func() bool) error {
	deadline := time.Now().Add(duration)

	// while we haven't passed the deadline
	for !time.Now().After(deadline) {

		// if callback returns true, we're done
		if callback() {
			return nil
		}

		time.Sleep(interval)
	}

	return errors.New("Timed out waiting until successful")
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
