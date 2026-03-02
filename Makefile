.PHONY: build test lint fmt coverage install clean release

build:
	go build -o llmdoc .

test:
	go test ./...

lint:
	go vet ./...

fmt:
	gofmt -w .

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report written to coverage.html"

install:
	go install .

clean:
	rm -f llmdoc coverage.out coverage.html

release:
	goreleaser release --clean
