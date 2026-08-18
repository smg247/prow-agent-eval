# Migration Analysis: prow-agent-eval into openshift/ci-tools-standalone

## Context

prow-agent-eval is a 5,074-line Go CLI (3 subcommands: init, judge, cleanup) that runs eval workflows for agentic CI jobs. It currently lives at github.com/smg247/prow-agent-eval with its own image build. The plan is to move it into openshift/ci-tools-standalone — a lighter-weight repo of independent CI tools extracted from ci-tools, each with minimal dependencies.

ci-tools-standalone was chosen over ci-tools because:
- Lighter presubmits (no ci-operator integration test suite)
- Smaller blast radius (no coupling to ci-operator or stable CI infra changes)
- Better fit for experimental/agentic tooling that may evolve quickly
- Still provides the key adoption wins (prow GitHub client, logrus, sigs.k8s.io/yaml)

## Adoption Opportunities

Moving into ci-tools-standalone is an opportunity to converge on better-established patterns. Here's what prow-agent-eval would gain:

| Area | Current (prow-agent-eval) | ci-tools-standalone pattern | Migration work |
|------|--------------------------|----------------------------|----------------|
| **GitHub client** | Raw `go-github/v67` — no retries, no throttling, no App auth | Prow's `github.Client` via `flagutil.GitHubOptions` — retries, rate limiting, token rotation, GitHub App support (already a dependency) | Rewrite `pkg/github/` (~300 lines) to use prow client. Gains: retry resilience, App auth path for future. Loses: `httptest.Server` mock pattern (prow client uses `fakegithub` instead). |
| **JUnit types** | Custom flat types in `pkg/report/junit.go` (~60 lines) | No existing JUnit package — add `internal/junit/` (~60 lines) or import ci-tools' `pkg/junit` as a module dep | Write a slim JUnit package in `internal/junit/` with nested suites, properties, and systemOut/systemErr. Gains: richer schema for Spyglass rendering. |
| **Test helpers** | `go-cmp` table-driven + `httptest` mocks | No golden-file testing utility | Optionally add a small `internal/testhelper/` with `CompareWithFixture` + `UPDATE=true` regeneration. Nice-to-have, not blocking. |
| **Logging** | `log/slog` (stdlib) | `logrus` used by several tools | Switch to logrus. Minor but pervasive (~30 call sites). |
| **Config loading** | `gopkg.in/yaml.v3` | `sigs.k8s.io/yaml` (already a dependency) | Switch YAML library. Eval config schema stays as-is. |
| **CLI pattern** | Cobra with subcommands | Mixed — cobra already in go.mod (webhooks use it) | Keep cobra. No change needed. |
| **HTML rendering** | Embedded Go template, dark theme | No equivalent | Keep custom template. Not applicable to ci-tools-standalone's existing tools. |
| **Git operations** | Exec-based `git` CLI wrapper | No equivalent | Keep as-is — exec-based git is correct for the eval workflow. |

## Benefits

1. **Prow GitHub client** — retries, rate limiting, and GitHub App auth support out of the box. The current raw go-github client has no retry logic and uses a single PAT. Already in ci-tools-standalone's go.mod.
2. **JUnit types** — a richer schema enables properties (eval metadata), systemOut (build logs inline), and nested suites (per-case grouping). Spyglass renders these better.
3. **Image pipeline** — ci-tools-standalone images are built per-tool via Makefile. No separate ci-operator config or standalone Prow job needed. The repeated-trigger issue with the standalone image postsubmit goes away.
4. **Visibility and review** — broader CI team sees the code; smg247 is already a ci-tools approver.
5. **Lighter repo** — ci-tools-standalone has ~10 tools vs ci-tools' ~69. Faster presubmits, less go.mod churn, less blast radius from unrelated changes.
6. **Established patterns** — cobra, logrus, sigs.k8s.io/yaml, and prow client are all already in the repo.

## Drawbacks

1. **No shared `pkg/`** — ci-tools-standalone uses `internal/` exclusively. Packages won't be importable by other repos. This is fine if prow-agent-eval is the only consumer, which it is.
2. **No vendoring** — ci-tools-standalone uses Go module proxy, not `vendor/`. If CI environments require vendoring, this would need to be added (but existing tools already build fine without it).
3. **GitHub client rewrite** — prow's client has a different API surface. Every GitHub call in prow-agent-eval (~15 methods) needs rewriting. The `httptest.Server` test pattern is replaced with prow's `fakegithub`.
4. **Python in container image** — the judge subprocess bridge (AGENT_EVAL_HARNESS_INTEGRATION plan) requires Python in the runtime image. ci-tools-standalone Dockerfiles are minimal `ubi-minimal` with just the binary. A custom Dockerfile with `python3`/`pip` is needed.
5. **Missing test utilities** — no golden-file testing (`CompareWithFixture`). Can be added as a small `internal/` package if desired.

## Migration Steps

### Phase 1: Code migration

1. Create `cmd/prow-agent-eval/main.go` in ci-tools-standalone (keep cobra entrypoint)
2. Move packages under `internal/prowagenteval/` (ci-tools-standalone convention is `internal/`):
   - `internal/cli/*` → `internal/prowagenteval/cli/`
   - `pkg/config` → `internal/prowagenteval/config/`
   - `pkg/github` → **rewrite** to use prow's `github.Client`
   - `pkg/git` → `internal/prowagenteval/git/` (keep exec-based)
   - `pkg/judge` → `internal/prowagenteval/judge/`
   - `pkg/report` → `internal/prowagenteval/report/`
   - `pkg/shared` → `internal/prowagenteval/shared/`
3. Create `internal/junit/` with richer JUnit types (nested suites, properties, systemOut/systemErr) — or import ci-tools' `pkg/junit` as a module dependency if preferred
4. Update all import paths
5. Switch logging from `slog` to `logrus`
6. Switch YAML from `gopkg.in/yaml.v3` to `sigs.k8s.io/yaml`
7. Remove `go-github/v67` dependency (replaced by prow client)
8. Add tool to `TOOLS` list in Makefile

### Phase 2: Image pipeline

9. Create `images/prow-agent-eval/Dockerfile` — needs `python3`, `pip`, `git`, and `make` in runtime image (for judge subprocess bridge and collect steps). This is a departure from ci-tools-standalone's standard `ubi-minimal` + single binary pattern.
10. Update agentic-dev image config to `COPY --from=` the ci-tools-standalone-built image
11. Remove standalone prow-agent-eval ci-operator config and Prow jobs

### Phase 3: Step registry

12. Verify step registry scripts work with the ci-tools-standalone-built binary (no changes needed — keeping cobra + single binary)

### Phase 4: Cleanup

13. Archive smg247/prow-agent-eval repo
14. Remove smg247/prow-agent-eval Prow job configs from release repo
15. Remove the standalone image promotion config

## Verification

- `go test ./internal/prowagenteval/...` passes in ci-tools-standalone
- `make build-prow-agent-eval` builds the binary
- Image builds successfully with python3 + git + make in runtime layer
- Step registry scripts work unchanged against the ci-tools-standalone-built binary
- Rehearsal job produces identical artifacts (JUnit, HTML, YAML) to current output

## Effort Estimate

- GitHub client rewrite to prow client: ~2 days (largest piece)
- Package moves + import path updates: ~half day
- Logging/YAML library switches: ~half day
- JUnit type creation in `internal/junit/`: ~half day
- Dockerfile with Python runtime: ~half day
- Image pipeline setup: ~half day
- **Total: ~5 days of focused work**

## Recommendation

The migration is worth doing if prow-agent-eval is going to be a long-lived tool. The prow GitHub client alone (retries, App auth, rate limiting) is a meaningful upgrade over raw go-github. ci-tools-standalone gives us the key adoption wins without the heavyweight presubmits and blast radius of ci-tools proper.

The main cost is the GitHub client rewrite (~2 days) and the custom Dockerfile for Python support. If the tool is still experimental and changing daily, wait until it stabilizes. If it's approaching steady-state, migrate now.
