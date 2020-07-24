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
)

func TestMain(m *testing.M) {
	oTcltest := flag.Bool("tcltest", false, "")
	flag.Parse()
	if *oTcltest {
		blacklist := []string{
			"exit-1.1",   // ---- errorInfo: couldn't fork child process: function not implemented
			"exit-1.2",   // ---- Result was: couldn't fork child process: function not implemented
			"basic-46.2", // ---- Result was: couldn't fork child process: function not implemented
			"basic-46.3", // ---- Result was: couldn't fork child process: function not implemented
			"basic-46.4", // ---- Result was: couldn't fork child process: function not implemented
			"basic-46.5", // ---- Result was: couldn't fork child process: function not implemented
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
			"-notfile", "[c-z]*",
			"-singleproc", "1",
			"-skip", strings.Join(blacklist, " "),
		)
		os.Args = argv
		tcltest.Main()
		panic("unreachable")
	}

	os.Exit(m.Run())
}

func testTclTest(t *testing.T, blacklist map[string]struct{}, stdout, stderr io.Writer) int {
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
		if _, ok := blacklist[filepath.Base(v)]; ok {
			continue
		}

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
	blacklist := map[string]struct{}{}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	pth, err := filepath.Abs(wd)
	if err != nil {
		t.Fatal(err)
	}

	os.Setenv("TCL_LIBRARY", filepath.FromSlash(pth+"/lib"))
	rc := testTclTest(t, blacklist, os.Stdout, os.Stderr)
	if rc != 0 {
		t.Fatal(rc)
	}
}
