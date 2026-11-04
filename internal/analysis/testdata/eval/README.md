# Analysis evaluation fixtures

`fixtures.json` freezes the expected facts-layer and framework-layer output for the
analysis evaluation set (llm-analysis 票 10). The test only reads this file; it never
regenerates it, so any output change fails CI.

- Facts cases: every `samples/expected/report/*.json` report × panel × entry mode
  (`panel` / `indicator`), 110 cases. Each case includes same-panel reports from the
  24 months before the target report, so window / sparse history is frozen too.
- Framework cases: hand-built boundary cases (R value, transaminase grade, BMI
  percentile / WS/T age cutoffs, WHO Hb cutoffs).

Regenerate deliberately from the repository root, then review the diff before
committing:

```bash
go run ./internal/analysis/gen/eval
```

Do not regenerate as part of a normal test run.
