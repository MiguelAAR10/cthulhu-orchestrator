---
name: orq-stopper
description: Use when any orq role (worker, planner, orchestrator) needs something only a human can give, such as a credential, an API key, a choice between options, a confirmation, or access to a system, or when Atena or the Owner resolves one. Covers `orq stop raise|list|resolve|cancel`, severity, and the secrets rule.
---

# Stoppers · orq

A stopper is a short, typed request to the Owner. Only the tasks you name wait on it. All other work continues. The board lists open stoppers first.

## When to raise one
- `credential` or `api_key`: you need a login or a key that you do not have.
- `access`: you need permission to a system, repo, or environment.
- `decision`: there is a product choice with options that the plan does not settle. `--options` is required.
- `confirmation`: you are about to do something that needs a yes, such as going over budget or taking a risky step.

Do not raise one for something that you or the Orchestrator can decide from the plan, the story, or memory.

## Severity
- `blocking`: the named `--tasks` cannot continue. Only those tasks are held (`orq plan listas` shows `stop ST-NNNN`).
- `soon`: needed before the slice closes. It holds nothing.
- `fyi`: the Owner should know. It holds nothing.

## Raise
```
orq stop raise --type api_key --severity blocking --slice S-04 --tasks T2 \
  --text "I need a Stripe test API key to check payments. Put it in env var STRIPE_TEST_KEY." \
  --where STRIPE_TEST_KEY
```
Then, if you are a worker, run `orq worker beat --phase blocked`, and keep working on anything that does not need the stopper. Raising the same `(slice, type, text)` again returns the same id.

## Secrets rule
- **Never put a secret value in any text.** Say where it should go, never what it is. orq rejects text that looks like a secret (exit 2): known prefixes (`sk-`, `ghp_`, `xox`, `AKIA`, `AIza`, `-----BEGIN`) or long random tokens.
- The Owner puts the secret in the place you named (an env var, a file outside the repo, or a vault). orq records only the location.

## Resolve (only Atena or the Owner)
```
orq stop resolve ST-0001 --provided-at env:STRIPE_TEST_KEY      # prints set / not set, never the value
orq stop resolve ST-0002 --choice blue                            # decision: saved to memory
orq stop cancel  ST-0003 --note "found it in the docs"            # the raiser no longer needs it
```
`--provided-at` accepts `env:NAME`, `file:PATH` (outside the repo or git-ignored), or `vault:REF`. In `free` autonomy mode, the Orchestrator or Planner may resolve `decision` and `confirmation` stoppers of severity `soon` or `fyi` with `--note "auto: <reason>"`. Everything else reaches the Owner.

`orq stop list --open` shows what is pending.
