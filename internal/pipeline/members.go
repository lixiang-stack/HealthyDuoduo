// 成员域的规则(显示名 + 是否孩子 + 性别 + 出生日期及其精度)。
// 存储边界与视图类型见 types.go 的 MemberStore/Member;领域规则由写入方
// (HTTP 适配器)在解析后调用,存储实现再镜像校验一次(与票 03 空指标项
// 不变量同一「边界保证 + 入库镜像」双保险惯例)。
package pipeline

import (
	"errors"
	"strings"
	"time"
)

// 成员性别枚举(llm-analysis 票 01;三选一,不强迫二选一)。
const (
	SexMale        = "male"
	SexFemale      = "female"
	SexUnspecified = "unspecified"
)

// 出生日期精度枚举:day = 完整日期;year = 只知出生年份,
// 存当年 1 月 1 日并标 year,避免 0-1 岁段按满岁计算。
const (
	BirthDatePrecisionDay  = "day"
	BirthDatePrecisionYear = "year"
)

// birthDateLayout 契约日期格式(YYYY-MM-DD;不承担时区语义,与 report_date 同口径)。
const birthDateLayout = "2006-01-02"

// ErrMemberNameBlank 显示名不合规:必须含至少一个非空白字符。
var ErrMemberNameBlank = errors.New("member name must contain a non-blank character")

// ErrMemberSexInvalid 性别不在 male/female/unspecified 三选一内。
var ErrMemberSexInvalid = errors.New("member sex must be one of male, female, unspecified")

// ErrMemberBirthDateInvalid 出生日期不是合法的 YYYY-MM-DD。
var ErrMemberBirthDateInvalid = errors.New("member birth_date must be a valid YYYY-MM-DD date")

// ErrMemberBirthDatePrecisionInvalid 出生日期精度不在 day/year 内。
var ErrMemberBirthDatePrecisionInvalid = errors.New("member birth_date_precision must be one of day, year")

// ValidateMemberName 成员名领域规则:空串与纯空白(含全角空格)拒绝;
// 前后空白不在此裁剪,由输入方(客户端)规整后提交。
func ValidateMemberName(name string) error {
	if strings.TrimSpace(name) == "" {
		return ErrMemberNameBlank
	}
	return nil
}

// ValidateMemberSex 性别领域规则:只接受三选一枚举。
func ValidateMemberSex(sex string) error {
	switch sex {
	case SexMale, SexFemale, SexUnspecified:
		return nil
	default:
		return ErrMemberSexInvalid
	}
}

// ValidateMemberBirthDate 出生日期领域规则:必须是合法公历日期。
func ValidateMemberBirthDate(birthDate string) error {
	if _, err := time.Parse(birthDateLayout, birthDate); err != nil {
		return ErrMemberBirthDateInvalid
	}
	return nil
}

// ValidateMemberBirthDatePrecision 出生日期精度领域规则:只接受 day/year。
func ValidateMemberBirthDatePrecision(precision string) error {
	switch precision {
	case BirthDatePrecisionDay, BirthDatePrecisionYear:
		return nil
	default:
		return ErrMemberBirthDatePrecisionInvalid
	}
}

// ValidateMemberProfile 建成员时的完整资料校验(名 + 是否孩子 + 性别 + 生日与精度)。
func ValidateMemberProfile(p MemberProfile) error {
	if err := ValidateMemberName(p.Name); err != nil {
		return err
	}
	if err := ValidateMemberSex(p.Sex); err != nil {
		return err
	}
	if err := ValidateMemberBirthDate(p.BirthDate); err != nil {
		return err
	}
	return ValidateMemberBirthDatePrecision(p.BirthDatePrecision)
}
