package 管理接口

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"git.asteriaxlabs.com/willtanoe/asterwaf/internal/clientip"
	配置 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/config"
	规则引擎 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/engine"
	存储包 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/store"
	"golang.org/x/crypto/bcrypt"
)

const 测试密码 = "correct-horse-battery"

func Test健康检查与未认证拒绝(t *testing.T) {
	服务, _, 管理器, _ := 新建测试服务(t, false)
	defer 管理器.Close()
	defer 服务.Close()

	响应, 错误 := http.Get(服务.URL + "/healthz")
	if 错误 != nil {
		t.Fatalf("healthz: %v", 错误)
	}
	体 := 读取体(t, 响应)
	if 响应.StatusCode != http.StatusOK || !strings.Contains(体, `"status":"ok"`) {
		t.Fatalf("healthz status=%d body=%s", 响应.StatusCode, 体)
	}

	响应, 错误 = http.Get(服务.URL + "/api/status")
	if 错误 != nil {
		t.Fatalf("status: %v", 错误)
	}
	体 = 读取体(t, 响应)
	if 响应.StatusCode != http.StatusUnauthorized || !strings.Contains(体, `"code":"unauthorized"`) {
		t.Fatalf("status=%d body=%s", 响应.StatusCode, 体)
	}
}

func Test登录会话与状态(t *testing.T) {
	服务, _, 管理器, 路径 := 新建测试服务(t, false)
	defer 管理器.Close()
	defer 服务.Close()

	客户端 := 新建客户端(t)
	响应 := 发送登录(t, 客户端, 服务.URL, "admin", 测试密码, "")
	体 := 读取体(t, 响应)
	if 响应.StatusCode != http.StatusOK {
		t.Fatalf("login status=%d body=%s", 响应.StatusCode, 体)
	}
	if !strings.Contains(体, `"username":"admin"`) {
		t.Fatalf("login body=%s", 体)
	}
	饼干 := 响应.Cookies()
	if len(饼干) != 1 || 饼干[0].Name != 会话Cookie {
		t.Fatalf("cookies=%+v", 饼干)
	}
	if !饼干[0].HttpOnly || 饼干[0].SameSite != http.SameSiteStrictMode || 饼干[0].Path != "/" {
		t.Fatalf("cookie attributes=%+v", 饼干[0])
	}
	if 饼干[0].Secure {
		t.Fatal("insecure test config produced Secure cookie")
	}
	if 饼干[0].MaxAge != int(会话有效期/time.Second) {
		t.Fatalf("cookie max age=%d", 饼干[0].MaxAge)
	}

	响应, 错误 := 客户端.Get(服务.URL + "/api/me")
	if 错误 != nil {
		t.Fatalf("me: %v", 错误)
	}
	体 = 读取体(t, 响应)
	if 响应.StatusCode != http.StatusOK || !strings.Contains(体, `"username":"admin"`) {
		t.Fatalf("me status=%d body=%s", 响应.StatusCode, 体)
	}

	响应, 错误 = 客户端.Get(服务.URL + "/api/status")
	if 错误 != nil {
		t.Fatalf("status: %v", 错误)
	}
	体 = 读取体(t, 响应)
	if 响应.StatusCode != http.StatusOK {
		t.Fatalf("status code=%d body=%s", 响应.StatusCode, 体)
	}
	for _, 键 := range []string{`"mode":"enforce"`, `"ruleset_version":1`, `"uptime_seconds"`, `"dropped_logs":0`, `"sites":[`, `"status":"unknown"`} {
		if !strings.Contains(体, 键) {
			t.Fatalf("status missing %s: %s", 键, 体)
		}
	}

	审计数 := 查询审计数(t, 路径, "login_success")
	if 审计数 != 1 {
		t.Fatalf("login_success audit rows=%d", 审计数)
	}

	响应 = 发送登出(t, 客户端, 服务.URL)
	if 响应.StatusCode != http.StatusNoContent {
		t.Fatalf("logout status=%d", 响应.StatusCode)
	}
	_ = 响应.Body.Close()
	响应, 错误 = 客户端.Get(服务.URL + "/api/me")
	if 错误 != nil {
		t.Fatalf("me after logout: %v", 错误)
	}
	if 响应.StatusCode != http.StatusUnauthorized {
		t.Fatalf("me after logout status=%d", 响应.StatusCode)
	}
	_ = 响应.Body.Close()
}

func Test登录失败与来源校验(t *testing.T) {
	服务, _, 管理器, 路径 := 新建测试服务(t, false)
	defer 管理器.Close()
	defer 服务.Close()
	客户端 := 新建客户端(t)

	响应 := 发送登录(t, 客户端, 服务.URL, "admin", "wrong-password", "")
	体 := 读取体(t, 响应)
	if 响应.StatusCode != http.StatusUnauthorized || !strings.Contains(体, `"code":"unauthorized"`) {
		t.Fatalf("failure status=%d body=%s", 响应.StatusCode, 体)
	}
	if 查询审计数(t, 路径, "login_failed") != 1 {
		t.Fatal("login_failed audit row missing")
	}

	响应 = 发送登录(t, 客户端, 服务.URL, "admin", 测试密码, "http://evil.test")
	体 = 读取体(t, 响应)
	if 响应.StatusCode != http.StatusForbidden || !strings.Contains(体, `"code":"forbidden"`) {
		t.Fatalf("origin status=%d body=%s", 响应.StatusCode, 体)
	}

	响应 = 发送登录(t, 客户端, 服务.URL, "admin", 测试密码, 服务.URL)
	if 响应.StatusCode != http.StatusOK {
		t.Fatalf("matching origin status=%d", 响应.StatusCode)
	}
	_ = 响应.Body.Close()
}

func Test登录限流忽略伪造XFF(t *testing.T) {
	服务, _, 管理器, _ := 新建测试服务(t, false)
	defer 管理器.Close()
	defer 服务.Close()
	客户端 := 新建客户端(t)

	for 次数 := 0; 次数 < 每IP上限; 次数++ {
		请求, 错误 := http.NewRequest(http.MethodPost, 服务.URL+"/api/login",
			bytes.NewBufferString(`{"username":"admin","password":"bad"}`))
		if 错误 != nil {
			t.Fatalf("request: %v", 错误)
		}
		请求.Header.Set("Content-Type", "application/json")
		请求.Header.Set("X-Forwarded-For", "203.0.113."+string(rune('1'+次数)))
		响应, 错误 := 客户端.Do(请求)
		if 错误 != nil {
			t.Fatalf("attempt %d: %v", 次数, 错误)
		}
		_ = 响应.Body.Close()
		if 响应.StatusCode != http.StatusUnauthorized {
			t.Fatalf("attempt %d status=%d", 次数, 响应.StatusCode)
		}
	}

	响应 := 发送登录(t, 客户端, 服务.URL, "admin", 测试密码, "")
	体 := 读取体(t, 响应)
	if 响应.StatusCode != http.StatusTooManyRequests || !strings.Contains(体, `"code":"rate_limited"`) {
		t.Fatalf("rate limit status=%d body=%s", 响应.StatusCode, 体)
	}
	if 响应.Header.Get("Retry-After") == "" {
		t.Fatal("Retry-After missing")
	}
}

func Test安全Cookie配置(t *testing.T) {
	服务, _, 管理器, _ := 新建测试服务(t, true)
	defer 管理器.Close()
	defer 服务.Close()
	客户端 := 新建客户端(t)
	响应 := 发送登录(t, 客户端, 服务.URL, "admin", 测试密码, "")
	if 响应.StatusCode != http.StatusOK {
		t.Fatalf("login status=%d", 响应.StatusCode)
	}
	_ = 响应.Body.Close()
	饼干 := 响应.Cookies()
	if len(饼干) != 1 || !饼干[0].Secure {
		t.Fatalf("Secure cookie missing: %+v", 饼干)
	}
}

func Test状态响应JSON往返(t *testing.T) {
	原始 := U状态响应{
		U模式:    "shadow",
		U规则集版本: 7,
		U运行秒数:  120,
		U丢弃日志数: 3,
		U站点: []U站点健康{
			{U主机: "app.example.com", U状态: "up", U延迟毫秒: 12, U失败次数: 0},
		},
	}
	数据, 错误 := json.Marshal(原始)
	if 错误 != nil {
		t.Fatalf("marshal: %v", 错误)
	}
	for _, 键 := range []string{`"mode"`, `"ruleset_version"`, `"uptime_seconds"`, `"dropped_logs"`, `"sites"`, `"host"`, `"latency_ms"`} {
		if !strings.Contains(string(数据), 键) {
			t.Fatalf("status JSON missing %s: %s", 键, 数据)
		}
	}
	var 还原 U状态响应
	if 错误 := json.Unmarshal(数据, &还原); 错误 != nil {
		t.Fatalf("unmarshal: %v", 错误)
	}
	if !reflect.DeepEqual(原始, 还原) {
		t.Fatalf("round trip mismatch: %+v", 还原)
	}
}

func 新建测试服务(t *testing.T, 安全Cookie bool) (*httptest.Server, *存储包.U存储, *U管理器, string) {
	t.Helper()
	哈希, 错误 := bcrypt.GenerateFromPassword([]byte(测试密码), bcrypt.MinCost)
	if 错误 != nil {
		t.Fatalf("hash: %v", 错误)
	}
	路径 := filepath.Join(t.TempDir(), "admin.db")
	存储, 错误 := 存储包.U打开(路径, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if 错误 != nil {
		t.Fatalf("store: %v", 错误)
	}
	解析器, 错误 := 客户端地址.U新建(nil)
	if 错误 != nil {
		t.Fatalf("resolver: %v", 错误)
	}
	引擎, 错误 := 规则引擎.U新建(规则引擎.U默认规则())
	if 错误 != nil {
		t.Fatalf("engine: %v", 错误)
	}
	配置值 := 配置.U默认配置()
	配置值.U模式 = "enforce"
	配置值.U认证.U用户名 = "admin"
	配置值.U认证.U密码哈希 = string(哈希)
	配置值.U认证.U安全Cookie = 安全Cookie
	配置值.U站点 = []配置.U站点{{
		U主机: "app.example.com", U上游: "http://127.0.0.1:1", U模式: "enforce",
	}}
	管理器, 错误 := U新建(U依赖{
		U配置:   配置值,
		U存储:   存储,
		U引擎:   引擎,
		U解析器:  解析器,
		U日志:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		U探测间隔: -1,
	})
	if 错误 != nil {
		_ = 存储.Close(context.Background())
		t.Fatalf("api: %v", 错误)
	}
	服务 := httptest.NewServer(管理器)
	t.Cleanup(func() {
		_ = 存储.Close(context.Background())
	})
	return 服务, 存储, 管理器, 路径
}

func 新建客户端(t *testing.T) *http.Client {
	t.Helper()
	罐, 错误 := cookiejar.New(nil)
	if 错误 != nil {
		t.Fatalf("cookie jar: %v", 错误)
	}
	return &http.Client{Jar: 罐, Timeout: 5 * time.Second}
}

func 发送登录(t *testing.T, 客户端 *http.Client, 基准, 用户名, 密码, 来源 string) *http.Response {
	t.Helper()
	负载, _ := json.Marshal(U登录请求{U用户名: 用户名, U密码: 密码})
	请求, 错误 := http.NewRequest(http.MethodPost, 基准+"/api/login", bytes.NewReader(负载))
	if 错误 != nil {
		t.Fatalf("login request: %v", 错误)
	}
	请求.Header.Set("Content-Type", "application/json")
	if 来源 != "" {
		请求.Header.Set("Origin", 来源)
	}
	响应, 错误 := 客户端.Do(请求)
	if 错误 != nil {
		t.Fatalf("login: %v", 错误)
	}
	return 响应
}

func 发送登出(t *testing.T, 客户端 *http.Client, 基准 string) *http.Response {
	t.Helper()
	请求, 错误 := http.NewRequest(http.MethodPost, 基准+"/api/logout", nil)
	if 错误 != nil {
		t.Fatalf("logout request: %v", 错误)
	}
	响应, 错误 := 客户端.Do(请求)
	if 错误 != nil {
		t.Fatalf("logout: %v", 错误)
	}
	return 响应
}

func 读取体(t *testing.T, 响应 *http.Response) string {
	t.Helper()
	defer func() { _ = 响应.Body.Close() }()
	数据, 错误 := io.ReadAll(响应.Body)
	if 错误 != nil {
		t.Fatalf("read body: %v", 错误)
	}
	return string(数据)
}

func 查询审计数(t *testing.T, 路径, 动作 string) int {
	t.Helper()
	数据库, 错误 := sql.Open("sqlite", 路径)
	if 错误 != nil {
		t.Fatalf("open db: %v", 错误)
	}
	defer func() { _ = 数据库.Close() }()
	var 数量 int
	if 错误 := 数据库.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action = ?`, 动作).Scan(&数量); 错误 != nil {
		t.Fatalf("audit count: %v", 错误)
	}
	return 数量
}
