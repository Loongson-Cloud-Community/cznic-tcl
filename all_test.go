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
	oDebug   = flag.String("debug", "", "argument of -debug passed to the Tcl test suite: https://www.tcl.tk/man/tcl8.4/TclCmd/tcltest.htm#M91")
	oFile    = flag.String("file", "", "argument of -file passed to the Tcl test suite: https://www.tcl.tk/man/tcl8.4/TclCmd/tcltest.htm#M110")
	oVerbose = flag.String("verbose", "", "argument of -verbose passed to the Tcl test suite: https://www.tcl.tk/man/tcl8.4/TclCmd/tcltest.htm#M96")
)

func TestMain(m *testing.M) {
	oTcltest := flag.Bool("tcltest", false, "execute the Tcl test suite in internal/tcltest (internal use only)")
	flag.Parse()
	if *oTcltest {
		skip := []string{}
		notFile := []string{
			"aaa_exit.test",
			"basic.test",
			"chan.test",
			"chanio.test",
			"clock.test",
			"cmdAH.test",
			"cmdIL.test",
			"compExpr-old.test",
			"compile.test",
			"encoding.test",
			"env.test",
			"event.test",
			"exec.test",
			"expr-old.test",
			"expr.test",
			"fCmd.test",
			"fileName.test",
			"fileSystem.test",
			"format.test",
			"http.test",
			"http11.test",
			"httpold.test",
			"info.test",
			"interp.test",
			"io.test",
			"ioCmd.test",
			"main.test",
			"mathop.test",
			"msgcat.test",
			"pid.test",
			"regexp.test",
			"regexpComp.test",
			"safe.test",
			"scan.test",
			"set-old.test",
			"socket.test",
			"stack.test",
			"subst.test",
			"tcltest.test",
			"timer.test",
			"trace.test",
			"unixFCmd.test",
			"unknown.test",
			"util.test",
			"zlib.test",
		}
		var argv []string
		for _, v := range os.Args {
			if !strings.HasPrefix(v, "-test.") && v != "-tcltest" {
				argv = append(argv, v)
			}
		}
		argv = append(
			argv,
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
	args := []string{"-tcltest", "all.tcl"}
	if *oDebug != "" {
		args = append(args, "-debug", *oDebug)
	}
	if *oFile != "" {
		args = append(args, "-file", *oFile)
	}
	if *oVerbose != "" {
		args = append(args, "-verbose", *oVerbose)
	}
	cmd := exec.Command(os.Args[0], args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if cmd.Run() != nil {
		return 1
	}

	return 0
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
