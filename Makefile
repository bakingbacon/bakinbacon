# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build -v
GOCLEAN=$(GOCMD) clean
GOFMT=gofmt -d -s
GOGET=$(GOCMD) get
BINARY_NAME=bakinbacon

LINUX_BINARY=$(BINARY_NAME)-linux-amd64
DARWIN_BINARY=$(BINARY_NAME)-darwin-amd64
WINDOWS_BASE=$(BINARY_NAME)-windows-amd64
WINDOWS_BINARY=$(WINDOWS_BASE).exe

GIT_COMMIT := $(shell git rev-list -1 HEAD | cut -c 1-6)
SOURCES := $(shell find ./ -name '*.go')
PWD=$(shell pwd)

FMT=$(fmt)
UI=$(ui)
UI_DEV=$(ui-dev)

all: build

build: $(UI) $(FMT) $(SOURCES)
	$(GOBUILD) -o $(LINUX_BINARY) -ldflags "-X main.commitHash=$(GIT_COMMIT)"

dist: $(LINUX_BINARY)
	tar -cvzf $(LINUX_BINARY).tar.gz $(LINUX_BINARY)

darwin: $(SOURCES)
	$(GOBUILD) -o $(DARWIN_BINARY) -ldflags "-X main.commitHash=$(GIT_COMMIT)"

darwin-dist: $(DARWIN_BINARY)
	tar -cvzf $(DARWIN_BINARY).tar.gz $(DARWIN_BINARY)

windows: $(SOURCES)
	docker run --rm -v golang-windows-cache:/go/pkg -v $(PWD):/go/src/bakinbacon -w /go/src/bakinbacon -e GOCACHE=/go/pkg/.cache x1unix/go-mingw /bin/sh -c "go build -v -o $(WINDOWS_BINARY) -ldflags '-X main.commitHash=$(GIT_COMMIT)'"

windows-dist: $(WINDOWS_BINARY)
	tar -cvzf $(WINDOWS_BASE).tar.gz $(WINDOWS_BINARY)

fmt: 
	$(GOFMT) baconclient/ nonce/ notifications/ storage/ util/ webserver/ *.go

clean:
	rm -f *.tar.gz $(LINUX_BINARY) $(DARWIN_BINARY) $(WINDOWS_BINARY)

ui-dev:
	npm --prefix webserver/ install

ui: $(ui-dev)
	npm --prefix webserver/ run build