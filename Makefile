BINARY   := zesh
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

.PHONY: all build clean

all: build

build: $(PLATFORMS)

$(PLATFORMS):
	$(eval OS   := $(word 1,$(subst /, ,$@)))
	$(eval ARCH := $(word 2,$(subst /, ,$@)))
	$(eval EXT  := $(if $(filter windows,$(OS)),.exe,))
	@mkdir -p $(OUT)
	GOOS=$(OS) GOARCH=$(ARCH) CGO_ENABLED=0 \
		go build -ldflags "$(LDFLAGS)" \
		-o $(OUT)/$(BINARY)-$(OS)-$(ARCH)$(EXT) \
		./cmd/zesh
	@echo "built $(OUT)/$(BINARY)-$(OS)-$(ARCH)$(EXT)"

clean:
	rm -rf $(OUT)
