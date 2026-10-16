package pipeline

// LinkOCRLines 单测(票 07):raw_text 反推组成它的 OCR 行。
// 用例数据取自共享栈真实报告(血常规 ce9a8587 / 尿常规 9da58c22 的 OCR 行与
// raw_text 形状):多格行的同一视觉行重建、重复碎片格(如 ↑ %、%)按 y 聚带消歧、
// 边界面(非 "  " 分隔不算格)、cx 单调约束、退化输入。

import (
	"testing"

	"healthyduoduo/internal/contract"
)

// boxAt 以左上角 + 固定尺寸(40×20)造行框(真实坐标为像素量纲;中心点即 (x+20,y+10))。
func boxAt(x, y float64) contract.Box {
	return contract.Box{{x, y}, {x + 40, y}, {x + 40, y + 20}, {x, y + 20}}
}

type ocrLine struct {
	text string
	x, y float64
}

// ocrWith 按声明顺序构造 OCR 结果(顺序即 txts/boxes 下标)。
func ocrWith(lines ...ocrLine) *contract.OCRResult {
	ocr := &contract.OCRResult{Engine: "onnxruntime"}
	for _, l := range lines {
		ocr.Txts = append(ocr.Txts, l.text)
		ocr.Boxes = append(ocr.Boxes, boxAt(l.x, l.y))
		ocr.Scores = append(ocr.Scores, 0.99)
	}
	return ocr
}

// cbcRowOCR 复刻真实血常规报告的两行(单核细胞/中性粒细胞)+ 表头碎片:
// 两行各含 "↑ %" 碎片格,表头另有一个 "%" 列头,行内还有 "10.8"/"81.3" 等唯一位。
func cbcRowOCR() *contract.OCRResult {
	return ocrWith(
		ocrLine{"%", 878, 55},       // 0 表头列头
		ocrLine{"单核细胞百分比", 51, 73},  // 1
		ocrLine{"10.8", 227, 76},    // 2
		ocrLine{"↑ %", 313, 74},     // 3
		ocrLine{"3-10", 403, 74},    // 4
		ocrLine{"中性粒细胞百分比", 52, 95}, // 5
		ocrLine{"81.3", 227, 99},    // 6
		ocrLine{"↑ %", 313, 98},     // 7
		ocrLine{"40-75", 404, 98},   // 8
	)
}

// TestLinkOCRLinesSplitsMultiCellRow 两格以上之行:按 raw_text 重建并选中同一视觉行的
// 重复碎片(y 聚带消歧),返回行下标与框坐标。
func TestLinkOCRLinesSplitsMultiCellRow(t *testing.T) {
	ocr := cbcRowOCR()
	got := LinkOCRLines("单核细胞百分比  10.8  ↑ %  3-10", ocr)
	if len(got) != 4 {
		t.Fatalf("links = %d, want 4: %+v", len(got), got)
	}
	wantLines := []int{1, 2, 3, 4}
	for i, want := range wantLines {
		if got[i].Line != want {
			t.Errorf("link[%d].line = %d, want %d(同一视觉行;不取他行重复碎片/表头)", i, got[i].Line, want)
		}
	}
	if got[2].Box != boxAt(313, 74) {
		t.Errorf("↑ %% 框 = %v, want 本行框(313,74), 非他行 (313,98)", got[2].Box)
	}
}

// TestLinkOCRLinesSecondRow 同一批重复碎片存在时,第二行的重建仍落其自身行。
func TestLinkOCRLinesSecondRow(t *testing.T) {
	got := LinkOCRLines("中性粒细胞百分比  81.3  ↑ %  40-75", cbcRowOCR())
	if len(got) != 4 {
		t.Fatalf("links = %d, want 4", len(got))
	}
	for i, want := range []int{5, 6, 7, 8} {
		if got[i].Line != want {
			t.Errorf("link[%d].line = %d, want %d", i, got[i].Line, want)
		}
	}
}

// TestLinkOCRLinesSingleCell 单格 raw_text(名称行/定性行):精确命中该行。
func TestLinkOCRLinesSingleCell(t *testing.T) {
	got := LinkOCRLines("单核细胞百分比", cbcRowOCR())
	if len(got) != 1 || got[0].Line != 1 {
		t.Fatalf("links = %+v, want [1]", got)
	}
	if got[0].Box != boxAt(51, 73) {
		t.Errorf("box = %v, want (51,73) 行框", got[0].Box)
	}
}

// TestLinkOCRLinesPercentFragment 纯 "%" 片段:表头列头与数据行都含 "%",取与本行
// 其余格 y 聚带一致者(真实嗜酸细胞行形状)。
func TestLinkOCRLinesPercentFragment(t *testing.T) {
	ocr := ocrWith(
		ocrLine{"%", 878, 55},        // 0 表头
		ocrLine{"嗜酸细胞百分比", 51, 124},  // 1
		ocrLine{"0.5", 227, 125},     // 2
		ocrLine{"%", 313, 123},       // 3 数据行
		ocrLine{"0.4-8.0", 403, 124}, // 4
	)
	got := LinkOCRLines("嗜酸细胞百分比  0.5  %  0.4-8.0", ocr)
	if len(got) != 4 {
		t.Fatalf("links = %d, want 4: %+v", len(got), got)
	}
	if got[2].Line != 3 || got[2].Box != boxAt(313, 123) {
		t.Errorf("%% 片段 = line %d box %v, want 数据行 line 3", got[2].Line, got[2].Box)
	}
}

// TestLinkOCRLinesRejectsNonMonotonicCX raw_cells 按 cx 升序 join:右 → 左的序列
// 不可能来自真实重建,必须拒绝(而非返回误链)。
func TestLinkOCRLinesRejectsNonMonotonicCX(t *testing.T) {
	ocr := ocrWith(
		ocrLine{"左", 100, 50},
		ocrLine{"右", 300, 50},
	)
	if got := LinkOCRLines("右  左", ocr); got != nil {
		t.Errorf("links = %+v, want nil(顺序违反 cx 单调)", got)
	}
	got := LinkOCRLines("左  右", ocr)
	if len(got) != 2 || got[0].Line != 0 || got[1].Line != 1 {
		t.Errorf("links = %+v, want [0 1]", got)
	}
}

// TestLinkOCRLinesRejectsPartialMatches 格的边界必须是 "  " 分隔或串首尾:
// 子串命中但分隔不合法不算格。
func TestLinkOCRLinesRejectsPartialMatches(t *testing.T) {
	ocr := ocrWith(ocrLine{"10.8", 227, 76})
	for _, raw := range []string{"10.8x", "x10.8", "10.8 10.8"} {
		if got := LinkOCRLines(raw, ocr); got != nil {
			t.Errorf("LinkOCRLines(%q) = %+v, want nil", raw, got)
		}
	}
	if got := LinkOCRLines("10.8", ocr); len(got) != 1 || got[0].Line != 0 {
		t.Errorf("LinkOCRLines(10.8) = %+v, want [0]", got)
	}
}

// TestLinkOCRLinesDegenerateInputs 空 raw_text / 空 OCR / 空行文本:返回 nil 不崩。
func TestLinkOCRLinesDegenerateInputs(t *testing.T) {
	if got := LinkOCRLines("", cbcRowOCR()); got != nil {
		t.Errorf("empty raw_text = %+v, want nil", got)
	}
	if got := LinkOCRLines("单核细胞百分比", nil); got != nil {
		t.Errorf("nil ocr = %+v, want nil", got)
	}
	ocr := ocrWith(ocrLine{"", 0, 0}, ocrLine{"  ", 10, 10})
	if got := LinkOCRLines("anything", ocr); got != nil {
		t.Errorf("blank lines = %+v, want nil", got)
	}
}

// TestLinkOCRLinesUnmatchable raw_text 与 OCR 行不构成重建(识别端留痕与 OCR 不同源等)
// → nil(退化为无行链接,不猜)。
func TestLinkOCRLinesUnmatchable(t *testing.T) {
	if got := LinkOCRLines("不存在的项  999", cbcRowOCR()); got != nil {
		t.Errorf("links = %+v, want nil", got)
	}
}

// TestLinkOCRLinesManyDuplicatesTerminates 同文本行大量重复(极端输入)仍终止并返回
// 一组合法重建(解空间封顶,退化不悬挂)。
func TestLinkOCRLinesManyDuplicatesTerminates(t *testing.T) {
	ocr := &contract.OCRResult{Engine: "onnxruntime"}
	for i := 0; i < 40; i++ {
		ocr.Txts = append(ocr.Txts, "0")
		ocr.Boxes = append(ocr.Boxes, boxAt(float64(100+i*30), 50))
	}
	got := LinkOCRLines("0  0  0  0", ocr)
	if len(got) != 4 {
		t.Fatalf("links = %d, want 4", len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i].Line <= got[i-1].Line {
			t.Errorf("lines not increasing: %+v", got)
		}
	}
}
