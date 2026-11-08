# Tonight's goal (2026-11-08)

1. **Slice 1, part A — H-01 closed with TDD.**
   - Red: a Go test that reproduces the false VERIFIED (a worker changes the acceptance command from its worktree; gate allows it; verify passes).
   - Green: `slice examen` and `slice alcance` become orchestrator-only in the gate **and** in the CLI handler; `--aprobado` must reference an existing recorded decision (Atena/Owner memory or request id), not free text; changing the exam invalidates previous receipts.
   - Full suite green: `cd harness && go vet ./... && go test ./...` and `bash validacion/e2e-v05.sh`.
   - One local commit on `harness/v2`. No push.
2. **Research brief** in `docs/v2/research/` on how Karpathy, Matt Pocock, pstack (Lauren Tan), Superpowers and Addy Osmani structure agent workflows, and what to adopt for phases 2 of the roadmap.

Morning check for the Owner: `git -C ~/projects/cthulhu-harness log --oneline -5`, read `docs/v2/research/RESEARCH.md` and the H-01 receipt in the commit message.
