package main

// 成员端点 HTTP 集成测试(viz-and-client 票 05 验收:建/列/改、排序口径、
// 成员与存量无主报告独立存在)。httptest 起 handler 栈,持久化以内存 fake 打底
// (编排层边界 MemberStore,不在低层新增 seam);契约形状由 httpapi 镜像解码锚定。

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"healthyduoduo/internal/httpapi"
	"healthyduoduo/internal/pipeline"
)

// fakeMemberStore 内存版成员存储:行为对齐 storage 实现——创建序 id 升序、
// PATCH 只改传入字段、缺失返回 nil。
type fakeMemberStore struct {
	mu     sync.Mutex
	nextID int64
	rows   []pipeline.Member
}

var _ pipeline.MemberStore = (*fakeMemberStore)(nil)

func (f *fakeMemberStore) CreateMember(ctx context.Context, name string, isChild bool) (*pipeline.Member, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	m := pipeline.Member{
		MemberID: f.nextID, Name: name, IsChild: isChild,
		// 确定性时钟:基准 + id 秒,断言 created_at 的 RFC3339 序列化。
		CreatedAt: time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC).Add(time.Duration(f.nextID) * time.Second),
	}
	f.rows = append(f.rows, m)
	return &m, nil
}

func (f *fakeMemberStore) ListMembers(ctx context.Context) ([]pipeline.Member, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]pipeline.Member(nil), f.rows...), nil
}

func (f *fakeMemberStore) MemberByID(ctx context.Context, id int64) (*pipeline.Member, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.rows {
		if f.rows[i].MemberID == id {
			m := f.rows[i]
			return &m, nil
		}
	}
	return nil, nil
}

func (f *fakeMemberStore) UpdateMember(ctx context.Context, id int64, name *string, isChild *bool) (*pipeline.Member, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.rows {
		if f.rows[i].MemberID != id {
			continue
		}
		if name != nil {
			f.rows[i].Name = *name
		}
		if isChild != nil {
			f.rows[i].IsChild = *isChild
		}
		m := f.rows[i]
		return &m, nil
	}
	return nil, nil
}

// newMembersTestServer 成员端点集成测试的 handler 栈(空成员库 + CLI fakes 打底)。
func newMembersTestServer(t *testing.T) (*httptest.Server, *fakeMemberStore) {
	t.Helper()
	ms := &fakeMemberStore{}
	d := &deps{store: fakeStoreCLI{}, members: ms, rec: fakeRecognizerCLI{}, obj: fakeObjectsCLI{}}
	srv := httptest.NewServer(newServeHandler(d))
	t.Cleanup(srv.Close)
	return srv, ms
}

// doJSON 发一次 JSON 请求(空 body 省略 Content-Type),返回状态码与响应体。
func doJSON(t *testing.T, srv *httptest.Server, method, path, body string) (int, []byte) {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, srv.URL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, raw
}

// createOne 建一个成员并解码为契约镜像形状。
func createOne(t *testing.T, srv *httptest.Server, body string) httpapi.Member {
	t.Helper()
	status, raw := doJSON(t, srv, http.MethodPost, "/api/v1/members", body)
	if status != http.StatusCreated {
		t.Fatalf("POST /members %s: status = %d, want 201; body = %s", body, status, raw)
	}
	var m httpapi.Member
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("decode created member %s: %v", raw, err)
	}
	return m
}

// memberIDs 列表响应的 id 序列。
func memberIDs(t *testing.T, srv *httptest.Server) []int64 {
	t.Helper()
	status, raw := doJSON(t, srv, http.MethodGet, "/api/v1/members", "")
	if status != http.StatusOK {
		t.Fatalf("GET /members: status = %d, want 200; body = %s", status, raw)
	}
	var list httpapi.MembersList
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatalf("decode members list %s: %v", raw, err)
	}
	ids := make([]int64, 0, len(list.Members))
	for _, m := range list.Members {
		ids = append(ids, m.MemberID)
	}
	return ids
}

// TestServeMembersCRUD 建/列/改全回路:创建序(id 升序)列表口径、PATCH 只改传入字段
// (name/is_child 各自单独可改,created_at 不变)、成员无删除入口。
func TestServeMembersCRUD(t *testing.T) {
	srv, _ := newMembersTestServer(t)

	child := createOne(t, srv, `{"name":"小虎","is_child":true}`)
	if child.MemberID != 1 || child.Name != "小虎" || !child.IsChild {
		t.Fatalf("created child = %+v", child)
	}
	if child.CreatedAt != "2026-10-15T08:00:01Z" {
		t.Errorf("created_at = %q, want RFC3339 UTC timestamp", child.CreatedAt)
	}
	adult := createOne(t, srv, `{"name":"妈妈","is_child":false}`)
	second := createOne(t, srv, `{"name":"妹妹","is_child":true}`)
	if adult.MemberID != 2 || second.MemberID != 3 {
		t.Fatalf("created ids = %d/%d, want 2/3", adult.MemberID, second.MemberID)
	}

	// 列表口径 = 创建序(id 升序):混序创建也不重排。
	wantOrder := []int64{1, 2, 3}
	if got := memberIDs(t, srv); !slices.Equal(got, wantOrder) {
		t.Fatalf("list order = %v, want %v (creation order)", got, wantOrder)
	}

	// 改名:只传 name,is_child 与 created_at 保持原样。
	status, raw := doJSON(t, srv, http.MethodPatch, "/api/v1/members/2", `{"name":"母亲"}`)
	if status != http.StatusOK {
		t.Fatalf("PATCH name: status = %d, want 200; body = %s", status, raw)
	}
	var renamed httpapi.Member
	if err := json.Unmarshal(raw, &renamed); err != nil {
		t.Fatal(err)
	}
	if renamed.Name != "母亲" || renamed.IsChild || renamed.CreatedAt != "2026-10-15T08:00:02Z" {
		t.Fatalf("renamed member = %+v, want name 母亲 / is_child false / created_at unchanged", renamed)
	}

	// 改档:只传 is_child,名字不动(孩子/成人两档只有这两个字段可改)。
	status, raw = doJSON(t, srv, http.MethodPatch, "/api/v1/members/2", `{"is_child":true}`)
	if status != http.StatusOK {
		t.Fatalf("PATCH is_child: status = %d, want 200; body = %s", status, raw)
	}
	var rechiled httpapi.Member
	if err := json.Unmarshal(raw, &rechiled); err != nil {
		t.Fatal(err)
	}
	if !rechiled.IsChild || rechiled.Name != "母亲" || rechiled.MemberID != 2 {
		t.Fatalf("updated member = %+v, want is_child true / name 母亲", rechiled)
	}

	// 再列:改动落库、顺序不变。
	if got := memberIDs(t, srv); !slices.Equal(got, []int64{1, 2, 3}) {
		t.Fatalf("list after patches = %v, want [1 2 3]", got)
	}
}

// TestServeMembersCreateValidation 建成员校验:缺 name/is_child、空名/空白名、
// 显式 null、类型不符、未知字段、畸形 JSON 一律 400 且不建出成员。
func TestServeMembersCreateValidation(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"missing name", `{"is_child":true}`},
		{"empty name", `{"name":"","is_child":false}`},
		{"blank name", `{"name":"   ","is_child":true}`},
		{"null name", `{"name":null,"is_child":true}`},
		{"missing is_child", `{"name":"小虎"}`},
		{"null is_child", `{"name":"小虎","is_child":null}`},
		{"wrong name type", `{"name":123,"is_child":true}`},
		{"unknown field", `{"name":"小虎","is_child":true,"age":3}`},
		{"malformed json", `{"name":`},
		{"not an object", `["小虎"]`},
		{"empty body", ``},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, ms := newMembersTestServer(t)
			status, raw := doJSON(t, srv, http.MethodPost, "/api/v1/members", c.body)
			if status != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body = %s", status, raw)
			}
			var apiErr httpapi.Error
			if err := json.Unmarshal(raw, &apiErr); err != nil {
				t.Fatalf("400 body is not an Error shape: %s", raw)
			}
			if apiErr.Message == "" {
				t.Errorf("error message empty: %s", raw)
			}
			rows, err := ms.ListMembers(context.Background())
			if err != nil || len(rows) != 0 {
				t.Errorf("rejected create left %d member row(s) (err=%v), want 0", len(rows), err)
			}
		})
	}
}

// TestServeMembersPatchValidation 改成员校验:未知成员 404、非法 id 400、
// 空 PATCH、空白名、显式 null、未知字段 400;全部走契约 Error 形状。
func TestServeMembersPatchValidation(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		body   string
		status int
	}{
		{"unknown member", "/api/v1/members/999", `{"name":"小明"}`, http.StatusNotFound},
		{"non-numeric id", "/api/v1/members/abc", `{"name":"小明"}`, http.StatusBadRequest},
		{"empty patch", "/api/v1/members/1", `{}`, http.StatusBadRequest},
		{"blank name", "/api/v1/members/1", `{"name":"  "}`, http.StatusBadRequest},
		{"null name alongside is_child", "/api/v1/members/1", `{"name":null,"is_child":true}`, http.StatusBadRequest},
		{"null is_child alongside name", "/api/v1/members/1", `{"name":"小虎","is_child":null}`, http.StatusBadRequest},
		{"unknown field", "/api/v1/members/1", `{"nickname":"小明"}`, http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, _ := newMembersTestServer(t)
			createOne(t, srv, `{"name":"小虎","is_child":true}`)
			status, raw := doJSON(t, srv, http.MethodPatch, c.path, c.body)
			if status != c.status {
				t.Fatalf("status = %d, want %d; body = %s", status, c.status, raw)
			}
			var apiErr httpapi.Error
			if err := json.Unmarshal(raw, &apiErr); err != nil {
				t.Fatalf("error body is not an Error shape: %s", raw)
			}
			if apiErr.Message == "" {
				t.Errorf("error message empty: %s", raw)
			}
		})
	}
}

// TestServeMembersListEmptyEnvelope 空成员列表必须回 {"members":[]} 而非 null
// (客户端按空数组渲染空态);建后可独立增列。存量无主报告与成员独立存在由
// 存储冒烟(store_member_test.go)与共享栈 e2e 冒烟覆盖。
func TestServeMembersListEmptyEnvelope(t *testing.T) {
	srv, _ := newMembersTestServer(t)
	status, raw := doJSON(t, srv, http.MethodGet, "/api/v1/members", "")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", status, raw)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if got := string(body["members"]); got != "[]" {
		t.Fatalf("members = %s, want [] (empty array, not null)", got)
	}
	// 建一个成员后依旧可独立增列。
	createOne(t, srv, `{"name":"小虎","is_child":true}`)
	if got := memberIDs(t, srv); !slices.Equal(got, []int64{1}) {
		t.Fatalf("after create, list = %v, want [1]", got)
	}
}
