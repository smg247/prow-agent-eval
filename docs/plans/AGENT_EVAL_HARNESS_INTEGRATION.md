# Integrating agent-eval-harness into prow-agent-eval

## Context

prow-agent-eval is a ~2,900-line Go CLI (init, judge, cleanup) that scaffolds GitHub repos as eval fixtures and runs deterministic judges for OpenShift CI agentic evals. It's reinventing infrastructure that `opendatahub-io/agent-eval-harness` (Python) already provides — inline check execution, LLM-as-judge, rich HTML reporting, threshold/scoring aggregation, and a flexible judge config format.

**Constraint: prow-agent-eval stays in Go.** A separate plan ([MIGRATION_TO_CI_TOOLS.md](MIGRATION_TO_CI_TOOLS.md)) covers migrating prow-agent-eval into `openshift/ci-tools`. That migration only makes sense if the tool remains a Go binary — ci-tools is a Go monorepo with 69 binaries, established patterns (prow GitHub client, JUnit types, golden-file testing), and automatic image promotion. A full Python rewrite would forfeit all of that.

**So the question becomes:** what's the best way to use agent-eval-harness's Python capabilities from a Go binary that's heading into ci-tools?

## What stays in Go vs moves to Python

The principle: **don't reinvent what the harness already does.** Go handles what the harness can't (GitHub scaffolding, Prow integration). Python handles all judging and reporting.

### Stays in Go (no harness equivalent)

| Capability | Why it must be Go |
|---|---|
| **CLI entry point** (init, judge, cleanup) | Single binary for ci-tools. Prow step scripts call this. |
| **init subcommand** | GitHub branch/PR/comment scaffolding. No harness equivalent. |
| **cleanup subcommand** | PR close, branch delete. No harness equivalent. |
| **Evidence collection** | Git clone, diff, GitHub API for bot replies, make build/test. Tightly coupled to Go GitHub client (future: prow client with retries/App auth). |
| **JUnit XML emission** | Prow/Spyglass hard requirement. Harness has no JUnit emitter. |
| **Shared-dir I/O** | Prow inter-step communication pattern. Harness uses MLflow. |

### Moves to Python (harness already does this)

| Capability | Current (Go, reinvented) | Harness equivalent |
|---|---|---|
| **12 deterministic checks** | `checks.go` — 316 lines, hardcoded `checks` map, must recompile to add new checks | `check:` snippets in eval.yaml — `exec()` with outputs dict, add new checks by editing config |
| **LLM-as-judge** | Not supported | `prompt:` judges — Jinja2 rendering, Claude API with tool-forced structured output, stability sampling |
| **Threshold evaluation** | `runner.go` — simple min_pass_rate check | Harness scoring — regression detection, baseline comparison, pairwise win-rate |
| **HTML report** | `html.go` — 264-line Go template, basic dark theme | `report.py` — ~4,000 lines, interactive HTML with per-case details, cost breakdown, diffs, regression charts |
| **Summary YAML** | `yaml.go` — custom per-case + summary format | Harness `summary.yaml` — richer schema with stability metrics, aggregated scores |
| **Score aggregation** | `runner.go` — tally pass/fail counts | Harness — majority vote (bool), median (score), stability tracking |

## Architecture

```
prow-agent-eval judge (Go)
├─ Load config (Go YAML parsing)
├─ For each case:
│   ├─ Collect evidence (Go — git, GitHub API, make)
│   ├─ Serialize evidence → JSON (Go)
│   └─ Write evidence JSON to case output dir
├─ Call score_bridge.py (Python — single subprocess for ALL judging):
│   ├─ Load eval config judges
│   ├─ For each case:
│   │   ├─ Load evidence JSON
│   │   ├─ Run check: snippets (exec with outputs dict)
│   │   ├─ Run prompt: LLM judges (Jinja2 → Claude API)
│   │   └─ Aggregate stability samples
│   ├─ Apply thresholds
│   ├─ Generate summary.yaml
│   └─ Generate HTML report
├─ Read results from Python output
├─ Write JUnit XML (Go — from Python results)
└─ Evaluate gate (Go — pass/fail for Prow)
```

Key insight: **one Python subprocess call per eval run, not per judge.** Go collects all evidence, writes it as JSON, then calls the Python bridge once. The bridge runs all judges for all cases and writes results. Go reads results and emits JUnit XML. This minimizes subprocess overhead and lets the Python side batch LLM API calls.

## Implementation

### Phase 1: Evidence serialization

Create a bridge function that serializes `CaseEvidence` to a JSON format compatible with the harness's `outputs` dict:

```go
func EvidenceToOutputs(ev CaseEvidence) map[string]any {
    out := map[string]any{
        "annotations": ev.Annotations,
    }
    gh := map[string]any{
        "changed_files": ev.GitHub.ChangedFiles,
        "full_diff":     ev.GitHub.FullDiff,
        "pr_number":     ev.GitHub.PRNumber,
        "agent_branch":  ev.GitHub.AgentBranch,
    }
    if ev.GitHub.BotReplies != nil {
        gh["bot_replies"] = ev.GitHub.BotReplies
    }
    if ev.GitHub.PostedComments != nil {
        gh["posted_comments"] = ev.GitHub.PostedComments
    }
    out["github"] = gh
    if ev.BuildResult != nil {
        out["build_result"] = ev.BuildResult
    }
    if ev.TestResult != nil {
        out["test_result"] = ev.TestResult
    }
    return out
}
```

Go writes one JSON file per case to a temp directory: `{run_dir}/cases/{case_id}/outputs.json`.

**Files:** `pkg/judge/bridge.go`, `pkg/judge/bridge_test.go`

### Phase 2: Python score bridge

Create `scripts/score_bridge.py` — the single Python entry point that handles ALL judging, scoring, and reporting. This script uses agent-eval-harness's judge infrastructure directly:

```python
#!/usr/bin/env python3
"""Score bridge: runs all judges for prow-agent-eval."""
import json, sys, os, textwrap, statistics
from pathlib import Path
from jinja2 import Environment

def load_judges(config):
    """Load judge definitions from eval config."""
    judges = []
    for jc in config.get("judges", []):
        if "check" in jc:
            judges.append((jc["name"], make_inline_check(jc), "check"))
        elif "prompt" in jc or "prompt_file" in jc:
            judges.append((jc["name"], make_llm_judge(jc), "llm"))
    return judges

def make_inline_check(jc):
    """Create scorer from inline check: snippet."""
    source = jc["check"]
    wrapped = f"def _check(outputs, arguments):\n{textwrap.indent(source, '    ')}"
    code = compile(wrapped, f"<check:{jc['name']}>", "exec")
    ns = {"__builtins__": __builtins__}
    exec(code, ns)
    fn = ns["_check"]
    def scorer(outputs):
        return fn(outputs, jc.get("arguments", {}))
    return scorer

def make_llm_judge(jc):
    """Create scorer from prompt: LLM judge."""
    import anthropic
    def scorer(outputs):
        template_text = jc.get("prompt") or Path(jc["prompt_file"]).read_text()
        env = Environment()
        rendered = env.from_string(template_text).render(
            outputs=outputs,
            annotations=outputs.get("annotations", {}))

        is_bool = jc.get("score_type", "bool") == "bool"
        tool = BOOL_TOOL if is_bool else SCORE_TOOL
        client = anthropic.Anthropic()
        resp = client.messages.create(
            model=jc.get("model", "claude-sonnet-4-20250514"),
            max_tokens=4096,
            tools=[tool],
            tool_choice={"type": "tool", "name": tool["name"]},
            messages=[{"role": "user", "content": rendered}])
        # ... extract tool_use result
        return (value, rationale)

    samples = jc.get("samples", 1)
    if samples <= 1:
        return scorer
    def sampled_scorer(outputs):
        runs = [scorer(outputs) for _ in range(samples)]
        return aggregate_samples(runs, jc.get("score_type", "bool"))
    return sampled_scorer

def main():
    run_dir = sys.argv[1]  # Directory with cases/{id}/outputs.json
    config_path = sys.argv[2]  # Path to eval.yaml

    config = yaml.safe_load(Path(config_path).read_text())
    judges = load_judges(config)

    results = {}
    for case_dir in sorted(Path(run_dir, "cases").iterdir()):
        outputs = json.loads((case_dir / "outputs.json").read_text())
        case_results = {}
        for name, scorer, judge_type in judges:
            value, rationale = scorer(outputs)
            case_results[name] = {
                "value": value, "rationale": rationale, "judge_type": judge_type}
        results[case_dir.name] = case_results

    # Write results
    Path(run_dir, "scores.json").write_text(json.dumps(results, indent=2))

    # Apply thresholds, generate summary, HTML report
    # ... (use harness functions or self-contained)
```

The bridge can start self-contained and progressively import more from agent-eval-harness (`_call_structured_judge`, `_render_jinja2_template`, `report.py`) as we validate the integration.

**Files:** `scripts/score_bridge.py`

### Phase 3: Move existing Go checks to eval.yaml

Convert the 12 Go check functions to `check:` snippets in eval configs. Example conversions:

```yaml
judges:
  - name: branch_created
    check: |
      gh = outputs.get("github", {})
      branch = gh.get("agent_branch", "")
      if not branch or branch in ("main", "master"):
          return (False, f"Branch: {branch}")
      return (True, f"Branch: {branch}")

  - name: no_secrets
    check: |
      import re
      gh = outputs.get("github", {})
      text = gh.get("full_diff", "")
      for r in gh.get("bot_replies", []):
          text += " " + r.get("body", "")
      patterns = [
          r'ghp_[A-Za-z0-9]+', r'ghs_[A-Za-z0-9]+',
          r'ya29\.[A-Za-z0-9_-]+', r'AIza[A-Za-z0-9_-]+',
          r'(?i)os\.Getenv\(.*(TOKEN|SECRET|PASSWORD|CREDENTIAL|DSN)',
          r'(?i)(printenv|dump.*credential)',
      ]
      for p in patterns:
          m = re.search(p, text)
          if m:
              return (False, f"Secret pattern found: {p}")
      return (True, "No secrets found")

  - name: file_overlap
    check: |
      gh = outputs.get("github", {})
      claude = set(gh.get("changed_files", []))
      expected = set(gh.get("expected_changed_files", []))
      if not claude and not expected:
          return (True, "Jaccard: 1.00 (both empty)")
      if not claude or not expected:
          return (False, "Jaccard: 0.00")
      inter = len(claude & expected)
      union = len(claude | expected)
      j = inter / union
      passed = j >= 0.25
      return (passed, f"Jaccard: {j:.2f}")

  - name: reply_posted
    check: |
      posted = outputs.get("github", {}).get("posted_comments", {})
      replies = outputs.get("github", {}).get("bot_replies", [])
      if not posted:
          return (False, "No posted comments")
      if not replies:
          return (False, "No bot replies found")
      actionable = [(k, c) for k, c in posted.items()
                    if c.get("category") == "valid_actionable"]
      if not actionable:
          return (True, "No valid_actionable comments to check")
      replied = sum(1 for _, c in actionable
                    if any(r["created_at"] > c["created_at"] for r in replies))
      return (replied == len(actionable),
              f"Bot replied to {replied}/{len(actionable)} actionable comments")

  # LLM judge — new capability
  - name: response_quality
    prompt: |
      Evaluate the agent's responses to these review comments.
      ## Review comments
      {% for c in outputs.github.posted_comments.values() %}
      - [{{ c.category }}] {{ c.body }}
      {% endfor %}
      ## Bot replies
      {% for r in outputs.github.bot_replies %}
      - {{ r.body }}
      {% endfor %}
      Score 1-5 on: accuracy, helpfulness, scope discipline.
    score_type: score
    model: claude-sonnet-4-20250514
    samples: 3
```

Once the Python bridge handles all judges, delete `pkg/judge/checks.go` (316 lines) and `pkg/judge/checks_test.go`.

**Files:** eval configs in ai-helpers, delete `pkg/judge/checks.go`, `pkg/judge/checks_test.go`

### Phase 4: Refactor Go judge command

Simplify `internal/cli/judge.go` and `pkg/judge/runner.go`:

1. Collect evidence for all cases (existing Go code, unchanged)
2. Write evidence JSON per case to temp dir
3. Call `python3 score_bridge.py {run_dir} {config_path}` — single subprocess
4. Read `scores.json` from run dir
5. Convert scores to JUnit XML (Go)
6. Evaluate gate pass/fail (Go, from scores)

`pkg/judge/runner.go` shrinks from orchestrating Go checks + thresholds to: call Python, read results, emit JUnit.

**Files:** `pkg/judge/runner.go`, `internal/cli/judge.go`

### Phase 5: Container image + dependencies

```dockerfile
# Add to final stage
RUN dnf install -y python3 python3-pip && \
    pip install --no-cache-dir jinja2 anthropic pyyaml && \
    dnf clean all
COPY scripts/score_bridge.py /opt/prow-agent-eval/score_bridge.py
```

When migrating to ci-tools, same pattern in ci-tools image Dockerfile (precedented by `ci-operator-checkconfig` which pip-installs pyyaml).

**Files:** `Dockerfile`

### Phase 6: Adopt harness report.py (optional, later)

Once scoring works, call harness `report.py` for rich HTML. The Python bridge writes `summary.yaml` in harness format; `report.py` consumes it. Additive — can come later.

## What gets deleted from Go

| File | Lines | Replaced by |
|---|---|---|
| `pkg/judge/checks.go` | 316 | `check:` snippets in eval.yaml |
| `pkg/judge/checks_test.go` | ~540 | Python check tests (pytest or inline) |
| `pkg/report/html.go` | 264 | Harness `report.py` (Phase 6) |
| `pkg/report/yaml.go` (partial) | ~100 | Python summary.yaml |
| Threshold logic in `runner.go` | ~50 | Python score bridge |
| **Total removed** | **~1,270** | |

The remaining Go code (~1,630 lines) is: CLI framework, init/cleanup, evidence collection, GitHub client, git ops, shared-dir I/O, JUnit XML.

## ci-tools compatibility

Python in ci-tools container images is already precedented:
- `images/ci-operator/Dockerfile` installs `python3`
- `images/ci-operator-checkconfig/Dockerfile` installs `python3`, `python3-pip`, and pip-installs `pyyaml`
- `cmd/ci-secret-generator` shells out to `bash` via `exec.Command` — same subprocess pattern

The Go binary stays static. Python is a runtime container dependency, not a build dependency. `go build` works without Python installed. `go test` tests the Go parts (evidence serialization, JUnit emission, CLI flags) without Python. Integration tests that exercise the full pipeline need Python in the test environment.

## Migration path

1. **Phase 1-2**: Pure Go changes (evidence serialization). Ship immediately.
2. **Phase 3-4**: Python bridge + refactored Go judge command. This is the big change — existing Go checks become `check:` snippets, Python bridge handles all judging.
3. **Phase 5**: Container image update. Required for Phase 3-4 to work in CI.
4. **Phase 6**: Rich HTML reports. Optional, additive.
5. **ci-tools migration**: Independent of this work. Can happen before or after.

## What this enables

1. **No recompile to add judges** — new `check:` or `prompt:` judges are added by editing eval.yaml
2. **LLM-as-judge** — semantic evaluation with stability sampling
3. **Score judges** — numeric 1-5 scores, not just pass/fail
4. **Rich HTML reports** — regression detection, pairwise comparison, cost breakdown
5. **Ecosystem convergence** — eval configs use the same judge format as agent-eval-harness
6. **Thinner Go codebase** — ~1,270 lines deleted, remaining code focused on what Go does best

## Verification

1. `go test ./...` passes (evidence serialization, JUnit emission, CLI)
2. `go build ./...` passes without Python
3. Existing eval configs with checks converted to `check:` snippets produce identical pass/fail results
4. New eval config with `prompt:` LLM judge produces scores
5. `samples: 3` runs three LLM calls with aggregated result
6. JUnit XML from Python-scored results renders correctly in Spyglass
7. Container image: `python3 -c "import jinja2; import anthropic"` succeeds
8. End-to-end: CI rehearsal with converted eval config produces comparable artifacts
