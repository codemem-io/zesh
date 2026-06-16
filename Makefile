MODULE   := zesh
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -s -w -X main.version=$(VERSION)
OUT      := dist

PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64

.PHONY: all build build-cli build-mcp clean

all: build

build: build-cli build-mcp

build-cli: $(addprefix cli-,$(PLATFORMS))
build-mcp: $(addprefix mcp-,$(PLATFORMS))

cli-%:
	$(eval OS   := $(word 1,$(subst /, ,$(subst cli-,,$@))))
	$(eval ARCH := $(word 2,$(subst /, ,$(subst cli-,,$@))))
	$(eval EXT  := $(if $(filter windows,$(OS)),.exe,))
	@mkdir -p $(OUT)
	GOOS=$(OS) GOARCH=$(ARCH) CGO_ENABLED=0 \
		go build -ldflags "$(LDFLAGS)" \
		-o $(OUT)/zesh-$(OS)-$(ARCH)$(EXT) \
		./cmd/cli
	@echo "built $(OUT)/zesh-$(OS)-$(ARCH)$(EXT)"

mcp-%:
	$(eval OS   := $(word 1,$(subst /, ,$(subst mcp-,,$@))))
	$(eval ARCH := $(word 2,$(subst /, ,$(subst mcp-,,$@))))
	$(eval EXT  := $(if $(filter windows,$(OS)),.exe,))
	@mkdir -p $(OUT)
	GOOS=$(OS) GOARCH=$(ARCH) CGO_ENABLED=0 \
		go build -ldflags "$(LDFLAGS)" \
		-o $(OUT)/zesh-mcp-$(OS)-$(ARCH)$(EXT) \
		./cmd/mcp
	@echo "built $(OUT)/zesh-mcp-$(OS)-$(ARCH)$(EXT)"

clean:
	rm -rf $(OUT)
