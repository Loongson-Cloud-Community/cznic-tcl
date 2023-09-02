// Copyright 2023 The Tcl Authors. All rights reserved.
// Use of the source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"modernc.org/cc/v4"
	util "modernc.org/ccgo/v3/lib"
	ccgo "modernc.org/ccgo/v4/lib"
)

const (
	archivePath = "tcl8.6.13.tar.gz"
	cCompiler   = "gcc"
)

var (
	goos   = runtime.GOOS
	goarch = runtime.GOARCH
)

func fail(rc int, msg string, args ...any) {
	fmt.Fprintln(os.Stderr, strings.TrimSpace(fmt.Sprintf(msg, args...)))
	os.Exit(rc)
}

func main() {
	if os.Getenv(ccgo.CCEnvVar) != "" {
		if err := ccgo.NewTask(goos, goarch, os.Args, os.Stdout, os.Stderr, nil).Main(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		return
	}

	f, err := os.Open(archivePath)
	if err != nil {
		fail(1, "cannot open tar file: %v\n", err)
	}

	_, extractedArchivePath := filepath.Split(archivePath)
	extractedArchivePath = extractedArchivePath[:len(extractedArchivePath)-len(".tar.gz")]
	tempDir := os.Getenv("GO_GENERATE_DIR")
	dev := os.Getenv("GO_GENERATE_DEV") != ""
	switch {
	case tempDir != "":
		util.MustShell(true, "sh", "-c", fmt.Sprintf("rm -rf %s", filepath.Join(tempDir, extractedArchivePath)))
	default:
		var err error
		if tempDir, err = os.MkdirTemp("", "tcl-v2-generate"); err != nil {
			fail(1, "creating temp dir: %v\n", err)
		}

		defer func() {
			switch os.Getenv("GO_GENERATE_KEEP") {
			case "":
				os.RemoveAll(tempDir)
			default:
				fmt.Printf("%s: temporary directory kept\n", tempDir)
			}
		}()
	}
	libRoot := filepath.Join(tempDir, extractedArchivePath)
	makeRoot := filepath.Join(libRoot, "unix")
	fmt.Fprintf(os.Stderr, "archivePath %s\n", archivePath)
	fmt.Fprintf(os.Stderr, "extractedArchivePath %s\n", extractedArchivePath)
	fmt.Fprintf(os.Stderr, "tempDir %s\n", tempDir)
	fmt.Fprintf(os.Stderr, "libRoot %s\n", libRoot)
	fmt.Fprintf(os.Stderr, "makeRoot %s\n", makeRoot)

	util.MustUntar(true, tempDir, f, nil)
	util.MustCopyFile(true, "LICENSE-TCL", filepath.Join(libRoot, "license.terms"), nil)
	result := filepath.FromSlash("libtcl8.6.so.go")
	util.MustInDir(true, makeRoot, func() (err error) {
		cflags := []string{
			"-DTCL_MEM_DEBUG", //TODO-
			"-UHAVE_CPUID",
			"-UHAVE_FTS",
			"-UNDEBUG", //TODO-
		}
		if s := cc.LongDouble64Flag(goos, goarch); s != "" {
			cflags = append(cflags, s)
		}
		util.MustShell(true, "sh", "-c", "go mod init example.com/tcl ; go get modernc.org/libc/v2@master")
		if dev {
			util.MustShell(true, "sh", "-c", "go work init ; go work use $GOPATH/src/modernc.org/libc/v2")
		}
		//TODO 64 bits?
		util.MustShell(true, "sh", "-c", fmt.Sprintf("CC=%s CFLAGS='%s' ./configure --enable-threads=no", cCompiler, strings.Join(cflags, " ")))
		args := []string{os.Args[0]}
		if dev {
			args = append(
				args,
				"-absolute-paths",
				"-positions",
				// "-verify-types",
			)
		}
		args = append(args,
			"--package-name=z",
			"--prefix-enumerator=_",
			"--prefix-external=x_",
			"--prefix-field=F",
			"--prefix-macro=m_",
			"--prefix-static-internal=_",
			"--prefix-static-none=_",
			"--prefix-tagged-enum=_",
			"--prefix-tagged-struct=T",
			"--prefix-tagged-union=T",
			"--prefix-typename=T",
			"--prefix-undefined=_",
			"-exec-cc", cCompiler,
			"-extended-errors",
			// "-ignore-asm-errors",               //TODO- it is possible
			// "-ignore-unsupported-alignment",    //TODO- only if possible
			// "-ignore-unsupported-atomic-sizes", //TODO- it is possible
		)
		return ccgo.NewTask(goos, goarch, append(args, "-exec", "make", "libtcl8.6.so"), os.Stdout, os.Stderr, nil).Main()
	})

	util.MustCopyFile(false, fmt.Sprintf("ccgo_%s_%s.go", goos, goarch), filepath.Join(makeRoot, result), nil)
	util.MustCopyFile(false, filepath.Join("include", goos, goarch, "tcl.h"), filepath.Join(libRoot, "generic", "tcl.h"), nil)
	util.MustCopyFile(false, filepath.Join("include", goos, goarch, "tclDecls.h"), filepath.Join(libRoot, "generic", "tclDecls.h"), nil)
	util.MustCopyFile(false, filepath.Join("include", goos, goarch, "tclPlatDecls.h"), filepath.Join(libRoot, "generic", "tclPlatDecls.h"), nil)
}
