BINARY=stackloom
VERSION=1.0.0

.PHONY: all build test clean web-build run lint-cli

all: build

web-build:
	cd web && npm install && npm run build

build: web-build
	go build -ldflags="-s -w" -o $(BINARY) ./cmd/stackloom

test:
	go test -v ./...

clean:
	rm -f $(BINARY) $(BINARY).exe
	rm -rf web/dist web/node_modules

run: build
	./$(BINARY) serve -p 9095

lint-cli:
	./$(BINARY) lint -f docker-compose.yml
