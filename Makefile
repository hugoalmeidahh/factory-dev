APP     = factorydev
VERSION ?= dev
LDFLAGS = -ldflags "-s -w -X main.Version=$(VERSION)"
TAGS    = systray,webview

.PHONY: dev build build-headless release clean lint verify-release

dev:
	go run -tags $(TAGS) ./cmd/factorydev/...

build:
	mkdir -p bin
	go build -tags $(TAGS) $(LDFLAGS) -o bin/$(APP) ./cmd/factorydev/...

build-headless:
	mkdir -p bin
	go build -tags systray $(LDFLAGS) -o bin/$(APP) ./cmd/factorydev/...

release:
	mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build -tags release $(LDFLAGS) \
		-o dist/$(APP)-linux-amd64 ./cmd/factorydev/...
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
		go build -tags release $(LDFLAGS) \
		-o dist/$(APP)-linux-arm64 ./cmd/factorydev/...

verify-release: release
	./dist/$(APP)-linux-amd64 version

clean:
	rm -rf bin/ dist/

lint:
	@which golangci-lint > /dev/null && golangci-lint run ./... || go vet ./...
