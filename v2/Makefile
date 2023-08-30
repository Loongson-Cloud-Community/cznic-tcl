# Copyright 2023 The Tcl Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

.PHONY:	all clean dev edit editor generate work

all: editor
	golint 2>&1
	staticcheck 2>&1

build_all_targets:
	./build_all_targets.sh
	echo done

clean:
	rm -f log-* cpu.test mem.test *.out go.work*
	go clean

edit:
	@touch log
	@if [ -f "Session.vim" ]; then gvim -S & else gvim -p Makefile *.go & fi

editor:
	gofmt -l -s -w . 2>&1 | tee log-editor
	go test -c -o /dev/null 2>&1 | tee -a log-editor
	go install -v  ./... 2>&1 | tee -a log-editor
	go build -o /dev/null generator*.go

generate:
	echo -n > /tmp/ccgo.log
	echo -n > log-generate
	echo -n > log-generate-errors
	go run generator*.go 2> log-generate-errors | tee log-generate
	cat log-generate-errors
	go build -v ./...
	# go install github.com/mdempsky/unconvert@latest
	./unconvert.sh
	go build -v ./...  | tee -a log-generate
	go test -v -count=1 ./... 2>&1 | tee -a log-generate
	git status
	grep TODO log-generate || true

dev:
	echo -n > /tmp/ccgo.log
	echo -n > log-generate
	echo -n > log-generate-errors
	date 2>&1 | tee -a log-generate
	GO_GENERATE_DEV=1 go run -tags=ccgo.dmesg,ccgo.assert generator*.go 2>&1 | tee -a log-generate
	date 2>&1 | tee -a log-generate
	./unconvert.sh
	date 2>&1 | tee -a log-generate
	go build -v ./...  | tee -a log-generate
	go test -v -count=1 ./... 2>&1 | tee -a log-generate
	git status
	grep 'TODO\|FAIL' /tmp/ccgo.log || true
	grep 'TODO\|FAIL' log-generate || true
	grep $(shell date '+%B') log-generate

work:
	rm -f go.work*
	go work init
	go work use .
	go work use ../../ccgo/v4
	go work use ../../ccgo/v3
	go work use ../../cc/v4
	go work use ../../libc/v2
