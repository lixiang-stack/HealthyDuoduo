package httpapi

// 契约互证(票 02「沿用识别端双向互证模式」):
//   1. OpenAPI 12 端点集合冻结(多/少/改名即失败);
//   2. OpenAPI 指标项共享字段子树 == 识别端 report.schema.json 的 reportItem
//      (识别端契约文件零触碰的前置防线);
//   3. HTTP fixtures(schemas/testdata/http/*.json)对 OpenAPI schema 校验,
//      并与 Go 镜像 struct 双向回转(marshal 与 unmarshal 双向互证)。
// schema 校验以 Go 内嵌迷你 validator 实现(见 schema_test.go):只支持契约
// 实际使用的 2020-12 子集(type/enum/const/required/properties/
// additionalProperties/minProperties/minLength/pattern/minimum/items/oneOf/anyOf)。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	// 本包固定在 internal/httpapi,根目录为 ../../(测试产物路径不依赖 pwd)。
	abs, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

const (
	trendPath             = "/members/{member_id}/indicators/{name}/trend"
	indicatorNameParamRef = "#/components/parameters/IndicatorName"
)

// loadFixture 读 HTTP fixture golden。
func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "schemas", "testdata", "http", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

// TestOpenAPIFrozenPathSet 15 端点集合冻结:实现票不得多加、少删或改路径/方法
// (契约变更须先改本文件与 OpenAPI 再随 PR 提走)。
func TestOpenAPIFrozenPathSet(t *testing.T) {
	doc := getOpenAPI(t)
	want := map[string]bool{
		"GET /health":  true,
		"GET /members": true, "POST /members": true,
		"PATCH /members/{member_id}": true, "DELETE /members/{member_id}": true,
		"GET /reports": true, "POST /reports": true,
		"GET /reports/{report_id}": true, "PATCH /reports/{report_id}": true,
		"DELETE /reports/{report_id}":                true,
		"PATCH /reports/{report_id}/items/{item_id}": true,
		"GET /images/{sha256}":                       true,
		"GET " + trendPath:                           true,
		"GET /members/{member_id}/panels":            true,
		"GET /members/{member_id}/panels/{panel}":    true,
	}
	got := map[string]bool{}
	for path, item := range doc["paths"].(map[string]any) {
		for method := range item.(map[string]any) {
			got[strings.ToUpper(method)+" "+path] = true // yaml 键为小写
		}
	}
	if len(got) != len(want) {
		t.Errorf("frozen path set = %d ops, want %d: %v", len(got), len(want), got)
	}
	for w := range want {
		if !got[w] {
			t.Errorf("frozen endpoint missing from OpenAPI: %s", w)
		}
	}
	for g := range got {
		if !want[g] {
			t.Errorf("unfrozen endpoint in OpenAPI: %s", g)
		}
	}
}

// TestOpenAPITrendNameParam 趋势路径的 {name} 必须在参数表声明为可复用的 path
// 参数;漏声明会让 codegen 的 getTrend 只有 member_id,App 无法类型安全调用。
func TestOpenAPITrendNameParam(t *testing.T) {
	doc := getOpenAPI(t)
	paths := doc["paths"].(map[string]any)
	trend := paths[trendPath].(map[string]any)
	get := trend["get"].(map[string]any)
	params, ok := get["parameters"].([]any)
	if !ok {
		t.Fatal("trend endpoint has no parameters")
	}

	var nameParam map[string]any
	for _, raw := range params {
		param, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if ref, ok := param["$ref"].(string); ok && ref == indicatorNameParamRef {
			nameParam = resolveRef(ref)
			break
		}
	}
	if nameParam == nil {
		t.Fatalf("trend endpoint must $ref %s", indicatorNameParamRef)
	}

	if got := nameParam["name"]; got != "name" {
		t.Errorf("IndicatorName.name = %v, want name", got)
	}
	if got := nameParam["in"]; got != "path" {
		t.Errorf("IndicatorName.in = %v, want path", got)
	}
	if got := nameParam["required"]; got != true {
		t.Errorf("IndicatorName.required = %v, want true", got)
	}
	desc, _ := nameParam["description"].(string)
	for _, want := range []string{"指标项名称", "通常为词典规范名", "修正后以修正值为准", "URL 编码"} {
		if !strings.Contains(desc, want) {
			t.Errorf("IndicatorName.description = %q, missing %q", desc, want)
		}
	}
	schema, ok := nameParam["schema"].(map[string]any)
	if !ok {
		t.Fatal("IndicatorName.schema is not an object")
	}
	if got := schema["type"]; got != "string" {
		t.Errorf("IndicatorName.schema.type = %v, want string", got)
	}
	if got := jsonCompact(schema["minLength"]); got != "1" {
		t.Errorf("IndicatorName.schema.minLength = %s, want 1", got)
	}
}

// TestOpenAPINotFoundDescription 趋势端点不做词典成员检查,空结果返回 200;
// NotFound 描述不能再声称「trend name 未收录」。
func TestOpenAPINotFoundDescription(t *testing.T) {
	doc := getOpenAPI(t)
	responses := doc["components"].(map[string]any)["responses"].(map[string]any)
	notFound := responses["NotFound"].(map[string]any)
	desc, _ := notFound["description"].(string)
	if strings.Contains(desc, "trend name 未收录") {
		t.Errorf("NotFound description still claims trend name 404: %q", desc)
	}
	if !strings.Contains(desc, "unknown member") {
		t.Errorf("NotFound description lost target list: %q", desc)
	}
}

// TestOpenAPIMirrorsRecognitionItem 共享字段子树逐项相等:HTTP 指标项的八个
// 识别端字段(name/value/unit/ref_range/method/flag/low_confidence/raw_text)
// 必须与 schemas/report.schema.json 的 reportItem 属性完全一致。
func TestOpenAPIMirrorsRecognitionItem(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "schemas", "report.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	reportItem := schema["$defs"].(map[string]any)["reportItem"].(map[string]any)
	props := reportItem["properties"].(map[string]any)

	doc := getOpenAPI(t)
	httpProps := componentSchema(t, doc, "ReportItem")["properties"].(map[string]any)

	shared := []string{
		"name", "value", "unit", "ref_range", "method", "flag", "low_confidence", "raw_text",
		"code", "loinc", "unit_display", "unit_status", "raw_value", "raw_unit",
		"raw_ref_range", "ref_kind", "result_kind", "result_basis",
	}
	for _, field := range shared {
		lookup, ok := props[field]
		if !ok {
			t.Fatalf("report.schema.json lost property %q — recognition contract changed?", field)
		}
		if rec, http := jsonCompact(lookup), jsonCompact(httpProps[field]); rec != http {
			t.Errorf("shared field %q drifted:\n  recognition: %s\n  openapi:     %s", field, rec, http)
		}
	}
}

// TestOpenAPIMirrorsRecognitionSection 报告段落:kind/text 与识别端 reportSection
// 逐字段相同;识别端 ocr_line_numbers 在 HTTP 侧映射为 ocr_lines: OcrLineLink[]。
func TestOpenAPIMirrorsRecognitionSection(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "schemas", "report.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	rec := schema["$defs"].(map[string]any)["reportSection"].(map[string]any)
	recProps := rec["properties"].(map[string]any)

	doc := getOpenAPI(t)
	httpProps := componentSchema(t, doc, "ReportSection")["properties"].(map[string]any)

	for _, field := range []string{"kind", "text"} {
		if recJSON, httpJSON := jsonCompact(recProps[field]), jsonCompact(httpProps[field]); recJSON != httpJSON {
			t.Errorf("shared section field %q drifted:\n  recognition: %s\n  openapi:     %s", field, recJSON, httpJSON)
		}
	}
	lineNumbers, ok := recProps["ocr_line_numbers"].(map[string]any)
	if !ok {
		t.Fatalf("report.schema.json lost ocr_line_numbers")
	}
	items, ok := lineNumbers["items"].(map[string]any)
	if !ok || items["type"] != "integer" {
		t.Errorf("ocr_line_numbers items = %v, want integer line indices", lineNumbers["items"])
	}
	lines, ok := httpProps["ocr_lines"].(map[string]any)
	if !ok {
		t.Fatalf("OpenAPI ReportSection lost ocr_lines")
	}
	lineItems, ok := lines["items"].(map[string]any)
	if !ok || lineItems["$ref"] != "#/components/schemas/OcrLineLink" {
		t.Errorf("ocr_lines items = %v, want OcrLineLink ref", lines["items"])
	}
}

// TestOpenAPIPanelEnumMirrorsRecognition 项目组枚举双端同源:OpenAPI PanelCode 与
// 识别端 report.schema.json 的 panel 枚举逐值一致;报告摘要 / 详情 / 指标项以必填
// 字段 + $ref 暴露,防止字段漂移。
func TestOpenAPIPanelEnumMirrorsRecognition(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "schemas", "report.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	recPanel := schema["$defs"].(map[string]any)["panel"].(map[string]any)

	doc := getOpenAPI(t)
	openPanel := componentSchema(t, doc, "PanelCode")
	if rec, open := jsonCompact(recPanel["enum"]), jsonCompact(openPanel["enum"]); rec != open {
		t.Errorf("PanelCode enum drifted:\n  recognition: %s\n  openapi:     %s", rec, open)
	}

	panelRef := "#/components/schemas/PanelCode"
	itemProps := componentSchema(t, doc, "ReportItem")["properties"].(map[string]any)
	if ref := itemProps["panel"].(map[string]any)["$ref"]; ref != panelRef {
		t.Errorf("ReportItem.panel $ref = %v, want %s", ref, panelRef)
	}
	for _, component := range []string{"ReportSummary", "ReportDetail"} {
		entry := componentSchema(t, doc, component)
		panels, ok := entry["properties"].(map[string]any)["panels"].(map[string]any)
		if !ok {
			t.Fatalf("%s.panels missing", component)
		}
		if ref := panels["items"].(map[string]any)["$ref"]; ref != panelRef {
			t.Errorf("%s.panels items $ref = %v, want %s", component, ref, panelRef)
		}
		if !requiredField(entry, "panels") {
			t.Errorf("%s.panels must be required", component)
		}
	}
	if !requiredField(componentSchema(t, doc, "ReportItem"), "panel") {
		t.Error("ReportItem.panel must be required")
	}
}

func requiredField(schema map[string]any, field string) bool {
	required, _ := schema["required"].([]any)
	for _, name := range required {
		if name == field {
			return true
		}
	}
	return false
}

// fixtureCase 一张表同时供「对 schema 校验」与「对 Go 镜像回转」两个方向使用,
// 免去两处清单各自漂移。
type fixtureCase struct {
	fixture   string
	component string
	decode    func([]byte) (any, error)
}

var reconcileCases = []fixtureCase{
	{"member.json", "Member", unmarshal[Member]},
	{"report.summary.json", "ReportSummary", unmarshal[ReportSummary]},
	{"report.summary.failed.json", "ReportSummary", unmarshal[ReportSummary]},
	{"report.detail.json", "ReportDetail", unmarshal[ReportDetail]},
	{"report.detail.imaging.json", "ReportDetail", unmarshal[ReportDetail]},
	{"upload.duplicate.json", "UploadResult", unmarshal[UploadResult]},
	{"report.conflict.json", "Error", unmarshal[Error]},
	{"review.request.confirm.json", "ReviewRequest", unmarshal[ReviewRequest]},
	{"review.request.no_reference.json", "ReviewRequest", unmarshal[ReviewRequest]},
	{"review.request.correct.json", "ReviewRequest", unmarshal[ReviewRequest]},
	{"review.result.json", "ReviewResult", unmarshal[ReviewResult]},
	{"correction.pair.json", "CorrectionPair", unmarshal[CorrectionPair]},
	{"trend.json", "TrendResult", unmarshal[TrendResult]},
	{"panel.list.json", "PanelsList", unmarshal[PanelsList]},
	{"panel.detail.json", "PanelDetail", unmarshal[PanelDetail]},
}

// TestFixturesReconcileOpenAPI 每个 fixture 对其声明的 OpenAPI component 校验;
// invalid fixture(手改 flag)必须被拒。
func TestFixturesReconcileOpenAPI(t *testing.T) {
	doc := getOpenAPI(t)
	for _, c := range reconcileCases {
		c := c
		t.Run(c.fixture, func(t *testing.T) {
			var body any
			if err := json.Unmarshal(loadFixture(t, c.fixture), &body); err != nil {
				t.Fatal(err)
			}
			if errs := validate(componentSchema(t, doc, c.component), body); len(errs) > 0 {
				t.Errorf("fixture %s does not satisfy component %s: %v", c.fixture, c.component, errs)
			}
		})
	}
	t.Run("review.request.flag.invalid.json rejected", func(t *testing.T) {
		var body any
		if err := json.Unmarshal(loadFixture(t, "review.request.flag.invalid.json"), &body); err != nil {
			t.Fatal(err)
		}
		if errs := validate(componentSchema(t, doc, "ReviewRequest"), body); len(errs) == 0 {
			t.Error("flag correction was accepted — contract must not allow manual flag edits")
		}
	})
}

// TestFixturesRoundTripGoMirror fixtures 与 Go 镜像双向:unmarshal fixture →
// marshal 回 → 语义等值 fixture(键序归一);方向二:构造值 marshal 锚键序与语义。
func TestFixturesRoundTripGoMirror(t *testing.T) {
	// 方向一:golden → struct → JSON → map,应与 golden 本体一致(零丢字段/零变形)。
	for _, c := range reconcileCases {
		c := c
		t.Run(c.fixture, func(t *testing.T) {
			raw := loadFixture(t, c.fixture)
			v, err := c.decode(raw)
			if err != nil {
				t.Fatalf("unmarshal into Go mirror: %v", err)
			}
			out, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			if !jsonEqual(raw, out) {
				t.Errorf("Go mirror round-trip differs from golden:\n got  %s\n want %s", out, raw)
			}
		})
	}
	// 方向二:构造 → marshal,锚关键语义字段(fixture 之外代码可自由产值)。
	want := FlagCounts{High: 5, Low: 2, Unknown: 1}
	out, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(out); got != `{"high":5,"low":2,"abnormal":0,"unknown":1}` {
		t.Errorf("FlagCounts marshal = %s", got)
	}
}

func unmarshal[T any](raw []byte) (any, error) {
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return v, nil
}

// jsonEqual 语义比较(map 归一,键序无关)。
func jsonEqual(a, b []byte) bool {
	var ma, mb any
	if json.Unmarshal(a, &ma) != nil || json.Unmarshal(b, &mb) != nil {
		return false
	}
	ea, _ := json.Marshal(ma)
	eb, _ := json.Marshal(mb)
	return string(ea) == string(eb)
}

// jsonString → jsonCompact(schema_test.go),缓存一致性 helper 单一来源。
