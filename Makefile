VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags "-s -w -X github.com/bnema/bnetctl/cmd.version=$(VERSION) -X github.com/bnema/bnetctl/cmd.commit=$(COMMIT)"

.PHONY: build fmt vet test tidy clean

build: fmt vet
	go build $(LDFLAGS) -buildmode=pie -trimpath -o build/bnetctl .

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test ./...

tidy:
	go mod tidy

clean:
	rm -rf build/
