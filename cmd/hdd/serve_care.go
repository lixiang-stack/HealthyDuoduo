// 护理事件端点（care-events 票 02）：GET/POST /api/v1/members/{member_id}/care-events
// 与 PATCH/DELETE /api/v1/care-events/{event_id}。handler 只做 HTTP 编解码、成员 / 孩子
// 校验与错误映射；类型规则在 internal/care，持久化在 internal/storage。
package main

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"healthyduoduo/internal/care"
	"healthyduoduo/internal/httpapi"
)

const careEventNotFoundMessage = "care event not found"

// handleListCareEvents GET /members/{member_id}/care-events?from=&to=：范围重叠读取。
func handleListCareEvents(d *deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		memberID, err := strconv.ParseInt(r.PathValue("member_id"), 10, 64)
		if err != nil {
			writeErrorJSON(w, http.StatusNotFound, memberNotFoundMessage)
			return
		}
		if !requireMember(w, r, d.members, memberID) {
			return
		}
		from, to, err := careRange(r)
		if err != nil {
			writeErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}
		events, err := d.care.List(r.Context(), memberID, from, to)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "list care events failed")
			return
		}
		list := httpapi.CareEventList{Events: make([]httpapi.CareEvent, 0, len(events))}
		for _, event := range events {
			list.Events = append(list.Events, toAPICareEvent(event))
		}
		writeJSON(w, http.StatusOK, list)
	}
}

// handleCreateCareEvent POST /members/{member_id}/care-events：目标必须是孩子。
func handleCreateCareEvent(d *deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		memberID, err := strconv.ParseInt(r.PathValue("member_id"), 10, 64)
		if err != nil {
			writeErrorJSON(w, http.StatusNotFound, memberNotFoundMessage)
			return
		}
		member, err := d.members.MemberByID(r.Context(), memberID)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "lookup member failed")
			return
		}
		if member == nil {
			writeErrorJSON(w, http.StatusNotFound, memberNotFoundMessage)
			return
		}
		if !member.IsChild {
			writeErrorJSON(w, http.StatusBadRequest, "care events target child members only")
			return
		}
		var body httpapi.CareEventCreate
		if err := decodeJSONBody(r, &body); err != nil {
			writeErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}
		input, err := careInput(body)
		if err != nil {
			writeErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}
		event, err := d.care.Create(r.Context(), memberID, input)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "create care event failed")
			return
		}
		writeJSON(w, http.StatusCreated, toAPICareEvent(*event))
	}
}

// handleUpdateCareEvent PATCH /care-events/{event_id}：完整替换可编辑内容。
func handleUpdateCareEvent(d *deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		eventID, err := strconv.ParseInt(r.PathValue("event_id"), 10, 64)
		if err != nil {
			writeErrorJSON(w, http.StatusNotFound, careEventNotFoundMessage)
			return
		}
		var body httpapi.CareEventCreate
		if err := decodeJSONBody(r, &body); err != nil {
			writeErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}
		input, err := careInput(body)
		if err != nil {
			writeErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}
		event, err := d.care.Update(r.Context(), eventID, input)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "update care event failed")
			return
		}
		if event == nil {
			writeErrorJSON(w, http.StatusNotFound, careEventNotFoundMessage)
			return
		}
		writeJSON(w, http.StatusOK, toAPICareEvent(*event))
	}
}

// handleDeleteCareEvent DELETE /care-events/{event_id}：204 或 404。
func handleDeleteCareEvent(d *deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		eventID, err := strconv.ParseInt(r.PathValue("event_id"), 10, 64)
		if err != nil {
			writeErrorJSON(w, http.StatusNotFound, careEventNotFoundMessage)
			return
		}
		deleted, err := d.care.Delete(r.Context(), eventID)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "delete care event failed")
			return
		}
		if !deleted {
			writeErrorJSON(w, http.StatusNotFound, careEventNotFoundMessage)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// careRange 解析必填的 from / to；两者都要求 RFC3339 带偏移，且 from < to。
func careRange(r *http.Request) (time.Time, time.Time, error) {
	rawFrom := r.URL.Query().Get("from")
	rawTo := r.URL.Query().Get("to")
	if rawFrom == "" || rawTo == "" {
		return time.Time{}, time.Time{}, errors.New("from and to are required (RFC3339)")
	}
	from, err := time.Parse(time.RFC3339, rawFrom)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("from must be RFC3339 with an offset")
	}
	to, err := time.Parse(time.RFC3339, rawTo)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("to must be RFC3339 with an offset")
	}
	if !from.Before(to) {
		return time.Time{}, time.Time{}, errors.New("from must be before to")
	}
	return from, to, nil
}

// careInput 把契约 body 映射为领域输入；时间解析与类型规则在返回前收口。
func careInput(body httpapi.CareEventCreate) (care.EventInput, error) {
	startedAt, err := time.Parse(time.RFC3339, body.StartedAt)
	if err != nil {
		return care.EventInput{}, errors.New("started_at must be RFC3339 with an offset")
	}
	var endedAt *time.Time
	if body.EndedAt != nil {
		parsed, err := time.Parse(time.RFC3339, *body.EndedAt)
		if err != nil {
			return care.EventInput{}, errors.New("ended_at must be RFC3339 with an offset")
		}
		endedAt = &parsed
	}
	input := care.Normalize(care.EventInput{
		Type:        body.Type,
		StartedAt:   startedAt,
		EndedAt:     endedAt,
		Note:        body.Note,
		Method:      body.Method,
		AmountML:    body.AmountML,
		DurationMin: body.DurationMin,
		DiaperType:  body.DiaperType,
		Food:        body.Food,
		AmountG:     body.AmountG,
	})
	if err := care.Validate(input); err != nil {
		return care.EventInput{}, err
	}
	return input, nil
}

// toAPICareEvent 领域事件 → 契约形状；时间统一输出 RFC3339。
func toAPICareEvent(event care.CareEvent) httpapi.CareEvent {
	return httpapi.CareEvent{
		ID:          event.ID,
		MemberID:    event.MemberID,
		Type:        event.Type,
		Source:      event.Source,
		StartedAt:   event.StartedAt.Format(time.RFC3339),
		EndedAt:     formatCareTime(event.EndedAt),
		Note:        event.Note,
		Method:      event.Method,
		AmountML:    event.AmountML,
		DurationMin: event.DurationMin,
		DiaperType:  event.DiaperType,
		Food:        event.Food,
		AmountG:     event.AmountG,
	}
}

func formatCareTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format(time.RFC3339)
	return &formatted
}
