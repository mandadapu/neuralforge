.PHONY: build test lint clean

BINARY := neuralforge
VERSION := 0.1.0-dev

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/$(BINARY) ./cmd/neuralforge

test:
	go test -race -count=1 ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/
