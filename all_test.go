// Copyright 2020 The Tcl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tcl // import "modernc.org/tcl"

import (
	"bufio"
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

	"modernc.org/tcl/lib"
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
	oDebug      = flag.String("debug", "", "argument of -debug passed to the Tcl test suite: https://www.tcl.tk/man/tcl8.4/TclCmd/tcltest.htm#M91")
	oFile       = flag.String("file", "", "argument of -file passed to the Tcl test suite: https://www.tcl.tk/man/tcl8.4/TclCmd/tcltest.htm#M110")
	oMatch      = flag.String("match", "", "argument of -match passed to the Tcl test suite: https://www.tcl.tk/man/tcl8.4/TclCmd/tcltest.htm#114")
	oSingleProc = flag.Bool("singleproc", false, "argument of -singleproc passed to the Tcl test suite: https://www.tcl.tk/man/tcl8.4/TclCmd/tcltest.htm#M90")
	oVerbose    = flag.String("verbose", "", "argument of -verbose passed to the Tcl test suite: https://www.tcl.tk/man/tcl8.4/TclCmd/tcltest.htm#M96")
	oXTags      = flag.String("xtags", "", "passed to go build of tcltest in TestTclTest")
)

func TestMain(m *testing.M) {
	fmt.Printf("test binary compiled for %s/%s\n", runtime.GOOS, runtime.GOARCH)
	flag.Parse()
	os.Exit(m.Run())
}

func TestTclTest(t *testing.T) {
	skip := []string{}
	notFile := []string{}
	if runtime.GOOS == "windows" {
		//	Tests ended at Sat Oct 03 19:32:00 +0200 2020
		//	all.tcl:	Total	33202	Passed	29178	Skipped	4024	Failed	0
		//	Sourced 146 Test Files.
		//	Number of tests skipped for each constraint:
		//		9	!ieeeFloatingPoint
		//		1	asyncPipeChan
		//		76	bigEndian
		//		5	bug-3057639
		//		23	cat32
		//		8	cdrom
		//		50	dde
		//		1	dontCopyLinks
		//		19	eformat
		//		65	emptyTest
		//		2	exdev
		//		5	fullutf
		//		2	hasIsoLocale
		//		1	interactive
		//		1	knownBadTest
		//		38	knownBug
		//		100	localeRegexp
		//		15	longIs64bit
		//		14	macosxFileAttr
		//		82	memory
		//		15	nonPortable
		//		5	notNetworkFilesystem
		//		1	notValgrind
		//		19	pkga.dllRequired
		//		20	pkgua.dllRequired
		//		126	reg
		//		1996	serverNeeded
		//		2	sharedCdrive
		//		1	symbolicLinkFile
		//		1	tempNotWin
		//		1	testexprparser && !ieeeFloatingPoint
		//		22	testfilehandler
		//		2	testfilewait
		//		7	testfindexecutable
		//		1	testfork
		//		1	testgetdefenc
		//		21	testwordend
		//		185	thread
		//		3	threaded
		//		3	tip389
		//		157	unix
		//		14	unixExecs
		//		6	win2000orXP
		//		3	winOlderThan2000
		//		6	xdev
		//	--- PASS: TestTclTest (97.65s)
		//	PASS
		//	ok  	modernc.org/tcl	98.128s
		skip = []string{
			//TODO other
			"chan-16.*",
			"clock-47.*",
			"cmdAH-20.2",
			"cmdAH-20.6",
			"env-4.*",
			"env-5.*",
			"env-7.*",
			"event-7.*",
			"exec-20.*",
			"filesystem-1.*",
			"filesystem-7.*",
			"info-22.*",
			"iocmd-31.*",
			"fCmd-10.*",
			"iocmd-8.*",
			"platform-3.1",
			"safe-13.*",
			"safe-16.*",
			"safe-7.*",
			"safe-8.*",
			"tcltest-5.*",
			"tcltest-10.*",
			"winFCmd-16.*",
			"winFCmd-6.*",
			"winFCmd-9.*",
			"winNotify-3.*",
			"winTime-2.*",
			"winpipe-8.*",
			"zlib-10.*",
			"zlib-8.*",
			"zlib-9.*",

			//TODO hangs
			"Tcl_Main-1.9",
			"Tcl_Main-4.*",
			"Tcl_Main-5.*",
			"chan-io-12.*",
			"chan-io-13.*",
			"chan-io-14.*",
			"chan-io-39.*",
			"chan-io-45.*",
			"clock-35.*",
			"cmdAH-24.*",
			"interp-34.*",
			"io-12.*",
			"io-13.*",
			"io-14.*",
			"io-27.*",
			"io-29.*",
			"io-3*.*",
			"io-4*.*",
			"io-5*.*",
			"timer-3.*",
			"timer-6.*",

			//TODO crashes
			"chan-io-28.*",
			"chan-io-29.*",
			"chan-io-51.*",
			"chan-io-53.*",
			"chan-io-54.*",
			"chan-io-57.*",
			"event-11.*",
		}
		notFile = []string{
			//TODO hangs
			"http.test",

			//TODO Test file error: child killed: unknown signal
			"http11.test",
			"httpold.test",
			"socket.test",
		}
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	pth, err := filepath.Abs(wd)
	if err != nil {
		t.Fatal(err)
	}

	g := newGolden(t, filepath.Join(pth, "testdata", fmt.Sprintf("tcltest_%s_%s.golden", runtime.GOOS, runtime.GOARCH)))

	defer g.close()

	m, err := filepath.Glob(filepath.FromSlash("testdata/tcl/*"))
	if err != nil {
		t.Fatal(err)
	}

	dir, err := ioutil.TempDir("", "tcl-test-")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(dir)

	tcltest := filepath.Join(dir, "tcltest")
	if runtime.GOOS == "windows" {
		tcltest += ".exe"
	}
	args0 := []string{"build", "-o", tcltest}
	if s := *oXTags; s != "" {
		args0 = append(args0, "-tags", s)
	}
	args0 = append(args0, "modernc.org/tcl/internal/tcltest")
	cmd := exec.Command("go", args0...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s\n%v", out, err)
	}

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
	args := []string{
		"all.tcl",
		"-notfile", strings.Join(notFile, " "),
		"-skip", strings.Join(skip, " "),
	}
	if *oDebug != "" {
		args = append(args, "-debug", *oDebug)
	}
	if *oFile != "" {
		args = append(args, "-file", *oFile)
	}
	if *oMatch != "" {
		args = append(args, "-match", *oMatch)
	}
	if *oSingleProc {
		args = append(args, "-singleproc", "1")
	}
	if *oVerbose != "" {
		args = append(args, "-verbose", *oVerbose)
	}
	os.Setenv("TCL_LIBRARY", filepath.Join(pth, "assets"))
	os.Setenv("PATH", fmt.Sprintf("%s%c%s", dir, os.PathListSeparator, os.Getenv("PATH")))
	cmd = exec.Command(tcltest, args...)
	cmd.Stdout = io.MultiWriter(g, os.Stdout)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Error(err)
	}
}

type golden struct {
	t *testing.T
	f *os.File
	w *bufio.Writer
}

func newGolden(t *testing.T, fn string) *golden {
	f, err := os.Create(filepath.FromSlash(fn))
	if err != nil { // Possibly R/O fs in a VM
		base := filepath.Base(filepath.FromSlash(fn))
		f, err = ioutil.TempFile("", base)
		if err != nil {
			t.Fatal(err)
		}

		t.Logf("writing results to %s\n", f.Name())
	}

	w := bufio.NewWriter(f)
	return &golden{t, f, w}
}

func (g *golden) Write(b []byte) (int, error) {
	return g.w.Write(b)
}

func (g *golden) close() {
	if g.f == nil {
		return
	}

	if err := g.w.Flush(); err != nil {
		g.t.Fatal(err)
	}

	if err := g.f.Close(); err != nil {
		g.t.Fatal(err)
	}
}

func TestEval(t *testing.T) {
	in, err := NewInterp()
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := in.Close(); err != nil {
			t.Error(err)
		}
	}()

	s, err := in.Eval("set a 42; incr a")
	if err != nil {
		t.Fatal(err)
	}

	if g, e := s, "43"; g != e {
		t.Errorf("got %q exp %q", g, e)
	}
}

func ExampleInterp_Eval() {
	in := MustNewInterp()
	s := in.MustEval(`

# This is the Tcl script
# ----------------------
set a 42
incr a
# ----------------------

`)
	in.MustClose()
	fmt.Println(s)
	// Output:
	// 43
}

func TestCreateCommand(t *testing.T) {
	in, err := NewInterp()
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if in == nil {
			return
		}

		if err := in.Close(); err != nil {
			t.Error(err)
		}
	}()

	var delTrace string
	_, err = in.NewCommand(
		"::go::echo",
		func(clientData interface{}, in *Interp, args []string) int {
			args = append(args[1:], fmt.Sprint(clientData))
			in.SetResult(strings.Join(args, " "))
			return tcl.TCL_OK
		},
		42,
		func(clientData interface{}) {
			delTrace = fmt.Sprint(clientData)
		},
	)
	if err != nil {
		t.Error(err)
		return
	}

	s, err := in.Eval("::go::echo 123 foo bar")
	if err != nil {
		t.Fatal(err)
	}

	if g, e := s, "123 foo bar 42"; g != e {
		t.Errorf("got %q exp %q", g, e)
		return
	}

	err = in.Close()
	in = nil
	if err != nil {
		t.Error(err)
		return
	}

	if g, e := delTrace, "42"; g != e {
		t.Errorf("got %q exp %q", g, e)
	}
}

func ExampleInterp_NewCommand() {
	in := MustNewInterp()
	var delTrace string
	in.MustNewCommand(
		"::go::echo",
		func(clientData interface{}, in *Interp, args []string) int {
			// Go implementation of the Tcl ::go::echo command
			args = append(args[1:], fmt.Sprint(clientData))
			in.SetResult(strings.Join(args, " "))
			return tcl.TCL_OK
		},
		42, // client data
		func(clientData interface{}) {
			// Go implemetation of the command delete handler
			delTrace = fmt.Sprint(clientData)
		},
	)
	fmt.Println(in.MustEval("::go::echo 123 foo bar"))
	in.MustClose()
	fmt.Println(delTrace)
	// Output:
	// 123 foo bar 42
	// 42
}
