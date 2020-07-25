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

func TestMain(m *testing.M) { // 24783
	oTcltest := flag.Bool("tcltest", false, "execute the Tcl test suite in internal/tcltest (internal use only)")
	flag.Parse()
	if *oTcltest {
		skip := []string{
			// Need fork. (Not implemented)
			"basic-46.2",
			"basic-46.3",
			"basic-46.4",
			"basic-46.5",
			"chan-io-14.3",
			"chan-io-14.4",
			"chan-io-28.6",
			"chan-io-28.7",
			"chan-io-29.33",
			"chan-io-60.1",
			"compile-12.2",
			"compile-13.1",
			"env-2.1",
			"env-2.2",
			"env-2.3",
			"env-2.4",
			"env-3.1",
			"env-4.1",
			"env-4.3",
			"env-4.4",
			"env-4.5",
			"event-13.8",
			"event-14.8",
			"event-7.5",
			"exit-1.1",
			"exit-1.2",
			"pid-1.2",
			"regexp-14.3",
			"regexpComp-14.3",
			"subst-5.8",
			"subst-5.9",
			"subst-5.10",

			//TODO Need socket.
			"chan-16.9",
			"chan-io-29.34",
			"chan-io-29.35",
			"chan-io-39.18",
			"chan-io-39.19",
			"chan-io-39.20",
			"chan-io-39.21",
			"chan-io-39.23",
			"chan-io-39.24",
			"chan-io-51.1",
			"chan-io-53.5",
			"chan-io-54.1",
			"chan-io-54.2",
			"chan-io-57.1",
			"chan-io-57.2",
			"event-11.5",
			"zlib-9.*",
			"zlib-10.0",
			"zlib-10.1",
			"zlib-10.2",

			//TODO other
			"chan-17.3",
			"chan-17.4",
			"clock-40.1",
			"clock-42.1",
			"cmdIL-6.*",
			"compExpr-old-19.1",
			"event-1.1",
			"event-13.3",
			"event-13.6",
			"event-14.3",
			"event-14.6",
			"expr-19.1",
			"expr-39.*",
			"expr-42.1",
			"expr-46.*",
			"expr-old-32.*",
			"expr-old-33.*",
			"expr-old-34.*",
			"expr-old-37.*",
			"expr-old-39.1",
			"format-5.*",
			"info-16.4",
			"interp-32.1",
			"safe-13.*",
			"safe-16.3",
			"safe-16.4",
			"scan-6.6",
			"set-old-10.8",
			"timer-7.*",
			"trace-24.5",
			"trace-25.*",
			"trace-34.1",
			"unixFCmd-1.*",
			"unixFCmd-2.*",
			"unixFCmd-12.2",
			"unixFCmd-15.1",
			"unixFCmd-16.*",
			"unixFCmd-13.2",
			"util-6.6",
			"util-10.*",
			"util-11.*",
			"util-15.*",
			"util-16.*",
			"zlib-8.*",
			"zlib-9.2",
			"zlib-9.3",
		}
		notFile := []string{
			// Need fork. (Not implemented)
			"exec.test",
			"stack.test",

			//TODO
			"cmdAH.test",      //TODO
			"encoding.test",   //TODO
			"fCmd.test",       //TODO
			"fileName.test",   //TODO
			"fileSystem.test", //TODO
			"http.test",       //TODO
			"http11.test",     //TODO
			"httpold.test",    //TODO
			"io.test",         //TODO
			"ioCmd.test",      //TODO
			"main.test",       //TODO
			"mathop.test",     //TODO
			"msgcat.test",     //TODO
			"socket.test",     //TODO
			"tcltest.test",    //TODO
			"unixNotfy.test",  //TODO
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
