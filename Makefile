BINARY_NAME=clickup-tui

.PHONY: all build run test clean fmt lint

all: build

build:
	go build -o $(BINARY_NAME) main.go

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
