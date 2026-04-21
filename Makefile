BIN := bin/seed-cli
PKG := ./...

.PHONY: build test lint fmt tidy run clean install

build:
	go build -o $(BIN) ./cmd/seed-cli

install:
	go install ./cmd/seed-cli

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
	go run ./cmd/seed-cli $(ARGS)

clean:
	rm -rf bin coverage.out coverage.html
