package common

import (
	"unsafe"

	"github.com/spf13/viper"
	"strings"
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
