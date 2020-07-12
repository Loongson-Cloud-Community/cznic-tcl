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
	fmt.Fprintf(os.Stderr, s, args...)
	os.Exit(1)
}

func main() {
	download()
	switch runtime.GOOS {
	case "linux":
		makeUnix()
	default:
		fail("unsupported GOOS: %s\n", runtime.GOOS)
	}
}

func makeUnix() {
	wd, err := os.Getwd()
	if err != nil {
		fail("%s\n", err)
	}

	defer os.Chdir(wd)

	if err := os.Chdir(filepath.Join(tclDir, "unix")); err != nil {
		fail("%s\n", err)
	}

	newCmd(nil, nil, "make", "distclean").Run()
	cmd := newCmd(nil, nil, "./configure",
		"--disable-dll-unload",
		"--disable-load",
		"--disable-threads",
	)
	if err = cmd.Run(); err != nil {
		fail("%s\n", err)
	}

	var stdout strings.Builder
	cmd = newCmd(&stdout, nil, "make")
	if err = cmd.Run(); err != nil {
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

				parseCCLine(cFiles, optm, &opts, v)
			}
		case "rm":
			// nop
		default:
			fail("unknown command: %q\n", k)
		}
	}
	args := []string{
		"-o", filepath.Join(wd, filepath.FromSlash(fmt.Sprintf("lib/tcl_%s_%s.go", runtime.GOOS, runtime.GOARCH))),
		"-ccgo-export-externs", "X",
		"-ccgo-export-fields", "F",
		"-ccgo-long-double-is-double",
		"-ccgo-pkgname", "tcl",
		//TODO- "-ccgo-watch-instrumentation", //TODO-
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
}

func parseCCLine(cFiles map[string]string, m map[string]struct{}, opts *[]string, line string) {
	// fmt.Printf("line `%s`\n", line) //TODO-
	for _, tok := range splitCCLine(line) {
		// fmt.Printf("tok `%s`\n", tok) //TODO-
		switch {
		case
			tok == "-c",
			tok == "-pipe",
			strings.HasPrefix(tok, "-W"),
			strings.HasPrefix(tok, "-O"):

			// nop
		case strings.HasPrefix(tok, "-D"):
			if tok == "-DHAVE_CPUID=1" {
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
							fail("invalid defintion: `%s`", tok)
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
			// fmt.Printf("`%s`\n", tok) //TODO-
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
			// fmt.Printf("`%s`\n", tok) //TODO-
		case strings.HasPrefix(tok, "/") && strings.HasSuffix(tok, ".c"):
			f := filepath.Base(tok)
			cFiles[f[:len(f)-len(".c")]] = tok
		default:
			fail("TODO %q\n", tok)
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
