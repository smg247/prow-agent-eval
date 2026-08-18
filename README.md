# prow-agent-eval

Python CLI for the agentic eval lifecycle in Prow: **init → (agent) → judge → cleanup**.

It creates eval branches/PRs, collects post-agent git/GitHub state, runs harness judges, and emits JUnit/HTML reports. Step-registry scripts invoke `prow-agent-eval` directly.

## Build

```bash
make install   # editable install in .venv
make test
make image     # docker image tagged prow-agent-eval
```

Requires Python 3.11+. The container image includes `git` and `make` for collection steps.

## Commands

```bash
prow-agent-eval init  --config eval.yaml --shared-dir DIR --mode solve|followup [--case CASE] [--token TOKEN]
prow-agent-eval judge --config eval.yaml --shared-dir DIR --artifact-dir DIR [--case CASE] [--token TOKEN]
prow-agent-eval cleanup --shared-dir DIR [--case CASE] [--token TOKEN]
```

`GITHUB_TOKEN` is used when `--token` is omitted.

| Mode | Behavior |
|------|----------|
| `solve` | Create/push eval branch from fixture `head_branch`; no PR |
| `followup` | Branch + PR + optional `comments.json` seeding |

## Lifecycle and SHARED_DIR

Prow `${SHARED_DIR}` is the handoff between steps (`--shared-dir`):

**init writes**
- `eval-cases` — case name list
- `<case>.eval-head-branch`, `.eval-base-branch`, `.fixture-head-sha`, `.eval-repo`
- optional `.pr-number`, `.jira-issue-key`, `.bot-login`
- followup: `<case>.comment-map.json`

**judge reads** metadata + collected GitHub/git state, writes artifacts to `--artifact-dir`

**cleanup reads** metadata, closes PR, deletes eval branch (continues on error)

## Eval config

Uses [agent-eval-harness](https://github.com/opendatahub-io/agent-eval-harness) `EvalConfig` YAML. See harness docs for judge definitions and case layout.

Judge exits non-zero if any case errors, thresholds are unmet, or JUnit cannot be written.

## Development

```bash
make vendor-harness   # clone harness for local tests
make test
make test-integration
```

Set `AGENT_EVAL_HARNESS_ROOT` to a harness clone if not using `vendor/agent-eval-harness`.
