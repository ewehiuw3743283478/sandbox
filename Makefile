PLUGIN_ID := codex-grok-force-search

GOOS   ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

ifeq ($(GOOS),darwin)
  EXT := dylib
else ifeq ($(GOOS),windows)
  EXT := dll
else
  EXT := so
endif

.PHONY: build install clean tidy

build:
	@echo "Building $(PLUGIN_ID) for $(GOOS)/$(GOARCH)..."
	cd go && \
		go mod tidy && \
		CGO_ENABLED=1 GOOS=$(GOOS) GOARCH=$(GOARCH) \
		go build -trimpath -buildmode=c-shared \
			-ldflags "-s -w" \
			-o ../$(PLUGIN_ID).$(EXT) .
	@echo "Build complete: $(PLUGIN_ID).$(EXT)"

install: build
	mkdir -p plugins/$(GOOS)/$(GOARCH)
	cp $(PLUGIN_ID).$(EXT) plugins/$(GOOS)/$(GOARCH)/
	@echo "Installed to plugins/$(GOOS)/$(GOARCH)/$(PLUGIN_ID).$(EXT)"

clean:
	rm -f $(PLUGIN_ID).so $(PLUGIN_ID).dylib $(PLUGIN_ID).dll
	rm -f go/$(PLUGIN_ID).h
	@echo "Cleaned build artifacts."

tidy:
	cd go && go mod tidy
	@echo "go mod tidy completed."
