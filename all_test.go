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
)

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func TestTclTest(t *testing.T) {
	skip := []string{}
	notFile := []string{}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	pth, err := filepath.Abs(wd)
	if err != nil {
		t.Fatal(err)
	}

	f, err := os.Create(filepath.Join(pth, "testdata", fmt.Sprintf("tcltest_%s_%s.golden", runtime.GOOS, runtime.GOARCH)))
	if err != nil {
		t.Fatal(err)
	}

	defer f.Close()

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
	cmd := exec.Command("go", "build", "-o", tcltest, "modernc.org/tcl/internal/tcltest")
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
	switch runtime.GOOS {
	case "windows":
		panic(todo(""))
	default:
		os.Setenv("PATH", fmt.Sprintf("%s:%s", dir, os.Getenv("PATH")))
	}
	cmd = exec.Command(tcltest, args...)
	cmd.Stdout = io.MultiWriter(f, os.Stdout)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Error(err)
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
