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
prow-agent-eval init  --config eval.yaml --shared-dir DIR --mode solve|followup [--case CASE] [--token TOKEN] [--seed-token TOKEN]
prow-agent-eval judge --config eval.yaml --shared-dir DIR --artifact-dir DIR [--case CASE] [--token TOKEN]
prow-agent-eval cleanup --shared-dir DIR [--case CASE] [--token TOKEN]
```

`--token` / `GITHUB_TOKEN` is the GitHub App installation token (branches, PRs, judge, cleanup).
`--seed-token` / `GITHUB_SEED_TOKEN` is followup-only; see [Onboarding a workflow](#onboarding-a-workflow).

| Mode | Behavior |
|------|----------|
| `solve` | Create/push eval branch from fixture `head_branch`; no PR. Multiple cases per job. |
| `followup` | Branch + PR + optional `comments.json` seeding. **One case per job** — the review worker reads only the first entry in `eval-cases`. |

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

## Onboarding a workflow

This is what you need before a Prow job can run `prow-agent-eval` against your agent. The TRT jobs (`jira-solver-eval`, `review-responder-eval`) are a worked example using `openshift-trt/sippy-eval`.

### 1. Dedicated eval repo (the fake fork)

Do not run evals against the real product repo. Create a separate GitHub repo that holds fixture branches only (for example `your-org/sippy-eval` standing in for `openshift/sippy`).

Production agents often clone a **fork**, push there, and open a PR against **upstream**. In eval you usually collapse that: set both `FORK_REPO` and `UPSTREAM_REPO` (and `init.repo` in the eval YAML) to the eval repo. That is the fake fork — one repo playing both roles so PRs stay on the fixture repo and never touch the real project.

If you truly need a fork/upstream split, install the GitHub App on **both** repos and mint two installation tokens. If they are the same repo, mint the same installation twice (see token table below).

Fixture branches live on this repo:

| Mode | Typical branches per case |
|------|---------------------------|
| `solve` | `eval/case-N-base`, `eval/case-N-expected` (and optionally a `head_branch`) |
| `followup` | `eval/rr-case-N-base` (PR base), `eval/rr-case-N-head` (incomplete fix; init copies this to a throwaway eval branch and opens a PR) |

`followup` (review-responder) currently accepts **a single case per job**. Ship one case in the dataset, or pass `--case` / `EVAL_CASE`. Extra cases are ignored by the worker (it uses `head -1` of `eval-cases`). `solve` still fans out across all cases.

Init always pushes a unique eval branch (`eval-<case>-<timestamp>-<rand>`). Cleanup deletes that branch and closes the PR. Fixture branches are left alone.

### 2. Two GitHub identities

You need **two** identities. Using one for everything will silently skip followup work.

| Identity | What it is | GitHub login | Used for |
|----------|------------|--------------|----------|
| **GitHub App** | App installed on the eval repo (and fork, if separate) | `your-app[bot]` | Clone, push eval branches, open/close PRs, agent `gh`/`git` in the worker, judge collection, cleanup |
| **Seed user** (followup only) | Fine-grained PAT for a **user** account, not the App | e.g. `openshift-trt` | Post `comments.json` onto the eval PR during init |

The App is the agent. Its comments and pushes show up as `your-app[bot]`. That is correct for the worker.

Seeded review comments must **not** come from the App. The review-responder gate (`check_authorized.py` in address-review-pr) treats `*[bot]` as unauthorized except CodeRabbit. Org-membership fallback only works when the repo owner is a GitHub **organization**. A user-owned eval repo (common, because a personal account can own both the "org-like" name and the PAT) has no org members — authorization is OWNERS (plus CodeRabbit) only.

So for followup:

1. Put the seed user's login in the eval repo's **default-branch** `OWNERS` (`approvers:` or `reviewers:`). The gate reads OWNERS from the default branch via the API, not from the PR head.
2. Mint a **fine-grained PAT** as that user, resource owner = the eval-repo owner, repository access = **the eval repo only**. Repository permissions:

   | Permission | Access | Why |
   |------------|--------|-----|
   | Issues | Read and write | Conversation comments (`issues/.../comments`) |
   | Pull requests | Read and write | Inline review comments (`pulls/.../comments`) |
   | Metadata | Read (automatic) | Repo lookup |

   That is enough. Do not grant Contents, Workflows, or Administration.

   Classic PAT alternative: `public_repo` only. Do not use `repo` — that covers every private repository the account can see.
3. Pass it only to init as `--seed-token` / `GITHUB_SEED_TOKEN`. Do not mount it on the worker.

If `GITHUB_SEED_TOKEN` is unset, init falls back to `GITHUB_TOKEN` and seeds as the App bot. The gate then reports `WORK=no` and the agent never runs.

### 3. Tokens by mode

Prow typically runs a `github-app-auth` step first. It exchanges App JWT + installation IDs for short-lived tokens in `${SHARED_DIR}` (`gh-fork-token`, `gh-upstream-token`) and writes `gh-app-bot-login` (`<slug>[bot]`). Init reads that file so the gate can ignore the agent's own replies.

Same-repo fake fork (TRT evals):

```
GITHUB_APP_TOKEN_OUTPUTS: installation-id:gh-fork-token,installation-id:gh-upstream-token
FORK_REPO: owner/eval-repo
UPSTREAM_REPO: owner/eval-repo
```

Real fork + upstream: two installation-id files, two tokens.

| Step | `solve` | `followup` |
|------|---------|------------|
| init | App token → `GITHUB_TOKEN`. If the case has `head_branch`, push an eval branch; otherwise metadata only (the agent opens the PR). No seed token. | App token → `GITHUB_TOKEN` (branch + PR). Seed PAT → `GITHUB_SEED_TOKEN` (comments only). |
| agent / worker | App token (`GH_FORK_TOKEN` / `GITHUB_TOKEN`) as `your-app[bot]`. | Same. Never mount the seed PAT here. |
| judge + cleanup | App token. | App token. |

CLI mapping:

- `--token` / `GITHUB_TOKEN` — App installation token
- `--seed-token` / `GITHUB_SEED_TOKEN` — user PAT; falls back to `GITHUB_TOKEN` if omitted

Keep the seed PAT in its own Kubernetes secret. Do not add it to the GitHub App credential.

### 4. Checklist

1. Create the eval GitHub repo. Push fixture branches. Point `init.repo`, `FORK_REPO`, and `UPSTREAM_REPO` at it unless you need a real fork.
2. Create a GitHub App, install it on that repo (and the fork if separate). Store `app-id`, `private-key`, and `installation-id` as a Prow credential.
3. **followup only:** add the seed user to default-branch `OWNERS`. Create a fine-grained PAT with Issues + Pull requests **read and write** on the eval repo only (see above). Mount it only on the init step as `GITHUB_SEED_TOKEN`.
4. Bake eval YAML + cases into the job image (`init.repo`, judges, thresholds). For followup, one case only.
5. Wire a Prow workflow: app-auth → `prow-agent-eval init --mode …` → agent → `judge` → `cleanup`.

## Development

```bash
make vendor-harness   # clone harness for local tests
make test
make test-integration
```

Set `AGENT_EVAL_HARNESS_ROOT` to a harness clone if not using `vendor/agent-eval-harness`.
