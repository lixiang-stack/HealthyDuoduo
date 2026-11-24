// Package care 是护理事件的纯领域层（care-events 票 01）：四类事件的类型常量、
// 输入 / 输出形状与校验，不依赖存储 / HTTP。事件只归属一个孩子成员，来源固定
// 家长记录，没有记录人 / 设备 / 全局 active child 字段。
package care

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

// 事件类型（契约判别字段）。
const (
	TypeFeed   = "feed"
	TypeDiaper = "diaper"
	TypeSleep  = "sleep"
	TypeSolid  = "solid"
)

// SourceParent 是唯一来源；由服务端固定，不接收客户端输入。
const SourceParent = "parent"

// 类型专属枚举。
const (
	FeedMethodBreast  = "breast"
	FeedMethodFormula = "formula"

	DiaperTypeWet   = "wet"
	DiaperTypeDirty = "dirty"
	DiaperTypeMixed = "mixed"
)

var (
	// ErrTypeInvalid 事件类型不在 feed / diaper / sleep / solid 内。
	ErrTypeInvalid = errors.New("care event type must be one of feed, diaper, sleep, solid")
	// ErrStartedAtRequired started_at 缺失（零值）。
	ErrStartedAtRequired = errors.New("care event started_at is required")
	// ErrEndedAtBeforeStarted ended_at 早于 started_at。
	ErrEndedAtBeforeStarted = errors.New("care event ended_at must not be before started_at")
	// ErrFieldsInvalid 类型专属字段与 type 不匹配（缺失、多余或枚举非法）。
	ErrFieldsInvalid = errors.New("care event fields do not match its type")
	// ErrValueInvalid amount_ml / amount_g / duration_min 存在但非正数。
	ErrValueInvalid = errors.New("care event amount_ml, amount_g, and duration_min must be positive")
)

// CareEvent 已持久化的护理事件；Source 由存储固定为 parent。
type CareEvent struct {
	ID          int64
	MemberID    int64
	Type        string
	Source      string
	StartedAt   time.Time
	EndedAt     *time.Time
	Note        *string
	Method      *string
	AmountML    *float64
	DurationMin *int
	DiaperType  *string
	Food        *string
	AmountG     *float64
}

// EventInput 创建 / 修改的可编辑内容；id / member_id / source 由服务端与存储维护。
type EventInput struct {
	Type        string
	StartedAt   time.Time
	EndedAt     *time.Time
	Note        *string
	Method      *string
	AmountML    *float64
	DurationMin *int
	DiaperType  *string
	Food        *string
	AmountG     *float64
}

// Normalize 清理文本：note 去首尾空白、空串转 nil；food 只在 solid 时处理，
// 其余类型保留原值，让 Validate 按「类型专属字段必须为 null」拒绝。
func Normalize(input EventInput) EventInput {
	input.Note = trimmedOrNil(input.Note)
	if input.Type == TypeSolid {
		input.Food = trimmedOrNil(input.Food)
	}
	return input
}

func trimmedOrNil(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// Validate 校验规范化后的输入；HTTP 边界与存储层共用（成员 is_child 由 HTTP 边界另校验）。
func Validate(input EventInput) error {
	switch input.Type {
	case TypeFeed, TypeDiaper, TypeSleep, TypeSolid:
	default:
		return ErrTypeInvalid
	}
	if input.StartedAt.IsZero() {
		return ErrStartedAtRequired
	}
	if input.EndedAt != nil && input.EndedAt.Before(input.StartedAt) {
		return ErrEndedAtBeforeStarted
	}
	if !positiveOrNil(input.AmountML) || !positiveOrNil(input.AmountG) ||
		!positiveDuration(input.DurationMin) {
		return ErrValueInvalid
	}
	return validateTypeFields(input)
}

func positiveOrNil(value *float64) bool {
	if value == nil {
		return true
	}
	return *value > 0 && !math.IsNaN(*value) && !math.IsInf(*value, 0)
}

func positiveDuration(value *int) bool {
	return value == nil || *value > 0
}

func validateTypeFields(input EventInput) error {
	switch input.Type {
	case TypeFeed:
		if input.Method == nil ||
			(*input.Method != FeedMethodBreast && *input.Method != FeedMethodFormula) {
			return fmt.Errorf("%w: feed requires method breast or formula", ErrFieldsInvalid)
		}
		if input.EndedAt != nil || input.DiaperType != nil || input.Food != nil || input.AmountG != nil {
			return fmt.Errorf("%w: feed cannot set ended_at, diaper_type, food, or amount_g", ErrFieldsInvalid)
		}
	case TypeDiaper:
		if input.DiaperType == nil ||
			(*input.DiaperType != DiaperTypeWet && *input.DiaperType != DiaperTypeDirty &&
				*input.DiaperType != DiaperTypeMixed) {
			return fmt.Errorf("%w: diaper requires diaper_type wet, dirty, or mixed", ErrFieldsInvalid)
		}
		if input.EndedAt != nil || input.Method != nil || input.AmountML != nil ||
			input.DurationMin != nil || input.Food != nil || input.AmountG != nil {
			return fmt.Errorf("%w: diaper cannot set ended_at, method, amount_ml, duration_min, food, or amount_g", ErrFieldsInvalid)
		}
	case TypeSleep:
		if (input.EndedAt == nil) != (input.DurationMin == nil) {
			return fmt.Errorf("%w: sleep requires ended_at and duration_min both set or both null", ErrFieldsInvalid)
		}
		if input.Method != nil || input.AmountML != nil || input.DiaperType != nil ||
			input.Food != nil || input.AmountG != nil {
			return fmt.Errorf("%w: sleep cannot set method, amount_ml, diaper_type, food, or amount_g", ErrFieldsInvalid)
		}
	case TypeSolid:
		if input.Food == nil || strings.TrimSpace(*input.Food) == "" {
			return fmt.Errorf("%w: solid requires non-empty food", ErrFieldsInvalid)
		}
		if input.EndedAt != nil || input.Method != nil || input.AmountML != nil ||
			input.DurationMin != nil || input.DiaperType != nil {
			return fmt.Errorf("%w: solid cannot set ended_at, method, amount_ml, duration_min, or diaper_type", ErrFieldsInvalid)
		}
	}
	return nil
}
