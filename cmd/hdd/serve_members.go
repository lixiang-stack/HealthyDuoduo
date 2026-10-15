// 成员端点(GET/POST /api/v1/members 与 PATCH /api/v1/members/{member_id};
// 契约冻结于 schemas/openapi.yaml,viz-and-client 票 05 实现)。
// 列表按创建序(id 升序)返回,孩子优先由客户端做;无删除入口(契约层就没有 DELETE)。
package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"healthyduoduo/internal/httpapi"
	"healthyduoduo/internal/pipeline"
)

// optField 请求体可选字段:区分「未带」与「显式有值」两态;显式 null 一律拒绝
// (契约字段类型不允许 null,与 additionalProperties:false 是同一执行立场)。
type optField[T any] struct {
	set   bool
	value T
}

func (f *optField[T]) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return errors.New("null is not allowed")
	}
	f.set = true
	return json.Unmarshal(data, &f.value)
}

// memberBody POST/PATCH 请求体解码形状:name/is_child 按「是否携带 + 值」两段读,
// 用于执行契约的 required(建)与 minProperties: 1(改)约束。
type memberBody struct {
	Name    optField[string] `json:"name"`
	IsChild optField[bool]   `json:"is_child"`
}

// create 建成员请求出口:传输校验(字段必带)与领域规则(名字非空白)在此收口,
// handler 只做 解码→取值→调用→映射错误;领域错误透传(中性说明)。
func (b memberBody) create() (name string, isChild bool, err error) {
	if !b.Name.set {
		return "", false, errors.New("name is required")
	}
	if err := pipeline.ValidateMemberName(b.Name.value); err != nil {
		return "", false, err
	}
	if !b.IsChild.set {
		return "", false, errors.New("is_child is required")
	}
	return b.Name.value, b.IsChild.value, nil
}

// patch 改成员请求出口:至少携带一项;所带 name 过领域规则,返回仅含所带字段的取值。
func (b memberBody) patch() (name *string, isChild *bool, err error) {
	if !b.Name.set && !b.IsChild.set {
		return nil, nil, errors.New("at least one of name, is_child is required")
	}
	if b.Name.set {
		if err := pipeline.ValidateMemberName(b.Name.value); err != nil {
			return nil, nil, err
		}
		name = &b.Name.value
	}
	if b.IsChild.set {
		isChild = &b.IsChild.value
	}
	return name, isChild, nil
}

// handleListMembers GET /api/v1/members:全量成员,创建序(id 升序)。
func handleListMembers(ms pipeline.MemberStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		members, err := ms.ListMembers(r.Context())
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "list members failed")
			return
		}
		list := httpapi.MembersList{Members: make([]httpapi.Member, 0, len(members))}
		for _, m := range members {
			list.Members = append(list.Members, toAPIMember(m))
		}
		writeJSON(w, http.StatusOK, list)
	}
}

// handleCreateMember POST /api/v1/members:name + is_child 必填;201 返回完整成员。
func handleCreateMember(ms pipeline.MemberStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body memberBody
		if err := decodeJSONBody(r, &body); err != nil {
			writeErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}
		name, isChild, err := body.create()
		if err != nil {
			writeErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}
		m, err := ms.CreateMember(r.Context(), name, isChild)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "create member failed")
			return
		}
		writeJSON(w, http.StatusCreated, toAPIMember(*m))
	}
}

// handleUpdateMember PATCH /api/v1/members/{member_id}:改名/改是否孩子,至少一项;
// 未知成员 404(无删除语义,只有存在与否)。
func handleUpdateMember(ms pipeline.MemberStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("member_id"), 10, 64)
		if err != nil {
			writeErrorJSON(w, http.StatusBadRequest, "member_id must be an integer")
			return
		}
		var body memberBody
		if err := decodeJSONBody(r, &body); err != nil {
			writeErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}
		name, isChild, err := body.patch()
		if err != nil {
			writeErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}
		m, err := ms.UpdateMember(r.Context(), id, name, isChild)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "update member failed")
			return
		}
		if m == nil {
			writeErrorJSON(w, http.StatusNotFound, "member not found")
			return
		}
		writeJSON(w, http.StatusOK, toAPIMember(*m))
	}
}

// toAPIMember 业务视图 → 契约形状(created_at 统一 RFC3339 UTC)。
func toAPIMember(m pipeline.Member) httpapi.Member {
	return httpapi.Member{
		MemberID:  m.MemberID,
		Name:      m.Name,
		IsChild:   m.IsChild,
		CreatedAt: m.CreatedAt.UTC().Format(time.RFC3339),
	}
}
