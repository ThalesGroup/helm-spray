VERSION := "v1.0"
DIST := $(CURDIR)/_dist
LDFLAGS := "-X main.version=${VERSION}"
TAR_LINUX := "helm-spray_linux_${VERSION}.tgz"
TAR_WINDOWS := "helm-spray_windows_${VERSION}.tgz"
BINARY_LINUX := "helm-spray"
BINARY_WINDOWS := "helm-spray.exe"

.PHONY: dist_linux
dist_linux:
	mkdir -p $(DIST)
	GOOS=linux GOARCH=amd64 go get -t -v ./...
	GOOS=linux GOARCH=amd64 go build -o $(BINARY_LINUX) -ldflags $(LDFLAGS) main.go
	sed -e "s/:VERSION:/$VERSION/g" -e "s/:BINARY:/$BINARY_LINUX/g" plugin.yaml.template > plugin.yaml
	tar -czvf $(DIST)/${TAR_LINUX} $(BINARY_LINUX) README.md LICENSE plugin.yaml

.PHONY: dist_windows
dist_windows:
	GOOS=windows GOARCH=amd64 go get -t -v ./...
	GOOS=windows GOARCH=amd64 go build -o $(BINARY_WINDOWS) -ldflags $(LDFLAGS) main.go
	sed -e "s/:VERSION:/$VERSION/g" -e "s/:BINARY:/$BINARY_WINDOWS/g" plugin.yaml.template > plugin.yaml
	tar -czvf $(DIST)/${TAR_WINDOWS} $(BINARY_WINDOWS) README.md LICENSE plugin.yaml
