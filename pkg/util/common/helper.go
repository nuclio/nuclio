package common

import "unsafe"

func ByteArrayToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}
