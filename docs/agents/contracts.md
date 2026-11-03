# Contract Changes

This repo mirrors the same contract through several layers. Use this page as a navigation map, not as a second source of truth.

## Sources of truth

- `schemas/report.schema.json` — recognition output (`Report`, `ReportItem`, `ReportSection`, panel enum/order).
- `schemas/openapi.yaml` — HTTP `/api/v1` contract.

## Mirrors

| Layer | File | How drift is caught |
| --- | --- | --- |
| Python recognition | `recognizer/recognizer/contract.py` | `recognizer/tests/test_contract.py` compares enum/order with `report.schema.json` |
| Go recognition | `internal/contract/contract.go` | `internal/contract` fixtures / strict decode |
| Go HTTP | `internal/httpapi/types.go` | `internal/httpapi/contract_test.go` compares fixtures, required fields, panel enum |
| App | `apps/mobile/src/api/schema.d.ts` | `npm run codegen`; pre-commit and CI fail on stale generated file |
| HTTP fixtures | `schemas/testdata/http/*.json` | round-trip against Go mirrors and OpenAPI schemas |
| Growth record | `schemas/openapi.yaml` `GrowthRecord` | `internal/httpapi/types.go`; App `src/api/growth.ts`; storage `growth_records` |
| Analysis facts / snapshot | `schemas/openapi.yaml` `Analysis*` | `internal/httpapi/analysis.go` → `internal/analysis` types; App `src/api/analyses.ts`; storage `analyses.layers/snapshot` |

### Cross-cutting mirrors

- Panel code / order: `schemas/report.schema.json` `$defs.panel` → `recognizer/recognizer/contract.py` → `internal/pipeline/panels.go` → `apps/mobile/src/reports/panels.ts`.
- Narrative report kinds: `recognizer/recognizer/postprocess.py` (`NARRATIVE_BODY_SECTION_KINDS`) → `internal/pipeline/reports.go` (`hasReportBody`) → `recognizer/tools/e2e.py` (`verify_narrative_reports`) → App `sections.ts` / `status.ts`.
- Report summary/detail projection: `internal/httpapi/types.go` ↔ `schemas/openapi.yaml` ↔ App `api/reports.ts`.
- Analysis facts: `internal/analysis` (`Facts` / `Snapshot` JSON tags) ↔ `schemas/openapi.yaml` `AnalysisFacts` / `AnalysisSnapshot` ↔ App `api/analyses.ts`; facts window / gating changes must keep the snapshot rule version in sync.

## Change checklist

1. Change the source schema first.
2. Update every mirror in the table above.
3. Update HTTP fixtures and contract tests.
4. Run `cd apps/mobile && npm run codegen`.
5. Sweep removed terms / fields across code, tests, docs, and user-facing copy:
   `rg 'old_field|old enum|old Chinese copy' .`
   Check empty/error states and settings screens, not only forms.
6. Run the relevant checks:
   - `go test ./... && go vet ./...`
   - `cd recognizer && uv run pytest && uv run ruff check .`
   - `cd apps/mobile && npm run typecheck && npm test && npx expo lint`
   - storage/migration changes: `bash scripts/test-db.sh`
7. Recognizer / panel / narrative changes: run the pipeline E2E path in `docs/agents/e2e.md`.

## Adding a narrative report type

Beyond the normal mirrors:

- add the dictionary / `report_type` classification;
- add the panel code + label + fixed order;
- add section kinds to `NARRATIVE_BODY_SECTION_KINDS` and the Go `hasReportBody`;
- update `recognizer/tools/e2e.py` `verify_narrative_reports` inputs;
- update App section labels and narrative status/date copy;
- add a zero-item + sections HTTP test and a recognizer split/fallback test.

## Removing a field or enum value

- remove it from request and response schemas, Go/Python/App mirrors, and fixtures;
- decide whether the DB column is legacy (document it) or migrated away;
- update request validation tests to assert the old shape is rejected;
- `rg` the old term in docs and App copy; stale empty/error copy has no compiler.
