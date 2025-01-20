.PHONY: build-lib

BUILD_PATH=$(realpath .)/build

ifeq ($(OS),Windows_NT)     # is Windows_NT on XP, 2000, 7, Vista, 10...
 detected_OS := Windows
else
 detected_OS := $(strip $(shell uname))
endif

ifeq ($(detected_OS),Darwin)
 LIB_EXT := dylib
 CGOFLAGS := CGO_ENABLED=1 GOOS=darwin
else ifeq ($(detected_OS),Windows)
 LIB_EXT:= dll
 LIBKEYCARD_EXT := dll
else
 LIB_EXT := so
endif

build-lib:
	mkdir -p $(BUILD_PATH)/libkeycard
	@echo "Building static library..."
	cd shared && \
		$(CGOFLAGS) go build -buildmode=c-shared -o $(BUILD_PATH)/libkeycard/libkeycard.$(LIB_EXT) .
	@echo "Static library built:"
	@ls -la $(BUILD_PATH)/libkeycard/*

build-mocked-lib:
	mkdir -p $(BUILD_PATH)/libkeycard
	@echo "Building mocked static library..."
	cd shared-mocked && \
		$(CGOFLAGS) go build -buildmode=c-shared -o $(BUILD_PATH)/libkeycard/libkeycard.$(LIB_EXT) .
	@echo "Static mocked library built:"
	@ls -la $(BUILD_PATH)/libkeycard/*

build-example-shared: build-lib
	mkdir -p $(BUILD_PATH)
	@echo "Building example-c..."
	cd examples/example-shared && \
		go build -o $(BUILD_PATH)/example-shared

run-example-shared: build-example-shared
		LD_LIBRARY_PATH=$(BUILD_PATH)/libkeycard $(BUILD_PATH)/example-shared

build-example-go:
	mkdir -p $(BUILD_PATH)
	@echo "Building example-c..."
	cd examples/example-go && \
		go build -o $(BUILD_PATH)/example-go

run-example-go: build-example-go
		$(BUILD_PATH)/example-go

lint:
	@golangci-lint run