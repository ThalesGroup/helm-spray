VERSION := "v1.0"
DIST := $(CURDIR)/_dist
LDFLAGS := "-X main.version=${VERSION}"
BINARY := "helm-spray"

.PHONY: dist
dist:
	mkdir -p $(DIST)
	GOOS=linux GOARCH=amd64 go get -t -v ./...
	GOOS=linux GOARCH=amd64 go build -o $(BINARY) -ldflags $(LDFLAGS) main.go
	tar -zcvf $(DIST)/${BINARY}_linux_$(VERSION).tar.gz $(BINARY) README.md LICENSE
	cd linux
	tar -zuvf $(DIST)/${BINARY}_linux_$(VERSION).tar.gz plugin.yaml
	GOOS=windows GOARCH=amd64 go get -t -v ./...
	GOOS=windows GOARCH=amd64 go build -o $(BINARY).exe -ldflags $(LDFLAGS) main.go
	tar -zcvf $(DIST)/${BINARY}_windows_$(VERSION).tar.gz $(BINARY).exe README.md LICENSE
	cd windows
	tar -zuvf $(DIST)/${BINARY}_linux_$(VERSION).tar.gz plugin.yaml
