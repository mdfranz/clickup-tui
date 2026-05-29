BINARY_NAME=clickup-tui
VERSION ?= 0.1.0
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS=-ldflags "-X clickup-tui/cmd.Version=$(VERSION) -X clickup-tui/cmd.Commit=$(COMMIT) -X clickup-tui/cmd.Date=$(DATE)"

.PHONY: all build run test clean fmt lint install

all: build

build:
	go build $(LDFLAGS) -o $(BINARY_NAME) main.go

install: build
	mkdir -p ~/bin
	install -m 755 $(BINARY_NAME) ~/bin/$(BINARY_NAME)

run: build
	./$(BINARY_NAME)

test:
	go test ./...

clean:
	rm -f $(BINARY_NAME)

fmt:
	go fmt ./...

lint:
	go vet ./...
