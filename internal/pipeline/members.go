// 成员域的规则(成员最小模型 = 显示名 + 是否孩子两档;viz-and-client 票 05)。
// 存储边界与视图类型见 types.go 的 MemberStore/Member;领域规则由写入方
// (HTTP 适配器)在解析后调用,存储实现再镜像校验一次(与票 03 空指标项
// 不变量同一「边界保证 + 入库镜像」双保险惯例)。
package pipeline

import (
	"errors"
	"strings"
)

// ErrMemberNameBlank 显示名不合规:必须含至少一个非空白字符。
var ErrMemberNameBlank = errors.New("member name must contain a non-blank character")

// ValidateMemberName 成员名领域规则:空串与纯空白(含全角空格)拒绝;
// 前后空白不在此裁剪,由输入方(客户端)规整后提交。
func ValidateMemberName(name string) error {
	if strings.TrimSpace(name) == "" {
		return ErrMemberNameBlank
	}
	return nil
}
