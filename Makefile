.PHONY: build test bench cover lint vet clean example

build:
	go build ./...

test:
	go test -v -race -count=1 ./...

bench:
	go test -bench=. -benchmem -benchtime=2s -run=^$$

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

lint:
	golangci-lint run ./...

vet:
	go vet ./...

example:
	go run examples/simple/main.go

clean:
	rm -f coverage.out coverage.html
