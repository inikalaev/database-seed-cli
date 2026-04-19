BIN := bin/seed-cli
PKG := ./...

.PHONY: build test lint fmt tidy run clean

build:
	go build -o $(BIN) ./cmd/seed

test:
	go test $(PKG)

lint:
	golangci-lint run

fmt:
	gofmt -w .
	@command -v goimports >/dev/null 2>&1 && goimports -w . || true

tidy:
	go mod tidy

run:
	go run ./cmd/seed $(ARGS)

clean:
	rm -rf bin coverage.out coverage.html
