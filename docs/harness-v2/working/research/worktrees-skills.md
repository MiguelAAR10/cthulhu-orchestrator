# R3 · working copies and SDLC skills (scout, read-only; 2026-11-13)
V = verified (source or my local run). I = my inference. No size numbers for big repos exist in the sources; the orq repo is 1.8 MB, too small to measure.

## 1. Isolated working copies
**git worktree / clone --shared / --reference** (https://git-scm.com/docs/git-clone)
- V: `--shared`/`--reference` write `objects/info/alternates`; deleting branches in the source can leave objects unreferenced and "normal Git operations (such as git commit)" that run `maintenance --auto` may remove them, "then the cloned repository will become corrupt". `git gc` in the clone is safe; `git repack -a` copies borrowed objects in (breaks the dependency, loses the saving). `--dissociate` copies borrowed objects after cloning.
- V: default local clone hardlinks `.git/objects`; `--no-local` copies. Local run (git 2.43, ext4): `--shared` 0.17 s, `--local` 0.14 s, `--no-local` 0.32 s on a 1.8 MB repo; `--filter=blob:none` is ignored for local paths ("filtering not recognized by server").
- I: a worker that can write the borrowed/hardlinked object files could corrupt the main repo; this is why the plan's "no shared or hard-linked objects" is right for untrusted workers. Cost is one pack copy per worker, cheap until the repo is hundreds of MB; then use a bare mirror inside the sandbox ro-bound as the only alternate and `--dissociate` on return (I, untested).
- V: partial clone (`--filter=blob:none`) needs a server that supports it (a `file://` upload-pack with `uploadpack.allowFilter` — I, untested); it defeats offline work because blobs are fetched on demand from the origin. Not worth it for a sandbox with no egress.
- worktree: shares `.git` (refs, hooks, config) with main, which is the isolation hole the plan already closes. I: keep worktree only for orq's own trusted lanes.

**Reflinks**
- V (local): `cp --reflink=always` on this root (`/dev/sdd`, ext4) fails: "failed to clone ... Operation not supported". ext4 has no reflink (https://bugzilla.kernel.org/show_bug.cgi?id=219341); btrfs, XFS, OCFS2 do (https://man.archlinux.org/man/FICLONE.2const.en). `--reflink=auto` silently copies.
- I: do not design around reflinks on WSL2 ext4. Optional later: a loopback XFS/btrfs image for the worker area, only if copies prove too slow.

**Hard links**: same filesystem only. Risk (I): an in-place write by a worker to a hardlinked file changes every copy, including main's. pnpm/uv avoid this by hardlinking only immutable store files; do not hardlink a source tree.

**overlayfs** (https://docs.kernel.org/filesystems/overlayfs.html)
- V: lower layer need not be writable; no upper/work = read-only overlay; changing the lower layer while mounted is not allowed (behaviour undefined); copy-up copies the whole file on first write (+ fsync by default), `metacopy` copies metadata only.
- V: bwrap `--overlay/--tmp-overlay/--ro-overlay/--overlay-src` arrived in 0.11.0, 2024-10-30 (https://github.com/containers/bubblewrap/releases). Host has 0.9.0 (spike). Spike: kernel overlay works under `unshare -Urm`, ro bind of `node_modules`/`.venv` PASS.
- I: overlay buys write-without-copy for a shared dep tree. Needs bwrap ≥0.11 (upgrade) and a lower layer frozen while any worker runs. The ro-bind already passes; overlay only matters when a worker must modify deps.

**Node**
- V: pnpm hardlinks files from a content-addressable store into `node_modules`; `packageImportMethod` default `auto`, Linux order hardlink → clone → copy; steps down only when a link fails (https://pnpm.io/settings/node-modules). Hardlinks need the store on the same filesystem, else copy (I from the same rule).
- V: `pnpm install --offline` uses only packages already in the store and fails if one is missing; `--frozen-lockfile` fails if the lockfile is out of sync; docs pair them for offline installs; `--frozen-store` is for a read-only store (https://pnpm.io/cli/install).
- V: `enableGlobalVirtualStore` (default false) makes `node_modules` only symlinks into one central virtual store (https://pnpm.io/settings/node-modules).
- I: for a worker, `pnpm install --offline --frozen-lockfile` in its clone costs links, not downloads, but the sandbox must see the store rw-or-ro and write `node_modules` in the clone: the "ro-bind the main `node_modules`" spike is cheaper when deps do not change; this path is the answer when they do. npm/yarn: not verified in this run (npm `--offline` uses the cacache but copies files; yarn berry PnP/zip cache avoid `node_modules`) — treat as unverified.

**Python**: V: uv `link-mode` default `clone` on macOS and Linux, `hardlink` on Windows; options clone/copy/hardlink/symlink; symlink "discouraged ... tight coupling between the cache and the target environment" (https://docs.astral.sh/uv/reference/settings/#link-mode). Cache default `~/.cache/uv`. I: on ext4 `clone` cannot work, so uv falls back (a secondary source reports a "falling back to full copy" warning); set `UV_LINK_MODE=hardlink` with the cache on the same filesystem as the clones, or ro-bind the main `.venv`.

**Go**: V: `go build` cache "is safe for concurrent invocations" (`go help cache`). V: `-mod=vendor` uses neither network nor module cache; `GOPROXY=off` forbids network; `-modcacherw` makes the module cache deletable (https://go.dev/ref/mod). I: share `GOMODCACHE` ro-bound with `GOFLAGS=-mod=mod GOPROXY=off`; the module cache is read-only by default, so ro-bind is natural. A shared `GOCACHE` must be writable (I, untested) — give each worker its own and seed by copying, or use `GOCACHEPROG` (not verified here).

**When a task changes deps**: I: the worker gets a rw private copy (`cp -a`, hardlink copy for pnpm, `uv sync` with hardlink) from the shared store, the manifest/lockfile change comes back in the bundle, and the exam re-runs `install --offline --frozen-lockfile` from the trusted store; a package missing from the store means orq (outside the sandbox) fetches it, then retries. This keeps the agent with no egress.

## 2. SDLC skills
Format V: SKILL.md with `name` + `description`; Claude and Codex keep only metadata in context until a skill triggers (progressive disclosure) (https://github.com/anthropics/skills). Codex docs moved to learn.chatgpt.com/docs/build-skills (not fetched; I assume the same format).
Evidence on cost V: ETH Zurich paper found LLM-generated AGENTS.md lowered success slightly and raised cost >20%; human-written minimal files helped slightly (https://arxiv.org/abs/2602.11988). A vendor eval (Vercel, secondary report) saw skills uninvoked in 56% of cases vs an always-loaded index (https://www.pulse.bot/ai/news/a-simple-text-file-beats-complex-skill-systems-for-ai-coding-agents-3c5bdc7e-65ad-4fb3-9d08-2a0cf7f278cf/). I: do not rely on auto-trigger; the harness should inject the one skill per role.

Collections surveyed (V, README listings): obra/superpowers 16 skills (https://github.com/obra/superpowers); mattpocock/skills (https://github.com/mattpocock/skills); garrytan/gstack (https://github.com/garrytan/gstack); anthropics/skills (documents, mcp-builder, webapp testing; almost no SDLC process).

| orq role | Worth its context | Why |
|---|---|---|
| Story interview (Atena) | grill-with-docs / grilling (Pocock), brainstorming (superpowers) | one question at a time until design branches close |
| Plan (Planner/Astra) | writing-plans; to-tickets; gstack `/spec` | executable plan with checks |
| Worker, build | test-driven-development, tdd | red-green gives the failing test orq counts |
| Worker, bug | systematic-debugging, diagnosing-bugs | root cause before fix |
| Review | Pocock `code-review` (diff vs spec); superpowers requesting/receiving-code-review | reviewer reads the spec |
| Verify | verification-before-completion (claim needs fresh output) | matches the receipt idea; orq's gate still decides |
| Docs | domain-modeling, writing-for-agents | glossary/ADRs; keep AGENTS.md minimal |

Conflicts with a harness that owns worktrees, merging and verification (V from listings; the conflict is I):
- superpowers `using-git-worktrees` (creates worktrees/branches) and `finishing-a-development-branch` (merge/PR/discard, cleans worktree): both act on git state orq owns.
- superpowers `subagent-driven-development`, `dispatching-parallel-agents`, `executing-plans`: second scheduler inside a worker.
- gstack `/ship`, `/land-and-deploy`, `/modernize`, `/captain`: push, open or merge PRs; `/verify` loops "until green" and `/qa` fixes code.
- Pocock `implement-spec` (one branch, parallel subagents), `to-spec`/`triage` (publish to an issue tracker).

## What orq should adopt (ranked)
1. Keep independent `--no-local` clones, one bundle back. Why: no corruption path from alternates or hardlinks, cheap now. Effort S (already decided). Replaces: worktrees for workers.
2. Dependency policy table per ecosystem: ro-bind main `node_modules`/`.venv`/`GOMODCACHE` by default; deps-change path = private copy + `install --offline --frozen-lockfile` / `UV_LINK_MODE=hardlink` / `GOPROXY=off`, then trusted re-install in the exam. Why: answers the Owner's worry; no egress needed. Effort M. Changes: sandbox spec + exam step.
3. Per-role skill allowlist injected by orq (Worker: tdd + systematic-debugging; Review: code-review; Verify: verification-before-completion; Atena: grilling). Why: auto-trigger is unreliable and context costs; fewer is cheaper. Effort S–M. Replaces: whatever global skills the agent finds.
4. Install skills read-only inside the sandbox, with git/merge/PR skills removed from the worker's skill dir. Why: stops skills from fighting the gate. Effort S.
5. Upgrade bwrap to ≥0.11 and trial `--ro-overlay`/`--overlay-src` only if workers must edit deps often. Effort M. Replaces: `cp -a` fallback.
6. Keep AGENTS.md short and human-written (paper above). Effort S.

## What orq should NOT adopt
- `clone --shared/--reference` for workers: borrowed objects can vanish or be written through; corrupts main.
- Hardlink copies of the source tree: one in-place write hits every copy.
- Partial/sparse clone: needs server filter support, blobs fetch lazily, breaks no-egress.
- Reflink-based copies: ext4 returns "Operation not supported" here.
- superpowers `using-git-worktrees`, `finishing-a-development-branch`, subagent/parallel/executing-plans skills: duplicate orq's scheduler and merge.
- gstack `/ship`, `/land-and-deploy`, `/captain`, `/qa` auto-fix: two owners of merge and verification.
- Installing whole collections: the paper and the Vercel report show context cost without a guaranteed gain.
