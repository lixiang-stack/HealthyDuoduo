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
| Analysis framework / explanation / quota | `schemas/openapi.yaml` `AnalysisFramework`, `AnalysisExplanation`, `AnalysisQuota` | `internal/analysis/framework*.go`, `knowledge/`, `explain.go`; App `src/analysis/components.tsx`, `framework.ts` |
| Analysis feedback / metrics | `schemas/openapi.yaml` `AnalysisFeedback*`, `AnalysisMetrics` | `cmd/hdd/serve_analysis_feedback.go`; `internal/analysis/store.go`; storage `analysis_feedback`, `analysis_events`, `analysis_llm_calls`; App `src/analysis/FeedbackModal.tsx` |

### Cross-cutting mirrors

- Panel code / order: `schemas/report.schema.json` `$defs.panel` → `recognizer/recognizer/contract.py` → `internal/pipeline/panels.go` → `apps/mobile/src/reports/panels.ts`.
- Report item analyte: `recognizer/recognizer/{chemistry,cbc}_dict.yaml` `analyte` → `schemas/report.schema.json` `reportItem.analyte` → `recognizer/recognizer/contract.py` / `internal/contract` / `internal/httpapi.ReportItem` → `report_items.analyte` → `internal/analysis` framework / knowledge matching. key 取 LOINC component 风格的全称小写 snake_case（如 `alanine_aminotransferase`、`bilirubin_total`），缩写只用于 UI / 框架文案；加指标时必须同时给词典项写 `analyte`，否则框架层与条目触发不会按该指标匹配。
- Narrative report kinds: `recognizer/recognizer/postprocess.py` (`NARRATIVE_BODY_SECTION_KINDS`) → `internal/pipeline/reports.go` (`hasReportBody`) → `recognizer/tools/e2e.py` (`verify_narrative_reports`) → App `sections.ts` / `status.ts`.
- Report summary/detail projection: `internal/httpapi/types.go` ↔ `schemas/openapi.yaml` ↔ App `api/reports.ts`.
- Analysis facts: `internal/analysis` (`Facts` / `Snapshot` JSON tags) ↔ `schemas/openapi.yaml` `AnalysisFacts` / `AnalysisSnapshot` ↔ App `api/analyses.ts`; facts window / gating changes must keep the snapshot rule version in sync.
- Analysis degradation events: `internal/analysis/explain.go` `Event*` constants ↔ `internal/storage/migrations/0016_analysis_observability.sql` `analysis_events.kind` CHECK ↔ `AnalysisMetrics.degradation_kinds`. Add a kind in all three places or the insert fails.

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
   - `cd apps/mobile && npm run typecheck && npm test && npm run lint -- --max-warnings=0`
   - storage/migration changes: `bash scripts/test-db.sh`
7. Recognizer / panel / narrative changes: run the pipeline E2E path in `docs/agents/e2e.md`.

## Adding an API endpoint / use case

Use this as the implementation order; the table above says where each mirror lives.

1. Add the path + request/response schemas to `schemas/openapi.yaml`.
2. Mirror the shapes in `internal/httpapi` (`types.go` or a feature file). The handler decodes this mirror; do not define a second request-body struct in `cmd/hdd`.
3. Put the business rules in the domain layer: `internal/pipeline` for orchestration / existing domains, `internal/analysis` for pure analysis rules.
4. Add the storage port + adapter in `internal/storage`; add a migration when a new table/column is needed.
5. Add the handler in `cmd/hdd/serve_<feature>.go` and register the route in `cmd/hdd/serve.go`.
6. Add an HTTP fixture under `schemas/testdata/http/` and a reconcile case in `internal/httpapi/contract_test.go`.
7. Add the App API wrapper under `apps/mobile/src/api/`, then the screen/component; cover the screen with RNTL. Use existing display helpers (`reportTypeLabel`, `flagLabel`, etc.) instead of raw wire enums.
8. Run `npm run codegen`, `go test ./...`, `npm test`, `npm run lint -- --max-warnings=0`; for storage/migration changes run `bash scripts/test-db.sh`.

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
