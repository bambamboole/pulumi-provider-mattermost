PROVIDER := mattermost

.PHONY: build install test tidy

build:
	go build -o bin/pulumi-resource-$(PROVIDER) ./cmd/pulumi-resource-$(PROVIDER)

install: build
	cp bin/pulumi-resource-$(PROVIDER) $$(go env GOPATH)/bin/

test:
	go test ./...

tidy:
	go mod tidy
