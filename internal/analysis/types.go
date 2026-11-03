// Package analysis 是 LLM 分析的纯领域层(llm-analysis 票 04):有效点筛选、回溯窗口、
// 稀疏扩展、事实层组装与快照。不依赖存储 / HTTP / 模型;输入是已读出的指标点与病历段落,
// 输出只陈述数据与病历原文,不做医学判断。
package analysis

import "time"

// 分析入口:项目详情页 / 单指标 / 病历详情。
const (
	EntryPanel        = "panel"
	EntryIndicator    = "indicator"
	EntryClinicalNote = "clinical_note"
)

// 分析状态:事实层正常 / 没有任何有效点时的降级。
const (
	StatusSuccess              = "success"
	StatusInsufficientEvidence = "insufficient_evidence"
)

// RuleVersion 事实层 + 框架层规则版本,随快照永久记录。
const RuleVersion = "llm-analysis-v3"

// 默认 / 稀疏扩展参数(spec §4.9)。
const (
	DefaultWindowMonths = 12
	MaxWindowMonths     = 24
	MaxPointsPerSeries  = 6
	SparseMaxPoints     = 4
)

// 排除原因码(spec §4.9 默认排除项 + 数据质量门)。
const (
	ReasonCrossMember     = "cross_member"
	ReasonWrongPanel      = "wrong_panel"
	ReasonFailedReport    = "failed_report"
	ReasonNarrativeReport = "narrative_report"
	ReasonNoDate          = "no_date"
	ReasonNotTested       = "not_tested"
	ReasonUnverifiedUnit  = "unverified_unit"
	ReasonLowConfidence   = "low_confidence"
	ReasonMissingValue    = "missing_value"
	ReasonUnitMismatch    = "unit_mismatch"
	ReasonOutsideWindow   = "outside_window"
	ReasonPointCap        = "point_cap"
	ReasonNoName          = "no_name"
	ReasonNoNoteBody      = "no_note_body"
)

// 历史变化方向。
const (
	ChangeUp      = "up"
	ChangeDown    = "down"
	ChangeFlat    = "flat"
	ChangeUnknown = "unknown"
)

// Request 分析入参;WindowMonths 为 0 时取默认 12 个月。
// MemberSex / MemberBirthDate 只用于框架层的年龄性别规则,不进入事实层。
type Request struct {
	MemberID        int64
	MemberName      string
	MemberSex       string
	MemberBirthDate string
	Panel           string
	Entry           string
	IndicatorName   string
	ReportSHA       string
	WindowMonths    int
	Now             time.Time
}

// InputPoint 存储侧读出的一条指标点(尚未经过窗口 / 门控)。
type InputPoint struct {
	MemberID      int64
	ReportID      string
	ReportDate    string // YYYY-MM-DD;空串 = 无日期
	ReportType    string
	ReportStatus  string // success | partial | failed
	Panel         string
	ItemID        int64
	Name          string
	Code          string
	Loinc         string
	Analyte       string
	Unit          string
	UnitDisplay   string
	UnitStatus    string // verified | unverified | 空串(NULL)
	Value         string
	RefRange      string
	Flag          string
	LowConfidence bool
	ReviewStatus  string
}

// InputGrowth 最近一次生长记录(框架层 BMI / 百分位用);Source 为 measured / report。
type InputGrowth struct {
	MeasuredOn string
	HeightCm   *float64
	WeightKg   *float64
	Source     string
}

// InputNote 存储侧读出的一份病历报告。
type InputNote struct {
	MemberID   int64
	ReportID   string
	ReportDate string
	ReportType string
	Sections   []InputSection
}

// InputSection 病历段落原文(只保留诊断 / 处理 / note 兜底进入事实层)。
type InputSection struct {
	Kind string
	Text string
}

// Facts 事实层输出。
type Facts struct {
	Entry                 string          `json:"entry"`
	Panel                 string          `json:"panel"`
	IndicatorName         *string         `json:"indicator_name"`
	ExpandedFromIndicator bool            `json:"expanded_from_indicator"`
	Window                Window          `json:"window"`
	Reports               []ReportRef     `json:"reports"`
	Notes                 []Note          `json:"notes"`
	Deviations            []EvidencePoint `json:"deviations"`
	History               []History       `json:"history"`
	Uncomparable          []Uncomparable  `json:"uncomparable"`
	Excluded              []Exclusion     `json:"excluded"`
	InsufficientEvidence  bool            `json:"insufficient_evidence"`
}

// Window 本次回溯窗口(锚点 = 分析时刻当日)。
type Window struct {
	Months int    `json:"months"`
	From   string `json:"from"`
	To     string `json:"to"`
	Sparse bool   `json:"sparse"`
}

// ReportRef 纳入的检验报告。
type ReportRef struct {
	ReportID   string  `json:"report_id"`
	ReportDate *string `json:"report_date"`
	ReportType string  `json:"report_type"`
}

// Note 纳入的病历报告;OutsideWindow 标记「早于分析窗口」的背景病历。
type Note struct {
	ReportID      string        `json:"report_id"`
	ReportDate    *string       `json:"report_date"`
	OutsideWindow bool          `json:"outside_window"`
	Sections      []NoteSection `json:"sections"`
}

// NoteSection 病历诊断 / 处理原文;切不出段落时 note 为整页兜底。
type NoteSection struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// EvidencePoint 事实层的一个证据点;每个数值都绑定报告日期、趋势键、单位与 flag。
type EvidencePoint struct {
	EvidenceID  string  `json:"evidence_id"`
	ReportID    string  `json:"report_id"`
	ReportDate  *string `json:"report_date"`
	ReportType  string  `json:"report_type"`
	TrendKey    string  `json:"trend_key"`
	Analyte     string  `json:"-"`
	Name        string  `json:"name"`
	Unit        *string `json:"unit"`
	UnitDisplay *string `json:"unit_display"`
	Value       *string `json:"value"`
	RefRange    *string `json:"ref_range"`
	Flag        string  `json:"flag"`
}

// History 与本人历史的比较(同趋势键 + 同 canonical unit)。
type History struct {
	TrendKey    string          `json:"trend_key"`
	Name        string          `json:"name"`
	Unit        *string         `json:"unit"`
	UnitDisplay *string         `json:"unit_display"`
	Points      []EvidencePoint `json:"points"`
	Latest      EvidencePoint   `json:"latest"`
	Previous    *EvidencePoint  `json:"previous"`
	Change      string          `json:"change"`
	Highest     bool            `json:"highest"`
	Lowest      bool            `json:"lowest"`
	PointCount  int             `json:"point_count"`
	Sparse      bool            `json:"sparse"`
	Comparable  bool            `json:"comparable"`
}

// Uncomparable 无法与参考范围比较的最近点(flag=unknown),不构成偏离证据。
type Uncomparable struct {
	TrendKey string        `json:"trend_key"`
	Name     string        `json:"name"`
	Unit     *string       `json:"unit"`
	Latest   EvidencePoint `json:"latest"`
	Reason   string        `json:"reason"`
}

// Exclusion 未纳入项及原因。
type Exclusion struct {
	ReportID   *string `json:"report_id"`
	ReportDate *string `json:"report_date"`
	TrendKey   *string `json:"trend_key"`
	Name       *string `json:"name"`
	Reason     string  `json:"reason"`
}

// Framework 框架层输出:代码算得出的判读标签 + 固定解释文案。
// 没有可用规则时 labels 为空数组,不输出空状态。
type Framework struct {
	Labels []FrameworkLabel `json:"labels"`
}

// FrameworkLabel 一个判读标签;Value 为计算值展示,EvidenceIDs 绑定事实层证据点,
// Sources 由代码渲染标准 / 指南来源。
type FrameworkLabel struct {
	Code        string   `json:"code"`
	Label       string   `json:"label"`
	Explanation string   `json:"explanation"`
	Value       *string  `json:"value"`
	EvidenceIDs []string `json:"evidence_ids"`
	Sources     []Source `json:"sources"`
}

// 知识条目审核决策(spec §4.6);只有 accepted 条目可被模型引用。
const ReviewDecisionAccepted = "accepted"

// KnowledgeReview 条目审核记录;单人审核,只记 decision / reviewed_at / note。
type KnowledgeReview struct {
	Decision   string
	ReviewedAt string
	Note       string
}

// KnowledgeEntry 已匹配的知识条目;医学原文由服务端从条目拼接,模型只写 bridge_text。
type KnowledgeEntry struct {
	ID              string
	Version         string
	Review          KnowledgeReview
	Explanation     string
	CheckDirections []string
	GeneralAdvice   []string
	TrackItems      []string
	Sources         []Source
}

// Explanation 解释层输出;07 只产出 bridge_text 与引用,四段式医学原文由 08 补齐。
// Truncated = 模型输出达到 max_tokens 被截断,结果页需标注。
type Explanation struct {
	Sections  []ExplanationSection `json:"sections"`
	Truncated bool                 `json:"truncated"`
}

// ExplanationSection 一条解释;EntryID / EvidenceIDs / BridgeText 来自模型并经服务端校验。
// 四段式医学原文与「查看依据」由工单 08 追加;本工单只锁定引用与来源。
type ExplanationSection struct {
	EntryID     string   `json:"entry_id"`
	EvidenceIDs []string `json:"evidence_ids"`
	BridgeText  string   `json:"bridge_text"`
	Sources     []Source `json:"source_ids,omitempty"`
}

// Snapshot 永久保留的分析快照:成员、Panel、窗口、有效点集合、病历引用与规则版本。
// 成员性别 / 生日用于解释框架层输入,旧快照缺字段时省略。
type Snapshot struct {
	MemberID        int64             `json:"member_id"`
	MemberName      string            `json:"member_name"`
	MemberSex       string            `json:"member_sex,omitempty"`
	MemberBirthDate string            `json:"member_birth_date,omitempty"`
	Panel           string            `json:"panel"`
	Entry           string            `json:"entry"`
	IndicatorName   *string           `json:"indicator_name"`
	WindowMonths    int               `json:"window_months"`
	WindowFrom      string            `json:"window_from"`
	WindowTo        string            `json:"window_to"`
	RuleVersion     string            `json:"rule_version"`
	PromptVersion   string            `json:"prompt_version,omitempty"` // 解释层 prompt 版本;随快照记录
	EntryVersions   map[string]string `json:"entry_versions,omitempty"` // 条目 id → version;08 接条目后写入
	DataVersion     string            `json:"data_version"`
	Points          []SnapshotPoint   `json:"points"`
	Notes           []SnapshotNote    `json:"notes"`
}

// SnapshotPoint 快照中的有效点。
type SnapshotPoint struct {
	EvidenceID string  `json:"evidence_id"`
	ReportID   string  `json:"report_id"`
	ReportDate string  `json:"report_date"`
	TrendKey   string  `json:"trend_key"`
	Unit       *string `json:"unit"`
	Value      *string `json:"value"`
	Flag       string  `json:"flag"`
}

// SnapshotNote 快照中的病历引用。
type SnapshotNote struct {
	ReportID      string   `json:"report_id"`
	ReportDate    *string  `json:"report_date"`
	OutsideWindow bool     `json:"outside_window"`
	SectionKinds  []string `json:"section_kinds"`
}
