//go:build android && authpoc

package main

/*
#include <stdlib.h>
*/
import "C"
import (
	"encoding/base64"
	"encoding/json"
	"github.com/Mag1cFall/AIStudio2API/internal/app"
	"unsafe"
)

//export AuthProbe
func AuthProbe(dir, key *C.char, data unsafe.Pointer, length C.int) *C.char {
	decoded, err := base64.StdEncoding.DecodeString(C.GoString(key))
	if err != nil {
		return C.CString(`{"success":false,"error":"KEY_INVALID"}`)
	}
	result := app.RunAuthProbe(C.GoString(dir), decoded, C.GoBytes(data, length))
	encoded, _ := json.Marshal(result)
	return C.CString(string(encoded))
}

//export AuthRestore
func AuthRestore(dir, key *C.char, generate C.int) *C.char {
	decoded, err := base64.StdEncoding.DecodeString(C.GoString(key))
	if err != nil {
		return C.CString(`{"success":false,"error":"KEY_INVALID"}`)
	}
	result := app.RunSavedAccountProbe(C.GoString(dir), decoded, generate != 0)
	encoded, _ := json.Marshal(result)
	return C.CString(string(encoded))
}

//export AuthProbeFree
func AuthProbeFree(p *C.char) { C.free(unsafe.Pointer(p)) }
func main()                   {}
