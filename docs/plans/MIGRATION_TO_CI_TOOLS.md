# Migration Analysis: prow-agent-eval into openshift/ci-tools

## Context

prow-agent-eval is a 5,074-line Go CLI (3 subcommands: init, judge, cleanup) that runs eval workflows for agentic CI jobs. It currently lives at github.com/smg247/prow-agent-eval with its own image build. The question is whether to move it into openshift/ci-tools (69 binaries, 339 go.mod deps) and adopt ci-tools patterns.

## Adoption Opportunities

Moving into ci-tools is an opportunity to converge on better-established patterns. Here's what prow-agent-eval would gain by adopting ci-tools conventions:

| Area | Current (prow-agent-eval) | ci-tools pattern | Migration work |
|------|--------------------------|-----------------|----------------|
| **GitHub client** | Raw `go-github/v67` — no retries, no throttling, no App auth | Prow's `github.Client` via `flagutil.GitHubOptions` — retries, rate limiting, token rotation, GitHub App support | Rewrite `pkg/github/` (~300 lines) to use prow client. Gains: retry resilience, App auth path for future. Loses: `httptest.Server` mock pattern (prow client uses `fakegithub` instead). |
| **JUnit types** | Custom flat types in `pkg/report/junit.go` (~60 lines) | `pkg/junit/types.go` — richer schema with nested suites, properties, systemOut/systemErr, skip messages | Replace custom types with ci-tools' `junit` package. Gains: properties for metadata, systemOut for build logs, nested suites for case grouping. Small refactor of `WriteJUnit`. |
| **Test helpers** | `go-cmp` table-driven + `httptest` mocks | `pkg/testhelper/` — fixture/golden-file testing with `CompareWithFixture`, `UPDATE=true` regeneration. Also uses `go-cmp` internally. | Adopt golden-file pattern for report output tests. Gains: `UPDATE=true` to regenerate fixtures when output format changes. Table-driven pattern can coexist. |
| **Logging** | `log/slog` (stdlib) | `logrus` everywhere | Switch to logrus. Minor but pervasive (~30 call sites). |
| **Config loading** | `gopkg.in/yaml.v3` | `sigs.k8s.io/yaml` (JSON-compatible YAML) + `ghodss/yaml` | Switch YAML library. Eval config schema stays as-is. |
| **CLI pattern** | Cobra with subcommands | `flag.FlagSet` + `gatherOptions()` + separate binaries | **Decision point** — see below |
| **HTML rendering** | Embedded Go template, dark theme | `pkg/html/html.go` — Bootstrap + template serving | Not directly usable (eval report is a static HTML artifact, not a web page). Keep custom template. |
| **Git operations** | Exec-based `git` CLI wrapper | No equivalent (uses BuildAPI) | Keep as-is — exec-based git is correct for the eval workflow. |

## CLI Pattern Decision

ci-tools uses `flag.FlagSet` with separate binaries per command. prow-agent-eval uses cobra with 3 subcommands. Two options:

**Option A: Keep cobra, single binary.** Precedent exists — cobra is already in ci-tools' go.mod. The step registry scripts stay unchanged. Reviewers may question the inconsistency but it's pragmatic.

**Option B: Split into 3 binaries** (`prow-agent-eval-init`, `prow-agent-eval-judge`, `prow-agent-eval-cleanup`). Matches ci-tools convention. Requires updating step registry scripts and the agentic-dev Dockerfile. More images to build/promote.

**Recommendation: Option A.** The subcommands share flags and dependency wiring (`internal/cli/flags.go`, `internal/cli/deps.go`). Splitting them would duplicate that code across 3 `main.go` files. The single binary also simplifies the agentic-dev image (one `COPY --from=`).

## Benefits

1. **Prow GitHub client** — retries, rate limiting, and GitHub App auth support out of the box. The current raw go-github client has no retry logic and uses a single PAT.
2. **JUnit types** — richer schema enables properties (eval metadata), systemOut (build logs inline), and nested suites (per-case grouping). Spyglass renders these better.
3. **Golden-file testing** — `UPDATE=true` regeneration when report formats change, vs manually updating expected strings.
4. **Image pipeline** — ci-tools images are built, tested, and promoted automatically. No separate ci-operator config or standalone Prow job needed. The repeated-trigger issue with the standalone image postsubmit goes away.
5. **Visibility and review** — broader CI team sees the code; smg247 is already a ci-tools approver.
6. **Single tooling home** — "where does CI tooling live?" has one answer.

## Drawbacks

1. **Iteration speed** — ci-tools presubmits are heavier than prow-agent-eval's 5-second test suite. PRs need ci-tools reviewer approval. go.mod merge conflicts are common in an active repo.
2. **Blast radius** — ci-tools-wide Go version bumps, dep updates, and linter changes touch eval code.
3. **GitHub client rewrite** — prow's client has a different API surface. Every GitHub call in prow-agent-eval (~15 methods) needs rewriting. The `httptest.Server` test pattern is replaced with prow's `fakegithub`.
4. **Dependency weight** — prow-agent-eval currently has 16 lines of go.mod. It would inherit ci-tools' 339-line go.mod including k8s, cloud SDKs, etc. Build time goes up.
5. **Domain coupling** — experimental agentic eval tooling lives alongside stable CI infrastructure. Future experimental deps (LLM-as-judge, etc.) would need ci-tools review.

## Migration Steps

### Phase 1: Code migration

1. Create `cmd/prow-agent-eval/main.go` in ci-tools (keep cobra entrypoint)
2. Create `pkg/prowagenteval/` as the package namespace. Move packages:
   - `internal/cli/*` → `pkg/prowagenteval/cli/` (ci-tools doesn't use `internal/`)
   - `pkg/config` → `pkg/prowagenteval/config/`
   - `pkg/github` → **rewrite** to use prow's `github.Client`
   - `pkg/git` → `pkg/prowagenteval/git/` (keep exec-based, no equivalent in ci-tools)
   - `pkg/judge` → `pkg/prowagenteval/judge/`
   - `pkg/report` → `pkg/prowagenteval/report/` (replace custom JUnit types with `pkg/junit`)
   - `pkg/shared` → `pkg/prowagenteval/shared/`
3. Update all import paths
4. Switch logging from `slog` to `logrus`
5. Switch YAML from `gopkg.in/yaml.v3` to `sigs.k8s.io/yaml`
6. Adopt `pkg/testhelper` golden-file pattern for report output tests
7. Remove `go-github/v67` dependency (replaced by prow client)

### Phase 2: Image pipeline

8. Create `images/prow-agent-eval/Dockerfile` — needs `git` and `make` in runtime image (for `make build`/`make test` collect steps)
9. Add image entry to ci-tools' ci-operator config in release repo
10. Update agentic-dev image config to `COPY --from=` the ci-tools-built image
11. Remove standalone prow-agent-eval ci-operator config and Prow jobs

### Phase 3: Step registry

12. Verify step registry scripts work with the ci-tools-built binary (no changes needed if keeping cobra + single binary)

### Phase 4: Cleanup

13. Archive smg247/prow-agent-eval repo
14. Remove smg247/prow-agent-eval Prow job configs from release repo
15. Remove the standalone image promotion config

## Verification

- `go test ./pkg/prowagenteval/...` passes in ci-tools
- `make production-install` builds the binary alongside other ci-tools binaries
- Image builds successfully with git + make in runtime layer
- Step registry scripts work unchanged against the ci-tools-built binary
- Rehearsal job produces identical artifacts (JUnit, HTML, YAML) to current output

## Effort Estimate

- GitHub client rewrite to prow client: ~2 days (largest piece)
- Package moves + import path updates: ~half day
- Logging/YAML library switches: ~half day
- JUnit type migration: ~half day
- Test pattern migration: ~1 day
- Image pipeline setup: ~half day
- **Total: ~5 days of focused work**

## Recommendation

The migration is worth doing if prow-agent-eval is going to be a long-lived tool. The prow GitHub client alone (retries, App auth, rate limiting) is a meaningful upgrade over raw go-github. The JUnit type adoption improves Spyglass rendering. And the repeated-trigger image build issue disappears.

The main cost is the GitHub client rewrite (~2 days) and the ongoing friction of ci-tools PR velocity. If the tool is still experimental and changing daily, wait until it stabilizes. If it's approaching steady-state, migrate now.
