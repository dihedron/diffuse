NAME := diffuse
DESCRIPTION := Replicated FUSE mount
COPYRIGHT := 2024 © Andrea Funtò
LICENSE := MIT
LICENSE_URL := https://opensource.org/license/mit/
VERSION_MAJOR := 0
VERSION_MINOR := 0
VERSION_PATCH := 1
VERSION=$(VERSION_MAJOR).$(VERSION_MINOR).$(VERSION_PATCH)
MAINTAINER=dihedron.dev@gmail.com
VENDOR=dihedron.dev@gmail.com
LICENSE="MIT"
RELEASE=1
PRODUCER_URL=https://github.com/dihedron/
DOWNLOAD_URL=$(PRODUCER_URL)diffuse

SHELL := /bin/bash

platforms="$$(go tool dist list)"
module := $$(grep "module .*" go.mod | sed 's/module //gi')
package := $(module)/version
now := $$(date --rfc-3339=seconds)
# comment this to disable compression; to improve compression
# consider replacing upx -9 with upx --brute (slow!)
strip := -w -s

#
# Linux x86-64 build settings
#
linux/amd64: GOAMD64 = v3

#
# Windows x86-64 build settings
#
windows/amd64: GOAMD64 = v3


.PHONY: default
default: linux/amd64 ;

%:
	@go mod tidy
ifeq (, $(shell which govulncheck))
	@go install golang.org/x/vuln/cmd/govulncheck@latest
endif	
	@govulncheck ./...
	@go generate ./...    
	@for platform in "$(platforms)"; do \
		if test "$(@)" = "$$platform"; then \
			echo "Building target $(@)..."; \
			mkdir -p dist/$(@); \
			GOOS=$(shell echo $(@) | cut -d "/" -f 1) \
			GOARCH=$(shell echo $(@) | cut -d "/" -f 2) \
			GOAMD64=$(GOAMD64) \
			CGO_ENABLED=0 \
			go build -v \
			-ldflags="\
			$(strip) \
			-X '$(package).Name=$(NAME)' \
			-X '$(package).Description=$(DESCRIPTION)' \
			-X '$(package).Copyright=$(COPYRIGHT)' \
			-X '$(package).License=$(LICENSE)' \
			-X '$(package).LicenseURL=$(LICENSE_URL)' \
			-X '$(package).BuildTime=$(now)' \
			-X '$(package).VersionMajor=$(VERSION_MAJOR)' \
			-X '$(package).VersionMinor=$(VERSION_MINOR)' \
			-X '$(package).VersionPatch=$(VERSION_PATCH)'" \
			-o dist/$(@)/ .;\
			echo ...done!; \
		fi; \
	done
	

.PHONY: optimise
optimise:
ifeq (, $(shell which upx))
	@sudo apt install upx
endif	
	@for binary in `find dist/ -name 'diffuse'`; do \
		upx -9 $$binary; \
	done;

.PHONY: clean
clean:
	@rm -rf dist
	@rm -rf fetch/server.key fetch/server.crt

.PHONY: install
install:
ifneq ($(shell id -u), 0)
	@echo "You must be root to perform this action."
else
ifeq ($(PREFIX),)
	$(eval PREFIX="/usr/local/bin")
endif
	@echo "Installing to $(PREFIX)/diffuse..."
	@cp dist/linux/amd64/diffuse $(PREFIX)
	@chmod 755 $(PREFIX)/diffuse
endif

.PHONY: uninstall
uninstall:
ifneq ($(shell id -u), 0)
	@echo "You must be root to perform this action."
else
ifeq ($(PREFIX),)
	$(eval PREFIX="/usr/local/bin")
endif
	@echo "uninstalling $(PREFIX)/diffuse..."
	@rm -rf $(PREFIX)/diffuse
endif


.PHONY: deb
deb: default
	@fpm -s dir -t deb \
	--prefix /usr/local/bin \
	--name $(NAME) \
	--version $(VERSION) \
	--iteration $(RELEASE) \
	--description "Replicated FUSE mount" \
	--vendor $(VENDOR) \
	--maintainer $(MAINTAINER) \
	--license $(LICENSE) \
	--url $(PRODUCER_URL) \
	--deb-compression bzip2 \
	dist/linux/amd64/diffuse=diffuse

.phony: rpm
rpm: default
	@fpm -s dir -t rpm \
	--prefix /usr/local \
	--name $(NAME) \
	--version $(VERSION) \
	--iteration $(RELEASE) \
	--description "Replicated FUSE mount" \
	--vendor $(VENDOR) \
	--maintainer $(MAINTAINER) \
	--license $(LICENSE) \
	--url $(PRODUCER_URL) \
	dist/linux/amd64/diffuse=diffuse


.PHONY: self-signed-cert
self-signed-cert:
	openssl req -x509 -newkey rsa:4096 -keyout fetch/server.key -out fetch/server.crt -sha256 -days 3650 -nodes -subj "/C=XX/ST=StateName/L=CityName/O=CompanyName/OU=CompanySectionName/CN=CommonNameOrHostname"
