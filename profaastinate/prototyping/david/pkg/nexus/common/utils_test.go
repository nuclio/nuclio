package common

import "testing"

func TestGetStructName(t *testing.T) {
	type ExampleStruct struct {
	}
	exampleStructInstance := ExampleStruct{}

	if "ExampleStruct" == GetStructName(exampleStructInstance) {
		t.Log("TestGetStructName passed")
	} else {
		t.Error("TestGetStructName failed")
	}
}
