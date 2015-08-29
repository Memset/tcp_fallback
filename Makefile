all: build

.PHONY: clean
clean:
	rm tcp-fallback-darwin-amd64 tcp-fallback-linux-amd64

.PHONY: test
test:
	go test

.PHONY: build
build:
	GOARCH=amd64 GOOS=darwin go build -o tcp-fallback-darwin-amd64 tcp_fallback.go
	GOARCH=amd64 GOOS=linux go build -o tcp-fallback-linux-amd64 tcp_fallback.go

.PHONY: run
run:
	go run tcp_fallback.go
