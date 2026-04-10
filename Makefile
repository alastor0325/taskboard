BINARY := taskboard
INSTALL_DIR := $(HOME)/.local/bin
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -X github.com/alastor0325/taskboard/internal/cmd.Version=$(VERSION)

.PHONY: build install install-skill test lint clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/taskboard

install-skill: build
	./$(BINARY) install-skill

install: build
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/$(BINARY)
	codesign --sign - $(INSTALL_DIR)/$(BINARY) 2>/dev/null || true
	./$(BINARY) install-skill

test:
	go test ./...

lint:
	go vet ./...

clean:
	rm -f $(BINARY)
