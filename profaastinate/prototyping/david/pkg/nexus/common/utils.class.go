package common

import "reflect"

func GetStructName(instance interface{}) string {
	t := reflect.TypeOf(instance)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}
