PLUGIN_ID := codex-force-websearch
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
EXT := so

ifeq ($(GOOS),darwin)
  EXT := dylib
endif
ifeq ($(GOOS),windows)
  EXT := dll
endif

build:
	cd go && go build -buildmode=c-shared -o ../$(PLUGIN_ID).$(EXT) .

install: build
	mkdir -p plugins/$(GOOS)/$(GOARCH)
	cp $(PLUGIN_ID).$(EXT) plugins/$(GOOS)/$(GOARCH)/
