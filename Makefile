HELM_HOME ?= $(shell helm home)
HELM_PLUGIN_DIR ?= $(HELM_HOME)/plugins/helm-spray
VERSION := $(shell sed -n -e 's/version:[ "]*\([^"]*\).*/\1/p' plugin.yaml)
DIST := $(CURDIR)/_dist
LDFLAGS := "-X main.version=${VERSION}"
BINARY := "helm-spray"

# go mod ftw
unexport GOPATH

.PHONY: dist
dist:
	mkdir -p $(DIST)
	GOOS=linux GOARCH=amd64 go get -t -v ./...
	GOOS=linux GOARCH=amd64 go build -o $(BINARY) -ldflags $(LDFLAGS) main.go
	tar -zcvf $(DIST)/${BINARY}_linux_$(VERSION).tar.gz $(BINARY) README.md LICENSE plugin.yaml
	GOOS=darwin GOARCH=amd64 go get -t -v ./...
	GOOS=darwin GOARCH=amd64 go build -o $(BINARY) -ldflags $(LDFLAGS) main.go
	tar -zcvf $(DIST)/${BINARY}_darwin_$(VERSION).tar.gz $(BINARY) README.md LICENSE plugin.yaml
	GOOS=windows GOARCH=amd64 go get -t -v ./...
	GOOS=windows GOARCH=amd64 go build -o $(BINARY).exe -ldflags $(LDFLAGS) main.go
	tar -zcvf $(DIST)/${BINARY}_windows_$(VERSION).tar.gz $(BINARY).exe README.md LICENSE plugin.yaml

.PHONY: test-all
test-all: vet lint test

.PHONY: test
test:
	go test -v -parallel=4 ./...

.PHONY: lint
lint:
	@go get -u golang.org/x/lint/golint
	go list ./... | xargs -n1 $${HOME}/go/bin/golint

.PHONY: vet
vet:
	go vet ./...