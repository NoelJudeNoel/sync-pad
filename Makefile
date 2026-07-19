.PHONY: build deploy run test clean fmt vet docker

BINARY=sync-pad

# Defaults — override at call time for your environment
DEPLOY_HOST ?= your-host
DEPLOY_PATH ?= /usr/local/bin
WEB_REMOTE_PATH ?= /var/www/sync-pad

build:
	CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o $(BINARY) ./cmd/server

# Usage: make deploy DEPLOY_HOST=your-host
deploy: build
	scp $(BINARY) $(DEPLOY_HOST):$(DEPLOY_PATH)/
	scp -r web $(DEPLOY_HOST):$(WEB_REMOTE_PATH)/
	ssh $(DEPLOY_HOST) "sudo systemctl restart sync-pad"

run:
	go run ./cmd/server

test:
	go test ./...

fmt:
	gofmt -w ./...

vet:
	go vet ./...

clean:
	rm -f $(BINARY)

docker:
	docker build -t sync-pad:latest .
