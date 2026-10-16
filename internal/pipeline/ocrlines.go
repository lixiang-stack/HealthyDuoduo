// OCR 行链接(票 07 详情纵切):指标项 raw_text 反推组成它的 OCR 行(行下标 + 框坐标),
// 供客户端「图像区域 ↔ 原始 OCR 行文 ↔ 解析值」三方对照。
//
// 口径来源:识别端 postprocess 的 raw_text = 该指标项各组成格 c.text.strip() 以
// 两个空格 join、格按 cx(中心 x)升序排列;格与 OCR 行一一对应(txts/boxes 同下标)。
// 故重建 = 把 raw_text 在 "  " 分隔边界上切回行文本序列;文本层面对纯 "%" / "↑ %"
// 等重复碎片有天然歧义,再以几何约束消解:cx 必须非降(cells 按 cx 排序)、
// 且同一视觉行的各格 y 中心聚拢(取 y 展开最小者,平局取 y 偏差和最小者)。
// 真实报告(共享栈 5 份 / 89 个指标项)验证:全部可重建且消歧唯一;单测见
// ocrlines_test.go。
//
// 重建失败(如识别端改规则导致 raw_text 与 OCR 不同源)→ nil:退化为无行链接,
// 不猜、不返回半截链。
package pipeline

import (
	"math"
	"strings"

	"healthyduoduo/internal/contract"
)

// OCRLineLink 指标项与一条 OCR 行的链接(行下标 0-based,与 ocr_result 的
// txts/boxes 下标一致;Box 为该行四点多边形)。
type OCRLineLink struct {
	Line int
	Box  contract.Box
}

const (
	// rawTextSep 识别端 postprocess 的格间分隔(两个空格)。
	rawTextSep = "  "
	// cxTolerance cx 非降比较的浮点容差(坐标量纲为像素)。
	cxTolerance = 1.0
	// maxLinkSolutions 解空间封顶:同文本碎片大量重复时完整解可能指数级;
	// 达到上限即停止枚举(保留当前最优解)。真实报告远低于上限。
	maxLinkSolutions = 4096
)

// ocrLineRef 候选 OCR 行(带几何量)。
type ocrLineRef struct {
	index  int
	text   string
	cx, cy float64
}

// LinkOCRLines 把 raw_text 重建为其组成 OCR 行(顺序 = raw_text 内出现顺序);
// 无法重建时返回 nil。
func LinkOCRLines(rawText string, ocr *contract.OCRResult) []OCRLineLink {
	if rawText == "" || ocr == nil {
		return nil
	}
	n := len(ocr.Txts)
	if len(ocr.Boxes) < n {
		n = len(ocr.Boxes)
	}
	lines := make([]ocrLineRef, 0, n)
	for i := 0; i < n; i++ {
		text := strings.TrimSpace(ocr.Txts[i])
		if text == "" {
			continue
		}
		cx, cy := boxCenter(ocr.Boxes[i])
		lines = append(lines, ocrLineRef{index: i, text: text, cx: cx, cy: cy})
	}
	if len(lines) == 0 {
		return nil
	}

	var best []ocrLineRef
	bestSpread, bestDev := math.Inf(1), math.Inf(1)
	solutions := 0

	var walk func(pos int, acc []ocrLineRef)
	walk = func(pos int, acc []ocrLineRef) {
		if solutions >= maxLinkSolutions {
			return
		}
		if pos == len(rawText) {
			solutions++
			spread, dev := ySpread(acc)
			if spread < bestSpread || (spread == bestSpread && dev < bestDev) {
				best, bestSpread, bestDev = append([]ocrLineRef(nil), acc...), spread, dev
			}
			return
		}
		remaining := rawText[pos:]
		for _, cand := range lines {
			if !strings.HasPrefix(remaining, cand.text) {
				continue
			}
			// 格的边界必须是 raw_text 串首尾或 "  " 分隔。
			end := pos + len(cand.text)
			if end != len(rawText) && !strings.HasPrefix(rawText[end:], rawTextSep) {
				continue
			}
			// raw_cells 按 cx 升序排列:违反非降的序列不可能来自真实重建。
			if len(acc) > 0 && cand.cx < acc[len(acc)-1].cx-cxTolerance {
				continue
			}
			// 一行只归属一个格(同格不得重复消费)。
			reused := false
			for _, a := range acc {
				if a.index == cand.index {
					reused = true
					break
				}
			}
			if reused {
				continue
			}
			next := end
			if next != len(rawText) {
				next += len(rawTextSep)
			}
			walk(next, append(acc, cand))
		}
	}
	walk(0, nil)

	if best == nil {
		return nil
	}
	links := make([]OCRLineLink, 0, len(best))
	for _, l := range best {
		links = append(links, OCRLineLink{Line: l.index, Box: ocr.Boxes[l.index]})
	}
	return links
}

// boxCenter 行的几何中心(四顶点均值;y 聚带消歧与 cx 排序都用同一口径)。
func boxCenter(b contract.Box) (cx, cy float64) {
	var sx, sy float64
	for _, p := range b {
		sx += p[0]
		sy += p[1]
	}
	return sx / 4, sy / 4
}

// ySpread 解的几何一致性:y 中心展开(主)与相对首格的 y 偏差和(平局判据)。
func ySpread(acc []ocrLineRef) (spread, dev float64) {
	minY, maxY := acc[0].cy, acc[0].cy
	for _, l := range acc {
		if l.cy < minY {
			minY = l.cy
		}
		if l.cy > maxY {
			maxY = l.cy
		}
		dev += math.Abs(l.cy - acc[0].cy)
	}
	return maxY - minY, dev
}
