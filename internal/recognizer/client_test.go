package recognizer_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"healthyduoduo/internal/contract"
	healthyduoduo "healthyduoduo/internal/recognizer"
)

// 端点路径与 sampleResponse 报文都收敛为常量;报文对应 schemas/testdata 种子(实施计划 4.1/4.2)。
const (
	reportPath  = "/report"
	reparsePath = "/reparse"

	reportBundleJSON  = `{"ocr_result":{"txts":["血红蛋白","128","g/L"],"boxes":[[[100,200],[180,200],[180,230],[100,230]],[[300,200],[360,200],[360,230],[300,230]],[[400,200],[460,200],[460,230],[400,230]]],"scores":[0.99,0.98,0.97],"elapse":1.23,"elapse_list":[0.4,0.05,0.78],"engine":"onnxruntime","model_info":{"det":"PP-OCRv6_small","cls":"PP-OCRv4_mobile","rec":"PP-OCRv6_small"}},"report":{"report_type":"血常规","report_date":"2026-10-01","status":"success","items":[{"name":"血红蛋白","value":128,"unit":"g/L","ref_range":"115-150","flag":"normal","low_confidence":false,"raw_text":"血红蛋白 128 g/L 115-150"}]}}`
	reparseReportJSON = `{"report_type":"血常规","report_date":null,"status":"partial","items":[]}`
)

// httptest 识别服务:按 /report 与 /reparse 分别回 schema 种子示例。
func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc(reportPath, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			t.Errorf("multipart parse: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// 请求侧断言:image 文件必须存在;date 可选字段取值仅接受测试已知补录值
		if _, _, err := r.FormFile("image"); err != nil {
			t.Errorf("multipart image form file: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		switch date := r.FormValue("date"); date {
		case "": // 未要求补录
		case "2026-10-01": // 测试的补录日期
		default:
			t.Errorf("unexpected date field %q", date)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(reportBundleJSON))
	})
	mux.HandleFunc(reparsePath, func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			OCRResult *contract.OCRResult `json:"ocr_result"`
			Date      *string             `json:"date"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			t.Errorf("reparse body decode: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if in.OCRResult == nil || len(in.OCRResult.Txts) == 0 {
			t.Error("reparse payload missing ocr_result")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(reparseReportJSON))
	})
	return httptest.NewServer(mux)
}

func sampleOCR() *contract.OCRResult {
	return &contract.OCRResult{Txts: []string{"血红蛋白", "128"}, Engine: "onnxruntime"}
}

func TestReport_ParsesBundle(t *testing.T) {
	srv := newServer(t)
	defer srv.Close()
	c := healthyduoduo.New(srv.URL)

	ocr, report, err := c.Report(context.Background(), []byte("fake"), "sample.jpeg", "")
	if err != nil {
		t.Fatalf("Report: %v", err)
	}
	if ocr.Engine != "onnxruntime" || len(ocr.Boxes) != 3 {
		t.Errorf("ocr = %+v", ocr)
	}
	if report.ReportType != "血常规" || report.Status != "success" {
		t.Errorf("report header = %+v", report)
	}
	if report.Items[0].Value == nil || *report.Items[0].Value != 128 {
		t.Errorf("item value = %+v", report.Items[0])
	}
}

func TestReport_SendsDateField(t *testing.T) {
	srv := newServer(t)
	defer srv.Close()
	c := healthyduoduo.New(srv.URL)
	if _, _, err := c.Report(context.Background(), []byte("fake"), "j.jpg", "2026-10-01"); err != nil {
		t.Fatalf("Report with date: %v", err)
	}
}

func TestReparse(t *testing.T) {
	srv := newServer(t)
	defer srv.Close()
	c := healthyduoduo.New(srv.URL)

	rep, err := c.Reparse(context.Background(), sampleOCR(), "")
	if err != nil {
		t.Fatalf("Reparse: %v", err)
	}
	if rep.Status != "partial" || rep.ReportDate != nil {
		t.Errorf("reparse report = %+v", rep)
	}
}

func TestReparse_SendsDate(t *testing.T) {
	srv := newServer(t)
	defer srv.Close()
	c := healthyduoduo.New(srv.URL)
	if _, err := c.Reparse(context.Background(), sampleOCR(), "2026-01-02"); err != nil {
		t.Fatalf("Reparse with date: %v", err)
	}
}

func TestNonJSONErrorBubbles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	if _, _, err := healthyduoduo.New(srv.URL).Report(context.Background(), []byte("x"), "a.jpg", ""); err == nil {
		t.Fatal("expected error on 500")
	}
}
