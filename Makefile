VERSION := "v1.0"
DIST := $(CURDIR)/_dist
LDFLAGS := "-X main.version=${VERSION}"
BINARY := "helm-spray"

.PHONY: dist
dist:
	ls -lR $(CURDIR)
	mkdir -p $(DIST)
	GOOS=linux GOARCH=amd64 go get -t -v ./...
	GOOS=linux GOARCH=amd64 go build -o $(BINARY) -ldflags $(LDFLAGS) main.go
	tar -cvf $(DIST)/${BINARY}_linux_$(VERSION).tar $(BINARY) README.md LICENSE
	cd linux
	tar -uvf $(DIST)/${BINARY}_linux_$(VERSION).tar plugin.yaml
	gzip $(DIST)/${BINARY}_linux_$(VERSION).tar
	GOOS=windows GOARCH=amd64 go get -t -v ./...
	GOOS=windows GOARCH=amd64 go build -o $(BINARY).exe -ldflags $(LDFLAGS) main.go
	tar -cvf $(DIST)/${BINARY}_windows_$(VERSION).tar $(BINARY).exe README.md LICENSE
	cd windows
	tar -uvf $(DIST)/${BINARY}_linux_$(VERSION).tar plugin.yaml
	gzip $(DIST)/${BINARY}_linux_$(VERSION).tar
