package analysis

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"healthyduoduo/internal/contract"
)

// analyte 是产品内部语义 key,不是 LOINC / WS/T 886 码;这里把「key ↔ 标准锚点」
// 固化在契约测试里,防止识别端词典与分析端常量只改一边。
type analyteDict struct {
	Items []struct {
		Name    string `yaml:"name"`
		Code    string `yaml:"code"`
		Loinc   string `yaml:"loinc"`
		Analyte string `yaml:"analyte"`
	} `yaml:"items"`
}

func TestAnalyteCatalogMatchesRecognizerDictionaries(t *testing.T) {
	want := map[string]struct {
		code  string
		loinc string
	}{
		contract.AnalyteALT:            {code: "1500104A", loinc: "1742-6"},
		contract.AnalyteAST:            {code: "1500204A", loinc: "1920-8"},
		contract.AnalyteALP:            {code: "1500404A", loinc: "6768-6"},
		contract.AnalyteBilirubinTotal: {code: "1700504A", loinc: "1975-2"},
		contract.AnalyteTotalProtein:   {code: "1100204A", loinc: "2885-2"},
		contract.AnalyteAlbumin:        {code: "1100104A", loinc: "1751-7"},
		contract.AnalyteHemoglobin:     {code: "0100401A", loinc: "718-7"},
		contract.AnalyteMCV:            {code: "0100601A", loinc: "787-2"},
		contract.AnalyteMCH:            {code: "0100701A", loinc: "785-6"},
		contract.AnalyteMCHC:           {code: "0100801A", loinc: "786-4"},
		contract.AnalyteRDWCV:          {loinc: "788-0"},
	}
	keyPattern := regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	seen := make(map[string]string)

	for _, name := range []string{"chemistry_dict.yaml", "cbc_dict.yaml"} {
		path := filepath.Join("..", "..", "recognizer", "recognizer", name)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var dict analyteDict
		if err := yaml.Unmarshal(raw, &dict); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, item := range dict.Items {
			analyte := strings.TrimSpace(item.Analyte)
			if analyte == "" {
				continue
			}
			if !keyPattern.MatchString(analyte) {
				t.Errorf("%s: analyte %q is not lowercase snake_case", name, analyte)
			}
			if previous, ok := seen[analyte]; ok {
				t.Errorf("analyte %q appears in both %s and %s", analyte, previous, name)
				continue
			}
			seen[analyte] = name
			spec, ok := want[analyte]
			if !ok {
				t.Errorf("%s: analyte %q has no Go constant in internal/contract", name, analyte)
				continue
			}
			if strings.TrimSpace(item.Code) != spec.code {
				t.Errorf("%s/%s: code = %q, want %q", name, analyte, item.Code, spec.code)
			}
			if strings.TrimSpace(item.Loinc) != spec.loinc {
				t.Errorf("%s/%s: loinc = %q, want %q", name, analyte, item.Loinc, spec.loinc)
			}
			if strings.TrimSpace(item.Name) == "" {
				t.Errorf("%s/%s: canonical name is empty", name, analyte)
			}
		}
	}

	for analyte := range want {
		if _, ok := seen[analyte]; !ok {
			t.Errorf("analyte %q has a Go constant but no recognizer dictionary item", analyte)
		}
	}
}
