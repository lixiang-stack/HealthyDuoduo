package main

// 护理事件端点 HTTP 测试（care-events 票 02）：创建 / 列表 / 修改 / 删除、成人成员
// 拒绝、未知成员 / 事件、范围校验、非法 body、空列表与存储错误。范围重叠 SQL 与
// 排序由 storage 冒烟覆盖；这里用窄 fake 验证 handler 的编解码与错误映射。

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"healthyduoduo/internal/care"
	"healthyduoduo/internal/httpapi"
	"healthyduoduo/internal/pipeline"
)

type fakeCareStore struct {
	events map[int64]care.CareEvent
	nextID int64
	err    error
}

var _ care.Store = (*fakeCareStore)(nil)

func newFakeCareStore() *fakeCareStore {
	return &fakeCareStore{events: map[int64]care.CareEvent{}}
}

func (f *fakeCareStore) List(ctx context.Context, memberID int64, from, to time.Time) ([]care.CareEvent, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []care.CareEvent
	for _, event := range f.events {
		if event.MemberID == memberID {
			out = append(out, event)
		}
	}
	return out, nil
}

func (f *fakeCareStore) Create(ctx context.Context, memberID int64, input care.EventInput) (*care.CareEvent, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.nextID++
	event := care.CareEvent{
		ID: f.nextID, MemberID: memberID, Type: input.Type, Source: care.SourceParent,
		StartedAt: input.StartedAt, EndedAt: input.EndedAt, Note: input.Note,
		Method: input.Method, AmountML: input.AmountML, DurationMin: input.DurationMin,
		DiaperType: input.DiaperType, Food: input.Food, AmountG: input.AmountG,
	}
	f.events[event.ID] = event
	return &event, nil
}

func (f *fakeCareStore) Update(ctx context.Context, eventID int64, input care.EventInput) (*care.CareEvent, error) {
	if f.err != nil {
		return nil, f.err
	}
	existing, ok := f.events[eventID]
	if !ok {
		return nil, nil
	}
	existing.Type = input.Type
	existing.StartedAt = input.StartedAt
	existing.EndedAt = input.EndedAt
	existing.Note = input.Note
	existing.Method = input.Method
	existing.AmountML = input.AmountML
	existing.DurationMin = input.DurationMin
	existing.DiaperType = input.DiaperType
	existing.Food = input.Food
	existing.AmountG = input.AmountG
	f.events[eventID] = existing
	return &existing, nil
}

func (f *fakeCareStore) Delete(ctx context.Context, eventID int64) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	if _, ok := f.events[eventID]; !ok {
		return false, nil
	}
	delete(f.events, eventID)
	return true, nil
}

func newCareTestServer(t *testing.T, store care.Store) (*httptest.Server, *fakeMemberStore) {
	t.Helper()
	ms := &fakeMemberStore{}
	d := &deps{members: ms, care: store}
	srv := httptest.NewServer(newServeHandler(d))
	t.Cleanup(srv.Close)
	return srv, ms
}

func carePath(memberID int64) string {
	return "/api/v1/members/" + strconv.FormatInt(memberID, 10) + "/care-events"
}

func careRangeQuery(path string) string {
	values := url.Values{}
	values.Set("from", "2026-11-24T00:00:00Z")
	values.Set("to", "2026-11-25T00:00:00Z")
	return path + "?" + values.Encode()
}

func TestServeCareEventCreateAndList(t *testing.T) {
	store := newFakeCareStore()
	srv, ms := newCareTestServer(t, store)
	child, _ := ms.CreateMember(context.Background(), pipeline.MemberProfile{
		Name: "小虎", Sex: pipeline.SexMale, BirthDate: "2020-05-20",
	})
	path := carePath(child.MemberID)

	status, raw := doJSON(t, srv, http.MethodPost, path,
		`{"type":"feed","started_at":"2026-11-24T14:05:00+08:00","ended_at":null,"note":null,"method":"formula","amount_ml":120}`)
	if status != http.StatusCreated {
		t.Fatalf("POST care event status = %d, want 201; body=%s", status, raw)
	}
	var created httpapi.CareEvent
	if err := json.Unmarshal(raw, &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 || created.MemberID != child.MemberID || created.Type != care.TypeFeed ||
		created.Source != care.SourceParent || created.StartedAt != "2026-11-24T14:05:00+08:00" ||
		created.EndedAt != nil || created.Note != nil || created.Method == nil ||
		*created.Method != care.FeedMethodFormula || created.AmountML == nil ||
		*created.AmountML != 120 || created.DurationMin != nil {
		t.Fatalf("created care event = %+v", created)
	}

	status, raw = doJSON(t, srv, http.MethodGet, careRangeQuery(path), "")
	if status != http.StatusOK {
		t.Fatalf("GET care events status = %d, want 200; body=%s", status, raw)
	}
	var list httpapi.CareEventList
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Events) != 1 || list.Events[0].ID != created.ID {
		t.Fatalf("care event list = %+v", list.Events)
	}

	// 另一个孩子没有事件：空列表必须是 [] 而不是 null。
	emptyChild, _ := ms.CreateMember(context.Background(), pipeline.MemberProfile{
		Name: "小满", Sex: pipeline.SexFemale, BirthDate: "2023-02-10",
	})
	status, raw = doJSON(t, srv, http.MethodGet, careRangeQuery(carePath(emptyChild.MemberID)), "")
	if status != http.StatusOK {
		t.Fatalf("GET empty care events status = %d; body=%s", status, raw)
	}
	if string(raw) != "{\"events\":[]}\n" && string(raw) != "{\"events\":[]}" {
		t.Fatalf("empty care event list = %s, want events:[]", raw)
	}
}

func TestServeCareEventUpdateAndDelete(t *testing.T) {
	store := newFakeCareStore()
	srv, ms := newCareTestServer(t, store)
	child, _ := ms.CreateMember(context.Background(), pipeline.MemberProfile{
		Name: "小虎", Sex: pipeline.SexMale, BirthDate: "2020-05-20",
	})
	path := carePath(child.MemberID)

	status, raw := doJSON(t, srv, http.MethodPost, path,
		`{"type":"feed","started_at":"2026-11-24T14:05:00+08:00","ended_at":null,"note":null,"method":"formula","amount_ml":120}`)
	if status != http.StatusCreated {
		t.Fatalf("POST status = %d; body=%s", status, raw)
	}
	var created httpapi.CareEvent
	if err := json.Unmarshal(raw, &created); err != nil {
		t.Fatal(err)
	}
	eventPath := "/api/v1/care-events/" + strconv.FormatInt(created.ID, 10)

	status, raw = doJSON(t, srv, http.MethodPatch, eventPath,
		`{"type":"diaper","started_at":"2026-11-24T15:00:00+08:00","ended_at":null,"note":"换尿布","diaper_type":"dirty"}`)
	if status != http.StatusOK {
		t.Fatalf("PATCH status = %d, want 200; body=%s", status, raw)
	}
	var updated httpapi.CareEvent
	if err := json.Unmarshal(raw, &updated); err != nil {
		t.Fatal(err)
	}
	if updated.ID != created.ID || updated.MemberID != child.MemberID ||
		updated.Source != care.SourceParent || updated.Type != care.TypeDiaper ||
		updated.StartedAt != "2026-11-24T15:00:00+08:00" || updated.DiaperType == nil ||
		*updated.DiaperType != care.DiaperTypeDirty || updated.Note == nil ||
		*updated.Note != "换尿布" || updated.Method != nil || updated.AmountML != nil ||
		updated.DurationMin != nil || updated.EndedAt != nil {
		t.Fatalf("updated care event = %+v", updated)
	}

	status, raw = doJSON(t, srv, http.MethodDelete, eventPath, "")
	if status != http.StatusNoContent {
		t.Fatalf("DELETE status = %d, want 204; body=%s", status, raw)
	}
	status, raw = doJSON(t, srv, http.MethodDelete, eventPath, "")
	if status != http.StatusNotFound {
		t.Fatalf("DELETE again status = %d, want 404; body=%s", status, raw)
	}
	status, raw = doJSON(t, srv, http.MethodPatch, eventPath,
		`{"type":"feed","started_at":"2026-11-24T15:00:00+08:00","ended_at":null,"note":null,"method":"breast"}`)
	if status != http.StatusNotFound {
		t.Fatalf("PATCH deleted status = %d, want 404; body=%s", status, raw)
	}
}

func TestServeCareEventValidation(t *testing.T) {
	store := newFakeCareStore()
	srv, ms := newCareTestServer(t, store)
	child, _ := ms.CreateMember(context.Background(), pipeline.MemberProfile{
		Name: "小虎", Sex: pipeline.SexMale, BirthDate: "2020-05-20",
	})
	adult, _ := ms.CreateMember(context.Background(), pipeline.MemberProfile{
		Name: "妈妈", Sex: pipeline.SexFemale, BirthDate: "1988-01-01",
	})
	path := carePath(child.MemberID)

	cases := []struct {
		name string
		path string
		body string
	}{
		{"missing method", path, `{"type":"feed","started_at":"2026-11-24T14:05:00+08:00","ended_at":null,"note":null}`},
		{"invalid type", path, `{"type":"pump","started_at":"2026-11-24T14:05:00+08:00","ended_at":null,"note":null}`},
		{"invalid started_at", path, `{"type":"feed","started_at":"not-a-time","ended_at":null,"note":null,"method":"breast"}`},
		{"invalid ended_at", path, `{"type":"feed","started_at":"2026-11-24T14:05:00+08:00","ended_at":"not-a-time","note":null,"method":"breast"}`},
		{"feed with diaper_type", path, `{"type":"feed","started_at":"2026-11-24T14:05:00+08:00","ended_at":null,"note":null,"method":"breast","diaper_type":"wet"}`},
		{"adult member", carePath(adult.MemberID), `{"type":"feed","started_at":"2026-11-24T14:05:00+08:00","ended_at":null,"note":null,"method":"breast"}`},
		{"unknown field", path, `{"type":"feed","started_at":"2026-11-24T14:05:00+08:00","ended_at":null,"note":null,"method":"breast","member_id":1}`},
		{"malformed", path, `{"type":`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, raw := doJSON(t, srv, http.MethodPost, c.path, c.body)
			if status != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", status, raw)
			}
			var apiErr httpapi.Error
			if err := json.Unmarshal(raw, &apiErr); err != nil || apiErr.Message == "" {
				t.Fatalf("error body = %s", raw)
			}
		})
	}

	for _, c := range []struct {
		name string
		path string
	}{
		{"missing range", path},
		{"invalid range", path + "?from=2026-11-24&to=2026-11-25"},
		{"reversed range", path + "?from=2026-11-25T00:00:00Z&to=2026-11-24T00:00:00Z"},
	} {
		t.Run(c.name, func(t *testing.T) {
			status, raw := doJSON(t, srv, http.MethodGet, c.path, "")
			if status != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", status, raw)
			}
		})
	}

	status, raw := doJSON(t, srv, http.MethodPatch, "/api/v1/care-events/1",
		`{"type":"feed","started_at":"2026-11-24T14:05:00+08:00","ended_at":null,"note":null}`)
	if status != http.StatusBadRequest {
		t.Fatalf("invalid PATCH status = %d, want 400; body=%s", status, raw)
	}
}

func TestServeCareEventUnknownMemberAndStoreError(t *testing.T) {
	store := newFakeCareStore()
	srv, ms := newCareTestServer(t, store)
	child, _ := ms.CreateMember(context.Background(), pipeline.MemberProfile{
		Name: "小虎", Sex: pipeline.SexMale, BirthDate: "2020-05-20",
	})
	path := carePath(child.MemberID)

	status, raw := doJSON(t, srv, http.MethodGet, careRangeQuery("/api/v1/members/999/care-events"), "")
	if status != http.StatusNotFound {
		t.Fatalf("unknown member GET = %d, want 404; body=%s", status, raw)
	}
	status, raw = doJSON(t, srv, http.MethodPost, "/api/v1/members/999/care-events",
		`{"type":"feed","started_at":"2026-11-24T14:05:00+08:00","ended_at":null,"note":null,"method":"breast"}`)
	if status != http.StatusNotFound {
		t.Fatalf("unknown member POST = %d, want 404; body=%s", status, raw)
	}
	status, raw = doJSON(t, srv, http.MethodPatch, "/api/v1/care-events/999",
		`{"type":"feed","started_at":"2026-11-24T14:05:00+08:00","ended_at":null,"note":null,"method":"breast"}`)
	if status != http.StatusNotFound {
		t.Fatalf("unknown event PATCH = %d, want 404; body=%s", status, raw)
	}
	status, raw = doJSON(t, srv, http.MethodDelete, "/api/v1/care-events/999", "")
	if status != http.StatusNotFound {
		t.Fatalf("unknown event DELETE = %d, want 404; body=%s", status, raw)
	}

	store.err = errors.New("boom")
	for _, c := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"list", http.MethodGet, careRangeQuery(path), ""},
		{"create", http.MethodPost, path, `{"type":"feed","started_at":"2026-11-24T14:05:00+08:00","ended_at":null,"note":null,"method":"breast"}`},
		{"update", http.MethodPatch, "/api/v1/care-events/1", `{"type":"feed","started_at":"2026-11-24T14:05:00+08:00","ended_at":null,"note":null,"method":"breast"}`},
		{"delete", http.MethodDelete, "/api/v1/care-events/1", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			status, raw := doJSON(t, srv, c.method, c.path, c.body)
			if status != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500; body=%s", status, raw)
			}
		})
	}
}
