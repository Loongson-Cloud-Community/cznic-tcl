# Copyright 2020 The Tcl Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

.PHONY:	all clean cover cpu editor internalError later mem nuke todo edit tcl

grep=--include=*.go --include=*.l --include=*.y --include=*.yy
ngrep='TODOOK\|internal\|.*stringer.*\.go\|assets\.go'
log=log-$(shell go env GOOS)-$(shell go env GOARCH)

all: editor
	date
	go version 2>&1 | tee $(log)
	./unconvert.sh
	gofmt -l -s -w *.go
	go test -i
	go test 2>&1 -timeout 1h | tee -a $(log)
	#TODO GOOS=linux GOARCH=arm go build -o /dev/null
	#TODO GOOS=linux GOARCH=arm64 go build -o /dev/null
	GOOS=linux GOARCH=386 go build -o /dev/null
	GOOS=linux GOARCH=amd64 go build -o /dev/null
	#TODO GOOS=windows GOARCH=386 go build -o /dev/null
	#TODO GOOS=windows GOARCH=amd64 go build -o /dev/null
	go vet 2>&1 | grep -v $(ngrep) || true
	golint 2>&1 | grep -v $(ngrep) || true
	#make todo
	misspell *.go | grep -v $(ngrep) || true
	staticcheck
	maligned || true
	grep -n 'FAIL\|PASS' $(log) 
	git diff --unified=0 testdata/*.golden || true
	grep -n Passed $(log) 
	go version
	date 2>&1 | tee -a $(log)

linux_386:
	\
		CCGO_CPP=i686-linux-gnu-cpp \
		GO_GENERATE_CC=i686-linux-gnu-gcc \
		TARGET_GOARCH=386 \
		TARGET_GOOS=linux \
		go generate 2>&1 | tee /tmp/log-generate-tcl-linux-386
	GOOS=linux GOARCH=386 go build -v ./...

linux_amd64:
	\
		TARGET_GOOS=linux \
		TARGET_GOARCH=amd64 \
		go generate 2>&1 | tee /tmp/log-generate-tcl-linux-amd64
	GOOS=linux GOARCH=amd64 go build -v ./...


clean:
	go clean
	rm -f *~ *.test *.out test.db* tt4-test*.db* test_sv.* testdb-*

cover:
	t=$(shell tempfile) ; go test -coverprofile $$t && go tool cover -html $$t && unlink $$t

cpu: clean
	go test -run @ -bench . -cpuprofile cpu.out
	go tool pprof -lines *.test cpu.out

edit:
	gvim -p Makefile *.go &

editor:
	gofmt -l -s -w *.go
	go install -v ./...

internalError:
	egrep -ho '"internal error.*"' *.go | sort | cat -n

later:
	@grep -n $(grep) LATER * || true
	@grep -n $(grep) MAYBE * || true

mem: clean
	go test -run @ -bench . -memprofile mem.out -memprofilerate 1 -timeout 24h
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
