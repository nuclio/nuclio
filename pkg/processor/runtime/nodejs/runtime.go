package main

import (
	"fmt"
)

/*
#cgo pkg-config: nodejs

#include "interface.h"
*/
import "C"

var (
	jscode = `
function handler(context, event) {
}
`
	handlerNamne = "handler"
)

func main() {
	C.initialize()
	fmt.Println("Initialized")
	worker := C.new_worker(C.CString(jscode), C.CString(handlerNamne))
	fmt.Printf("WORKER: %+v\n", worker)
	if worker.error_message != nil {
		fmt.Printf("ERROR: %s\n", C.GoString(worker.error_message))
	}
	//C.set_handler(C.CString("add"), C.CString(jscode))
}
