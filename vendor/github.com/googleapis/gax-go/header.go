package gax

import v2 "github.com/googleapis/gax-go/v2"

// XGoogHeader is for use by the Google Cloud Libraries only.
//
// XGoogHeader formats key-value pairs.
// The resulting string is suitable for x-goog-api-client header.
func XGoogHeader(keyval ...string) string {
	return v2.XGoogHeader(keyval...)
}
