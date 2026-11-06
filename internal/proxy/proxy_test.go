package 反向代理

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"git.asteriaxlabs.com/willtanoe/asterwaf/internal/clientip"
	"git.asteriaxlabs.com/willtanoe/asterwaf/internal/config"
	"git.asteriaxlabs.com/willtanoe/asterwaf/internal/engine"
)

func Test站点路由(t *testing.T) {
	上游 := func(标识 string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(写 http.ResponseWriter, _ *http.Request) {
			_, _ = 写.Write([]byte(标识))
		}))
	}
	精确 := 上游("exact")
	通配 := 上游("wildcard")
	深层 := 上游("deep")
	defer 精确.Close()
	defer 通配.Close()
	defer 深层.Close()

	处理器 := 新建处理器(t, 配置.U配置{
		U模式: "enforce",
		U站点: []配置.U站点{
			{U主机: "*.example.com", U上游: 通配.URL},
			{U主机: "*.api.example.com", U上游: 深层.URL},
			{U主机: "app.example.com", U上游: 精确.URL},
		},
	}, nil)

	测试用例 := []struct {
		主机 string
		状态 int
		内容 string
	}{
		{"app.example.com", http.StatusOK, "exact"},
		{"app.example.com:8080", http.StatusOK, "exact"},
		{"x.api.example.com", http.StatusOK, "deep"},
		{"api.example.com", http.StatusOK, "wildcard"},
		{"example.com", http.StatusNotFound, "no site configured for host\n"},
		{"unknown.test", http.StatusNotFound, "no site configured for host\n"},
	}
	for _, 用例 := range 测试用例 {
		t.Run(用例.主机, func(t *testing.T) {
			请求 := httptest.NewRequest("GET", "http://"+用例.主机+"/", nil)
			记录 := httptest.NewRecorder()
			处理器.ServeHTTP(记录, 请求)
			if 记录.Code != 用例.状态 {
				t.Fatalf("status=%d, want %d", 记录.Code, 用例.状态)
			}
			if 用例.状态 == http.StatusOK && 记录.Body.String() != 用例.内容 {
				t.Fatalf("body=%q, want %q", 记录.Body.String(), 用例.内容)
			}
		})
	}
}

func Test不受信对端的转发头被覆盖(t *testing.T) {
	上游 := httptest.NewServer(http.HandlerFunc(func(写 http.ResponseWriter, 请求 *http.Request) {
		写.Header().Set("X-Seen-Xff", 请求.Header.Get("X-Forwarded-For"))
		写.Header().Set("X-Seen-Real", 请求.Header.Get("X-Real-IP"))
		写.Header().Set("X-Seen-Proto", 请求.Header.Get("X-Forwarded-Proto"))
		写.Header().Set("X-Seen-Forwarded", 请求.Header.Get("Forwarded"))
		写.Header().Set("X-Seen-Forwarded-Host", 请求.Header.Get("X-Forwarded-Host"))
		写.Header().Set("X-Seen-True-Client", 请求.Header.Get("True-Client-IP"))
		写.Header().Set("X-Seen-Cf", 请求.Header.Get("CF-Connecting-IP"))
		写.Header().Set("X-Seen-Client", 请求.Header.Get("X-Client-IP"))
		写.Header().Set("X-Seen-Host", 请求.Host)
	}))
	defer 上游.Close()

	处理器 := 新建处理器(t, 配置.U配置{
		U模式: "enforce",
		U站点: []配置.U站点{{U主机: "app.example.com", U上游: 上游.URL}},
	}, nil)

	请求 := httptest.NewRequest("GET", "http://app.example.com/", nil)
	请求.RemoteAddr = "198.51.100.7:4444"
	请求.Header.Set("X-Forwarded-For", "203.0.113.99")
	请求.Header.Set("X-Forwarded-Proto", "https")
	请求.Header.Set("X-Forwarded-Host", "evil.example.com")
	请求.Header.Set("Forwarded", "for=203.0.113.99")
	请求.Header.Set("True-Client-IP", "203.0.113.99")
	请求.Header.Set("CF-Connecting-IP", "203.0.113.99")
	请求.Header.Set("X-Client-IP", "203.0.113.99")
	记录 := httptest.NewRecorder()
	处理器.ServeHTTP(记录, 请求)

	if 记录.Code != http.StatusOK {
		t.Fatalf("status=%d", 记录.Code)
	}
	期望 := map[string]string{
		"X-Seen-Xff":            "198.51.100.7",
		"X-Seen-Real":           "198.51.100.7",
		"X-Seen-Proto":          "http",
		"X-Seen-Forwarded":      "",
		"X-Seen-Forwarded-Host": "",
		"X-Seen-True-Client":    "",
		"X-Seen-Cf":             "",
		"X-Seen-Client":         "",
		"X-Seen-Host":           "app.example.com",
	}
	for 键, 值 := range 期望 {
		if 实际 := 记录.Header().Get(键); 实际 != 值 {
			t.Fatalf("%s=%q, want %q", 键, 实际, 值)
		}
	}
}

func Test受信对端使用解析地址(t *testing.T) {
	上游 := httptest.NewServer(http.HandlerFunc(func(写 http.ResponseWriter, 请求 *http.Request) {
		写.Header().Set("X-Seen-Xff", 请求.Header.Get("X-Forwarded-For"))
		写.Header().Set("X-Seen-Proto", 请求.Header.Get("X-Forwarded-Proto"))
		写.Header().Set("X-Seen-Forwarded-Host", 请求.Header.Get("X-Forwarded-Host"))
	}))
	defer 上游.Close()

	处理器 := 新建处理器(t, 配置.U配置{
		U模式:   "enforce",
		U受信代理: []string{"10.0.0.0/8"},
		U站点:   []配置.U站点{{U主机: "app.example.com", U上游: 上游.URL}},
	}, []string{"10.0.0.0/8"})

	请求 := httptest.NewRequest("GET", "http://app.example.com/", nil)
	请求.RemoteAddr = "10.1.2.3:4444"
	请求.Header.Set("X-Forwarded-For", "203.0.113.9, 10.1.2.4")
	请求.Header.Set("X-Forwarded-Proto", "https")
	请求.Header.Set("X-Forwarded-Host", "evil.example.com")
	记录 := httptest.NewRecorder()
	处理器.ServeHTTP(记录, 请求)

	if 记录.Code != http.StatusOK {
		t.Fatalf("status=%d", 记录.Code)
	}
	if 值 := 记录.Header().Get("X-Seen-Xff"); 值 != "203.0.113.9" {
		t.Fatalf("xff=%q", 值)
	}
	if 值 := 记录.Header().Get("X-Seen-Proto"); 值 != "https" {
		t.Fatalf("proto=%q", 值)
	}
	if 值 := 记录.Header().Get("X-Seen-Forwarded-Host"); 值 != "" {
		t.Fatalf("forwarded host leaked: %q", 值)
	}
}

func Test请求体逐字节转发(t *testing.T) {
	原体 := []byte{0x00, 0x01, 0xff, 0xfe, 'a', '\n', 0x80, 0x7f}
	期望 := sha256.Sum256(原体)
	上游 := httptest.NewServer(http.HandlerFunc(func(写 http.ResponseWriter, 请求 *http.Request) {
		体, _ := io.ReadAll(请求.Body)
		和 := sha256.Sum256(体)
		写.Header().Set("X-Seen-Sha", hex.EncodeToString(和[:]))
	}))
	defer 上游.Close()

	处理器 := 新建处理器(t, 配置.U配置{
		U模式: "enforce",
		U站点: []配置.U站点{{U主机: "app.example.com", U上游: 上游.URL}},
	}, nil)

	请求 := httptest.NewRequest("POST", "http://app.example.com/upload", bytes.NewReader(原体))
	请求.Header.Set("Content-Type", "application/octet-stream")
	记录 := httptest.NewRecorder()
	处理器.ServeHTTP(记录, 请求)

	if 记录.Code != http.StatusOK {
		t.Fatalf("status=%d", 记录.Code)
	}
	if 实际 := 记录.Header().Get("X-Seen-Sha"); 实际 != hex.EncodeToString(期望[:]) {
		t.Fatalf("body changed in transit: sha=%s", 实际)
	}
}

func Test上游不可达返回502(t *testing.T) {
	处理器 := 新建处理器(t, 配置.U配置{
		U模式: "enforce",
		U站点: []配置.U站点{{U主机: "app.example.com", U上游: "http://127.0.0.1:1"}},
	}, nil)
	请求 := httptest.NewRequest("GET", "http://app.example.com/", nil)
	记录 := httptest.NewRecorder()
	处理器.ServeHTTP(记录, 请求)
	if 记录.Code != http.StatusBadGateway {
		t.Fatalf("status=%d", 记录.Code)
	}
	if !strings.Contains(记录.Body.String(), "upstream unavailable") {
		t.Fatalf("body=%q", 记录.Body.String())
	}
}

func 新建处理器(t *testing.T, 配置值 配置.U配置, 受信 []string) *U处理器 {
	t.Helper()
	解析器, 错误 := 客户端地址.U新建(受信)
	if 错误 != nil {
		t.Fatalf("resolver: %v", 错误)
	}
	引擎, 错误 := 引擎.U新建(引擎.U默认规则())
	if 错误 != nil {
		t.Fatalf("engine: %v", 错误)
	}
	处理器, 错误 := U新建(配置值, 解析器, 引擎, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if 错误 != nil {
		t.Fatalf("proxy: %v", 错误)
	}
	return 处理器
}

func Test请求体超过站点上限返回413(t *testing.T) {
	上游 := httptest.NewServer(http.HandlerFunc(func(写 http.ResponseWriter, _ *http.Request) {
		t.Error("upstream must not be called for an oversized body")
	}))
	defer 上游.Close()

	处理器 := 新建处理器(t, 配置.U配置{
		U模式: "enforce",
		U站点: []配置.U站点{{U主机: "app.example.com", U上游: 上游.URL, U最大请求体: 8}},
	}, nil)

	请求 := httptest.NewRequest("POST", "http://app.example.com/", strings.NewReader("0123456789"))
	记录 := httptest.NewRecorder()
	处理器.ServeHTTP(记录, 请求)
	if 记录.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d", 记录.Code)
	}
}

func Test压缩请求体原样转发(t *testing.T) {
	var 压缩 bytes.Buffer
	写器 := gzip.NewWriter(&压缩)
	_, _ = 写器.Write([]byte("user=admin&password=abc"))
	_ = 写器.Close()
	期望 := sha256.Sum256(压缩.Bytes())

	上游 := httptest.NewServer(http.HandlerFunc(func(写 http.ResponseWriter, 请求 *http.Request) {
		体, _ := io.ReadAll(请求.Body)
		和 := sha256.Sum256(体)
		写.Header().Set("X-Seen-Sha", hex.EncodeToString(和[:]))
	}))
	defer 上游.Close()

	处理器 := 新建处理器(t, 配置.U配置{
		U模式: "enforce",
		U站点: []配置.U站点{{U主机: "app.example.com", U上游: 上游.URL, U最大请求体: 1 << 20}},
	}, nil)

	请求 := httptest.NewRequest("POST", "http://app.example.com/login", bytes.NewReader(压缩.Bytes()))
	请求.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	请求.Header.Set("Content-Encoding", "gzip")
	记录 := httptest.NewRecorder()
	处理器.ServeHTTP(记录, 请求)
	if 记录.Code != http.StatusOK {
		t.Fatalf("status=%d", 记录.Code)
	}
	if 实际 := 记录.Header().Get("X-Seen-Sha"); 实际 != hex.EncodeToString(期望[:]) {
		t.Fatalf("compressed body was altered in transit: %s", 实际)
	}
}

func Test规则模式(t *testing.T) {
	上游 := httptest.NewServer(http.HandlerFunc(func(写 http.ResponseWriter, _ *http.Request) {
		_, _ = 写.Write([]byte("upstream ok"))
	}))
	defer 上游.Close()

	测试用例 := []struct {
		模式 string
		状态 int
		动作 string
	}{
		{"enforce", http.StatusForbidden, "block"},
		{"shadow", http.StatusOK, "shadow"},
		{"off", http.StatusOK, ""},
	}
	for _, 用例 := range 测试用例 {
		t.Run(用例.模式, func(t *testing.T) {
			解析器, 错误 := 客户端地址.U新建(nil)
			if 错误 != nil {
				t.Fatalf("resolver: %v", 错误)
			}
			引擎, 错误 := 引擎.U新建(引擎.U默认规则())
			if 错误 != nil {
				t.Fatalf("engine: %v", 错误)
			}
			var 日志缓冲 bytes.Buffer
			日志 := slog.New(slog.NewJSONHandler(&日志缓冲, nil))
			处理器, 错误 := U新建(配置.U配置{
				U模式: 用例.模式,
				U站点: []配置.U站点{{U主机: "app.example.com", U上游: 上游.URL, U模式: 用例.模式}},
			}, 解析器, 引擎, 日志)
			if 错误 != nil {
				t.Fatalf("proxy: %v", 错误)
			}

			请求 := httptest.NewRequest("GET", "http://app.example.com/search?q=1%20union%20select%201", nil)
			记录 := httptest.NewRecorder()
			处理器.ServeHTTP(记录, 请求)
			if 记录.Code != 用例.状态 {
				t.Fatalf("status=%d, want %d", 记录.Code, 用例.状态)
			}
			if 用例.模式 == "enforce" {
				if 记录.Body.String() != "request blocked\n" {
					t.Fatalf("block body=%q", 记录.Body.String())
				}
				if strings.Contains(记录.Body.String(), "sqli") {
					t.Fatal("block response leaked rule details")
				}
			}
			if 用例.模式 == "enforce" || 用例.模式 == "shadow" {
				if !strings.Contains(日志缓冲.String(), `"rule_id":"sqli.union-select"`) {
					t.Fatalf("decision log missing rule id: %s", 日志缓冲.String())
				}
				if !strings.Contains(日志缓冲.String(), `"action":"`+用例.动作+`"`) {
					t.Fatalf("decision log missing action: %s", 日志缓冲.String())
				}
			} else if strings.Contains(日志缓冲.String(), "rule_id") {
				t.Fatalf("off mode logged a rule decision: %s", 日志缓冲.String())
			}
			if 用例.模式 == "shadow" && !strings.Contains(日志缓冲.String(), `"shadow":true`) {
				t.Fatalf("shadow flag missing: %s", 日志缓冲.String())
			}
		})
	}
}
