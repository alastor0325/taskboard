BINARY := taskboard
INSTALL_DIR := $(HOME)/.local/bin

.PHONY: build install install-skill test lint clean

build:
	go build -o $(BINARY) ./cmd/taskboard

install-skill: build
	$(BINARY) install-skill

install: build install-skill
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/$(BINARY)
	codesign --sign - $(INSTALL_DIR)/$(BINARY) 2>/dev/null || true

test:
	go test ./...

lint:
	go vet ./...

clean:
	rm -f $(BINARY)
