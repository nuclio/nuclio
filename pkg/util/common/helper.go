package common

import (
	"github.com/spf13/viper"
	"unsafe"
)

func ByteArrayToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
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
