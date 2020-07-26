// Copyright 2020 The Tcl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go run generator.go
//go:generate assets -package tcl
//go:generate gofmt -l -s -w .

package tcl

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"modernc.org/httpfs"
)

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

func todo(s string, args ...interface{}) string { //TODO-
	switch {
	case s == "":
		s = fmt.Sprintf(strings.Repeat("%v ", len(args)), args...)
	default:
		s = fmt.Sprintf(s, args...)
	}
	r := fmt.Sprintf("%s: TODOTODO %s", origin(2), s) //TODOOK
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
	r := fmt.Sprintf("\n%s: TRC %s", origin(2), s)
	fmt.Fprintf(os.Stdout, "%s\n", r)
	os.Stdout.Sync()
	return r
}

// LibraryFileSystem returns a http.FileSystem containing the Tcl library.
func LibraryFileSystem() http.FileSystem {
	return httpfs.NewFileSystem(assets, time.Now())
}

// Library writes the Tcl library to directory.
func Library(directory string) error {
	var a []string
	for k := range assets {
		a = append(a, k)
	}
	sort.Strings(a)
	dirs := map[string]struct{}{}
	for _, nm := range a {
		pth := filepath.Join(directory, filepath.FromSlash(nm))
		dir := filepath.Dir(pth)
		if _, ok := dirs[dir]; !ok {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return err
			}

			dirs[dir] = struct{}{}
		}
		f, err := os.Create(pth)
		if err != nil {
			return err
		}

		if _, err := f.Write([]byte(assets[nm])); err != nil {
			f.Close()
			return err
		}

		if err = f.Close(); err != nil {
			return err
		}
	}
	return nil
}
