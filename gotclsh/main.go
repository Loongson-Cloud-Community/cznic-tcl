// Copyright 2020 The Tcl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"modernc.org/crt/v3"
	"modernc.org/tcl"
	"modernc.org/tcl/internal/tclsh"
)

func main() {
	dir, err := ioutil.TempDir("", "gotclsh-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := tcl.Library(dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	os.Setenv("TCL_LIBRARY", dir)
	crt.Start(tclsh.Main)
}
