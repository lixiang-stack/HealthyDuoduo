package main

// 成员端点 HTTP 集成测试(viz-and-client 票 05 验收:建/列/改、排序口径、
// 成员与存量无主报告独立存在)。httptest 起 handler 栈,持久化以内存 fake 打底
// (编排层边界 MemberStore,不在低层新增 seam);契约形状由 httpapi 镜像解码锚定。

import (
	"context"
	"encoding/json"
	"fmt"
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
// PATCH 只改传入字段、删除按 id 移除、缺失返回 nil / false。
type fakeMemberStore struct {
	mu     sync.Mutex
	nextID int64
	rows   []pipeline.Member
}

var _ pipeline.MemberStore = (*fakeMemberStore)(nil)

func (f *fakeMemberStore) CreateMember(ctx context.Context, profile pipeline.MemberProfile) (*pipeline.Member, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	m := pipeline.Member{
		MemberID: f.nextID, Name: profile.Name, IsChild: pipeline.IsChild(profile.BirthDate, time.Now()),
		Sex: profile.Sex, BirthDate: profile.BirthDate,
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

func (f *fakeMemberStore) UpdateMember(ctx context.Context, id int64, patch pipeline.MemberPatch) (*pipeline.Member, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.rows {
		if f.rows[i].MemberID != id {
			continue
		}
		if patch.Name != nil {
			f.rows[i].Name = *patch.Name
		}
		if patch.Sex != nil {
			f.rows[i].Sex = *patch.Sex
		}
		if patch.BirthDate != nil {
			f.rows[i].BirthDate = *patch.BirthDate
		}
		f.rows[i].IsChild = pipeline.IsChild(f.rows[i].BirthDate, time.Now())
		m := f.rows[i]
		return &m, nil
	}
	return nil, nil
}

func (f *fakeMemberStore) DeleteMember(ctx context.Context, id int64) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.rows {
		if f.rows[i].MemberID != id {
			continue
		}
		f.rows = append(f.rows[:i], f.rows[i+1:]...)
		return true, nil
	}
	return false, nil
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

// memberCreateBody 合法建成员请求体(必填字段齐全;性别 male;
// isChild 仅决定夹具用 2020 还是 1988 的生日,服务端按生日自行计算)。
func memberCreateBody(name string, isChild bool) string {
	birthDate := "1988-01-01"
	if isChild {
		birthDate = "2020-01-01"
	}
	return fmt.Sprintf(`{"name":%q,"sex":"male","birth_date":%q}`, name, birthDate)
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
// (name/is_child 各自单独可改,created_at 不变);删除见 TestServeMembersDelete。
func TestServeMembersCRUD(t *testing.T) {
	srv, _ := newMembersTestServer(t)

	child := createOne(t, srv, `{"name":"小虎","sex":"male","birth_date":"2020-05-20"}`)
	if child.MemberID != 1 || child.Name != "小虎" || !child.IsChild {
		t.Fatalf("created child = %+v", child)
	}
	if child.Sex != "male" || child.BirthDate != "2020-05-20" {
		t.Errorf("created child profile = %+v, want male/2020-05-20", child)
	}
	if child.CreatedAt != "2026-10-15T08:00:01Z" {
		t.Errorf("created_at = %q, want RFC3339 UTC timestamp", child.CreatedAt)
	}
	adult := createOne(t, srv, memberCreateBody("妈妈", false))
	second := createOne(t, srv, `{"name":"妹妹","sex":"female","birth_date":"2022-01-01"}`)
	if second.Sex != "female" || !second.IsChild {
		t.Errorf("created second profile = %+v, want female/child", second)
	}
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

	// 改生日:is_child 随生日实时重算(1988 → 2015 变为孩子),名字不动。
	status, raw = doJSON(t, srv, http.MethodPatch, "/api/v1/members/2", `{"birth_date":"2015-03-04"}`)
	if status != http.StatusOK {
		t.Fatalf("PATCH birth_date: status = %d, want 200; body = %s", status, raw)
	}
	var rechiled httpapi.Member
	if err := json.Unmarshal(raw, &rechiled); err != nil {
		t.Fatal(err)
	}
	if !rechiled.IsChild || rechiled.Name != "母亲" || rechiled.MemberID != 2 {
		t.Fatalf("updated member = %+v, want is_child true / name 母亲", rechiled)
	}

	// 改性别:只覆盖所带字段,生日与名字不动。
	status, raw = doJSON(t, srv, http.MethodPatch, "/api/v1/members/2", `{"sex":"female"}`)
	if status != http.StatusOK {
		t.Fatalf("PATCH profile: status = %d, want 200; body = %s", status, raw)
	}
	var reprofiled httpapi.Member
	if err := json.Unmarshal(raw, &reprofiled); err != nil {
		t.Fatal(err)
	}
	if reprofiled.Sex != "female" || reprofiled.BirthDate != "2015-03-04" ||
		reprofiled.Name != "母亲" || !reprofiled.IsChild {
		t.Fatalf("updated profile = %+v, want female/2015-03-04/child with name 母亲", reprofiled)
	}

	// 再列:改动落库、顺序不变。
	if got := memberIDs(t, srv); !slices.Equal(got, []int64{1, 2, 3}) {
		t.Fatalf("list after patches = %v, want [1 2 3]", got)
	}
}

// TestServeMembersDelete 删成员:成功 204 无响应体、列表移除;未知成员 404、
// 非法 id 400,均走契约 Error 形状。
func TestServeMembersDelete(t *testing.T) {
	srv, _ := newMembersTestServer(t)
	createOne(t, srv, memberCreateBody("小虎", true))
	createOne(t, srv, memberCreateBody("妈妈", false))

	status, raw := doJSON(t, srv, http.MethodDelete, "/api/v1/members/1", "")
	if status != http.StatusNoContent {
		t.Fatalf("DELETE member: status = %d, want 204; body = %s", status, raw)
	}
	if len(raw) != 0 {
		t.Errorf("204 body = %q, want empty", raw)
	}
	if got := memberIDs(t, srv); !slices.Equal(got, []int64{2}) {
		t.Fatalf("list after delete = %v, want [2]", got)
	}

	status, raw = doJSON(t, srv, http.MethodDelete, "/api/v1/members/999", "")
	if status != http.StatusNotFound {
		t.Fatalf("DELETE unknown: status = %d, want 404; body = %s", status, raw)
	}
	var apiErr httpapi.Error
	if err := json.Unmarshal(raw, &apiErr); err != nil {
		t.Fatalf("404 body is not an Error shape: %s", raw)
	}
	if apiErr.Message != memberNotFoundMessage {
		t.Errorf("404 message = %q, want %q", apiErr.Message, memberNotFoundMessage)
	}

	status, raw = doJSON(t, srv, http.MethodDelete, "/api/v1/members/abc", "")
	if status != http.StatusBadRequest {
		t.Fatalf("DELETE invalid id: status = %d, want 400; body = %s", status, raw)
	}

	// 再删已删成员:同样 404,不伪装成成功。
	status, _ = doJSON(t, srv, http.MethodDelete, "/api/v1/members/1", "")
	if status != http.StatusNotFound {
		t.Fatalf("DELETE deleted member: status = %d, want 404", status)
	}
}

// TestServeMembersCreateValidation 建成员校验:缺 name/sex/birth_date、空名/空白名、
// 非法枚举/日期、显式 null、类型不符、已移除的 is_child / precision、未知字段、
// 畸形 JSON 一律 400 且不建出成员。
func TestServeMembersCreateValidation(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"missing name", `{"sex":"male","birth_date":"2020-01-01"}`},
		{"empty name", `{"name":"","sex":"male","birth_date":"2020-01-01"}`},
		{"blank name", `{"name":"   ","sex":"male","birth_date":"2020-01-01"}`},
		{"null name", `{"name":null,"sex":"male","birth_date":"2020-01-01"}`},
		{"missing sex", `{"name":"小虎","birth_date":"2020-01-01"}`},
		{"null sex", `{"name":"小虎","sex":null,"birth_date":"2020-01-01"}`},
		{"invalid sex", `{"name":"小虎","sex":"other","birth_date":"2020-01-01"}`},
		{"removed unspecified sex", `{"name":"小虎","sex":"unspecified","birth_date":"2020-01-01"}`},
		{"missing birth_date", `{"name":"小虎","sex":"male"}`},
		{"null birth_date", `{"name":"小虎","sex":"male","birth_date":null}`},
		{"invalid birth_date", `{"name":"小虎","sex":"male","birth_date":"2020-13-01"}`},
		{"removed is_child", `{"name":"小虎","is_child":true,"sex":"male","birth_date":"2020-01-01"}`},
		{"removed precision", `{"name":"小虎","sex":"male","birth_date":"2020-01-01","birth_date_precision":"day"}`},
		{"wrong name type", `{"name":123,"sex":"male","birth_date":"2020-01-01"}`},
		{"unknown field", `{"name":"小虎","sex":"male","birth_date":"2020-01-01","age":3}`},
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
		{"null name", "/api/v1/members/1", `{"name":null}`, http.StatusBadRequest},
		{"null sex", "/api/v1/members/1", `{"sex":null}`, http.StatusBadRequest},
		{"null birth_date", "/api/v1/members/1", `{"birth_date":null}`, http.StatusBadRequest},
		{"invalid sex", "/api/v1/members/1", `{"sex":"other"}`, http.StatusBadRequest},
		{"removed unspecified sex", "/api/v1/members/1", `{"sex":"unspecified"}`, http.StatusBadRequest},
		{"invalid birth_date", "/api/v1/members/1", `{"birth_date":"2020-02-30"}`, http.StatusBadRequest},
		{"removed is_child", "/api/v1/members/1", `{"is_child":true}`, http.StatusBadRequest},
		{"removed precision", "/api/v1/members/1", `{"birth_date_precision":"day"}`, http.StatusBadRequest},
		{"unknown field", "/api/v1/members/1", `{"nickname":"小明"}`, http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, _ := newMembersTestServer(t)
			createOne(t, srv, memberCreateBody("小虎", true))
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
	createOne(t, srv, memberCreateBody("小虎", true))
	if got := memberIDs(t, srv); !slices.Equal(got, []int64{1}) {
		t.Fatalf("after create, list = %v, want [1]", got)
	}
}
