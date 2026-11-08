# Skills in orq: routing, vetting, never-list

## How routing works
1. `orq profile` detects the project from evidence only (no project code runs): code files, git age, commit count, authors, tests, CI, manifests (`package.json`+`tsconfig.json` ⇒ typescript, `pyproject.toml`/`requirements.txt` ⇒ python, `go.mod` ⇒ go) and a stage hint (CI plus deploy files or tags ⇒ production). Result: `project_type` (greenfield | brownfield | legacy), `stacks`, `stage`, `criticality` (default medium).
2. The Owner can force any value: `orq profile set type=brownfield criticality=high stage=production`. Forced values live in `.orq/profile.json` and always win. orq warns when the evidence disagrees.
3. `orq skills plan --phase T|C|A|D|V|DOC --role R` returns the minimal set: entries whose phase, role, project type and stack match, and whose `min_criticality` is at or below the effective stakes. Production raises stakes by one level. **Trivial ⇒ at most one skill; production-critical ⇒ every matching skill.** Requirements (e.g. `grill-with-docs` → `grilling`, `domain-modeling`) come along.
4. `orq skill ROL` and `orq contexto` show only that phase×role set. Workers use D, scouts C, planner/astra A, verifier V. Atena and the Orchestrator follow `orq fase`.
5. Matt Pocock skills are token-hungry, so they only run at the front: `grilling` in T from medium stakes, `grill-with-docs` from high.

## How to add and vet a skill
1. Add an entry to `.orq/skills.catalog.json` (`{"version":1,"skills":[…]}`). The same `id` replaces a default entry. Fields: `id`, `name`, `source`, `install` (`npx …` or `copy:NAME`, read from orq's own skills or from `.orq/NAME/`), `ref`, `sha256`, `phases`, `roles`, `project_types`, `stacks`, `min_criticality` (low|medium|high), `token_tier`, `adapt` (the harness rule that overrides the skill), `never`.
2. `orq skills vet ID [--file SKILL.md]` statically checks it. The frontmatter must have name and description, the file must be ≤20 KB, and it must not use `git push`, `--force`, `rm -rf`, `reset --hard`, `branch -D`, agent spawning or `curl | sh`. A line that forbids the action does not count. The verdict (`validated` | `rejected` + reason) goes to `.orq/skills.gallery.json`.
3. `orq skills install ID…` installs only that selection into `.claude/skills/` and `.agents/skills/`. It vets again and pins the content hash in `.orq/skills.lock.json`. Later the install is refused if the content no longer matches the lock or the catalog `sha256`. Review the change, then run `orq skills fijar`.
4. The TypeScript and Python best-practice entries have no published npx path. Vendor their SKILL.md under `.orq/vendor/…` and vet it before first use.

## Never-list (install refuses these)
| Skill | Why |
|---|---|
| swarm, arena, poteto-mode, subagent-driven-development, dispatching-parallel-agents | Agents that spawn agents bypass orq's admission (H-02) |
| ship, land-and-deploy, canary | Shipping and deploying need the Owner |
| worktree-cleanup | Uses `rm -rf`; cleanup is `orq gc`, never `--force` |
| using-superpowers | Turns every session into a coordinator and burns tokens every turn |
| using-git-worktrees, finishing-a-development-branch | orq creates, merges and closes worktrees in code |
| brainstorming, grill-me | The interview is Atena's `grilling` |
| tdd, handoff, webapp-testing | Duplicates `test-driven-development`, duplicates `orq relevo`, and relies on networkidle, in that order |
