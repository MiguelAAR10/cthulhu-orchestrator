# D2a review 3 (a8af2f4..d34e363): independent. Throwaway modules and temp ORQ_* dirs (and a temp HOME for items 2 and 3) are under scratchpad/reviewd2c/{e2e2,deny,x127b}

1. E2E with no sandbox.ro (ORQ_LIMITS points at a missing file): fixed. `orq check --caja` prints `gofmt → ~/.local/bin/gofmt … (exit 2)` for lint and `go → ~/.local/bin/go · go version go1.25.8` for types and tests, mounting `~/.local/bin, ~/.local/lib`, exit 0. Then `orq init`, a red exam (`go test ./accept/`, FAIL), a green commit in the clone, and `orq slice verificar` gave VERIFICADO with exit 0. The receipt has exam exit 0 and lint/types/tests/supresiones at exit 0 and "paso", with real output from `go test ./...` ("ok example.com/m/accept").
2. Deny list across symlink hops: holds. With a temp HOME, these all print "no corre en la caja … nunca entra" and nothing is mounted (RO nil, `sandbox/toolchain.go:146`):
   - `~/.local/bin/ca -> ~/.claude-alt/t/ca`
   - `~/.local/bin/cf -> ~/.config/t/bin/cf`
   - a relative two-hop link `~/.local/bin/ss -> ../lib/ss -> ~/.ssh/t/ss`
   - a directory hop `~/.local/bin/dl -> ~/.local/lib/dlroot/bin/dl` with `dlroot -> ~/.config/dl`
   - a `..` path `~/.local/bin/dots -> ~/ok/bin/../../.ssh/t/ss`

   A clean tool (`~/ok/bin/good`) mounts `~/.local/bin` and `~/ok`.

Defect (confirmed, the builder's open item): a slice can reach VERIFICADO with no work from the worker.
- `slice/nucleo.go:215-220`: `abrir` rejects only "already passes" and "0 failed in the report". An exam that exits 127 (tool missing) counts as a valid red, and with `--sin-conteo` there is no count to catch it.
- `Verificar` does not require head != SHARojo.
- Repro (scratchpad/reviewd2c/x127b, temp HOME):
  1. `orq slice abrir 01 --test accept/spec.txt --cmd "checktool accept/spec.txt" --sin-conteo r …` → "sh: 1: checktool: not found", "abierto en rojo".
  2. Install `~/.local/bin/checktool` (exit 0). The worker makes no commit.
  3. `orq slice verificar` → exit 0, "S-01/a1 … VERIFICADO", "alineado: 0 archivo(s)".
- Since 7379b9f, exit 127 never uses up a repair (`nucleo.go:444`). So a red that was only ever "tool missing" stays open with free retries, and it turns green as soon as the environment changes.
- Fix: in `abrir`, refuse a red whose exam exit is 126/127 (same rule as `infraestructura`, `nucleo.go:465`).

Minor (not blocking): `orq init --help` does not print help; it runs a real init in the cwd. It wrote `.orq/` and `.claude/` into the review clone `reviewd2c/repo`. The guard hook blocked my `rm -rf`, so they are still there for the Owner to delete.

VERDICT: fix
