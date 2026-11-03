package main

// 生长记录端点 HTTP 测试(llm-analysis 票 02):正常录入 / BMI 实时计算 / 非法来源 /
// 缺测量值 / 列表排序 / 空数组 / 未知成员。持久化以窄 fake 打底,SQL 排序与
// 成员最近记录由 storage 冒烟覆盖。

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"healthyduoduo/internal/httpapi"
	"healthyduoduo/internal/pipeline"
)

type fakeGrowthStore struct {
	rows []pipeline.GrowthRecord
	err  error
}

var _ pipeline.GrowthStore = (*fakeGrowthStore)(nil)

func (f *fakeGrowthStore) CreateGrowth(ctx context.Context, memberID int64, profile pipeline.GrowthProfile) (*pipeline.GrowthRecord, error) {
	if f.err != nil {
		return nil, f.err
	}
	record := pipeline.GrowthRecord{
		GrowthID:   int64(len(f.rows) + 1),
		MemberID:   memberID,
		MeasuredOn: profile.MeasuredOn,
		HeightCm:   profile.HeightCm,
		WeightKg:   profile.WeightKg,
		Source:     profile.Source,
		BMI:        pipeline.BMI(profile.HeightCm, profile.WeightKg),
	}
	f.rows = append(f.rows, record)
	return &record, nil
}

func (f *fakeGrowthStore) ListGrowth(ctx context.Context, memberID int64) ([]pipeline.GrowthRecord, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]pipeline.GrowthRecord(nil), f.rows...), nil
}

func newGrowthTestServer(t *testing.T, growth pipeline.GrowthStore) (*httptest.Server, *fakeMemberStore) {
	t.Helper()
	ms := &fakeMemberStore{}
	d := &deps{members: ms, growth: growth}
	srv := httptest.NewServer(newServeHandler(d))
	t.Cleanup(srv.Close)
	return srv, ms
}

func TestServeGrowthCreateAndList(t *testing.T) {
	srv, ms := newGrowthTestServer(t, &fakeGrowthStore{})
	member, err := ms.CreateMember(context.Background(), pipeline.MemberProfile{
		Name: "小虎", Sex: pipeline.SexMale, BirthDate: "2020-05-20",
	})
	if err != nil {
		t.Fatal(err)
	}
	memberID := member.MemberID
	path := "/api/v1/members/" + strconv.FormatInt(memberID, 10) + "/growth"

	status, raw := doJSON(t, srv, http.MethodPost, path,
		`{"measured_on":"2026-10-01","height_cm":118.5,"weight_kg":22.4,"source":"measured"}`)
	if status != http.StatusCreated {
		t.Fatalf("POST growth status = %d, want 201; body=%s", status, raw)
	}
	var created httpapi.GrowthRecord
	if err := json.Unmarshal(raw, &created); err != nil {
		t.Fatal(err)
	}
	if created.MemberID != memberID || created.MeasuredOn != "2026-10-01" ||
		created.HeightCm == nil || *created.HeightCm != 118.5 ||
		created.WeightKg == nil || *created.WeightKg != 22.4 ||
		created.Source != pipeline.GrowthSourceMeasured ||
		created.BMI == nil || *created.BMI != 16 ||
		created.WeightStatus != nil {
		t.Fatalf("created growth = %+v", created)
	}

	status, raw = doJSON(t, srv, http.MethodGet, path, "")
	if status != http.StatusOK {
		t.Fatalf("GET growth status = %d, want 200; body=%s", status, raw)
	}
	var list httpapi.GrowthList
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Records) != 1 || list.Records[0].GrowthID != created.GrowthID {
		t.Fatalf("growth list = %+v", list.Records)
	}

	// 单填身高也合法;手输来源只给 BMI(缺体重时为 null),weight_status 永远 null。
	status, raw = doJSON(t, srv, http.MethodPost, path,
		`{"measured_on":"2026-10-02","height_cm":119,"source":"manual"}`)
	if status != http.StatusCreated {
		t.Fatalf("POST height-only status = %d, want 201; body=%s", status, raw)
	}
	var manual httpapi.GrowthRecord
	if err := json.Unmarshal(raw, &manual); err != nil {
		t.Fatal(err)
	}
	if manual.WeightKg != nil || manual.BMI != nil || manual.WeightStatus != nil {
		t.Fatalf("manual height-only = %+v, want nil weight/BMI/status", manual)
	}

	status, raw = doJSON(t, srv, http.MethodGet, path, "")
	if status != http.StatusOK {
		t.Fatalf("GET growth after manual status = %d; body=%s", status, raw)
	}
	if !strings.Contains(string(raw), `"records":[`) {
		t.Errorf("growth list envelope = %s", raw)
	}
}

func TestServeGrowthEmptyList(t *testing.T) {
	srv, ms := newGrowthTestServer(t, &fakeGrowthStore{})
	member, _ := ms.CreateMember(context.Background(), pipeline.MemberProfile{
		Name: "小虎", Sex: pipeline.SexMale, BirthDate: "2020-05-20",
	})
	status, raw := doJSON(t, srv, http.MethodGet, "/api/v1/members/"+strconv.FormatInt(member.MemberID, 10)+"/growth", "")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", status, raw)
	}
	if !strings.Contains(string(raw), `"records":[]`) {
		t.Fatalf("empty growth list = %s, want records:[]", raw)
	}
}

func TestServeGrowthValidation(t *testing.T) {
	srv, ms := newGrowthTestServer(t, &fakeGrowthStore{})
	member, _ := ms.CreateMember(context.Background(), pipeline.MemberProfile{
		Name: "小虎", Sex: pipeline.SexMale, BirthDate: "2020-05-20",
	})
	path := "/api/v1/members/" + strconv.FormatInt(member.MemberID, 10) + "/growth"

	cases := []struct {
		name string
		body string
	}{
		{"missing measured_on", `{"height_cm":118.5,"source":"measured"}`},
		{"missing source", `{"measured_on":"2026-10-01","height_cm":118.5}`},
		{"invalid source", `{"measured_on":"2026-10-01","height_cm":118.5,"source":"scale"}`},
		{"missing measurements", `{"measured_on":"2026-10-01","source":"measured"}`},
		{"invalid date", `{"measured_on":"2026-02-30","height_cm":118.5,"source":"measured"}`},
		{"future date", `{"measured_on":"2999-01-01","height_cm":118.5,"source":"measured"}`},
		{"zero height", `{"measured_on":"2026-10-01","height_cm":0,"source":"measured"}`},
		{"negative weight", `{"measured_on":"2026-10-01","weight_kg":-1,"source":"measured"}`},
		{"unknown field", `{"measured_on":"2026-10-01","height_cm":118.5,"source":"measured","age":3}`},
		{"malformed", `{"measured_on":`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, raw := doJSON(t, srv, http.MethodPost, path, c.body)
			if status != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", status, raw)
			}
			var apiErr httpapi.Error
			if err := json.Unmarshal(raw, &apiErr); err != nil || apiErr.Message == "" {
				t.Fatalf("error body = %s", raw)
			}
		})
	}
}

func TestServeGrowthUnknownMember(t *testing.T) {
	srv, _ := newGrowthTestServer(t, &fakeGrowthStore{})
	for _, path := range []string{
		"/api/v1/members/999/growth",
		"/api/v1/members/not-a-number/growth",
	} {
		status, raw := doJSON(t, srv, http.MethodGet, path, "")
		if status != http.StatusNotFound {
			t.Fatalf("GET %s status = %d, want 404; body=%s", path, status, raw)
		}
	}
}

func TestServeGrowthStoreError(t *testing.T) {
	srv, ms := newGrowthTestServer(t, &fakeGrowthStore{err: errors.New("boom")})
	member, _ := ms.CreateMember(context.Background(), pipeline.MemberProfile{
		Name: "小虎", Sex: pipeline.SexMale, BirthDate: "2020-05-20",
	})
	path := "/api/v1/members/" + strconv.FormatInt(member.MemberID, 10) + "/growth"
	status, raw := doJSON(t, srv, http.MethodGet, path, "")
	if status != http.StatusInternalServerError {
		t.Fatalf("GET status = %d, want 500; body=%s", status, raw)
	}
}
