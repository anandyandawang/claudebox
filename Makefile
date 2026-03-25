.PHONY: build test test-unit test-integration test-all clean

build:
	go build -o claudebox ./cmd/claudebox

test: test-unit

test-unit:
	go test ./...

test-integration:
	go test -tags integration -v ./tests/integration/ -timeout 300s

test-all: test-unit test-integration

clean:
	rm -f claudebox
