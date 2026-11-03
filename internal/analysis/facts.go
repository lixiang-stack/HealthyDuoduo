// 事实层组装:窗口 + 每趋势键点数上限同时生效;稀疏时向前扩展到 24 个月 / 最多 4 点;
// 有效点必须同成员、同趋势键 + 同 canonical unit、unit_status=verified、flag≠not_tested
// 且有日期、通过信任门控。未纳入项带原因码,不参与任何结论依据。
package analysis

import (
	"cmp"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"healthyduoduo/internal/contract"
)

// BuildFacts 根据已读出的指标点与病历组装事实层。
func BuildFacts(req Request, points []InputPoint, notes []InputNote) Facts {
	months := req.WindowMonths
	if months == 0 {
		months = DefaultWindowMonths
	}
	to := dayString(req.Now)
	from := dayString(req.Now.AddDate(0, -months, 0))
	facts := Facts{
		Entry:                 req.Entry,
		Panel:                 req.Panel,
		ExpandedFromIndicator: req.Entry == EntryIndicator,
		Window:                Window{Months: months, From: from, To: to},
		Reports:               []ReportRef{},
		Notes:                 []Note{},
		Deviations:            []EvidencePoint{},
		History:               []History{},
		Uncomparable:          []Uncomparable{},
		Excluded:              []Exclusion{},
	}
	if req.Entry == EntryIndicator && req.IndicatorName != "" {
		name := req.IndicatorName
		facts.IndicatorName = &name
	}

	var excluded []Exclusion

	groups := make(map[string][]InputPoint)
	var groupOrder []string
	for _, point := range points {
		switch {
		case point.MemberID != req.MemberID:
			excludePoint(&excluded, point, ReasonCrossMember)
		case point.Panel != req.Panel:
			excludePoint(&excluded, point, ReasonWrongPanel)
		case point.ReportStatus == contract.StatusFailed:
			excludePoint(&excluded, point, ReasonFailedReport)
		case contract.IsNarrativeReportType(point.ReportType):
			excludePoint(&excluded, point, ReasonNarrativeReport)
		case contract.TrendKey(point.Code, point.Loinc, point.Name) == "":
			excludePoint(&excluded, point, ReasonNoName)
		default:
			key := contract.TrendKey(point.Code, point.Loinc, point.Name)
			if _, seen := groups[key]; !seen {
				groupOrder = append(groupOrder, key)
			}
			groups[key] = append(groups[key], point)
		}
	}

	var evidencePoints []EvidencePoint
	for _, key := range groupOrder {
		series := buildSeries(req, key, groups[key], from, to, months, &facts.Window, &excluded)
		if series == nil {
			continue
		}
		evidencePoints = append(evidencePoints, series.points...)
		facts.History = append(facts.History, series.history)
		if isDeviation(series.history.Latest.Flag) {
			facts.Deviations = append(facts.Deviations, series.history.Latest)
		}
		if !series.history.Comparable {
			facts.Uncomparable = append(facts.Uncomparable, Uncomparable{
				TrendKey: series.history.TrendKey,
				Name:     series.history.Name,
				Unit:     series.history.Unit,
				Latest:   series.history.Latest,
				Reason:   series.history.Latest.Flag,
			})
		}
	}

	facts.Reports = reportRefs(evidencePoints)
	facts.Notes, excluded = buildNotes(req, notes, facts.Window.From, to, excluded)
	facts.Excluded = normalizeExclusions(excluded)
	// 检验项目全未通过门控时降级为「证据不足」;病历入口本身以病历为证据。
	facts.InsufficientEvidence = len(evidencePoints) == 0 && req.Entry != EntryClinicalNote

	slices.SortStableFunc(facts.History, func(a, b History) int {
		return compareEvidenceDesc(a.Latest, b.Latest)
	})
	slices.SortStableFunc(facts.Deviations, func(a, b EvidencePoint) int {
		return compareEvidenceDesc(a, b)
	})
	slices.SortStableFunc(facts.Uncomparable, func(a, b Uncomparable) int {
		return compareEvidenceDesc(a.Latest, b.Latest)
	})
	return facts
}

// BuildSnapshot 从事实层生成永久快照;有效点集合与病历引用必须与事实层一致。
func BuildSnapshot(req Request, facts Facts) Snapshot {
	name := facts.IndicatorName
	snapshot := Snapshot{
		MemberID:        req.MemberID,
		MemberName:      req.MemberName,
		MemberSex:       req.MemberSex,
		MemberBirthDate: req.MemberBirthDate,
		Panel:           facts.Panel,
		Entry:           facts.Entry,
		IndicatorName:   name,
		WindowMonths:    facts.Window.Months,
		WindowFrom:      facts.Window.From,
		WindowTo:        facts.Window.To,
		RuleVersion:     RuleVersion,
		Points:          []SnapshotPoint{},
		Notes:           []SnapshotNote{},
	}
	for _, history := range facts.History {
		for _, point := range history.Points {
			snapshot.Points = append(snapshot.Points, SnapshotPoint{
				EvidenceID: point.EvidenceID,
				ReportID:   point.ReportID,
				ReportDate: valueOrEmpty(point.ReportDate),
				TrendKey:   point.TrendKey,
				Unit:       point.Unit,
				Value:      point.Value,
				Flag:       point.Flag,
			})
		}
	}
	for _, note := range facts.Notes {
		kinds := make([]string, 0, len(note.Sections))
		for _, section := range note.Sections {
			kinds = append(kinds, section.Kind)
		}
		snapshot.Notes = append(snapshot.Notes, SnapshotNote{
			ReportID:      note.ReportID,
			ReportDate:    note.ReportDate,
			OutsideWindow: note.OutsideWindow,
			SectionKinds:  kinds,
		})
	}
	slices.SortStableFunc(snapshot.Points, func(a, b SnapshotPoint) int {
		if a.ReportDate != b.ReportDate {
			return cmp.Compare(a.ReportDate, b.ReportDate)
		}
		if a.ReportID != b.ReportID {
			return cmp.Compare(a.ReportID, b.ReportID)
		}
		return cmp.Compare(a.EvidenceID, b.EvidenceID)
	})
	return snapshot
}

// EvidenceID 证据点稳定标识:报告短 sha + 指标项 id(快照与解释层引用共用)。
func EvidenceID(reportID string, itemID int64) string {
	return "ev-" + shortID(reportID) + "-" + strconv.FormatInt(itemID, 10)
}

type seriesResult struct {
	points  []EvidencePoint
	history History
}

func buildSeries(req Request, key string, points []InputPoint, from, to string, months int, window *Window, excluded *[]Exclusion) *seriesResult {
	slices.SortStableFunc(points, func(a, b InputPoint) int { return compareInputAsc(a, b) })
	canonicalUnit := canonicalUnit(points)

	var candidates []InputPoint
	for _, point := range points {
		switch {
		case point.ReportDate == "":
			excludePoint(excluded, point, ReasonNoDate)
		case point.Flag == contract.FlagNotTested:
			excludePoint(excluded, point, ReasonNotTested)
		case point.Unit != canonicalUnit:
			excludePoint(excluded, point, ReasonUnitMismatch)
		case point.UnitStatus != contract.UnitStatusVerified:
			excludePoint(excluded, point, ReasonUnverifiedUnit)
		case !passesTrustGate(point):
			excludePoint(excluded, point, ReasonLowConfidence)
		case point.Value == "":
			excludePoint(excluded, point, ReasonMissingValue)
		default:
			candidates = append(candidates, point)
		}
	}
	if len(candidates) == 0 {
		return nil
	}

	inWindow := countInWindow(candidates, from, to)
	sparse := months == DefaultWindowMonths && inWindow < 2
	effectiveFrom := from
	capPoints := MaxPointsPerSeries
	if sparse {
		effectiveFrom = dayString(req.Now.AddDate(0, -MaxWindowMonths, 0))
		capPoints = SparseMaxPoints
		window.Sparse = true
	}

	var inRange []InputPoint
	for _, point := range candidates {
		if point.ReportDate < effectiveFrom || point.ReportDate > to {
			excludePoint(excluded, point, ReasonOutsideWindow)
			continue
		}
		inRange = append(inRange, point)
	}
	slices.SortStableFunc(inRange, func(a, b InputPoint) int { return compareInputDesc(a, b) })
	kept := inRange
	if len(kept) > capPoints {
		for _, point := range kept[capPoints:] {
			excludePoint(excluded, point, ReasonPointCap)
		}
		kept = kept[:capPoints]
	}
	if len(kept) == 0 {
		return nil
	}
	slices.SortStableFunc(kept, func(a, b InputPoint) int { return compareInputAsc(a, b) })

	evidence := make([]EvidencePoint, 0, len(kept))
	for _, point := range kept {
		evidence = append(evidence, evidencePoint(point))
	}
	history := buildHistory(key, evidence, sparse)
	return &seriesResult{points: evidence, history: history}
}

func buildHistory(key string, points []EvidencePoint, sparse bool) History {
	latest := points[len(points)-1]
	history := History{
		TrendKey:    key,
		Name:        latest.Name,
		Unit:        latest.Unit,
		UnitDisplay: latest.UnitDisplay,
		Points:      points,
		Latest:      latest,
		Change:      ChangeUnknown,
		PointCount:  len(points),
		Sparse:      sparse,
		Comparable:  latest.Flag != contract.FlagUnknown,
	}
	if len(points) > 1 {
		previous := points[len(points)-2]
		history.Previous = &previous
		history.Change = changeDirection(previous.Value, latest.Value)
	}
	latestValue, ok := numericValue(latest.Value)
	if ok {
		highest, lowest := true, true
		for _, point := range points {
			value, ok := numericValue(point.Value)
			if !ok {
				continue
			}
			if value > latestValue {
				highest = false
			}
			if value < latestValue {
				lowest = false
			}
		}
		history.Highest = highest
		history.Lowest = lowest
	}
	return history
}

func buildNotes(req Request, notes []InputNote, from, to string, excluded []Exclusion) ([]Note, []Exclusion) {
	type candidate struct {
		note    InputNote
		date    string
		outside bool
	}
	var inWindow []candidate
	var outside []candidate
	var selected []candidate
	for _, note := range notes {
		if note.MemberID != req.MemberID {
			excluded = append(excluded, Exclusion{
				ReportID: nonEmptyPtr(note.ReportID),
				Reason:   ReasonCrossMember,
			})
			continue
		}
		if len(noteSections(note.Sections)) == 0 {
			excluded = append(excluded, Exclusion{
				ReportID:   nonEmptyPtr(note.ReportID),
				ReportDate: nonEmptyPtr(note.ReportDate),
				Reason:     ReasonNoNoteBody,
			})
			continue
		}
		if note.ReportDate == "" {
			// 病历入口显式选中的无日期病历仍要展示;其他入口无法按窗口定位。
			if req.Entry == EntryClinicalNote && req.ReportSHA == note.ReportID {
				selected = append(selected, candidate{note: note})
				continue
			}
			excluded = append(excluded, Exclusion{
				ReportID: nonEmptyPtr(note.ReportID),
				Reason:   ReasonNoDate,
			})
			continue
		}
		switch {
		case note.ReportDate >= from && note.ReportDate <= to:
			inWindow = append(inWindow, candidate{note: note, date: note.ReportDate})
		case note.ReportDate < from:
			outside = append(outside, candidate{note: note, date: note.ReportDate, outside: true})
		default:
			excluded = append(excluded, Exclusion{
				ReportID:   nonEmptyPtr(note.ReportID),
				ReportDate: nonEmptyPtr(note.ReportDate),
				Reason:     ReasonOutsideWindow,
			})
		}
	}
	slices.SortStableFunc(inWindow, func(a, b candidate) int { return cmp.Compare(b.date, a.date) })
	slices.SortStableFunc(outside, func(a, b candidate) int { return cmp.Compare(b.date, a.date) })

	var kept []candidate
	kept = append(kept, selected...)
	kept = append(kept, inWindow...)
	// 窗口外只保留最近一次相关病历,并标「早于分析窗口」;其余排除。
	if req.Entry == EntryClinicalNote && req.ReportSHA != "" {
		for _, item := range outside {
			if item.note.ReportID == req.ReportSHA {
				kept = append(kept, item)
				outside = slices.DeleteFunc(outside, func(c candidate) bool { return c.note.ReportID == item.note.ReportID })
				break
			}
		}
	}
	if len(outside) > 0 {
		kept = append(kept, outside[0])
		for _, item := range outside[1:] {
			excluded = append(excluded, Exclusion{
				ReportID:   nonEmptyPtr(item.note.ReportID),
				ReportDate: nonEmptyPtr(item.note.ReportDate),
				Reason:     ReasonOutsideWindow,
			})
		}
	}

	notesOut := make([]Note, 0, len(kept))
	for _, item := range kept {
		notesOut = append(notesOut, Note{
			ReportID:      item.note.ReportID,
			ReportDate:    nonEmptyPtr(item.note.ReportDate),
			OutsideWindow: item.outside,
			Sections:      noteSections(item.note.Sections),
		})
	}
	return notesOut, excluded
}

// noteSections 只把诊断 / 处理原文带入事实层;切不出段落时 note 是整页兜底。
func noteSections(sections []InputSection) []NoteSection {
	var diagnosis, treatment, fallback []NoteSection
	for _, section := range sections {
		if strings.TrimSpace(section.Text) == "" {
			continue
		}
		item := NoteSection{Kind: section.Kind, Text: section.Text}
		switch section.Kind {
		case contract.SectionKindDiagnosis:
			diagnosis = append(diagnosis, item)
		case contract.SectionKindTreatment:
			treatment = append(treatment, item)
		case contract.SectionKindNote:
			fallback = append(fallback, item)
		}
	}
	out := append(diagnosis, treatment...)
	if len(out) == 0 {
		return fallback
	}
	return out
}

func reportRefs(points []EvidencePoint) []ReportRef {
	seen := make(map[string]ReportRef)
	for _, point := range points {
		if _, ok := seen[point.ReportID]; ok {
			continue
		}
		seen[point.ReportID] = ReportRef{
			ReportID:   point.ReportID,
			ReportDate: point.ReportDate,
			ReportType: point.ReportType,
		}
	}
	out := make([]ReportRef, 0, len(seen))
	for _, ref := range seen {
		out = append(out, ref)
	}
	slices.SortStableFunc(out, func(a, b ReportRef) int {
		return compareReportRef(a, b)
	})
	return out
}

func compareReportRef(a, b ReportRef) int {
	ad, bd := valueOrEmpty(a.ReportDate), valueOrEmpty(b.ReportDate)
	if ad != bd {
		if ad == "" {
			return 1
		}
		if bd == "" {
			return -1
		}
		return cmp.Compare(bd, ad)
	}
	return cmp.Compare(a.ReportID, b.ReportID)
}

func normalizeExclusions(excluded []Exclusion) []Exclusion {
	if excluded == nil {
		return []Exclusion{}
	}
	slices.SortStableFunc(excluded, func(a, b Exclusion) int {
		if a.Reason != b.Reason {
			return cmp.Compare(a.Reason, b.Reason)
		}
		ad, bd := valueOrEmpty(a.ReportDate), valueOrEmpty(b.ReportDate)
		if ad != bd {
			return cmp.Compare(bd, ad)
		}
		return cmp.Compare(valueOrEmpty(a.ReportID), valueOrEmpty(b.ReportID))
	})
	return excluded
}

func evidencePoint(point InputPoint) EvidencePoint {
	return EvidencePoint{
		EvidenceID:  EvidenceID(point.ReportID, point.ItemID),
		ReportID:    point.ReportID,
		ReportDate:  nonEmptyPtr(point.ReportDate),
		ReportType:  point.ReportType,
		TrendKey:    contract.TrendKey(point.Code, point.Loinc, point.Name),
		Name:        point.Name,
		Unit:        nonEmptyPtr(point.Unit),
		UnitDisplay: nonEmptyPtr(point.UnitDisplay),
		Value:       nonEmptyPtr(point.Value),
		RefRange:    nonEmptyPtr(point.RefRange),
		Flag:        point.Flag,
	}
}

func canonicalUnit(points []InputPoint) string {
	// 优先取最近一个通过基础门控(有日期 / 非未检测 / 单位已验证)的点,
	// 避免把某个未验证单位当成整条趋势的 canonical unit。
	for i := len(points) - 1; i >= 0; i-- {
		point := points[i]
		if point.ReportDate == "" || point.Flag == contract.FlagNotTested {
			continue
		}
		if point.UnitStatus == contract.UnitStatusVerified && point.Unit != "" {
			return point.Unit
		}
	}
	for i := len(points) - 1; i >= 0; i-- {
		if points[i].Unit != "" {
			return points[i].Unit
		}
	}
	return ""
}

func countInWindow(points []InputPoint, from, to string) int {
	count := 0
	for _, point := range points {
		if point.ReportDate >= from && point.ReportDate <= to {
			count++
		}
	}
	return count
}

func passesTrustGate(point InputPoint) bool {
	if point.ReviewStatus == contract.ReviewStatusConfirmed || point.ReviewStatus == contract.ReviewStatusCorrected {
		return true
	}
	return !point.LowConfidence
}

func isDeviation(flag string) bool {
	switch flag {
	case contract.FlagHigh, contract.FlagLow, contract.FlagAbnormal:
		return true
	default:
		return false
	}
}

func excludePoint(excluded *[]Exclusion, point InputPoint, reason string) {
	*excluded = append(*excluded, Exclusion{
		ReportID:   nonEmptyPtr(point.ReportID),
		ReportDate: nonEmptyPtr(point.ReportDate),
		TrendKey:   nonEmptyPtr(contract.TrendKey(point.Code, point.Loinc, point.Name)),
		Name:       nonEmptyPtr(point.Name),
		Reason:     reason,
	})
}

func changeDirection(previous, latest *string) string {
	prev, okPrev := numericValue(previous)
	next, okNext := numericValue(latest)
	if !okPrev || !okNext {
		return ChangeUnknown
	}
	switch {
	case next > prev:
		return ChangeUp
	case next < prev:
		return ChangeDown
	default:
		return ChangeFlat
	}
}

func numericValue(value *string) (float64, bool) {
	if value == nil {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(*value), 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0, false
	}
	return parsed, true
}

func compareInputAsc(a, b InputPoint) int {
	ad, bd := a.ReportDate, b.ReportDate
	if ad != bd {
		if ad == "" {
			return 1
		}
		if bd == "" {
			return -1
		}
		return cmp.Compare(ad, bd)
	}
	if a.ReportID != b.ReportID {
		return cmp.Compare(a.ReportID, b.ReportID)
	}
	return cmp.Compare(a.ItemID, b.ItemID)
}

func compareInputDesc(a, b InputPoint) int { return compareInputAsc(b, a) }

func compareEvidenceDesc(a, b EvidencePoint) int {
	ad, bd := valueOrEmpty(a.ReportDate), valueOrEmpty(b.ReportDate)
	if ad != bd {
		return cmp.Compare(bd, ad)
	}
	return cmp.Compare(a.Name, b.Name)
}

func nonEmptyPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func dayString(t time.Time) string {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02")
}

func shortID(reportID string) string {
	if len(reportID) > 12 {
		return reportID[:12]
	}
	return reportID
}
