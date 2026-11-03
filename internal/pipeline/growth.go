// 生长记录域规则(llm-analysis 票 02):身高体重是带测量日期的测量值,不是成员静态属性。
// BMI 用测量时点的身高体重实时计算,不缓存;手输来源只给 BMI 数值,不输出超重 / 肥胖判定。
package pipeline

import (
	"errors"
	"math"
	"time"
)

// 生长记录来源枚举:实测 / 体检单 / 手输。手输数据不参与超重 / 肥胖判定。
const (
	GrowthSourceMeasured = "measured"
	GrowthSourceReport   = "report"
	GrowthSourceManual   = "manual"
)

// bmiDisplayScale BMI 展示保留一位小数(计算本身不落库)。
const bmiDisplayScale = 10

var (
	// ErrGrowthMeasuredOnInvalid 测量日期不是合法的 YYYY-MM-DD,或晚于今天。
	ErrGrowthMeasuredOnInvalid = errors.New("growth measured_on must be a valid date not in the future")
	// ErrGrowthSourceInvalid 来源不在 measured/report/manual 内。
	ErrGrowthSourceInvalid = errors.New("growth source must be one of measured, report, manual")
	// ErrGrowthMeasurementMissing 身高与体重至少填一项。
	ErrGrowthMeasurementMissing = errors.New("growth requires at least one of height_cm or weight_kg")
	// ErrGrowthMeasurementInvalid 身高或体重不是正数。
	ErrGrowthMeasurementInvalid = errors.New("growth height_cm and weight_kg must be positive numbers")
)

// GrowthProfile 新增生长记录的可写资料;身高体重至少一项。
type GrowthProfile struct {
	MeasuredOn string
	HeightCm   *float64
	WeightKg   *float64
	Source     string
}

// GrowthRecord 生长记录业务视图;BMI 由测量时点的身高体重实时计算。
// WeightStatus 预留给后续百分位规则:手输来源永远为 nil。
type GrowthRecord struct {
	GrowthID     int64
	MemberID     int64
	MeasuredOn   string
	HeightCm     *float64
	WeightKg     *float64
	Source       string
	BMI          *float64
	WeightStatus *string
	CreatedAt    time.Time
}

// ValidateGrowthProfile 新增生长记录的资料校验(日期 / 来源 / 至少一项测量值 / 正数)。
// now 用于拒绝未来日期;调用方传当前时刻。
func ValidateGrowthProfile(p GrowthProfile, now time.Time) error {
	measured, err := time.Parse(birthDateLayout, p.MeasuredOn)
	if err != nil || measured.After(dayStart(now)) {
		return ErrGrowthMeasuredOnInvalid
	}
	if err := ValidateGrowthSource(p.Source); err != nil {
		return err
	}
	if p.HeightCm == nil && p.WeightKg == nil {
		return ErrGrowthMeasurementMissing
	}
	if !validMeasurement(p.HeightCm) || !validMeasurement(p.WeightKg) {
		return ErrGrowthMeasurementInvalid
	}
	return nil
}

// ValidateGrowthSource 来源枚举规则(HTTP 边界与存储镜像共用)。
func ValidateGrowthSource(source string) error {
	switch source {
	case GrowthSourceMeasured, GrowthSourceReport, GrowthSourceManual:
		return nil
	default:
		return ErrGrowthSourceInvalid
	}
}

func validMeasurement(value *float64) bool {
	if value == nil {
		return true
	}
	return *value > 0 && !math.IsNaN(*value) && !math.IsInf(*value, 0)
}

func dayStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// BMI 用测量时点的身高体重计算,保留一位小数;缺任一项返回 nil。
// 公式:体重(kg) / 身高(m)²。
func BMI(heightCm, weightKg *float64) *float64 {
	if heightCm == nil || weightKg == nil || *heightCm <= 0 {
		return nil
	}
	heightM := *heightCm / 100
	value := *weightKg / (heightM * heightM)
	rounded := math.Round(value*bmiDisplayScale) / bmiDisplayScale
	return &rounded
}
