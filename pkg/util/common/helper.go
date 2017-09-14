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
	"strings"
	"unsafe"

	"github.com/spf13/viper"
	"os"
)

func ByteArrayToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func StringMapToString(source map[string]string) string {
	list := []string{}

	for k, v := range source {
		list = append(list, k+"="+v)
	}

	return strings.Join(list, ",")
}

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

// this function extracts a list of objects from a viper instance. there may be a better way to do this with viper
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

// converts a strcuture to a map, flattening
func StructureToMap(input interface{}) map[string]interface{} {
	var decodedInput interface{}

	// TODO: find a more elegent mechanism than JSON encode/decode
	encodedInput, _ := json.Marshal(input)
	json.Unmarshal(encodedInput, &decodedInput)

	return decodedInput.(map[string]interface{})
}

func IsFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}

func IsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return info.IsDir()
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
