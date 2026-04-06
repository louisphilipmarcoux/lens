.PHONY: build test test-race lint proto dev clean fmt vet

BINARY_DIR := bin
AGENT_BIN := $(BINARY_DIR)/lens-agent
INGEST_BIN := $(BINARY_DIR)/lens-ingest
QUERY_BIN := $(BINARY_DIR)/lens-query

build:
	go build -o $(AGENT_BIN) ./cmd/agent
	go build -o $(INGEST_BIN) ./cmd/ingest
	go build -o $(QUERY_BIN) ./cmd/query

test:
	go test ./... -count=1

test-race:
	go test ./... -race -count=1

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

proto:
	./scripts/proto-gen.sh

dev:
	docker compose -f deploy/docker-compose.yml up --build

clean:
	rm -rf $(BINARY_DIR)
