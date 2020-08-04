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
	oMatch   = flag.String("match", "", "argument of -match passed to the Tcl test suite: https://www.tcl.tk/man/tcl8.4/TclCmd/tcltest.htm#114")
	oVerbose = flag.String("verbose", "", "argument of -verbose passed to the Tcl test suite: https://www.tcl.tk/man/tcl8.4/TclCmd/tcltest.htm#M96")
)

func TestMain(m *testing.M) {
	oTcltest := flag.Bool("tcltest", false, "execute the Tcl test suite in internal/tcltest (internal use only)")
	flag.Parse()
	if !*oTcltest {
		os.Exit(m.Run())
	}

	tclTestMain()
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
	if *oMatch != "" {
		args = append(args, "-match", *oMatch)
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

	os.Setenv("TCL_LIBRARY", filepath.Join(pth, "assets"))
	f, err := os.Create(filepath.Join(pth, "testdata", fmt.Sprintf("tcltest_%s_%s.golden", runtime.GOOS, runtime.GOARCH)))
	if err != nil {
		t.Fatal(err)
	}

	defer f.Close()

	rc := testTclTest(t, io.MultiWriter(f, os.Stdout), os.Stderr)
	if rc != 0 {
		t.Fatal(rc)
	}
}

// all.tcl:	Total	31434	Passed	27940	Skipped	3461	Failed	33
func tclTestMain() {
	skip := []string{
		//TODO crashers
		"cmdIL-6.*",
		"trace-24.5",
		"trace-25.*",
		"trace-34.1",

		// Needs fork. (Not implemented)
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
		"encoding-24.1",
		"encoding-24.2",
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
		"io-14.3",
		"io-14.4",
		"pid-1.2",
		"regexp-14.3",
		"regexpComp-14.3",
		"subst-5.10",
		"subst-5.8",
		"subst-5.9",
		"unixFCmd-13.2",
		"io-29.33",
		"io-29.33b",
		"iocmd-32.*",
		"iocmd-31.*",

		//TODO Needs socket.
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
		"zlib-10.0",
		"zlib-10.1",
		"zlib-10.2",
		"zlib-9.*",
		"io-39.18",
		"io-39.19",
		"io-39.20",
		"io-39.21",
		"io-39.23",
		"io-39.24",
		"io-51.1",
		"io-53.5",
		"io-54.1",
		"io-54.2",
		"io-57.1",
		"io-57.2",
		"io-60.1",
		"zlib-8.3",
		"io-29.34",
		"io-29.35",

		//TODO getpwnam
		"info-16.4",
		"unixFCmd-16.*",
		"cmdAH-2.5",
		"cmdAH-8.39",
		"cmdAH-15.1",
		"cmdAH-19.*",
		"cmdAH-30.8",

		//TODO getgrgid
		"unixFCmd-12.2",
		"cmdAH-7.1",

		//TODO getgrnam
		"unixFCmd-15.1",

		//TODO other
		"cmdAH-32.3",
		"tcltest-1.1",
		"tcltest-1.2",
		"tcltest-6.2",
		"tcltest-6.3",
		"tcltest-6.4",
		"tcltest-7.2",
		"tcltest-7.3",
		"tcltest-7.4",
		"tcltest-7.5",
		"tcltest-9.5",
		"tcltest-12.2",
		"tcltest-14.1",
		"tcltest-22.1",
	}
	notFile := []string{
		// Needs fork. (Not implemented)
		"exec.test",
		"http11.test",
		"ioCmd.test",
		"main.test",  // all tests want fork
		"stack.test", // all tests want fork

		//TODO Needs socket.
		"socket.test",

		// getpwnam
		"fCmd.test",
		"fileName.test",
		"fileSystem.test",
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
