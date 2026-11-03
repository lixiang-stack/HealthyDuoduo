// 成员域的规则(显示名 + 性别 + 出生日期;是否孩子由出生日期实时计算)。
// 存储边界与视图类型见 types.go 的 MemberStore/Member;领域规则由写入方
// (HTTP 适配器)在解析后调用,存储实现再镜像校验一次(与票 03 空指标项
// 不变量同一「边界保证 + 入库镜像」双保险惯例)。
package pipeline

import (
	"errors"
	"strings"
	"time"
)

// 成员性别枚举:产品只提供男 / 女两个选项(llm-analysis 票 01 修订)。
const (
	SexMale   = "male"
	SexFemale = "female"
)

// ChildMaxAgeYears 孩子 / 成人的年龄分界:未满 18 岁为孩子。
const ChildMaxAgeYears = 18

// BirthYearMin 出生年份下限(App 滚轮同源;避免明显错误的历史日期)。
const BirthYearMin = 1900

// birthDateLayout 契约日期格式(YYYY-MM-DD;不承担时区语义,与 report_date 同口径)。
const birthDateLayout = "2006-01-02"

// ErrMemberNameBlank 显示名不合规:必须含至少一个非空白字符。
var ErrMemberNameBlank = errors.New("member name must contain a non-blank character")

// ErrMemberSexInvalid 性别不在 male/female 内。
var ErrMemberSexInvalid = errors.New("member sex must be one of male, female")

// ErrMemberBirthDateInvalid 出生日期不是合法的 YYYY-MM-DD。
var ErrMemberBirthDateInvalid = errors.New("member birth_date must be a valid YYYY-MM-DD date")

// ValidateMemberName 成员名领域规则:空串与纯空白(含全角空格)拒绝;
// 前后空白不在此裁剪,由输入方(客户端)规整后提交。
func ValidateMemberName(name string) error {
	if strings.TrimSpace(name) == "" {
		return ErrMemberNameBlank
	}
	return nil
}

// ValidateMemberSex 性别领域规则:只接受 male/female。
func ValidateMemberSex(sex string) error {
	switch sex {
	case SexMale, SexFemale:
		return nil
	default:
		return ErrMemberSexInvalid
	}
}

// ValidateMemberBirthDate 出生日期领域规则:必须是合法公历日期,且年份不早于 1900。
func ValidateMemberBirthDate(birthDate string) error {
	born, err := time.Parse(birthDateLayout, birthDate)
	if err != nil || born.Year() < BirthYearMin {
		return ErrMemberBirthDateInvalid
	}
	return nil
}

// IsChild 按出生日期与当前时刻计算是否孩子:未满 18 岁为 true。
// 出生日期非法时返回 false(写入路径已校验,这里只防御存量脏数据)。
func IsChild(birthDate string, now time.Time) bool {
	born, err := time.Parse(birthDateLayout, birthDate)
	if err != nil {
		return false
	}
	// 用本地日历日与生日零点比较,避免 18 岁生日当天因时区偏移被算成孩子。
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return today.Before(born.AddDate(ChildMaxAgeYears, 0, 0))
}

// ValidateMemberProfile 建成员时的完整资料校验(名 + 性别 + 出生日期)。
func ValidateMemberProfile(p MemberProfile) error {
	if err := ValidateMemberName(p.Name); err != nil {
		return err
	}
	if err := ValidateMemberSex(p.Sex); err != nil {
		return err
	}
	return ValidateMemberBirthDate(p.BirthDate)
}
