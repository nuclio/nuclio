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

package compare

import (
	"fmt"
	"reflect"
)

// isSimple return true if kind is simple (not a map, slice or array)
func isSimple(kind reflect.Kind) bool {
	switch kind {
	case reflect.Array, reflect.Slice, reflect.Map:
		return false
	}

	return true
}

// compareArrays compares to arrays/slices
func compareArrays(v1, v2 interface{}) bool {
	slice1 := reflect.ValueOf(v1)
	slice2 := reflect.ValueOf(v2)

	if slice1.Len() != slice2.Len() {
		return false
	}

	// Indices in slice2 that equal to values in slice1
	matched := make(map[int]bool)

	for i := 0; i < slice1.Len(); i++ {
		found := false
		item1 := slice1.Index(i).Interface()

		for j := 0; j < slice2.Len(); j++ {
			// Already matched to item in slice1
			if matched[j] {
				continue
			}
			item2 := slice2.Index(j).Interface()
			if CompareNoOrder(item1, item2) {
				matched[j] = true
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// toInterfaces returns a slice of the interfaces behinnd values
func toInterfaces(values []reflect.Value) []interface{} {
	interfaces := make([]interface{}, len(values))
	for i := range values {
		interfaces[i] = values[i].Interface()
	}

	return interfaces
}

func compareMaps(v1, v2 interface{}) bool {
	map1 := reflect.ValueOf(v1)
	map2 := reflect.ValueOf(v2)

	if map1.Len() != map2.Len() {
		return false
	}

	keys1 := map1.MapKeys()
	if !CompareNoOrder(toInterfaces(keys1), toInterfaces(map2.MapKeys())) {
		return false
	}

	for _, key := range map1.MapKeys() {
		val1 := map1.MapIndex(key)
		val2 := map2.MapIndex(key)
		if !CompareNoOrder(val1.Interface(), val2.Interface()) {
			return false
		}

	}

	return true
}

// CompareNoOrder compares two values regardless of order
func CompareNoOrder(v1, v2 interface{}) bool {
	if v1 == nil || v2 == nil {
		return v1 == v2
	}

	type1 := reflect.TypeOf(v1)
	type2 := reflect.TypeOf(v2)

	if type1 != type2 {
		return false
	}

	if isSimple(type1.Kind()) {
		return reflect.DeepEqual(v1, v2)
	}

	switch type1.Kind() {
	case reflect.Array, reflect.Slice:
		return compareArrays(v1, v2)
	case reflect.Map:
		return compareMaps(v1, v2)
	}

	panic(fmt.Sprintf("NoOrderCompare: unknown type - %T", v1))
}
