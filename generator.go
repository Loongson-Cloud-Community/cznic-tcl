// Copyright 2020 The Tcl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

//TODO vfs
//TODO enable threads
//TODO 8.6.11

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"modernc.org/ccgo/v3/lib"
)

const (
	tarDir  = "tcl8.6.10"
	tarFile = tarName + ".tar.gz"
	tarName = tarDir + "-src"
)

type supportedKey = struct{ os, arch string }

var (
	gcc       = os.Getenv("GO_GENERATE_CC")
	goarch    = ccgo.Env("TARGET_GOARCH", runtime.GOARCH)
	goos      = ccgo.Env("TARGET_GOOS", runtime.GOOS)
	supported = map[supportedKey]struct{}{
		{"darwin", "amd64"}:  {},
		{"linux", "386"}:     {},
		{"linux", "amd64"}:   {},
		{"linux", "arm"}:     {},
		{"linux", "arm64"}:   {},
		{"windows", "386"}:   {},
		{"windows", "amd64"}: {},
	}
	tmpDir = ccgo.Env("GO_GENERATE_TMPDIR", "")
)

func main() {
	if _, ok := supported[supportedKey{goos, goarch}]; !ok {
		ccgo.Fatalf(true, "unsupported target: %s/%s", goos, goarch)
	}

	ccgo.MustMkdirs(
		true,
		"internal/tclsh",
		"internal/tcltest",
		"lib",
	)
	if tmpDir == "" {
		tmpDir = ccgo.MustTempDir(true, "", "go-generate-")
		defer os.RemoveAll(tmpDir)
	}
	srcDir := tmpDir + "/" + tarDir
	os.RemoveAll(srcDir)
	ccgo.MustUntarFile(true, tmpDir, tarFile, nil)
	ccgo.MustCopyDir(true, "assets", srcDir+"/library", nil)
	ccgo.MustCopyDir(true, "testdata/tcl", srcDir+"/tests", nil)
	ccgo.MustCopyFile(true, "assets/tcltests/pkgIndex.tcl", "testdata/tcl/pkgIndex.tcl", nil)
	ccgo.MustCopyFile(true, "assets/tcltests/tcltests.tcl", "testdata/tcl/tcltests.tcl", nil)
	cdb, err := filepath.Abs(tmpDir + "/cdb.json")
	if err != nil {
		ccgo.Fatal(true, err)
	}

	if _, err := os.Stat(cdb); err != nil {
		if !os.IsNotExist(err) {
			ccgo.Fatal(true, err)
		}

		cfg := []string{
			"--disable-dll-unload",
			"--disable-load",
			"--disable-shared",
			"--disable-threads", //TODO-
			// "--enable-symbols=mem", //TODO-
		}
		platformDir := "/unix"
		switch goos {
		case "windows":
			platformDir = "/win"
			if goarch == "amd64" {
				cfg = append(cfg, "--enable-64bit")
			}
		case "darwin":
			cfg = append(cfg, "--enable-corefoundation=no")
		}
		ccgo.MustInDir(true, srcDir+platformDir, func() error {
			if gcc != "" {
				os.Setenv("CC", gcc)
			}
			ccgo.MustShell(true, "./configure", cfg...)
			ccgo.MustCompile(true, "-compiledb", cdb, "make", "CFLAGS=-UHAVE_CPUID", "binaries", "tcltest")
			return nil
		})
	}
	ccgo.MustCompile(true,
		"-D__printf__=printf",
		"-export-defines", "",
		"-export-enums", "",
		"-export-externs", "X",
		"-export-fields", "F",
		"-export-structs", "",
		"-export-typedefs", "",
		"-hide", "TclpCreateProcess",
		"-lmodernc.org/z/lib",
		"-o", filepath.Join("lib", fmt.Sprintf("tcl_%s_%s.go", goos, goarch)),
		"-pkgname", "tcl",
		"-replace-fd-zero", "__ccgo_fd_zero",
		"-replace-tcl-default-double-rounding", "__ccgo_tcl_default_double_rounding",
		"-replace-tcl-ieee-double-rounding", "__ccgo_tcl_ieee_double_rounding",
		"-trace-translation-units",
		cdb,
		"libtcl8.6.a",
		"libtclstub8.6.a",
	)
	ccgo.MustCompile(true,
		"-export-defines", "",
		"-lmodernc.org/tcl/lib",
		"-nocapi",
		"-o", filepath.Join("internal", "tclsh", fmt.Sprintf("tclsh_%s_%s.go", goos, goarch)),
		"-pkgname", "tclsh",
		"-replace-fd-zero", "__ccgo_fd_zero",
		"-replace-tcl-default-double-rounding", "__ccgo_tcl_default_double_rounding",
		"-replace-tcl-ieee-double-rounding", "__ccgo_tcl_ieee_double_rounding",
		"-trace-translation-units",
		cdb, "tclsh",
	)
	ccgo.MustCompile(true,
		"-export-defines", "",
		"-lmodernc.org/tcl/lib",
		"-nocapi",
		"-o", filepath.Join("internal", "tcltest", fmt.Sprintf("tcltest_%s_%s.go", goos, goarch)),
		"-trace-translation-units",
		cdb,
		"tclAppInit.o#1",
		"tclTest.o",
		"tclTestObj.o",
		"tclTestProcBodyObj.o",
		"tclThreadTest.o",
		"tclUnixTest.o",
	)
}
