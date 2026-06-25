VERSION := $(shell sed -n -e 's/version:[ "]*\([^"]*\).*/\1/p' plugin.yaml)
DIST := $(CURDIR)/_dist
LDFLAGS := -s -w -X github.com/gemalto/helm-spray/v4/cmd.version=$(VERSION)
DIST_TARGETS := dist_linux_amd64 dist_linux_arm64 dist_darwin_amd64 dist_darwin_arm64 dist_windows_amd64

BINARY_linux := helm-spray
BINARY_darwin := helm-spray
BINARY_windows := helm-spray.exe

.PHONY: build clean dist helm4-integration helm4-smoke test $(DIST_TARGETS)

build:
	CGO_ENABLED=0 go build -trimpath -o bin/helm-spray -ldflags "$(LDFLAGS)" main.go

test:
	go test ./...

dist: clean test $(DIST_TARGETS)

helm4-smoke:
	scripts/helm4_smoke_test.sh

helm4-integration:
	scripts/helm4_integration_tests.sh

clean:
	rm -rf bin $(DIST)

define build_archive
dist_$(1)_$(2):
	mkdir -p $(DIST)/$(1)-$(2)/bin
	CGO_ENABLED=0 GOOS=$(1) GOARCH=$(2) go build -trimpath -o $(DIST)/$(1)-$(2)/bin/$$(BINARY_$(1)) -ldflags "$(LDFLAGS)" main.go
	cp README.md LICENSE plugin.yaml $(DIST)/$(1)-$(2)/
	tar -C $(DIST)/$(1)-$(2) -czvf $(DIST)/helm-spray-$(1)-$(2).tar.gz bin README.md LICENSE plugin.yaml
endef

$(eval $(call build_archive,linux,amd64))
$(eval $(call build_archive,linux,arm64))
$(eval $(call build_archive,darwin,amd64))
$(eval $(call build_archive,darwin,arm64))
$(eval $(call build_archive,windows,amd64))
