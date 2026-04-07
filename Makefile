BINARY := taskboard
INSTALL_DIR := $(HOME)/.local/bin

.PHONY: build install test lint clean

build:
	go build -o $(BINARY) ./cmd/taskboard

install: build
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/$(BINARY)

test:
	go test ./...

lint:
	go vet ./...

clean:
	rm -f $(BINARY)
