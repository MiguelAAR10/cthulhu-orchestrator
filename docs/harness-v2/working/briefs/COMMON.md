# Common rules for every agent on harness-v2

Repo: orq, a multi-agent coding harness in Go (stdlib only). Read README.md and docs/REGLAS-HARNESS.md first.
- Go standard library only. The repository is English: code comments, identifiers you add, CLI messages, usage text, docs, commit messages. Only text that a role says to the Owner stays Spanish.
- Work only in your own worktree and only in the files your brief owns. Another agent owns the rest.
- Commit small, on your branch. No push, no force, no branch deletion, no `make install`:
  ~/bin/orq is in use by live workers in another project and must stay as it is.
- Use ORQ_STATE, ORQ_LIMITS, ORQ_WORKTREES pointing at temp dirs for anything you run; never the real ~/.config/orq or ~/.local/state/orq.
- Done = `cd harness && gofmt -l . ` prints nothing, `go vet ./...` clean, `go test ./... -count=1` green, work committed.
- Final reply, 15 lines or fewer: DONE | BLOCKED | QUESTION · head sha · files changed · what you ran and its result · what you did not do · risks.
- If the brief is wrong or needs a file you do not own, stop and reply QUESTION.
