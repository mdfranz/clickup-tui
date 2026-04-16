BINARY_NAME=clickup-tui

.PHONY: all build run test clean fmt lint install

all: build

build:
	go build -o $(BINARY_NAME) main.go

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
