package 反向代理

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	访问列表 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/accesslist"
	"git.asteriaxlabs.com/willtanoe/asterwaf/internal/clientip"
	"git.asteriaxlabs.com/willtanoe/asterwaf/internal/config"
	"git.asteriaxlabs.com/willtanoe/asterwaf/internal/engine"
	限流包 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/ratelimit"
	存储包 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/store"
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
	处理器, 错误 := U新建(配置值, 解析器, 引擎, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if 错误 != nil {
		t.Fatalf("proxy: %v", 错误)
	}
	t.Cleanup(处理器.U关闭空闲连接)
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
			}, 解析器, 引擎, nil, 日志)
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

func Test访问列表优先于规则与请求体限制(t *testing.T) {
	var 上游次数 int32
	上游 := httptest.NewServer(http.HandlerFunc(func(写 http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&上游次数, 1)
		_, _ = 写.Write([]byte("ok"))
	}))
	defer 上游.Close()

	处理器, 日志缓冲 := 新建带日志(t, 配置.U配置{
		U模式: "enforce",
		U站点: []配置.U站点{{U主机: "app.example.com", U上游: 上游.URL, U最大请求体: 8}},
		U访问列表: 访问列表.U列表{U条目: []访问列表.U条目{
			{UID: "deny-me", U动作: "deny", U地址: "198.51.100.7", U启用: true},
			{UID: "allow-me-too", U动作: "allow", U地址: "198.51.100.7", U启用: true},
		}},
	})

	请求 := httptest.NewRequest("POST", "http://app.example.com/", strings.NewReader(strings.Repeat("x", 100)))
	请求.RemoteAddr = "198.51.100.7:1234"
	记录 := httptest.NewRecorder()
	处理器.ServeHTTP(记录, 请求)
	if 记录.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", 记录.Code)
	}
	if atomic.LoadInt32(&上游次数) != 0 {
		t.Fatal("denied request reached upstream")
	}
	if !strings.Contains(日志缓冲.String(), `"rule_id":"acl:deny"`) {
		t.Fatalf("deny decision not logged: %s", 日志缓冲.String())
	}
	if !strings.Contains(日志缓冲.String(), `"category":"accesslist"`) {
		t.Fatalf("accesslist category missing: %s", 日志缓冲.String())
	}
}

func Test允许列表跳过访问列表但仍受规则约束(t *testing.T) {
	var 上游次数 int32
	上游 := httptest.NewServer(http.HandlerFunc(func(写 http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&上游次数, 1)
		_, _ = 写.Write([]byte("ok"))
	}))
	defer 上游.Close()

	配置值 := 配置.U配置{
		U模式: "enforce",
		U站点: []配置.U站点{{U主机: "app.example.com", U上游: 上游.URL}},
		U访问列表: 访问列表.U列表{U仅允许: true, U条目: []访问列表.U条目{
			{UID: "office", U动作: "allow", U地址: "203.0.113.0/24", U启用: true},
		}},
	}

	处理器, 日志缓冲 := 新建带日志(t, 配置值)
	未允许 := httptest.NewRequest("GET", "http://app.example.com/", nil)
	未允许.RemoteAddr = "198.51.100.7:1234"
	记录 := httptest.NewRecorder()
	处理器.ServeHTTP(记录, 未允许)
	if 记录.Code != http.StatusForbidden {
		t.Fatalf("allow_only status=%d", 记录.Code)
	}
	if !strings.Contains(日志缓冲.String(), `"rule_id":"acl:allow-only"`) {
		t.Fatalf("allow-only decision missing: %s", 日志缓冲.String())
	}

	日志缓冲.Reset()
	允许正常 := httptest.NewRequest("GET", "http://app.example.com/rest/products/search?q=apple", nil)
	允许正常.RemoteAddr = "203.0.113.7:1234"
	记录 = httptest.NewRecorder()
	处理器.ServeHTTP(记录, 允许正常)
	if 记录.Code != http.StatusOK {
		t.Fatalf("allowlisted normal status=%d", 记录.Code)
	}

	允许攻击 := httptest.NewRequest("GET", "http://app.example.com/rest/products/search?q=1%20union%20select%201", nil)
	允许攻击.RemoteAddr = "203.0.113.7:1234"
	记录 = httptest.NewRecorder()
	处理器.ServeHTTP(记录, 允许攻击)
	if 记录.Code != http.StatusForbidden {
		t.Fatalf("allowlisted attack status=%d", 记录.Code)
	}
	if !strings.Contains(日志缓冲.String(), `"rule_id":"sqli.union-select"`) {
		t.Fatalf("rule did not evaluate allowlisted request: %s", 日志缓冲.String())
	}
	if atomic.LoadInt32(&上游次数) != 1 {
		t.Fatalf("upstream calls=%d, want 1", 上游次数)
	}
}

func Test访问列表不采用伪造XFF(t *testing.T) {
	上游 := httptest.NewServer(http.HandlerFunc(func(写 http.ResponseWriter, _ *http.Request) {
		_, _ = 写.Write([]byte("ok"))
	}))
	defer 上游.Close()

	处理器, _ := 新建带日志(t, 配置.U配置{
		U模式: "enforce",
		U站点: []配置.U站点{{U主机: "app.example.com", U上游: 上游.URL}},
		U访问列表: 访问列表.U列表{U条目: []访问列表.U条目{
			{UID: "deny-real", U动作: "deny", U地址: "198.51.100.7", U启用: true},
		}},
	})
	请求 := httptest.NewRequest("GET", "http://app.example.com/", nil)
	请求.RemoteAddr = "198.51.100.7:1234"
	请求.Header.Set("X-Forwarded-For", "203.0.113.7")
	记录 := httptest.NewRecorder()
	处理器.ServeHTTP(记录, 请求)
	if 记录.Code != http.StatusForbidden {
		t.Fatalf("spoofed XFF bypassed deny: status=%d", 记录.Code)
	}
}

func 新建带日志(t *testing.T, 配置值 配置.U配置) (*U处理器, *bytes.Buffer) {
	t.Helper()
	解析器, 错误 := 客户端地址.U新建(配置值.U受信代理)
	if 错误 != nil {
		t.Fatalf("resolver: %v", 错误)
	}
	引擎, 错误 := 引擎.U新建(引擎.U默认规则())
	if 错误 != nil {
		t.Fatalf("engine: %v", 错误)
	}
	var 缓冲 bytes.Buffer
	处理器, 错误 := U新建(配置值, 解析器, 引擎, nil, slog.New(slog.NewJSONHandler(&缓冲, nil)))
	if 错误 != nil {
		t.Fatalf("proxy: %v", 错误)
	}
	t.Cleanup(处理器.U关闭空闲连接)
	return 处理器, &缓冲
}

func Test限流返回429(t *testing.T) {
	上游 := httptest.NewServer(http.HandlerFunc(func(写 http.ResponseWriter, _ *http.Request) {
		_, _ = 写.Write([]byte("ok"))
	}))
	defer 上游.Close()

	处理器, 日志缓冲 := 新建带日志(t, 配置.U配置{
		U模式: "enforce",
		U站点: []配置.U站点{{U主机: "app.example.com", U上游: 上游.URL, U最大请求体: 8}},
		U限流: 限流包.U配置{U启用: true, U每分钟: 1, U突发: 1},
	})

	第一次 := httptest.NewRequest("GET", "http://app.example.com/", nil)
	第一次.RemoteAddr = "203.0.113.7:1234"
	记录 := httptest.NewRecorder()
	处理器.ServeHTTP(记录, 第一次)
	if 记录.Code != http.StatusOK {
		t.Fatalf("first status=%d", 记录.Code)
	}

	第二次 := httptest.NewRequest("POST", "http://app.example.com/", strings.NewReader(strings.Repeat("x", 100)))
	第二次.RemoteAddr = "203.0.113.7:1234"
	记录 = httptest.NewRecorder()
	处理器.ServeHTTP(记录, 第二次)
	if 记录.Code != http.StatusTooManyRequests {
		t.Fatalf("second status=%d, want 429", 记录.Code)
	}
	if 记录.Header().Get("Retry-After") == "" {
		t.Fatal("Retry-After header missing")
	}
	if !strings.Contains(日志缓冲.String(), `"action":"ratelimited"`) {
		t.Fatalf("rate limit decision not logged: %s", 日志缓冲.String())
	}
	if !strings.Contains(日志缓冲.String(), `"rule_id":"ratelimit"`) {
		t.Fatalf("rate limit rule id missing: %s", 日志缓冲.String())
	}

	其他 := httptest.NewRequest("GET", "http://app.example.com/", nil)
	其他.RemoteAddr = "203.0.113.8:1234"
	记录 = httptest.NewRecorder()
	处理器.ServeHTTP(记录, 其他)
	if 记录.Code != http.StatusOK {
		t.Fatalf("other address status=%d", 记录.Code)
	}
}

func Test白名单跳过限流(t *testing.T) {
	上游 := httptest.NewServer(http.HandlerFunc(func(写 http.ResponseWriter, _ *http.Request) {
		_, _ = 写.Write([]byte("ok"))
	}))
	defer 上游.Close()

	处理器, _ := 新建带日志(t, 配置.U配置{
		U模式: "enforce",
		U站点: []配置.U站点{{U主机: "app.example.com", U上游: 上游.URL}},
		U限流: 限流包.U配置{U启用: true, U每分钟: 1, U突发: 1},
		U访问列表: 访问列表.U列表{U条目: []访问列表.U条目{
			{UID: "trusted", U动作: "allow", U地址: "203.0.113.8", U启用: true},
		}},
	})
	for 次数 := 0; 次数 < 2; 次数++ {
		请求 := httptest.NewRequest("GET", "http://app.example.com/", nil)
		请求.RemoteAddr = "203.0.113.8:1234"
		记录 := httptest.NewRecorder()
		处理器.ServeHTTP(记录, 请求)
		if 记录.Code != http.StatusOK {
			t.Fatalf("allowlisted request %d status=%d", 次数, 记录.Code)
		}
	}
}

func Test请求日志落库与脱敏(t *testing.T) {
	路径 := filepath.Join(t.TempDir(), "waf.db")
	存储, 错误 := 存储包.U打开(路径, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if 错误 != nil {
		t.Fatalf("store: %v", 错误)
	}

	上游 := httptest.NewServer(http.HandlerFunc(func(写 http.ResponseWriter, _ *http.Request) {
		_, _ = 写.Write([]byte("ok"))
	}))
	defer 上游.Close()

	解析器, 错误 := 客户端地址.U新建(nil)
	if 错误 != nil {
		t.Fatalf("resolver: %v", 错误)
	}
	引擎, 错误 := 引擎.U新建(引擎.U默认规则())
	if 错误 != nil {
		t.Fatalf("engine: %v", 错误)
	}
	处理器, 错误 := U新建(配置.U配置{
		U模式: "enforce",
		U站点: []配置.U站点{{U主机: "app.example.com", U上游: 上游.URL, U模式: "enforce"}},
	}, 解析器, 引擎, 存储, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if 错误 != nil {
		t.Fatalf("proxy: %v", 错误)
	}

	允许 := httptest.NewRequest("POST", "http://app.example.com/login", strings.NewReader("user=bob&password=hunter2"))
	允许.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	允许.Header.Set("Cookie", "session=secret-cookie")
	允许.RemoteAddr = "203.0.113.10:1234"
	允许记录 := httptest.NewRecorder()
	处理器.ServeHTTP(允许记录, 允许)
	if 允许记录.Code != http.StatusOK {
		t.Fatalf("allow status=%d", 允许记录.Code)
	}
	允许ID := 允许记录.Header().Get("X-AsterWAF-Request-ID")
	if len(允许ID) != 16 {
		t.Fatalf("request id=%q", 允许ID)
	}

	拦截 := httptest.NewRequest("GET", "http://app.example.com/search?q=1%20union%20select%201", nil)
	拦截.RemoteAddr = "203.0.113.11:1234"
	拦截.Header.Set("User-Agent", "sqlmap/1.8")
	拦截记录 := httptest.NewRecorder()
	处理器.ServeHTTP(拦截记录, 拦截)
	if 拦截记录.Code != http.StatusForbidden {
		t.Fatalf("block status=%d", 拦截记录.Code)
	}
	拦截ID := 拦截记录.Header().Get("X-AsterWAF-Request-ID")

	阴影 := httptest.NewRequest("POST", "http://app.example.com/search?q=1%20union%20select%201", strings.NewReader("q=x&token=top-secret"))
	阴影.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	阴影.RemoteAddr = "203.0.113.12:1234"
	阴影记录 := httptest.NewRecorder()
	阴影处理器, 错误 := U新建(配置.U配置{
		U模式: "shadow",
		U站点: []配置.U站点{{U主机: "app.example.com", U上游: 上游.URL, U模式: "shadow"}},
	}, 解析器, 引擎, 存储, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if 错误 != nil {
		t.Fatalf("shadow proxy: %v", 错误)
	}
	阴影处理器.ServeHTTP(阴影记录, 阴影)
	if 阴影记录.Code != http.StatusOK {
		t.Fatalf("shadow status=%d", 阴影记录.Code)
	}
	阴影ID := 阴影记录.Header().Get("X-AsterWAF-Request-ID")

	处理器.U关闭空闲连接()
	阴影处理器.U关闭空闲连接()
	上下文, 取消 := context.WithTimeout(context.Background(), 5*time.Second)
	defer 取消()
	if 错误 := 存储.Close(上下文); 错误 != nil {
		t.Fatalf("close store: %v", 错误)
	}

	数据库, 错误 := sql.Open("sqlite", 路径)
	if 错误 != nil {
		t.Fatalf("reopen: %v", 错误)
	}
	defer func() { _ = 数据库.Close() }()

	var 动作, 类别, 规则, 请求头, 请求体 string
	var 影子 int
	if 错误 := 数据库.QueryRow(`SELECT action, category, rule_id, shadow, request_headers, request_body FROM logs WHERE request_id = ?`, 允许ID).Scan(&动作, &类别, &规则, &影子, &请求头, &请求体); 错误 != nil {
		t.Fatalf("allow row: %v", 错误)
	}
	if 动作 != "allow" || 影子 != 0 {
		t.Fatalf("allow row action=%s shadow=%d", 动作, 影子)
	}
	if 请求头 != "" || 请求体 != "" {
		t.Fatalf("allow request stored headers/body: %q %q", 请求头, 请求体)
	}

	if 错误 := 数据库.QueryRow(`SELECT action, category, rule_id, shadow, request_headers, request_body FROM logs WHERE request_id = ?`, 拦截ID).Scan(&动作, &类别, &规则, &影子, &请求头, &请求体); 错误 != nil {
		t.Fatalf("block row: %v", 错误)
	}
	if 动作 != "block" || 类别 != "sqli" || 规则 != "sqli.union-select" || 影子 != 0 {
		t.Fatalf("block row action=%s category=%s rule=%s shadow=%d", 动作, 类别, 规则, 影子)
	}
	if !strings.Contains(请求头, "sqlmap") {
		t.Fatalf("block headers not stored: %s", 请求头)
	}

	if 错误 := 数据库.QueryRow(`SELECT action, category, rule_id, shadow, request_headers, request_body FROM logs WHERE request_id = ?`, 阴影ID).Scan(&动作, &类别, &规则, &影子, &请求头, &请求体); 错误 != nil {
		t.Fatalf("shadow row: %v", 错误)
	}
	if 动作 != "shadow" || 影子 != 1 {
		t.Fatalf("shadow row action=%s shadow=%d", 动作, 影子)
	}
	if strings.Contains(请求体, "top-secret") {
		t.Fatalf("shadow body leaked token: %s", 请求体)
	}
	if !strings.Contains(请求体, "<redacted>") {
		t.Fatalf("shadow body was not redacted: %s", 请求体)
	}
}
