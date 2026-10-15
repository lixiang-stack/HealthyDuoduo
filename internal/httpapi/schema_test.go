package httpapi

// OpenAPI 精简 validator(仅测试用):契约文件只使用 JSON Schema 2020-12 的小子集——
// type(含 [X, "null"] 形式)/enum/const/required/properties/additionalProperties/
// minItems·maxItems·minProperties·minLength·pattern·minimum/items/oneOf/anyOf/$ref。
// 未支持的 keyword 会假绿——为防契约面无意识扩大,新增组合能力必须先补这里并加
// 断言用例(与识别端"双向互证"同一思路:另一份契约文件经逐字段子树比较在
// TestOpenAPIMirrorsRecognitionItem 锁死)。
// 注意:oneOf 分支经 const action 值互斥,matchesOneBranch 的 anyOf 语义即够;
// 若契约未来放宽到可重叠分支,此处需升级为「恰好一个」。

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

var (
	openAPILoadOnce sync.Once
	openAPIDoc      map[string]any
	openAPILoadErr  error
)

func getOpenAPI(t interface{ Fatalf(string, ...any) }) map[string]any {
	openAPILoadOnce.Do(func() {
		b, err := os.ReadFile(filepath.Join("..", "..", "schemas", "openapi.yaml"))
		if err != nil {
			openAPILoadErr = err
			return
		}
		var y any
		if err := yaml.Unmarshal(b, &y); err != nil {
			openAPILoadErr = err
			return
		}
		jb, err := json.Marshal(y)
		if err != nil {
			openAPILoadErr = err
			return
		}
		var doc map[string]any
		if err := json.Unmarshal(jb, &doc); err != nil {
			openAPILoadErr = err
			return
		}
		openAPIDoc = doc
	})
	if openAPILoadErr != nil {
		t.Fatalf("load schemas/openapi.yaml: %v", openAPILoadErr)
	}
	return openAPIDoc
}

func componentSchema(t interface{ Fatalf(string, ...any) }, doc map[string]any, name string) map[string]any {
	schemas := doc["components"].(map[string]any)["schemas"].(map[string]any)
	if s, ok := schemas[name]; ok {
		return s.(map[string]any)
	}
	t.Fatalf("openapi.yaml missing component schema %q", name)
	return nil
}

// validate 值 → 错误列表(空 = 通过)。
func validate(schema map[string]any, value any) []string {
	return checkSchema(schema, value, nil)
}

func checkSchema(schema map[string]any, value any, errs []string) []string {
	if ref, ok := schema["$ref"].(string); ok {
		target := resolveRef(ref)
		if target == nil {
			return append(errs, "unresolvable $ref: "+ref)
		}
		return checkSchema(target, value, errs)
	}
	if types, ok := schema["type"]; ok {
		var allowed []string
		switch ty := types.(type) {
		case string:
			allowed = []string{ty}
		case []any:
			for _, item := range ty {
				allowed = append(allowed, item.(string))
			}
		}
		if !matchesJSONType(allowed, value) {
			return append(errs, fmt.Sprintf("type %s required, got %s", jsonCompact(allowed), dataTypeOf(value)))
		}
	}
	if en, ok := schema["enum"].([]any); ok && !containsScalar(en, value) {
		errs = append(errs, "value not in enum: "+jsonCompact(value))
	}
	if c, ok := schema["const"]; ok && !scalarEq(c, value) {
		errs = append(errs, fmt.Sprintf("const mismatch: want %s, got %s", jsonCompact(c), jsonCompact(value)))
	}
	if obj, isObj := value.(map[string]any); isObj {
		props, _ := schema["properties"].(map[string]any)
		for key, sub := range obj {
			if child, ok := props[key].(map[string]any); ok {
				errs = checkSchema(child, sub, errs)
			}
		}
		for _, req := range requiredOf(schema) {
			if _, present := obj[req]; !present {
				errs = append(errs, "missing required field: "+req)
			}
		}
		if addl, ok := schema["additionalProperties"].(bool); ok && !addl {
			for key := range obj {
				if _, defined := props[key]; !defined {
					errs = append(errs, "field not allowed: "+key)
				}
			}
		}
	}
	if items, ok := schema["items"].(map[string]any); ok {
		if list, isList := value.([]any); isList {
			for _, item := range list {
				errs = checkSchema(items, item, errs)
			}
		}
	}
	if n, ok := num(schema["minItems"]); ok {
		if list, isList := value.([]any); isList && len(list) < n {
			errs = append(errs, "array shorter than minItems")
		}
	}
	if n, ok := num(schema["maxItems"]); ok {
		if list, isList := value.([]any); isList && len(list) > n {
			errs = append(errs, "array longer than maxItems")
		}
	}
	if n, ok := num(schema["minProperties"]); ok {
		if obj, isObj := value.(map[string]any); isObj && len(obj) < n {
			errs = append(errs, "object smaller than minProperties")
		}
	}
	if n, ok := num(schema["minLength"]); ok {
		if s, isStr := value.(string); isStr && utf8.RuneCountInString(s) < n {
			errs = append(errs, "string shorter than minLength")
		}
	}
	if pat, ok := schema["pattern"].(string); ok {
		if s, isStr := value.(string); isStr && !regexp.MustCompile(pat).MatchString(s) {
			errs = append(errs, "string does not match pattern")
		}
	}
	if n, ok := num(schema["minimum"]); ok {
		if f, isNum := value.(float64); isNum && f < float64(n) {
			errs = append(errs, "below minimum")
		}
	}
	if branches, ok := schema["oneOf"].([]any); ok && !matchesOneBranch(branches, value) {
		errs = append(errs, "no oneOf branch matched")
	}
	if branches, ok := schema["anyOf"].([]any); ok && !matchesOneBranch(branches, value) {
		errs = append(errs, "no anyOf branch matched")
	}
	return errs
}

// num 数字断言统一取整型语义(yaml→json 归一后所有数字为 float64)。
func num(v any) (int, bool) {
	if f, ok := v.(float64); ok {
		return int(f), true
	}
	return 0, false
}

func requiredOf(schema map[string]any) []string {
	req, ok := schema["required"].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(req))
	for _, r := range req {
		out = append(out, r.(string))
	}
	return out
}

func matchesJSONType(allowed []string, value any) bool {
	got := dataTypeOf(value)
	if got == "number" {
		for _, a := range allowed {
			if a == "number" || a == "integer" {
				return true
			}
		}
		return false
	}
	for _, a := range allowed {
		if a == got {
			return true
		}
	}
	return false
}

func dataTypeOf(value any) string {
	switch value.(type) {
	case string:
		return "string"
	case float64, int:
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	case bool:
		return "boolean"
	default:
		return "null"
	}
}

// containsScalar 枚举值比对(本契约枚举全为标量)。
func containsScalar(list []any, value any) bool {
	for _, item := range list {
		if scalarEq(item, value) {
			return true
		}
	}
	return false
}

func scalarEq(want, got any) bool { return jsonCompact(want) == jsonCompact(got) }

func matchesOneBranch(branches []any, value any) bool {
	for _, b := range branches {
		if sub, ok := b.(map[string]any); ok && len(checkSchema(sub, value, nil)) == 0 {
			return true
		}
	}
	return false
}

func resolveRef(ref string) map[string]any {
	if !strings.HasPrefix(ref, "#/") {
		return nil
	}
	parts := strings.Split(strings.TrimPrefix(ref, "#/"), "/")
	cur := any(openAPIDoc)
	for _, seg := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		if cur, ok = m[seg]; !ok {
			return nil
		}
	}
	schema, ok := cur.(map[string]any)
	if !ok {
		return nil // 指向非 schema/参数对象的 ref 视为悬挂
	}
	return schema
}

// jsonCompact 任意值 → 稳定比较串(枚举/const/type 组都是标量或标量数组)。
func jsonCompact(v any) string {
	out, err := json.Marshal(v)
	if err != nil {
		return "<unserializable>"
	}
	return string(out)
}

// TestOpenAPIRefIntegrity 全文档 $ref 完整性:悬挂引用当场失败。
func TestOpenAPIRefIntegrity(t *testing.T) {
	doc := getOpenAPI(t)
	walk(t, doc)
}

// walk 递归收集 $ref;schema/参数/响应 三类组件都可被 ref 目标化。
func walk(t *testing.T, node any) {
	if m, ok := node.(map[string]any); ok {
		if ref, ok := m["$ref"].(string); ok {
			if resolveRef(ref) == nil {
				t.Errorf("dangling $ref: %s", ref)
			}
		}
		for _, v := range m {
			walk(t, v)
		}
		return
	}
	if list, ok := node.([]any); ok {
		for _, v := range list {
			walk(t, v)
		}
	}
}
