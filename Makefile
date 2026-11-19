GO      ?= go
BIN     ?= $(HOME)/bin
CFG     ?= $(HOME)/.config/orq
STATE   ?= $(HOME)/.local/state/orq

.PHONY: build test ci install limits status

build:
	cd harness && $(GO) build -trimpath -o ../dist/orq ./cmd/orq

test:
	cd harness && $(GO) vet ./... && $(GO) test ./...

# ci runs every suite. Every step gets its own ORQ_STATE, ORQ_LIMITS, ORQ_WORKTREES and ORQ_CONFIG
# under a temp dir and its own orq binary there: nothing reads or writes the installed orq or its state.
# The candidate is built once, in a local clone carrying this tree's files: Go records the VCS
# revision only when .git is a directory, and in a linked worktree it is a file.
ci:
	@set -e; tmp=$$(mktemp -d "$${TMPDIR:-/tmp}/orq-ci.XXXXXX"); trap 'rm -rf "$$tmp"' EXIT; \
	mkdir -p "$$tmp/state" "$$tmp/wt" "$$tmp/bin"; \
	export ORQ_STATE="$$tmp/state" ORQ_LIMITS="$$tmp/limits.json" ORQ_WORKTREES="$$tmp/wt" ORQ_CONFIG="$$tmp/config.json" ORQ_SIN_DOC=1; \
	echo "== candidate"; git clone -q --shared . "$$tmp/src"; \
	git ls-files -co --exclude-standard -z | tar -c --null -T - -f - | tar -x -C "$$tmp/src" -f -; \
	git ls-files -d -z | (cd "$$tmp/src" && xargs -0 -r rm -f --); \
	(cd "$$tmp/src/harness" && $(GO) build -o "$$tmp/bin/orq" ./cmd/orq); \
	export ORQ_CANDIDATO="$$tmp/bin/orq"; echo "candidate: $$ORQ_CANDIDATO · $$("$$ORQ_CANDIDATO" version)"; \
	echo "== gofmt"; fmt=$$(gofmt -l harness); if [ -n "$$fmt" ]; then echo "$$fmt"; echo "gofmt: the files above need formatting"; exit 1; fi; \
	echo "== go vet"; (cd harness && $(GO) vet ./...); \
	echo "== go test -race"; (cd harness && $(GO) test -race ./...); \
	echo "== e2e-v05"; env -u ORQ_SIN_DOC PATH="$$tmp/bin:$$PATH" ORQ_STATE= ORQ_LIMITS= bash validacion/e2e-v05.sh "$$tmp/e2e"; \
	echo "== aislamiento"; TMPDIR="$$tmp" ORQ="$$tmp/bin/orq" bash validacion/aislamiento.sh

install: build
	mkdir -p $(BIN) $(STATE)
	cp dist/orq $(BIN)/.orq.new && mv -f $(BIN)/.orq.new $(BIN)/orq
	@test -f $(CFG)/limits.json || (mkdir -p $(CFG) && cp templates/limits.json $(CFG)/limits.json && echo "límites por defecto en $(CFG)/limits.json")
	@echo "instalado: $(BIN)/orq $$($(BIN)/orq version)"

status:
	$(BIN)/orq status
