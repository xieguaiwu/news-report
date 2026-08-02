package fetch

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestBytesBasic(t *testing.T) {
	var gotUA atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA.Store(r.UserAgent())
		fmt.Fprint(w, "hello feed")
	}))
	defer srv.Close()

	f := New(Options{UserAgent: "test-agent/1.0", Timeout: 5 * time.Second, Retries: 1})
	body, err := f.Bytes(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	if string(body) != "hello feed" {
		t.Errorf("响应内容错误: %q", body)
	}
	if ua, _ := gotUA.Load().(string); ua != "test-agent/1.0" {
		t.Errorf("UA 错误: %q", ua)
	}
}

func TestBytesRetryOn500(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			fmt.Fprint(w, "User-agent: *\nDisallow:\n")
			return
		}
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			w.WriteHeader(503)
			return
		}
		fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	f := New(Options{Timeout: 5 * time.Second, Retries: 3})
	body, err := f.Bytes(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("重试后仍失败: %v", err)
	}
	if string(body) != "ok" {
		t.Errorf("重试后响应错误: %q", body)
	}
	if calls != 3 {
		t.Errorf("应重试 2 次共 3 次数据请求，实际 %d", calls)
	}
}

func TestBytesNoRetryOn404(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			fmt.Fprint(w, "User-agent: *\nDisallow:\n")
			return
		}
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(404)
	}))
	defer srv.Close()

	f := New(Options{Timeout: 5 * time.Second, Retries: 3})
	if _, err := f.Bytes(context.Background(), srv.URL); err == nil {
		t.Fatal("404 应报错")
	}
	if calls != 1 {
		t.Errorf("4xx 不应重试，实际 %d 次数据请求", calls)
	}
}

func TestBytesFollowsRedirect(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/old", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/new", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/new", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "redirected")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	f := New(Options{Timeout: 5 * time.Second})
	body, err := f.Bytes(context.Background(), srv.URL+"/old")
	if err != nil {
		t.Fatalf("重定向: %v", err)
	}
	if string(body) != "redirected" {
		t.Errorf("重定向后内容错误: %q", body)
	}
}

func TestRobotsDisallow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			fmt.Fprint(w, "User-agent: *\nDisallow: /private/\n")
		default:
			fmt.Fprint(w, "content")
		}
	}))
	defer srv.Close()

	f := New(Options{Timeout: 5 * time.Second})
	if _, err := f.Bytes(context.Background(), srv.URL+"/private/data"); err == nil {
		t.Error("robots 禁止的路径应报错")
	}
	if _, err := f.Bytes(context.Background(), srv.URL+"/public/data"); err != nil {
		t.Errorf("robots 允许的路径不应报错: %v", err)
	}
}

func TestRobotsDisabledFlag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			fmt.Fprint(w, "User-agent: *\nDisallow: /\n")
		default:
			fmt.Fprint(w, "content")
		}
	}))
	defer srv.Close()

	f := New(Options{Timeout: 5 * time.Second, NoRobots: true})
	if _, err := f.Bytes(context.Background(), srv.URL+"/anything"); err != nil {
		t.Errorf("NoRobots 应跳过检查: %v", err)
	}
}

func TestRobotsGlobPattern(t *testing.T) {
	re := globToRegexp("/private/*.pdf$")
	if re == nil {
		t.Fatal("globToRegexp 返回 nil")
	}
	if !re.MatchString("/private/report.pdf") {
		t.Error("应匹配 /private/report.pdf")
	}
	if re.MatchString("/private/report.txt") {
		t.Error("不应匹配非 pdf")
	}
}

func TestStripHTMLTags(t *testing.T) {
	got := StripHTMLTags("<p>Hello <b>world</b></p>  ")
	if got != "Hello world" {
		t.Errorf("StripHTMLTags 结果: %q", got)
	}
}

func TestTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer srv.Close()

	f := New(Options{Timeout: 300 * time.Millisecond, Retries: 0})
	start := time.Now()
	if _, err := f.Bytes(context.Background(), srv.URL); err == nil {
		t.Error("慢响应应超时报错")
	}
	if time.Since(start) > 2*time.Second {
		t.Error("超时控制失效")
	}
}

func TestIsRetriable(t *testing.T) {
	if !IsRetriable(&StatusError{Code: 503, Status: "Service Unavailable"}) {
		t.Error("503 应可重试")
	}
	if IsRetriable(&StatusError{Code: 404, Status: "Not Found"}) {
		t.Error("404 不应重试")
	}
	if !IsRetriable(fmt.Errorf("dial tcp: connection refused")) {
		t.Error("网络错误应可重试")
	}
	if IsRetriable(nil) {
		t.Error("nil 不应可重试")
	}
	if !strings.Contains(LangHeader("de"), "de-DE") {
		t.Error("LangHeader 应包含 de-DE")
	}
}
