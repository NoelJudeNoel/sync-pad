.PHONY: build deploy run test clean fmt vet

BINARY=sync-pad
DEPLOY_HOST=abj
DEPLOY_PATH=/opt/sync-server/

build:
	CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o $(BINARY) ./cmd/server

deploy: build
	scp $(BINARY) $(DEPLOY_HOST):$(DEPLOY_PATH)
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
