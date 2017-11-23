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
package main

import (
	"fmt"
	"os"
	"unsafe"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/zap"
)

/*
#cgo pkg-config: nodejs

#include "interface.h"
*/
import "C"

var (
	jscode = `
function handler(context, event) {
	context.log_info_with('info message', {"x": 1});
	return event.path + event.timestamp;
}
`
	handlerNamne = "handler"
)

func main() {
	C.initialize()
	fmt.Println("Initialized")
	result := C.new_worker(C.CString(jscode), C.CString(handlerNamne))
	fmt.Printf("WORKER: %+v\n", result)
	if result.error_message != nil {
		fmt.Printf("ERROR: %s\n", C.GoString(result.error_message))
		os.Exit(1)
	}

	log, err := nucliozap.NewNuclioZapTest("node")
	if err != nil {
		log.Fatal(err)
	}

	ctx := &nuclio.Context{
		Logger: log,
	}

	var evt nuclio.Event = &Event{}
	resp := C.handle_event(result.worker, unsafe.Pointer(ctx), unsafe.Pointer(&evt))
	if resp.error_message != nil {
		fmt.Printf("ERROR: %s\n", C.GoString(resp.error_message))
	} else {
		fmt.Printf("BODY: %s\n", C.GoString(resp.body))
	}
}
