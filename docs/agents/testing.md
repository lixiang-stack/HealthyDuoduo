# Testing Notes

Operational rules that complement the test commands in `AGENTS.md`.

## Fakes must mirror real adapters

In-memory fakes are useful for HTTP/domain tests, but they are not evidence that the real adapter works.

- If a fake implements a derived or normalized field (for example `is_child`, year→Jan-1 normalization, unit normalization), it must mirror the real rule, not the test's expected output.
- Every derived field needs at least one real-store / DB-backed test. A pure fake test passing is not enough.
- If a fake intentionally diverges, document why and add an integration test that covers the real path.

This rule exists because fake-only tests have produced false-green results: HTTP tests passed while the real store normalized or derived values differently.

## Database-backed tests

`go test ./...` skips `internal/storage` when `HDD_TEST_DSN` is unset. For storage, migration, or derived-field changes run:

```bash
bash scripts/test-db.sh
```

The script starts a disposable PostgreSQL and creates a fresh database per `Test*` function. CI runs the same script in the `storage` job.

## UI tests

- RNTL screen tests live in `apps/mobile/src/test/screens/` and mock the API at `resolveApiClient`; do not mock individual hooks.
- Custom interaction controls (wheels, pickers, gestures) should expose a pure function for scroll/index math and unit-test it; RNTL then covers selection/submit.
- Real-device smoke is still required for feel and accessibility; see `apps/mobile/ANDROID.md`.
