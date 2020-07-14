package tcl

import (
	"fmt"
	"unsafe"

	"modernc.org/crt/v3"
)


// func XTcl_PutsObjCmd(tls *crt.TLS, dummy ClientData, interp uintptr, objc int32, objv uintptr) int32 { /* tclIOCmd.c:105:1: */
// func XTcl_GetStringFromObj(tls *crt.TLS, objPtr uintptr, lengthPtr uintptr) uintptr { /* tclObj.c:1684:6: */

var length int32

func Xdbg(tls *crt.TLS, interp uintptr, objc int32, objv uintptr) {
	var a []string
	for i := 0; i < int(objc); i++ {
		p := *(*uintptr)(unsafe.Pointer(objv+uintptr(i)*unsafe.Sizeof(uintptr(0))))
		q := XTcl_GetStringFromObj(tls, p, uintptr(unsafe.Pointer(&length)))
		var b []byte
		for length != 0 {
			b = append(b, *(*byte)(unsafe.Pointer(q)))
			q++
			length--
		}
		a = append(a, string(b))
	}
	crt.Watch(fmt.Sprint(a))
}
