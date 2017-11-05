package main

/*
#cgo CFLAGS: -I/opt/pypy/include
#cgo LDFLAGS: -lpypy-c

#include "PyPy.h"
#include "interface.h"

#include <stdlib.h> // for free
*/
import "C"

import (
	"fmt"
	"log"
	"unsafe"

	"github.com/nuclio/nuclio-sdk"
)

var (
	pyCode = `
import sys; sys.path.insert(0, '.')
import interface
interface.fill_api(c_argument)
`
)

func free(cp *C.char) {
	C.free(unsafe.Pointer(cp))
}

func main() {
	C.rpython_startup_code()
	if i := C.pypy_setup_home(C.CString("/opt/pypy/"), C.int(0)); i != 0 {
		log.Fatal("can't set home")
	}
	if i := C.pypy_execute_source_ptr(C.CString(pyCode), unsafe.Pointer(&C.api)); i != 0 {
		log.Fatal("can't execute")
	}

	C.init()
	C.set_handler(C.CString("handler:event_handler"))

	var evt nuclio.Event
	evt = &TestEvent{}
	res := C.handle_event(unsafe.Pointer(&evt))
	defer free(res)
	fmt.Printf("res: %q\n", C.GoString(res))
}
