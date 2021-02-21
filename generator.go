// Copyright 2020 The Tcl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

//TODO enable threads
//TODO 8.6.11
//TODO vfs

package main

import (
	"archive/tar"
	"bufio"
	"bytes"
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

	gcc    = os.Getenv("GO_GENERATE_CC")
	tclDir = filepath.FromSlash("testdata/tcl8.6.10")
)

func fail(s string, args ...interface{}) {
	s = fmt.Sprintf(s, args...)
	fmt.Fprintf(os.Stderr, "\n%v: FAIL\n%s\n", origin(2), s)
	os.Exit(1)
}

func main() {
	if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
		main2()
		return
	}

	env := os.Getenv("GO_GENERATE")
	goarch := runtime.GOARCH
	goos := runtime.GOOS
	if s := os.Getenv("TARGET_GOOS"); s != "" {
		goos = s
	}
	if s := os.Getenv("TARGET_GOARCH"); s != "" {
		goarch = s
	}
	var more []string
	if env != "" {
		more = strings.Split(env, ",")
	}
	download()
	switch goos {
	case "darwin", "linux":
		generate(goos, goarch, "unix", more)
	case "windows":
		generate(goos, goarch, "win", more)
	default:
		fail("unsupported GOOS: %s\n", goos)
	}

	if err := newCmd(nil, nil, "sh", "-c", fmt.Sprintf("cp -r %s %s", filepath.FromSlash(tclDir+"/library/*"), "assets/")).Run(); err != nil {
		fail("error copying tcl library: %v", err)
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

	dir := filepath.FromSlash("assets/tcltests")
	if err := os.MkdirAll(dir, 0770); err != nil {
		fail("cannot create %q: %v", dir, err)
	}

	copyFile("testdata/tcl/pkgIndex.tcl", "assets/tcltests/pkgIndex.tcl")
	copyFile("testdata/tcl/tcltests.tcl", "assets/tcltests/tcltests.tcl")
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

func generate(goos, goarch, dir string, more []string) {
	testWD, err := os.Getwd()
	if err != nil {
		fail("%s\n", err)
	}

	defer os.Chdir(testWD)

	if err := os.Chdir(filepath.Join(tclDir, dir)); err != nil {
		fail("%s\n", err)
	}

	fmt.Printf("pwd: %s\n", filepath.Join(tclDir, dir))
	cmd := newCmd(nil, nil, "make", "distclean")
	if gcc != "" {
		cmd.Env = append(os.Environ(), fmt.Sprintf("CC=%s", gcc))
	}
	cmd.Run()
	args := []string{
		"--disable-dll-unload",
		"--disable-load",
		"--disable-shared",
		"--disable-threads",
		// "--enable-symbols=mem", //TODO-
	}
	if goos == "windows" && goarch == "amd64" {
		args = append(args, "--enable-64bit")
	}
	if goos == "darwin" {
		args = append(args, "--enable-corefoundation=no")
	}
	cmd = newCmd(nil, nil, "./configure", args...)
	if gcc != "" {
		cmd.Env = append(os.Environ(), fmt.Sprintf("CC=%s", gcc))
	}
	if err = cmd.Run(); err != nil {
		fail("%s\n", err)
	}

	makeLibAndShell(testWD, goos, goarch, more)
	makeTclTest(testWD, goos, goarch, more)
}

func makeLibAndShell(testWD string, goos, goarch string, more []string) {
	var stdout strings.Builder
	cmd := newCmd(&stdout, nil, "make")
	if gcc != "" {
		cmd.Env = append(os.Environ(), fmt.Sprintf("CC=%s", gcc))
	}
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
		case
			"i686-w64-mingw32-gcc",
			"x86_64-w64-mingw32-gcc":

			for _, v := range lines {
				if strings.Contains(v, "tclAppInit.o") {
					continue
				}

				for _, w := range strings.Split(v, " ") {
					if strings.HasSuffix(w, ".o") {
						objectFiles = append(objectFiles, w[:len(w)-len(".o")])
					}
				}
				parseCCLine(nil, cFiles, optm, &opts, v, goos)
			}
		case
			"gcc",
			gcc:

			for _, v := range lines {
				if strings.Contains(v, "tclAppInit.o") {
					continue
				}

				parseCCLine(nil, cFiles, optm, &opts, v, goos)
			}
		case
			"cp",
			"i686-w64-mingw32-ar",
			"i686-w64-mingw32-ranlib",
			"i686-w64-mingw32-windres",
			"make[1]:",
			"make[2]:",
			"rm",
			"x86_64-w64-mingw32-ar",
			"x86_64-w64-mingw32-ranlib",
			"x86_64-w64-mingw32-windres":

			// nop
		default:
			fail("unknown command: `%s` in %v\n", k, lines)
		}
	}
	args := []string{
		//"-E", //TODO-
		"-D__printf__=printf",
		"-all-errors",
		"-err-trace",
		"-export-defines", "",
		"-export-enums", "",
		"-export-externs", "X",
		"-export-fields", "F",
		"-export-structs", "",
		"-export-typedefs", "",
		"-o", filepath.Join(testWD, filepath.FromSlash(fmt.Sprintf("lib/tcl_%s_%s.go", goos, goarch))),
		"-pkgname", "tcl",
		"-replace-fd-zero", "__ccgo_fd_zero",
		"-replace-tcl-default-double-rounding", "__ccgo_tcl_default_double_rounding",
		"-replace-tcl-ieee-double-rounding", "__ccgo_tcl_ieee_double_rounding",
		"-trace-translation-units",
	}
	if goos == "windows" && goarch == "386" {
		args = append(args, "-D_USE_32BIT_TIME_T")
	}
	args = append(args, more...)
	args = append(args, opts...)
	args = append(args, "-UHAVE_CAST_TO_UNION")
	switch goos {
	case "windows":
		switch goarch {
		case "amd64":
			args = append(args, "-hide", "TclWinCPUID")
		case "386":
			args = append(args, "-hide", "TclWinCPUID,DoRenameFile,DoCopyFile,Tcl_MakeFileChannel")
		}
	default:
		args = append(args,
			"-hide", "TclpCreateProcess",
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
		)
	}
	cm := map[string]struct{}{}
	for _, v := range objectFiles {
		cFile := cFiles[v]
		if cFile == "" {
			continue
		}

		if _, ok := cm[cFile]; !ok {
			args = append(args, cFile)
			cm[cFile] = struct{}{}
		}
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

	fmt.Println("====")
	args = []string{
		"-D__printf__=printf",
		"-all-errors",
		"-err-trace",
		"-pkgname", "tclsh",
		"-trace-translation-units",
		"-lmodernc.org/tcl/lib",
		"-o", filepath.Join(testWD, filepath.FromSlash(fmt.Sprintf("internal/tclsh/tclsh_%s_%s.go", goos, goarch))),
		"-replace-fd-zero", "__ccgo_fd_zero",
		"-replace-tcl-default-double-rounding", "_ccgo_tcl_default_double_rounding",
		"-replace-tcl-ieee-double-rounding", "_ccgo_tcl_ieee_double_rounding",
		"tclAppInit.c",
	}
	args = append(args, opts...)
	args = append(args, "-UHAVE_CAST_TO_UNION")
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

func makeTclTest(testWD string, goos, goarch string, more []string) {
	var stdout strings.Builder
	cmd := newCmd(&stdout, nil, "make", "tcltest")
	if gcc != "" {
		cmd.Env = append(os.Environ(), fmt.Sprintf("CC=%s", gcc))
	}
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
		case
			"gcc",
			"i686-w64-mingw32-gcc",
			"x86_64-w64-mingw32-gcc",
			gcc:

			for _, v := range lines {
				if strings.Contains(v, "cat32.o") {
					continue
				}

				parseCCLine(&cPaths, cFiles, optm, &opts, v, goos)
			}
		case
			"/Library/Developer/CommandLineTools/usr/bin/make",
			"Create",
			"cp",
			"make",
			"make[1]:",
			"make[2]:",
			"mv",
			"rm",
			"x86_64-w64-mingw32-ar",
			"x86_64-w64-mingw32-ranlib",
			"x86_64-w64-mingw32-windres":

			// nop
		default:
			fail("unknown command: `%s` %v\n", k, lines)
		}
	}
	args := []string{
		"-o", filepath.Join(testWD, filepath.FromSlash(fmt.Sprintf("internal/tcltest/tcltest_%s_%s.go", goos, goarch))),
		"-D__printf__=printf",
		"-all-errors",
		"-err-trace",
		"-replace-fd-zero", "__ccgo_fd_zero",
		"-replace-tcl-default-double-rounding", "_ccgo_tcl_default_double_rounding",
		"-replace-tcl-ieee-double-rounding", "_ccgo_tcl_ieee_double_rounding",
		"-trace-translation-units",
		"../generic/tclOOStubLib.c",
		"../generic/tclStubLib.c",
		"../generic/tclTomMathStubLib.c",
		"-lmodernc.org/tcl/lib",
	}
	args = append(args, more...)
	args = append(args, opts...)
	args = append(args, "-UHAVE_CAST_TO_UNION")
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
	os.Remove(filepath.Join(testWD, filepath.FromSlash(fmt.Sprintf("internal/tcltest/capi_%s_%s.go", goos, goarch))))
}

func parseCCLine(cPaths *[]string, cFiles map[string]string, m map[string]struct{}, opts *[]string, line, goos string) {
	if strings.HasSuffix(line, "-o tcltest") {
		return
	}

	skip := false
	for _, tok := range splitCCLine(line) {
		if skip {
			skip = false
			continue
		}

		tok = unquote(tok)
		switch {
		case tok == "-o":
			skip = true
		case
			strings.HasPrefix(tok, "-O"),
			strings.HasPrefix(tok, "-W"),
			strings.HasPrefix(tok, "-f"),
			strings.HasPrefix(tok, "-l"),
			strings.HasSuffix(tok, ".a"),
			strings.HasSuffix(tok, ".o"),
			tok == "-c",
			tok == "-g",
			tok == "-mconsole",
			tok == "-mdynamic-no-pic",
			tok == "-pipe",
			tok == "-shared",
			tok == "-static-libgcc":

			// nop
		case strings.HasPrefix(tok, "-D"):
			if tok == "-DHAVE_CPUID=1" {
				break
			}

			if goos == "darwin" && strings.HasPrefix(tok, "-DHAVE_COPYFILE") {
				break
			}

			//TODO- if tok == "-DNDEBUG=1" { //TODO-
			//TODO- 	break
			//TODO- }

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
			s := unquote(tok[2:])
			tok = "-I" + s
			if _, ok := m[tok]; ok {
				break
			}

			m[tok] = struct{}{}
			*opts = append(*opts, tok)
		case strings.HasSuffix(tok, ".c"):
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

func unquote(s string) string {
	if strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"") {
		var err error
		if s, err = strconv.Unquote(s); err != nil {
			fail("%q", s)
		}
	}

	return s
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
		i := strings.IndexByte(v, ' ')
		j := strings.IndexByte(v, '\t')
		switch {
		case i < 0:
			i = j
		case j < 0:
			// nop
		case j < i:
			i = j
		}
		if i > 0 {
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
				return untar0(root, bufio.NewReader(f))
			}

			panic("internal error") //TODOOK
		}(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}

func untar0(root string, r io.Reader) error {
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

// ============================================================== new generator

const (
	tarDir = "tcl8.6.10"
	tarFn  = tarVer + ".tar.gz"
	tarVer = tarDir + "-src"
)

func origin(skip int) string {
	pc, fn, fl, _ := runtime.Caller(skip)
	fn = filepath.Base(fn)
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

func todo(s string, args ...interface{}) string { //TODO-
	switch {
	case s == "":
		s = fmt.Sprintf(strings.Repeat("%v ", len(args)), args...)
	default:
		s = fmt.Sprintf(s, args...)
	}
	pc, fn, fl, _ := runtime.Caller(1)
	f := runtime.FuncForPC(pc)
	var fns string
	if f != nil {
		fns = f.Name()
		if x := strings.LastIndex(fns, "."); x > 0 {
			fns = fns[x+1:]
		}
	}
	r := fmt.Sprintf("%s:%d:%s: TODOTODO %s", fn, fl, fns, s) //TODOOK
	fmt.Fprintf(os.Stdout, "%s\n", r)
	os.Stdout.Sync()
	return r
}

func trc(s string, args ...interface{}) string { //TODO-
	switch {
	case s == "":
		s = fmt.Sprintf(strings.Repeat("%v ", len(args)), args...)
	default:
		s = fmt.Sprintf(s, args...)
	}
	_, fn, fl, _ := runtime.Caller(1)
	r := fmt.Sprintf("%s:%d: TRC %s", fn, fl, s)
	fmt.Fprintf(os.Stdout, "%s\n", r)
	os.Stdout.Sync()
	return r
}

type supportedKey = struct{ os, arch string }

var (
	goos      = env("TARGET_GOOS", runtime.GOOS)
	goarch    = env("TARGET_GOARCH", runtime.GOARCH)
	supported = map[supportedKey]struct{}{
		{"darwin", "amd64"}:  {},
		{"linux", "386"}:     {},
		{"linux", "amd64"}:   {},
		{"linux", "arm"}:     {},
		{"linux", "arm64"}:   {},
		{"windows", "386"}:   {},
		{"windows", "amd64"}: {},
	}
	tmpDir = os.Getenv("GO_GENERATE_TMPDIR")
	ccgo   string
)

func env(key, dflt string) string {
	if s := os.Getenv(key); s != "" {
		return s
	}

	return dflt
}

func fatalf(s string, args ...interface{}) {
	s = strings.TrimRight(fmt.Sprintf(s, args...), " \t\n\r")
	fmt.Fprintln(os.Stderr, s)
	os.Exit(1)
}

func fatal(args ...interface{}) {
	s := strings.TrimRight(fmt.Sprint(args...), " \t\n\r")
	fmt.Fprintln(os.Stderr, s)
	// trc("\n%s", debug.Stack()) //TODO-
	os.Exit(1)
}

func main2() {
	if _, ok := supported[supportedKey{goos, goarch}]; !ok {
		fatalf("unknown/unsupported os/arch combination: %s/%s\n", goos, goarch)
	}

	var err error
	if ccgo, err = exec.LookPath("ccgo"); err != nil {
		fatal(err)
	}

	if err := mkdir("lib"); err != nil {
		fatal(err)
	}

	if err := mkdir(
		"internal/tclsh",
		"internal/tcltest",
		"lib",
		"testdata/tcl",
	); err != nil {
		fatal(err)
	}

	if tmpDir == "" {
		var err error
		if tmpDir, err = ioutil.TempDir("", "go-generate-tcl-"); err != nil {
			fatal(err)
		}

		defer os.RemoveAll(tmpDir)
	}
	if tmpDir, err = filepath.Abs(tmpDir); err != nil {
		fatal(err)
	}

	f, err := os.Open(tarFn)
	if err != nil {
		fatal(err)
	}

	defer f.Close()

	if err := inDir(tmpDir, func() error {
		os.RemoveAll(tarDir)
		if err := untar("", bufio.NewReader(f)); err != nil {
			fatal(err)
		}

		return nil
	}); err != nil {
		fatal(err)
	}

	cdb := filepath.Join(tmpDir, "cdb.json")
	mkDB := false
	if _, err := os.Stat(cdb); err != nil {
		if !os.IsNotExist(err) {
			fatal(err)
		}

		mkDB = true
	}

	if mkDB {
		cfg := []string{
			"--disable-dll-unload",
			"--disable-load",
			"--disable-shared",
			"--disable-threads", //TODO-
			// "--enable-symbols=mem", //TODO-
		}
		switch goos {
		case "windows":
			panic(todo(""))
		default:
			if err := inDir(filepath.Join(tmpDir, tarDir, "unix"), func() error {
				if gcc != "" {
					os.Setenv("CC", gcc)
				}
				if out, err := shell("./configure", cfg...); err != nil {
					fatalf("%s\n%v", out, err)
				}

				if out, err := shell("ccgo", "-compiledb", cdb, "make", "CFLAGS=-UHAVE_CPUID", "binaries", "tcltest"); err != nil {
					fatalf("%s\n%v", out, err)
				}

				return nil
			}); err != nil {
				fatal(err)
			}
		}
	}
	mustCC(os.Stdout, os.Stderr,
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
	mustCC(os.Stdout, os.Stderr,
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
	mustCC(os.Stdout, os.Stderr,
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

	if err := newCmd(nil, nil, "sh", "-c", fmt.Sprintf("cp -r %s %s", filepath.FromSlash(tclDir+"/library/*"), "assets/")).Run(); err != nil {
		fail("error copying tcl library: %v", err)
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

	dir := filepath.FromSlash("assets/tcltests")
	if err := os.MkdirAll(dir, 0770); err != nil {
		fail("cannot create %q: %v", dir, err)
	}

	copyFile("testdata/tcl/pkgIndex.tcl", "assets/tcltests/pkgIndex.tcl")
	copyFile("testdata/tcl/tcltests.tcl", "assets/tcltests/tcltests.tcl")
}

func mkdir(paths ...string) error {
	for _, path := range paths {
		path = filepath.FromSlash(path)
		if err := os.MkdirAll(path, 0770); err != nil {
			return err
		}
	}
	return nil
}

func inDir(dir string, f func() error) (err error) {
	var cwd string
	if cwd, err = os.Getwd(); err != nil {
		return err
	}

	defer func() {
		if err2 := os.Chdir(cwd); err2 != nil {
			err = err2
		}
	}()

	if err = os.Chdir(dir); err != nil {
		return err
	}

	return f()
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
			if err = os.MkdirAll(dir, 0770); err != nil {
				return err
			}
		case tar.TypeSymlink, tar.TypeXGlobalHeader:
			// skip
		case tar.TypeReg, tar.TypeRegA:
			dir := filepath.Dir(filepath.Join(root, hdr.Name))
			if _, err := os.Stat(dir); err != nil {
				if !os.IsNotExist(err) {
					return err
				}

				if err = os.MkdirAll(dir, 0770); err != nil {
					return err
				}
			}

			fn := filepath.Join(root, hdr.Name)
			f, err := os.OpenFile(fn, os.O_CREATE|os.O_WRONLY, os.FileMode(hdr.Mode))
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

			if err := os.Chtimes(fn, hdr.AccessTime, hdr.ModTime); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unexpected tar header typeflag %#02x", hdr.Typeflag)
		}
	}
}

type echoWriter struct {
	w bytes.Buffer
}

func (w *echoWriter) Write(b []byte) (int, error) {
	os.Stdout.Write(b)
	return w.w.Write(b)
}

func shell(cmd string, args ...string) ([]byte, error) {
	wd, err := absCwd()
	if err != nil {
		return nil, err
	}

	fmt.Printf("execute %s %q in %s\n", cmd, args, wd)
	var b echoWriter
	c := exec.Command(cmd, args...)
	c.Stdout = &b
	c.Stderr = &b
	err = c.Run()
	return b.w.Bytes(), err
}

func absCwd() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	if wd, err = filepath.Abs(wd); err != nil {
		return "", err
	}

	return wd, nil
}

func cc(stdout, stderr io.Writer, args ...string) error {
	cmd := exec.Command(ccgo, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func mustCC(stdout, stderr io.Writer, args ...string) {
	if err := cc(stdout, stderr, args...); err != nil {
		fatal(err)
	}
}
