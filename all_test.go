// Copyright 2020 The Tcl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tcl // import "modernc.org/tcl"

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"testing"

	"modernc.org/tcl/internal/tcltest"
)

func caller(s string, va ...interface{}) {
	if s == "" {
		s = strings.Repeat("%v ", len(va))
	}
	_, fn, fl, _ := runtime.Caller(2)
	fmt.Fprintf(os.Stderr, "# caller: %s:%d: ", path.Base(fn), fl)
	fmt.Fprintf(os.Stderr, s, va...)
	fmt.Fprintln(os.Stderr)
	_, fn, fl, _ = runtime.Caller(1)
	fmt.Fprintf(os.Stderr, "# \tcallee: %s:%d: ", path.Base(fn), fl)
	fmt.Fprintln(os.Stderr)
	os.Stderr.Sync()
}

func dbg(s string, va ...interface{}) {
	if s == "" {
		s = strings.Repeat("%v ", len(va))
	}
	_, fn, fl, _ := runtime.Caller(1)
	fmt.Fprintf(os.Stderr, "# dbg %s:%d: ", path.Base(fn), fl)
	fmt.Fprintf(os.Stderr, s, va...)
	fmt.Fprintln(os.Stderr)
	os.Stderr.Sync()
}

var traceLevel int32

func trace() func() {
	return func() {}
	n := atomic.AddInt32(&traceLevel, 1)
	pc, file, line, _ := runtime.Caller(1)
	s := strings.Repeat("· ", int(n)-1)
	fn := runtime.FuncForPC(pc)
	fmt.Fprintf(os.Stderr, "%s# trace %s:%d:%s: in\n", s, path.Base(file), line, fn.Name())
	os.Stderr.Sync()
	return func() {
		atomic.AddInt32(&traceLevel, -1)
		fmt.Fprintf(os.Stderr, "%s# trace %s:%d:%s: out\n", s, path.Base(file), line, fn.Name())
		os.Stderr.Sync()
	}
}

func TODO(...interface{}) string { //TODOOK
	_, fn, fl, _ := runtime.Caller(1)
	return fmt.Sprintf("# TODO: %s:%d:\n", path.Base(fn), fl) //TODOOK
}

func stack() string { return string(debug.Stack()) }

func use(...interface{}) {}

func init() {
	use(caller, dbg, TODO, trace, stack) //TODOOK
}

// ============================================================================

var (
	oDbgTcl = flag.Bool("dbg.tcl", false, "")
	oMatch  = flag.String("match", "", "argument of -match passed to the Tcl test suite")
)

func TestMain(m *testing.M) {
	oTcltest := flag.Bool("tcltest", false, "")
	flag.Parse()
	if *oTcltest {
		skip := []string{
			// These will probably never work w/o adjusting Tcl C sources.
			"basic-46.2",    // ---- Result was: couldn't fork child process: function not implemented
			"basic-46.3",    // ---- Result was: couldn't fork child process: function not implemented
			"basic-46.4",    // ---- Result was: couldn't fork child process: function not implemented
			"basic-46.5",    // ---- Result was: couldn't fork child process: function not implemented
			"chan-io-14.3",  // ---- errorInfo: couldn't fork child process: function not implemented
			"chan-io-14.4",  // ---- errorInfo: couldn't fork child process: function not implemented
			"chan-io-28.6",  // ---- errorInfo: couldn't fork child process: function not implemented
			"chan-io-28.7",  // ---- errorInfo: couldn't fork child process: function not implemented
			"chan-io-29.33", // ---- errorInfo: couldn't fork child process: function not implemented
			"chan-io-60.1",  // ---- errorInfo: couldn't fork child process: function not implemented
			"compile-12.2",  // ---- errorInfo: couldn't fork child process: function not implemented
			"compile-13.1",  // ---- errorInfo: couldn't fork child process: function not implemented
			"exit-1.1",      // ---- errorInfo: couldn't fork child process: function not implemented
			"exit-1.2",      // ---- Result was: couldn't fork child process: function not implemented

			//TODO
			//
			// These should probably all work. Some fail due to missing crt
			// implementations, some are - or possibly are - bugs in ccgo and/or crt.
			"chan-16.9",     // ---- Test setup failed: couldn't open socket: function not implemented
			"chan-17.3",     // ==== chan-17.3 chan command: pipe subcommand FAILED
			"chan-17.4",     // ==== chan-17.4 chan command: pipe subcommand FAILED
			"chan-io-29.34", // ---- errorInfo: couldn't open socket: function not implemented
			"chan-io-29.35", // ---- errorInfo: couldn't open socket: function not implemented
			"chan-io-39.18", // ---- errorInfo: couldn't open socket: function not implemented
			"chan-io-39.19", // ---- errorInfo: couldn't open socket: function not implemented
			"chan-io-39.20", // ---- errorInfo: couldn't open socket: function not implemented
			"chan-io-39.21", // ---- errorInfo: couldn't open socket: function not implemented
			"chan-io-39.23", // ---- errorInfo: couldn't open socket: function not implemented
			"chan-io-39.24", // ---- errorInfo: couldn't open socket: function not implemented
			"chan-io-51.1",  // ---- errorInfo: couldn't open socket: function not implemented
			"chan-io-53.5",  // ---- errorInfo: couldn't open socket: function not implemented
			"chan-io-54.1",  // ---- errorInfo: couldn't open socket: function not implemented
			"chan-io-54.2",  // ---- errorInfo: couldn't open socket: function not implemented
			"chan-io-57.1",  // ---- errorInfo: couldn't open socket: function not implemented
			"chan-io-57.2",  // ---- errorInfo: couldn't open socket: function not implemented
			"clock-40.1",    // ==== clock-40.1 regression - bad month with -timezone :localtime FAILED
			"clock-42.1",    // ==== clock-42.1 regression test - %z in :localtime when west of Greenwich FAILED
		}
		notFile := []string{
			"cmdAH.test",        // modernc.org/crt/v3/crt.go:2699:Xgetpwnam: TODOTODO
			"cmdIL.test",        // unexpected fault address 0x7fd10000001c
			"compExpr-old.test", // modernc.org/crt/v3/crt.go:2357:Xmodf: TODOTODO
			"[d-z]*",            //TODO
		}
		var argv []string
		for _, v := range os.Args {
			if !strings.HasPrefix(v, "-test.") && v != "-tcltest" {
				argv = append(argv, v)
			}
		}
		// -asidefromdir, -constraints, -debug, -errfile, -file, -limitconstraints,
		// -load, -loadfile, -match, -notfile, -outfile, -preservecore, -relateddir,
		// -singleproc, -skip, -testdir, -tmpdir, or -verbose
		argv = append(
			argv,
			"-debug", "1",
			"-notfile", strings.Join(notFile, " "),
			"-singleproc", "1",
			"-skip", strings.Join(skip, " "),
		)
		os.Args = argv
		tcltest.Main()
		panic("unreachable")
	}

	os.Exit(m.Run())
}

func testTclTest(t *testing.T, stdout, stderr io.Writer) int {
	m, err := filepath.Glob(filepath.FromSlash("testdata/tcl/*"))
	if err != nil {
		t.Fatal(err)
	}

	dir, err := ioutil.TempDir("", "tcl-test-")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(dir)

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	defer os.Chdir(wd)

	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	for _, v := range m {
		s := filepath.Join(wd, v)
		d := filepath.Join(dir, filepath.Base(v))
		f, err := ioutil.ReadFile(s)
		if err != nil {
			t.Fatal(err)
		}

		fi, err := os.Stat(s)
		if err != nil {
			t.Fatal(err)
		}

		if err := ioutil.WriteFile(d, f, fi.Mode()&os.ModePerm); err != nil {
			t.Fatal(err)
		}
	}
	var rc int
	var cmd *exec.Cmd
	switch {
	case *oMatch != "":
		cmd = exec.Command(os.Args[0], "-tcltest", "all.tcl", "-match", *oMatch)
	case *oDbgTcl:
		cmd = exec.Command(os.Args[0], "-tcltest", "dbg.tcl")
	default:
		cmd = exec.Command(os.Args[0], "-tcltest", "all.tcl")
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if cmd.Run() != nil {
		rc = 1
	}
	return rc
}

func TestTclTest(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	pth, err := filepath.Abs(wd)
	if err != nil {
		t.Fatal(err)
	}

	os.Setenv("TCL_LIBRARY", filepath.FromSlash(pth+"/lib"))
	rc := testTclTest(t, os.Stdout, os.Stderr)
	if rc != 0 {
		t.Fatal(rc)
	}
}
