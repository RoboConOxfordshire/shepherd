.PHONY: build run fmt lint test clean

BINARY=shepherd
GOOS?=linux
GOARCH?=arm64

build:
	go build -o $(BINARY) ./cmd/shepherd

run: build
	./$(BINARY)

fmt:
	go fmt ./...

lint:
	golangci-lint run ./...

test:
	go test ./... -v

clean:
	rm -f $(BINARY)
	rm -rf usercode
	rm -f shepherd/shepherd/static/image.jpg

cross: $(BINARY)

$(BINARY):
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $(BINARY) ./cmd/shepherd

.DEFAULT_GOAL := build
