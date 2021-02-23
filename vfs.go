// Copyright 2021 The Tcl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tcl // import "modernc.org/tcl"

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
	"unsafe"

	"modernc.org/httpfs"
	"modernc.org/libc"
	"modernc.org/libc/sys/types"
	ctime "modernc.org/libc/time"
	"modernc.org/tcl/lib"
)

const (
	tclChannelVersion_2   = 2
	tclFilesystemVersion1 = 1
	vfsName               = "govfs"
)

var (
	_               = copy(cVFSName[:], vfsName)
	cVFSName        [len(vfsName) + 1]byte
	vfsIsRegistered bool
	vfsMounts       = map[string]FileSystem{}
	vfsMu           sync.Mutex
	vfsPoints       []string
)

// FileSystem represents a read-only virtual file system.
type FileSystem interface {
	http.FileSystem
}

// MountFileSystem mounts a virtual file system at point, which should be an
// absolute, slash separated path.
func MountFileSystem(point string, fs FileSystem) error {
	point, err := normalizeMountPoint(point)
	if err != nil {
		return err
	}

	vfsMu.Lock()

	defer vfsMu.Unlock()

	if !vfsIsRegistered {
		tls := libc.NewTLS()

		defer tls.Close()

		if rc := tcl.XTcl_FSRegister(tls, 0, uintptr(unsafe.Pointer(&vfs))); rc != tcl.TCL_OK {
			return fmt.Errorf("virtual file system initialization failed: %d", rc)
		}

		vfsIsRegistered = true
	}

	lockedUnmountFileSystem(point)
	vfsMounts[point] = fs
	vfsPoints = append(vfsPoints, point)
	sort.Strings(vfsPoints)
	return nil
}

// UnmountFileSystem unmounts a virtual file system at point, which should be
// an absolute, slash separated path.
func UnmountFileSystem(point string) error {
	vfsMu.Lock()

	defer vfsMu.Unlock()

	return lockedUnmountFileSystem(point)
}

func lockedUnmountFileSystem(point string) error {
	point, err := normalizeMountPoint(point)
	if err != nil {
		return err
	}

	if vfsMounts[point] == nil {
		return fmt.Errorf("no file system mounted: %q", point)
	}

	i := sort.Search(len(vfsPoints), func(i int) bool { return vfsPoints[i] >= point })
	vfsPoints = append(vfsPoints[:i], vfsPoints[i+1:]...)
	delete(vfsMounts, point)
	return nil
}

func normalizeMountPoint(s string) (string, error) {
	s = path.Clean(s)
	if s == "." {
		return "", fmt.Errorf("invalid file system mount point: %s", s)
	}

	if s != "/" {
		s += "/"
	}

	return s, nil
}

// MountLibraryVFS mounts the Tcl library virtual file system and returns the
// mount point. This is how it's used, for example, in gotclsh:
//
//	package main
//
//	import (
//		"os"
//
//		"modernc.org/libc"
//		"modernc.org/tcl"
//		"modernc.org/tcl/internal/tclsh"
//	)
//
//	const envVar = "TCL_LIBRARY"
//
//	func main() {
//		if os.Getenv(envVar) == "" {
//			if s, err := tcl.MountLibraryVFS(); err == nil {
//				os.Setenv(envVar, s)
//			}
//		}
//		libc.Start(tclsh.Main)
//	}
func MountLibraryVFS() (string, error) {
	point := tcl.TCL_LIBRARY
	if err := MountFileSystem(point, httpfs.NewFileSystem(assets, time.Now())); err != nil {
		return "", err
	}

	return point, nil
}

var vfs = tcl.Tcl_Filesystem{
	FtypeName:        uintptr(unsafe.Pointer(&cVFSName[0])),
	FstructureLength: int32(unsafe.Sizeof(tcl.Tcl_Filesystem{})),
	Fversion:         tclFilesystemVersion1,
	FpathInFilesystemProc: *(*uintptr)(unsafe.Pointer(&struct {
		f func(tls *libc.TLS, pathPtr uintptr, clientDataPtr uintptr) int32
	}{vfsPathInFilesystem})),
	FstatProc: *(*uintptr)(unsafe.Pointer(&struct {
		f func(tls *libc.TLS, pathPtr uintptr, bufPtr uintptr) int32
	}{vfsStat})),
	FaccessProc: *(*uintptr)(unsafe.Pointer(&struct {
		f func(tls *libc.TLS, pathPtr uintptr, mode int32) int32
	}{vfsAccess})),
	FopenFileChannelProc: *(*uintptr)(unsafe.Pointer(&struct {
		f func(tls *libc.TLS, interp uintptr, pathPtr uintptr, mode int32, permissions int32) tcl.Tcl_Channel
	}{vfsOpenFileChannel})),
}

func vfsPathInFilesystem(tls *libc.TLS, pathPtr uintptr, clientDataPtr uintptr) int32 {
	path := libc.GoString(tcl.XTcl_GetString(tls, pathPtr))

	vfsMu.Lock()

	defer vfsMu.Unlock()

	if findVFSprefix(path) >= 0 {
		return tcl.TCL_OK
	}

	return -1
}

func vfsStat(tls *libc.TLS, pathPtr uintptr, bufPtr uintptr) int32 {
	vfsMu.Lock()

	defer vfsMu.Unlock()

	fi := vfsFileInfo(libc.GoString(tcl.XTcl_GetString(tls, pathPtr)))
	if fi == nil {
		return -1
	}

	tm := ctime.Timespec{Ftv_sec: fi.ModTime().Unix()}
	*(*tcl.Tcl_StatBuf)(unsafe.Pointer(bufPtr)) = tcl.Tcl_StatBuf{
		Fst_atim: tm,
		Fst_ctim: tm,
		Fst_mode: types.Mode_t(fi.Mode()),
		Fst_mtim: tm,
		Fst_size: types.Off_t(fi.Size()),
	}
	return 0
}

func vfsFile(path string) http.File {
	point := vfsPoints[findVFSprefix(path)]
	fs := vfsMounts[point]
	abs := path[len(point)-1:]
	file, err := fs.Open(abs)
	if err != nil {
		if !strings.HasSuffix(abs, "/") {
			file, err = fs.Open(abs + "/")
		}
	}
	if err != nil {
		return nil
	}

	return file
}

func vfsFileInfo(path string) os.FileInfo {
	file := vfsFile(path)
	if file == nil {
		return nil
	}

	fi, err := file.Stat()
	if err != nil {
		return nil
	}

	return fi
}

func vfsAccess(tls *libc.TLS, pathPtr uintptr, mode int32) int32 {
	vfsMu.Lock()

	defer vfsMu.Unlock()

	fi := vfsFileInfo(libc.GoString(tcl.XTcl_GetString(tls, pathPtr)))
	if fi == nil {
		return -1
	}

	switch {
	case fi.IsDir():
		if mode&0222 != 0 { // deny write
			return -1
		}
	default:
		if mode&0333 != 0 { // deny write, exec
			return -1
		}
	}

	return 0
}

func vfsOpenFileChannel(tls *libc.TLS, interp uintptr, pathPtr uintptr, mode int32, permissions int32) tcl.Tcl_Channel {
	vfsMu.Lock()

	defer vfsMu.Unlock()

	cPath := tcl.XTcl_GetString(tls, pathPtr)
	path := libc.GoString(cPath)
	file := vfsFile(path)
	if file == nil {
		panic(todo("%q", path))
	}

	return tcl.XTcl_CreateChannel(tls, uintptr(unsafe.Pointer(&channel)), cPath, addObject(file), tcl.TCL_READABLE)
}

func findVFSprefix(path string) int {
	if len(vfsPoints) == 0 {
		return -1
	}

	i := sort.Search(len(vfsPoints), func(i int) bool { return vfsPoints[i] >= path })
	if i == len(vfsPoints) {
		if strings.HasPrefix(path, vfsPoints[i-1]) {
			return i - 1
		}

		return -1
	}

	if strings.HasPrefix(path, vfsPoints[i]) {
		return i
	}

	return -1
}

var channel = tcl.Tcl_ChannelType{
	FtypeName: uintptr(unsafe.Pointer(&cVFSName[0])),
	Fversion:  tclChannelVersion_2,
	FcloseProc: *(*uintptr)(unsafe.Pointer(&struct {
		f func(tls *libc.TLS, instanceData tcl.ClientData, interp uintptr) int32
	}{channelClose})),
	FinputProc: *(*uintptr)(unsafe.Pointer(&struct {
		f func(tls *libc.TLS, instanceData tcl.ClientData, buf uintptr, toRead int32, errorCodePtr uintptr) int32
	}{channelInput})),
	FseekProc: *(*uintptr)(unsafe.Pointer(&struct {
		f func(tls *libc.TLS, instanceData tcl.ClientData, offset int64, mode int32, errorCodePtr uintptr) int32
	}{channelSeek})),
	FwatchProc: *(*uintptr)(unsafe.Pointer(&struct {
		f func(tls *libc.TLS, instanceData tcl.ClientData, mask int32)
	}{channelWatch})),
}

func channelClose(tls *libc.TLS, instanceData tcl.ClientData, interp uintptr) int32 {
	removeObject(instanceData)
	return 0
}

func channelInput(tls *libc.TLS, instanceData tcl.ClientData, buf uintptr, toRead int32, errorCodePtr uintptr) int32 {
	if buf == 0 || toRead == 0 {
		return 0
	}

	n, err := getObject(instanceData).(http.File).Read((*libc.RawMem)(unsafe.Pointer(buf))[:toRead:toRead])
	if n != 0 {
		return int32(n)
	}

	if err != nil && err != io.EOF {
		return -1
	}

	return 0
}

func channelSeek(tls *libc.TLS, instanceData tcl.ClientData, offset int64, mode int32, errorCodePtr uintptr) int32 {
	panic(todo(""))
}

func channelWatch(tls *libc.TLS, instanceData tcl.ClientData, mask int32) {
	if mask != 0 {
		panic(todo(""))
	}
}
