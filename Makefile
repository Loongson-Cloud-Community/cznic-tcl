# Copyright 2020 The Tcl Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

.PHONY:	all clean cover cpu editor internalError later mem nuke todo edit tcl

grep=--include=*.go --include=*.l --include=*.y --include=*.yy
ngrep='TODOOK\|internal\|.*stringer.*\.go\|assets\.go'
host=$(shell go env GOOS)-$(shell go env GOARCH)
testlog=testdata/testlog-$(shell echo $$GOOS)-$(shell echo $$GOARCH)-on-$(shell go env GOOS)-$(shell go env GOARCH)

all: editor
	date
	go version 2>&1 | tee log
	./unconvert.sh
	gofmt -l -s -w *.go
	go test -i
	go test 2>&1 -timeout 1h | tee -a log
	GOOS=darwin GOARCH=amd64 go build -o /dev/null
	GOOS=linux GOARCH=386 go build -o /dev/null
	GOOS=linux GOARCH=amd64 go build -o /dev/null
	GOOS=linux GOARCH=arm go build -o /dev/null
	GOOS=linux GOARCH=arm64 go build -o /dev/null
	GOOS=windows GOARCH=386 go build -o /dev/null
	GOOS=windows GOARCH=amd64 go build -o /dev/null
	go vet 2>&1 | grep -v $(ngrep) || true
	golint 2>&1 | grep -v $(ngrep) || true
	#make todo
	misspell *.go | grep -v $(ngrep) || true
	staticcheck
	maligned || true
	grep -n 'FAIL\|PASS' log 
	git diff --unified=0 testdata/*.golden || true
	grep -n Passed log 
	go version
	date 2>&1 | tee -a log

darwin_amd64:
	TARGET_GOOS=darwin TARGET_GOARCH=amd64 go generate 2>&1 | tee /tmp/log-generate-tcl-darwin-amd64
	GOOS=darwin GOARCH=amd64 go build -v ./...

linux_amd64:
	TARGET_GOOS=linux TARGET_GOARCH=amd64 go generate 2>&1 | tee /tmp/log-generate-tcl-linux-amd64
	GOOS=linux GOARCH=amd64 go build -v ./...

linux_386:
	CCGO_CPP=i686-linux-gnu-cpp GO_GENERATE_CC=i686-linux-gnu-gcc TARGET_GOOS=linux TARGET_GOARCH=386 go generate 2>&1 | tee /tmp/log-generate-tcl-linux-386
	GOOS=linux GOARCH=386 go build -v ./...

linux_arm:
	QEMU_LD_PREFIX=/usr/arm-linux-gnueabi CCGO_CPP=arm-linux-gnueabi-cpp-8 GO_GENERATE_CC=arm-linux-gnueabi-gcc-8 TARGET_GOOS=linux TARGET_GOARCH=arm go generate 2>&1 | tee /tmp/log-generate-tcl-linux-arm
	GOOS=linux GOARCH=arm go build -v ./...

linux_arm64:
	QEMU_LD_PREFIX=/usr/aarch64-linux-gnu CCGO_CPP=aarch64-linux-gnu-cpp-8 GO_GENERATE_CC=aarch64-linux-gnu-gcc-8 TARGET_GOOS=linux TARGET_GOARCH=arm64 go generate 2>&1 | tee /tmp/log-generate-tcl-linux-arm64
	GOOS=linux GOARCH=arm64 go build -v ./...

windows_amd64:
	CCGO_CPP=x86_64-w64-mingw32-cpp TARGET_GOOS=windows TARGET_GOARCH=amd64 go generate 2>&1 | tee /tmp/log-generate-tcl-windows-amd64
	GOOS=windows GOARCH=amd64 go build -v ./...

windows_386:
	CCGO_CPP=i686-w64-mingw32-cpp TARGET_GOOS=windows TARGET_GOARCH=386 go generate 2>&1 | tee /tmp/log-generate-tcl-windows-386
	GOOS=windows GOARCH=386 go build -v ./...

all_targets: linux_amd64 linux_386 linux_arm linux_arm64 windows_amd64 windows_386
	echo done

test:
	go version | tee $(testlog)
	uname -a | tee -a $(testlog)
	go test -v -timeout 24h | tee -a $(testlog)
	grep -ni fail $(testlog) | tee -a $(testlog) || true
	LC_ALL=C date | tee -a $(testlog)
	grep -ni --color=always fail $(testlog) || true

test_darwin_amd64:
	GOOS=darwin GOARCH=amd64 make test

test_linux_amd64:
	GOOS=linux GOARCH=amd64 make test

test_linux_386:
	GOOS=linux GOARCH=386 make test

test_linux_386_hosted:
	GOOS=linux GOARCH=386 make test

test_linux_arm:
	GOOS=linux GOARCH=arm make test

test_linux_arm64:
	GOOS=linux GOARCH=arm64 make test

test_windows_amd64:
	rm -f y:\\libc.log
	go version | tee %TEMP%\testlog-windows-amd64
	go test -v -timeout 24h | tee -a %TEMP%\testlog-windows-amd64
	date /T | tee -a %TEMP%\testlog-windows-amd64
	time /T | tee -a %TEMP%\testlog-windows-amd64

tmp: #TODO-
	cls
	go test -v -timeout 24h -run Tcl -verbose "start pass error" | tee -a %TEMP%\testlog-windows-amd64

clean:
	go clean
	rm -f *~ *.test *.out test.db* tt4-test*.db* test_sv.* testdb-*

cover:
	t=$(shell tempfile) ; go test -coverprofile $$t && go tool cover -html $$t && unlink $$t

cpu: clean
	go test -run @ -bench . -cpuprofile cpu.out
	go tool pprof -lines *.test cpu.out

edit:
	@touch log
	@if [ -f "Session.vim" ]; then gvim -S & else gvim -p Makefile *.go & fi

editor:
	gofmt -l -s -w *.go
	go install -v ./...

internalError:
	egrep -ho '"internal error.*"' *.go | sort | cat -n

later:
	@grep -n $(grep) LATER * || true
	@grep -n $(grep) MAYBE * || true

mem: clean
	go test -run Mem -mem -memprofile mem.out -timeout 24h
	go tool pprof -lines -web -alloc_space *.test mem.out

nuke: clean
	go clean -i

todo:
	@grep -nr $(grep) ^[[:space:]]*_[[:space:]]*=[[:space:]][[:alpha:]][[:alnum:]]* * | grep -v $(ngrep) || true
	@grep -nr $(grep) TODO * | grep -v $(ngrep) || true
	@grep -nr $(grep) BUG * | grep -v $(ngrep) || true
	@grep -nr $(grep) [^[:alpha:]]println * | grep -v $(ngrep) || true

tcl:
	cp log log-0
	go test -run Tcl$$ 2>&1 -timeout 24h -trc | tee log
	grep -c '\.\.\. \?Ok' log || true
	grep -c '^!' log || true
	# grep -c 'Error:' log || true

tclshort:
	cp log log-0
	go test -run Tcl$$ -short 2>&1 -timeout 24h -trc | tee log
	grep -c '\.\.\. \?Ok' log || true
	grep -c '^!' log || true
	# grep -c 'Error:' log || true
