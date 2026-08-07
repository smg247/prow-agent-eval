# prow-agent-eval

Go CLI for the agentic eval lifecycle in Prow: **init → (agent) → judge → cleanup**.

It creates eval branches/PRs, collects post-agent git/GitHub state, runs deterministic builtin judges, and emits JUnit/YAML/HTML reports. Step-registry scripts can shrink to thin wrappers around this binary.

## Build

```bash
make build   # bin/prow-agent-eval
make test
make image   # docker image tagged prow-agent-eval
```

Requires Go (see `go.mod`). The container image includes `git` and `make` for collection.

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

```yaml
name: my-eval
init:
  repo: owner/repo          # default; overridable per case

dataset:
  path: cases/my-eval

collect:
  bot_replies: false
  comment_map: false
  build_result: true
  test_result: true
  expected_branch_diff: true

judges:
  - name: compiles
    type: build_passed
  - name: tests
    type: test_passed
  - name: overlap
    type: file_overlap

thresholds:
  compiles:
    min_pass_rate: 1.0
  tests:
    min_pass_rate: 1.0
  overlap:
    min_pass_rate: 1.0
```

### Case layout

```
cases/my-eval/<case>/
  input.yaml          # base_branch, head_branch, jira_key; optional repo, expected_branch
  annotations.yaml    # optional (e.g. expected_files)
  comments.json       # optional; seeded in followup mode
```

`expected_branch` is required when `collect.expected_branch_diff` is true (golden overlap vs fixture SHA).

### Builtin judges

| `type` | Passes when |
|--------|-------------|
| `branch_created` | Agent branch is set and not `main`/`master` |
| `pr_exists` | PR number > 0 |
| `build_passed` | `make build` succeeded (`build_result`) |
| `test_passed` | `make test` succeeded (`test_result`) |
| `file_overlap` | Jaccard(`changed_files`, `expected_changed_files`) ≥ 0.25 |
| `expected_files_changed` | All `annotations.expected_files` appear in `changed_files` |
| `no_secrets` | No common credential patterns in bot replies / diff |

If `type` is omitted, `name` is used as the type.

### Artifacts

Written under `--artifact-dir`:

- `junit_<eval-name>.xml`
- `eval-<case>.yaml`
- `eval-summary.yaml`
- `eval-summary.html`

Judge exits non-zero if any case errors, thresholds are unmet, or JUnit cannot be written.

## Development

```bash
make test
make lint
```

Module: `github.com/smg247/prow-agent-eval`
