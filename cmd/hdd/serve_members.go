// 成员端点(GET/POST /api/v1/members 与 PATCH/DELETE /api/v1/members/{member_id};
// 契约冻结于 schemas/openapi.yaml,viz-and-client 票 05 实现)。
// 列表按创建序(id 升序)返回;删除会一并删除该成员的全部报告(原图与 OCR 历史保留)。
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

// memberBody POST/PATCH 请求体解码形状:字段按「是否携带 + 值」两段读,
// 用于执行契约的 required(建)与 minProperties: 1(改)约束。
type memberBody struct {
	Name      optField[string] `json:"name"`
	Sex       optField[string] `json:"sex"`
	BirthDate optField[string] `json:"birth_date"`
}

// create 建成员请求出口:传输校验(必填字段)与领域规则在此收口,
// handler 只做 解码→取值→调用→映射错误;领域错误透传(中性说明)。
func (b memberBody) create() (pipeline.MemberProfile, error) {
	if !b.Name.set {
		return pipeline.MemberProfile{}, errors.New("name is required")
	}
	if !b.Sex.set {
		return pipeline.MemberProfile{}, errors.New("sex is required")
	}
	if !b.BirthDate.set {
		return pipeline.MemberProfile{}, errors.New("birth_date is required")
	}
	profile := pipeline.MemberProfile{
		Name: b.Name.value, Sex: b.Sex.value, BirthDate: b.BirthDate.value,
	}
	if err := pipeline.ValidateMemberProfile(profile); err != nil {
		return pipeline.MemberProfile{}, err
	}
	return profile, nil
}

// patch 改成员请求出口:至少携带一项;所带字段过领域规则,返回仅含所带字段的取值。
func (b memberBody) patch() (pipeline.MemberPatch, error) {
	if !b.Name.set && !b.Sex.set && !b.BirthDate.set {
		return pipeline.MemberPatch{}, errors.New("at least one of name, sex, birth_date is required")
	}
	var patch pipeline.MemberPatch
	if b.Name.set {
		if err := pipeline.ValidateMemberName(b.Name.value); err != nil {
			return pipeline.MemberPatch{}, err
		}
		patch.Name = &b.Name.value
	}
	if b.Sex.set {
		if err := pipeline.ValidateMemberSex(b.Sex.value); err != nil {
			return pipeline.MemberPatch{}, err
		}
		patch.Sex = &b.Sex.value
	}
	if b.BirthDate.set {
		if err := pipeline.ValidateMemberBirthDate(b.BirthDate.value); err != nil {
			return pipeline.MemberPatch{}, err
		}
		patch.BirthDate = &b.BirthDate.value
	}
	return patch, nil
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

// handleCreateMember POST /api/v1/members:name/sex/birth_date 必填;is_child
// 由服务端按出生日期计算。201 返回完整成员。
func handleCreateMember(ms pipeline.MemberStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body memberBody
		if err := decodeJSONBody(r, &body); err != nil {
			writeErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}
		profile, err := body.create()
		if err != nil {
			writeErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}
		m, err := ms.CreateMember(r.Context(), profile)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "create member failed")
			return
		}
		writeJSON(w, http.StatusCreated, toAPIMember(*m))
	}
}

// handleUpdateMember PATCH /api/v1/members/{member_id}:改名/改性别/改生日,至少一项;
// is_child 随生日重算;未知成员 404。
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
		patch, err := body.patch()
		if err != nil {
			writeErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}
		m, err := ms.UpdateMember(r.Context(), id, patch)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "update member failed")
			return
		}
		if m == nil {
			writeErrorJSON(w, http.StatusNotFound, memberNotFoundMessage)
			return
		}
		writeJSON(w, http.StatusOK, toAPIMember(*m))
	}
}

// handleDeleteMember DELETE /api/v1/members/{member_id}:硬删成员及其全部报告
// (报告/指标项/修正对级联;原图与 OCR 历史保留;不可恢复)。未知成员 404,成功 204。
func handleDeleteMember(ms pipeline.MemberStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("member_id"), 10, 64)
		if err != nil {
			writeErrorJSON(w, http.StatusBadRequest, "member_id must be an integer")
			return
		}
		deleted, err := ms.DeleteMember(r.Context(), id)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "delete member failed")
			return
		}
		if !deleted {
			writeErrorJSON(w, http.StatusNotFound, memberNotFoundMessage)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// toAPIMember 业务视图 → 契约形状(created_at 统一 RFC3339 UTC;无生长记录时 latest_growth 为 null)。
func toAPIMember(m pipeline.Member) httpapi.Member {
	var latest *httpapi.GrowthRecord
	if m.LatestGrowth != nil {
		record := toAPIGrowthRecord(*m.LatestGrowth)
		latest = &record
	}
	return httpapi.Member{
		MemberID:     m.MemberID,
		Name:         m.Name,
		IsChild:      m.IsChild,
		Sex:          m.Sex,
		BirthDate:    m.BirthDate,
		CreatedAt:    m.CreatedAt.UTC().Format(time.RFC3339),
		LatestGrowth: latest,
	}
}
