package main

//#include "bridge.h"
import "C"
import "cfa/native/capture"

//export captureCommand
func captureCommand(command, payload C.c_string) *C.char {
	return C.CString(capture.Default.Command(C.GoString(command), C.GoString(payload)))
}
