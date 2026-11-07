GO      ?= go
BIN     ?= $(HOME)/bin
CFG     ?= $(HOME)/.config/orq
STATE   ?= $(HOME)/.local/state/orq

.PHONY: build test install limits status

build:
	cd harness && $(GO) build -trimpath -o ../dist/orq ./cmd/orq

test:
	cd harness && $(GO) vet ./... && $(GO) test ./...

install: build
	mkdir -p $(BIN) $(STATE)
	cp dist/orq $(BIN)/.orq.new && mv -f $(BIN)/.orq.new $(BIN)/orq
	@test -f $(CFG)/limits.json || (mkdir -p $(CFG) && cp templates/limits.json $(CFG)/limits.json && echo "límites por defecto en $(CFG)/limits.json")
	@echo "instalado: $(BIN)/orq $$($(BIN)/orq version)"

status:
	$(BIN)/orq status
