HELM_HOME ?= $(shell helm home)
HELM_PLUGIN_DIR ?= $(HELM_HOME)/plugins/helm-spray
VERSION := $(shell sed -n -e 's/version:[ "]*\([^"]*\).*/\1/p' plugin.yaml)
DIST := $(CURDIR)/_dist
TMP := $(CURDIR)/_tmp
LDFLAGS := "-X main.version=${VERSION}"
BINARY := "helm-spray"

.PHONY: dist
dist_linux:
	mkdir -p $(DIST)
	mkdir -p $(TMP)/helm-spray
	GOOS=linux GOARCH=amd64 go get -t -v "./..."
	GOOS=linux GOARCH=amd64 go build -o ${TMP}/helm-spray/$(BINARY) -ldflags $(LDFLAGS) main.go
	cp README.md LICENSE plugin.yaml ${TMP}/helm-spray
	tar -zcvf $(DIST)/${BINARY}_linux_$(VERSION).tar.gz ${TMP}
dist_win:
	GOOS=windows GOARCH=amd64 go get -t -v "./..."
	GOOS=windows GOARCH=amd64 go build -o $(BINARY).exe -ldflags $(LDFLAGS) main.go
	tar -zcvf $(DIST)/${BINARY}_windows_$(VERSION).tar.gz $(BINARY).exe README.md LICENSE plugin.yaml
