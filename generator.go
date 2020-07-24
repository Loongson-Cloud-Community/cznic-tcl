// Copyright 2020 The Tcl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

var (
	downloads = []struct {
		dir, url string
		sz       int
		dev      bool
	}{
		{tclDir, "https://downloads.sourceforge.net/project/tcl/Tcl/8.6.10/tcl8.6.10-src.tar.gz", 9700, false},
	}

	tclDir = filepath.FromSlash("testdata/tcl8.6.10")
)

func fail(s string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, origin(2)+":"+s, args...)
	os.Exit(1)
}

func origin(skip int) string {
	pc, fn, fl, _ := runtime.Caller(skip)
	f := runtime.FuncForPC(pc)
	var fns string
	if f != nil {
		fns = f.Name()
		if x := strings.LastIndex(fns, "."); x > 0 {
			fns = fns[x+1:]
		}
	}
	return fmt.Sprintf("%s:%d:%s", fn, fl, fns)
}

func main() {
	download()
	switch runtime.GOOS {
	case "linux":
		makeUnix()
		if err := newCmd(nil, nil, "sh", "-c", fmt.Sprintf("cp -r %s %s", filepath.FromSlash(tclDir+"/library/*"), "lib/")).Run(); err != nil {
			fail("error copying tcl library: %v", err)
		}
	default:
		fail("unsupported GOOS: %s\n", runtime.GOOS)
	}

	dst := filepath.FromSlash("testdata/tcl")
	if err := os.MkdirAll(dst, 0770); err != nil {
		fail("cannot create %q: %v", dst, err)
	}

	m, err := filepath.Glob(filepath.Join(tclDir, "tests/*"))
	if err != nil {
		fail("cannot glob tests/*: %v", err)
	}

	for _, v := range m {
		copyFile(v, filepath.Join(dst, filepath.Base(v)))
	}

	dir := filepath.FromSlash("lib/tcltests")
	if err := os.MkdirAll(dir, 0770); err != nil {
		fail("cannot create %q: %v", dir, err)
	}

	copyFile("testdata/tcl/pkgIndex.tcl", "lib/tcltests/pkgIndex.tcl")
	copyFile("testdata/tcl/tcltests.tcl", "lib/tcltests/tcltests.tcl")
}

func copyFile(src, dest string) {
	src = filepath.FromSlash(src)
	dest = filepath.FromSlash(dest)
	f, err := ioutil.ReadFile(src)
	if err != nil {
		fail("cannot read %v: %v", src, err)
	}

	if err := ioutil.WriteFile(dest, f, 0660); err != nil {
		fail("cannot write %v: %v", dest, err)
	}
}

func makeUnix() {
	testWD, err := os.Getwd()
	if err != nil {
		fail("%s\n", err)
	}

	defer os.Chdir(testWD)

	if err := os.Chdir(filepath.Join(tclDir, "unix")); err != nil {
		fail("%s\n", err)
	}

	newCmd(nil, nil, "make", "distclean").Run()
	cmd := newCmd(nil, nil, "./configure",
		"--disable-dll-unload",
		"--disable-load",
		"--disable-threads",
		"--enable-symbols=mem", //TODO- adds TCL_MEM_DEBUG
	)
	if err = cmd.Run(); err != nil {
		fail("%s\n", err)
	}

	makeUnixLibAndShell(testWD)
	makeUnixTclTest(testWD)
}

func makeUnixLibAndShell(testWD string) {
	var stdout strings.Builder
	cmd := newCmd(&stdout, nil, "make")
	err := cmd.Run()
	if err != nil {
		fail("%s\n", err)
	}

	groups := makeGroups(stdout.String())
	var objectFiles, opts []string
	optm := map[string]struct{}{}
	cFiles := map[string]string{}
	for k, lines := range groups {
		switch k {
		case "ar":
			for _, v := range lines {
				if strings.Contains(v, "libtclstub") {
					continue
				}

				for _, w := range strings.Split(v, " ") {
					if strings.HasSuffix(w, ".o") {
						objectFiles = append(objectFiles, w[:len(w)-len(".o")])
					}
				}
			}
		case "gcc":
			for _, v := range lines {
				if strings.Contains(v, "tclAppInit.o") {
					break
				}

				parseCCLine(nil, cFiles, optm, &opts, v)
			}
		case "rm":
			// nop
		default:
			fail("unknown command: `%s` in %v\n", k, lines)
		}
	}
	args := []string{
		"-o", filepath.Join(testWD, filepath.FromSlash(fmt.Sprintf("lib/tcl_%s_%s.go", runtime.GOOS, runtime.GOARCH))),
		"-ccgo-export-externs", "X",
		"-ccgo-export-fields", "F",
		"-ccgo-long-double-is-double",
		"-ccgo-pkgname", "tcl",
		"-ccgo-verify-structs", //TODO-
		"../compat/zlib/adler32.c",
		"../compat/zlib/compress.c",
		"../compat/zlib/crc32.c",
		"../compat/zlib/deflate.c",
		"../compat/zlib/infback.c",
		"../compat/zlib/inffast.c",
		"../compat/zlib/inflate.c",
		"../compat/zlib/inftrees.c",
		"../compat/zlib/trees.c",
		"../compat/zlib/uncompr.c",
		"../compat/zlib/zutil.c",
	}
	args = append(args, opts...)
	for _, v := range objectFiles {
		args = append(args, cFiles[v])
	}
	fmt.Println("====\nccgo")
	for _, v := range args {
		fmt.Println(v)
	}
	fmt.Println("====")
	cmd = newCmd(nil, nil, "ccgo", args...)
	if err = cmd.Run(); err != nil {
		fail("%s\n", err)
	}

	args = []string{
		"-o", filepath.Join(testWD, filepath.FromSlash(fmt.Sprintf("tclsh/tclsh_%s_%s.go", runtime.GOOS, runtime.GOARCH))),
		"-ccgo-long-double-is-double",
		"-ccgo-verify-structs", //TODO-
		"tclAppInit.c",
		"-lmodernc.org/tcl/lib",
	}
	args = append(args, opts...)
	fmt.Println("====\nccgo")
	for _, v := range args {
		fmt.Println(v)
	}
	fmt.Println("====")
	cmd = newCmd(nil, nil, "ccgo", args...)
	if err = cmd.Run(); err != nil {
		fail("%s\n", err)
	}
}

func makeUnixTclTest(testWD string) {
	var stdout strings.Builder
	cmd := newCmd(&stdout, nil, "make", "tcltest")
	err := cmd.Run()
	if err != nil {
		fail("%s\n", err)
	}

	groups := makeGroups(stdout.String())
	var opts, cPaths []string
	optm := map[string]struct{}{}
	cFiles := map[string]string{}
	for k, lines := range groups {
		switch k {
		case "gcc":
			for _, v := range lines {
				parseCCLine(&cPaths, cFiles, optm, &opts, v)
			}
		case "mv", "rm", "make", "make[1]:":
			// nop
		default:
			fail("unknown command: `%s` %v\n", k, lines)
		}
	}
	args := []string{
		"-o", filepath.Join(testWD, filepath.FromSlash(fmt.Sprintf("internal/tcltest/tcltest_%s_%s.go", runtime.GOOS, runtime.GOARCH))),
		"-ccgo-long-double-is-double",
		"-ccgo-pkgname", "tcltest",
		"-ccgo-verify-structs", //TODO-
		"../generic/tclOOStubLib.c",
		"../generic/tclStubLib.c",
		"../generic/tclTomMathStubLib.c",
		"-lmodernc.org/tcl/lib",
	}
	args = append(args, opts...)
	args = append(args, cPaths...)
	fmt.Println("====\nccgo")
	for _, v := range args {
		fmt.Println(v)
	}
	fmt.Println("====")
	cmd = newCmd(nil, nil, "ccgo", args...)
	if err = cmd.Run(); err != nil {
		fail("%s\n", err)
	}
	os.Remove(filepath.Join(testWD, filepath.FromSlash(fmt.Sprintf("internal/tcltest/capi_%s_%s.go", runtime.GOOS, runtime.GOARCH))))
}

func parseCCLine(cPaths *[]string, cFiles map[string]string, m map[string]struct{}, opts *[]string, line string) {
	if strings.HasSuffix(line, "-o tcltest") {
		return
	}

	for _, tok := range splitCCLine(line) {
		switch {
		case
			tok == "-c",
			tok == "-g",
			tok == "-pipe",
			strings.HasPrefix(tok, "-W"),
			strings.HasPrefix(tok, "-O"):

			// nop
		case strings.HasPrefix(tok, "-D"):
			if tok == "-DHAVE_CPUID=1" {
				break
			}

			if tok == "-DNDEBUG=1" { //TODO-
				break
			}

			if i := strings.IndexByte(tok, '='); i > 0 {
				def := tok[:i+1]
				val := tok[i+1:]
				if strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"") {
					val = val[1 : len(val)-1]
				}
				var b strings.Builder
				for len(val) != 0 {
					var c byte
					switch c = val[0]; c {
					case '\\':
						val = val[1:]
						if len(val) == 0 {
							fail("invalid definition: `%s`", tok)
						}

						c = val[0]
					}
					b.WriteByte(c)
					val = val[1:]
				}
				tok = def + b.String()
			}
			if _, ok := m[tok]; ok {
				break
			}

			m[tok] = struct{}{}
			*opts = append(*opts, tok)
		case strings.HasPrefix(tok, "-I"):
			s := tok[2:]
			if strings.HasPrefix(s, "\"") {
				var err error
				if s, err = strconv.Unquote(s); err != nil {
					fail("%q", tok)
				}
			}
			tok = "-I" + s
			if _, ok := m[tok]; ok {
				break
			}

			m[tok] = struct{}{}
			*opts = append(*opts, tok)
		case strings.HasPrefix(tok, "/") && strings.HasSuffix(tok, ".c"):
			if cPaths != nil {
				*cPaths = append(*cPaths, tok)
			}
			f := filepath.Base(tok)
			cFiles[f[:len(f)-len(".c")]] = tok
		default:
			fail("TODO `%s` in `%s`\n", tok, line)
		}
	}
}

func splitCCLine(s string) (r []string) {
	s0 := s
	for len(s) != 0 {
		switch c := s[0]; c {
		case ' ', '\t':
			s = s[1:]
			continue
		}

		// Non blank
		var b strings.Builder
		inStr := false
	tok:
		for len(s) != 0 {
			var c byte
			switch c = s[0]; c {
			case ' ', '\t':
				if !inStr {
					break tok
				}
			case '\\':
				s = s[1:]
				if len(s) == 0 {
					fail("invalid escape: `%s`\n", s0)
				}

				c = s[0]
				if inStr || strings.HasPrefix(b.String(), "-D") {
					b.WriteByte('\\')
				}
			case '"':
				inStr = !inStr
			}
			b.WriteByte(c)
			s = s[1:]
		}
		if t := strings.TrimSpace(b.String()); t != "" {
			r = append(r, t)
		}
		b.Reset()
	}
	return r
}

func makeGroups(s string) map[string][]string {
	s = strings.ReplaceAll(s, "\\\n", "")
	a := strings.Split(s, "\n")
	r := map[string][]string{}
	for _, v := range a {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}

		var key string
		if i := strings.IndexByte(v, ' '); i > 0 {
			key = v[:i]
			v = strings.TrimSpace(v[i+1:])
		}
		r[key] = append(r[key], v)
	}
	return r
}

func newCmd(stdout, stderr io.Writer, bin string, args ...string) *exec.Cmd {
	r := exec.Command(bin, args...)
	r.Stdout = multiWriter(os.Stdout, stdout)
	r.Stderr = multiWriter(os.Stderr, stderr)
	return r
}

func multiWriter(w ...io.Writer) io.Writer {
	var a []io.Writer
	for _, v := range w {
		if v != nil {
			a = append(a, v)
		}
	}
	switch len(a) {
	case 0:
		panic("internal error")
	case 1:
		return a[0]
	default:
		return io.MultiWriter(a...)
	}
}

func download() {
	tmp, err := ioutil.TempDir("", "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return
	}

	defer os.RemoveAll(tmp)

	for _, v := range downloads {
		dir := filepath.FromSlash(v.dir)
		root := filepath.Dir(v.dir)
		fi, err := os.Stat(dir)
		switch {
		case err == nil:
			if !fi.IsDir() {
				fmt.Fprintf(os.Stderr, "expected %s to be a directory\n", dir)
			}
			continue
		default:
			if !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "%s", err)
				continue
			}

		}

		if err := func() error {
			fmt.Printf("Downloading %v MB from %s\n", float64(v.sz)/1000, v.url)
			resp, err := http.Get(v.url)
			if err != nil {
				return err
			}

			defer resp.Body.Close()

			base := filepath.Base(v.url)
			name := filepath.Join(tmp, base)
			f, err := os.Create(name)
			if err != nil {
				return err
			}

			defer os.Remove(name)

			if _, err = io.Copy(f, resp.Body); err != nil {
				return err
			}

			if _, err := f.Seek(0, io.SeekStart); err != nil {
				return err
			}

			switch {
			case strings.HasSuffix(base, ".tar.gz"):
				return untar(root, bufio.NewReader(f))
			}
			panic("internal error") //TODOOK
		}(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}

func untar(root string, r io.Reader) error {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err != nil {
			if err != io.EOF {
				return err
			}

			return nil
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			dir := filepath.Join(root, hdr.Name)
			if strings.Contains(dir, "/pkgs/") || strings.HasSuffix(dir, "/pkgs") {
				break
			}

			if err = os.MkdirAll(dir, 0770); err != nil {
				return err
			}
		case tar.TypeSymlink, tar.TypeXGlobalHeader:
			// skip
		case tar.TypeReg, tar.TypeRegA:
			dir := filepath.Dir(filepath.Join(root, hdr.Name))
			if strings.Contains(dir, "/pkgs/") || strings.HasSuffix(dir, "/pkgs") {
				break
			}

			if _, err := os.Stat(dir); err != nil {
				if !os.IsNotExist(err) {
					return err
				}

				if err = os.MkdirAll(dir, 0770); err != nil {
					return err
				}
			}

			f, err := os.OpenFile(filepath.Join(root, hdr.Name), os.O_CREATE|os.O_WRONLY, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}

			w := bufio.NewWriter(f)
			if _, err = io.Copy(w, tr); err != nil {
				return err
			}

			if err := w.Flush(); err != nil {
				return err
			}

			if err := f.Close(); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unexpected tar header typeflag %#02x", hdr.Typeflag)
		}
	}
}
