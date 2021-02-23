// Copyright 2020 The Tcl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"

	"modernc.org/libc"
	"modernc.org/tcl"
	"modernc.org/tcl/internal/tclsh"
)

const envVar = "TCL_LIBRARY"

func main() {
	if os.Getenv(envVar) == "" {
		if s, err := tcl.MountLibraryVFS(); err == nil {
			os.Setenv(envVar, s)
		}
	}
	libc.Start(tclsh.Main)
}
