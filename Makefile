GO      ?= go
BIN     ?= $(HOME)/bin
CFG     ?= $(HOME)/.config/orq
STATE   ?= $(HOME)/.local/state/orq
# staticcheck is a dev tool in ~/go/bin, pinned; orq itself stays standard library only.
STATICCHECK_VERSION := 2025.1.1
STATICCHECK ?= $(HOME)/go/bin/staticcheck

.PHONY: build build2 test rapido lint mutation ci install install2 limits status

build:
	cd harness && $(GO) build -trimpath -o ../dist/orq ./cmd/orq

test:
	cd harness && $(GO) vet ./... && $(GO) test ./...

# rapido is the inner loop: gofmt, go vet and go test (no -race) only on the Go packages changed
# against RAPIDO_BASE (committed, staged, unstaged or untracked), or ./... when none is detected.
# Own temp ORQ_STATE, ORQ_LIMITS, ORQ_WORKTREES and ORQ_CONFIG, like ci. Not a substitute for ci.
RAPIDO_BASE ?= harness-v2
rapido:
	@set -e; tmp=$$(mktemp -d "$${TMPDIR:-/tmp}/orq-rapido.XXXXXX"); trap 'rm -rf "$$tmp"' EXIT; \
	mkdir -p "$$tmp/state" "$$tmp/wt"; \
	export ORQ_STATE="$$tmp/state" ORQ_LIMITS="$$tmp/limits.json" ORQ_WORKTREES="$$tmp/wt" ORQ_CONFIG="$$tmp/config.json" ORQ_SIN_DOC=1; \
	base=$$(git merge-base $(RAPIDO_BASE) HEAD 2>/dev/null || true); pkgs=""; \
	if [ -n "$$base" ]; then \
		for d in $$( { git diff --name-only "$$base" -- harness; git ls-files -o --exclude-standard -- harness; } \
			| grep '\.go$$' | xargs -r -n1 dirname | sort -u); do \
			p=./$${d#harness}; p=$${p%/}; p=$${p:-.}; p=$$(echo "$$p" | sed 's#^\./*/#./#'); \
			if ls "$$d"/*.go >/dev/null 2>&1; then pkgs="$$pkgs $$p"; fi; \
		done; \
	fi; \
	[ -n "$$pkgs" ] || pkgs="./..."; echo "== packages:$$pkgs"; \
	echo "== gofmt"; fmt=$$(gofmt -l harness); if [ -n "$$fmt" ]; then echo "$$fmt"; echo "gofmt: the files above need formatting"; exit 1; fi; \
	echo "== go vet"; (cd harness && $(GO) vet $$pkgs); \
	echo "== go test"; (cd harness && $(GO) test $$pkgs)

# lint looks for ignored values and dead branches in the packages that decide what a worker may do.
lint:
	@$(STATICCHECK) -version 2>/dev/null | grep -q '^staticcheck $(STATICCHECK_VERSION) ' || \
	{ echo "lint needs staticcheck $(STATICCHECK_VERSION). install: go install honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION)"; exit 1; }
	cd harness && $(STATICCHECK) ./internal/gate/... ./internal/slice/...

# mutation reports, without failing on survivors, which mutants of the four critical decisions the
# tests miss: who is a worker, the worker git allowlist, the exam fingerprint, the bundle descent.
# Not part of ci; 10 minutes at most.
MUTAR = internal/gate/gate.go:IsWorker internal/gate/repo.go:gitAjeno \
	internal/slice/slice.go:huellaDe internal/slice/nucleo.go:examen internal/slice/nucleo.go:Verificar \
	internal/slice/clones.go:ingerir internal/slice/caja.go:esAncestro
mutation:
	@set -e; tmp=$$(mktemp -d "$${TMPDIR:-/tmp}/orq-mut.XXXXXX"); trap 'rm -rf "$$tmp"' EXIT; mkdir -p "$$tmp/state" "$$tmp/wt"; \
	export ORQ_STATE="$$tmp/state" ORQ_LIMITS="$$tmp/limits.json" ORQ_WORKTREES="$$tmp/wt" ORQ_CONFIG="$$tmp/config.json" ORQ_SIN_DOC=1; \
	cd harness && $(GO) run ./tools/mutar -t 10m -j 6 $(MUTAR)

# ci runs every suite. Every step gets its own ORQ_STATE, ORQ_LIMITS, ORQ_WORKTREES and ORQ_CONFIG
# under a temp dir and its own orq binary there: nothing reads or writes the installed orq or its state.
# The candidate is built once, in a local clone carrying this tree's files: Go records the VCS
# revision only when .git is a directory, and in a linked worktree it is a file.
ci: lint
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

# install2 puts v2 side by side as $(BIN)/orq2, the only file it writes: never $(BIN)/orq, never
# ~/.config/orq or ~/.local/state/orq. Named orq2, the binary keeps its own ~/.config/orq2 and
# ~/.local/state/orq2 (created on first use) and hooks itself by absolute path.
build2:
	cd harness && $(GO) build -trimpath -o ../dist/orq2 ./cmd/orq

install2: build2
	mkdir -p $(BIN)
	cp dist/orq2 $(BIN)/.orq2.new && mv -f $(BIN)/.orq2.new $(BIN)/orq2
	@echo "instalado: $(BIN)/orq2 $$($(BIN)/orq2 version)"

status:
	$(BIN)/orq status
