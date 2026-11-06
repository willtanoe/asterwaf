package 管理接口

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	配置 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/config"
)

// 登录必须签发新令牌，不能复用请求里已有的会话 ID。
func Test登录不采用预置会话令牌(t *testing.T) {
	服务, _, 管理器, _ := 新建测试服务(t, false)
	defer 管理器.Close()
	defer 服务.Close()

	预置 := "attacker-chosen-session-value"
	负载, 错误 := json.Marshal(U登录请求{U用户名: "admin", U密码: 测试密码})
	if 错误 != nil {
		t.Fatalf("marshal: %v", 错误)
	}
	请求, 错误 := http.NewRequest(http.MethodPost, 服务.URL+"/api/login", bytes.NewReader(负载))
	if 错误 != nil {
		t.Fatalf("request: %v", 错误)
	}
	请求.Header.Set("Content-Type", "application/json")
	请求.Header.Set("Cookie", 会话Cookie+"="+预置)
	响应, 错误 := http.DefaultClient.Do(请求)
	if 错误 != nil {
		t.Fatalf("login: %v", 错误)
	}
	defer func() { _ = 响应.Body.Close() }()
	if 响应.StatusCode != http.StatusOK {
		t.Fatalf("login status=%d", 响应.StatusCode)
	}
	饼干 := 响应.Cookies()
	if len(饼干) != 1 {
		t.Fatalf("cookies=%+v", 饼干)
	}
	if 饼干[0].Value == 预置 {
		t.Fatal("server reused the client-supplied session identifier")
	}
	if _, 存在 := 管理器.会话.U获取(预置); 存在 {
		t.Fatal("client-supplied session value became valid")
	}
}

func Test会话令牌不落库(t *testing.T) {
	服务, _, 管理器, 路径 := 新建测试服务(t, false)
	defer 管理器.Close()
	defer 服务.Close()
	客户端 := 新建客户端(t)
	响应 := 发送登录(t, 客户端, 服务.URL, "admin", 测试密码, "")
	if 响应.StatusCode != http.StatusOK {
		t.Fatalf("login status=%d", 响应.StatusCode)
	}
	_ = 响应.Body.Close()

	令牌 := ""
	for _, 饼干 := range 响应.Cookies() {
		if 饼干.Name == 会话Cookie {
			令牌 = 饼干.Value
		}
	}
	if len(令牌) < 40 {
		t.Fatalf("session token too short: %d chars", len(令牌))
	}
	if 数据库含(t, 路径, 令牌) {
		t.Fatal("session token was written to the database")
	}
}

func Test未认证的写请求被拒绝(t *testing.T) {
	服务, _, 管理器, _ := 新建测试服务(t, false)
	defer 管理器.Close()
	defer 服务.Close()
	罐, _ := cookiejar.New(nil)
	客户端 := &http.Client{Jar: 罐, Timeout: 5 * time.Second}
	for _, 路径 := range []string{
		"/api/rules",
		"/api/access-lists",
		"/api/rate-limit",
		"/api/sites/app.example.com/mode",
	} {
		响应 := 发送JSON(t, 客户端, http.MethodPost, 服务.URL+路径, map[string]any{"mode": "off"})
		_ = 响应.Body.Close()
		if 响应.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s status=%d, want 401", 路径, 响应.StatusCode)
		}
	}
}

func Test管理端口拒绝跨站写请求(t *testing.T) {
	服务, _, 管理器, _ := 新建测试服务(t, false)
	defer 管理器.Close()
	defer 服务.Close()
	客户端 := 登录客户端(t, 服务.URL)

	负载, _ := json.Marshal(map[string]any{"id": "x", "action": "deny", "address": "203.0.113.9", "enabled": true})
	请求, 错误 := http.NewRequest(http.MethodPost, 服务.URL+"/api/access-lists", bytes.NewReader(负载))
	if 错误 != nil {
		t.Fatalf("request: %v", 错误)
	}
	请求.Header.Set("Content-Type", "application/json")
	请求.Header.Set("Origin", "https://evil.test")
	响应, 错误 := 客户端.Do(请求)
	if 错误 != nil {
		t.Fatalf("send: %v", 错误)
	}
	_ = 响应.Body.Close()
	if 响应.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-origin write status=%d, want 403", 响应.StatusCode)
	}

	// 无 Origin 的脚本客户端仍允许，否则 CLI 无法工作。
	再次, _ := http.NewRequest(http.MethodGet, 服务.URL+"/api/access-lists", nil)
	响应, 错误 = 客户端.Do(再次)
	if 错误 != nil {
		t.Fatalf("get: %v", 错误)
	}
	_ = 响应.Body.Close()
	if 响应.StatusCode != http.StatusOK {
		t.Fatalf("same-origin read status=%d", 响应.StatusCode)
	}
}

func 数据库含(t *testing.T, 路径, 片段 string) bool {
	t.Helper()
	数据库, 错误 := sql.Open("sqlite", 路径)
	if 错误 != nil {
		t.Fatalf("open db: %v", 错误)
	}
	defer func() { _ = 数据库.Close() }()
	查询 := []string{
		`SELECT COUNT(*) FROM audit_log WHERE before LIKE ? OR after LIKE ?`,
		`SELECT COUNT(*) FROM rules WHERE pattern LIKE ? OR name LIKE ?`,
		`SELECT COUNT(*) FROM ruleset_versions WHERE rules_json LIKE ? OR note LIKE ?`,
		`SELECT COUNT(*) FROM access_entries WHERE note LIKE ? OR id LIKE ?`,
	}
	参数 := "%" + 片段 + "%"
	for _, 语句 := range 查询 {
		var 数量 int
		if 错误 := 数据库.QueryRow(语句, 参数, 参数).Scan(&数量); 错误 != nil {
			t.Fatalf("query %q: %v", 语句, 错误)
		}
		if 数量 > 0 {
			return true
		}
	}
	return false
}

func Test响应不包含凭据(t *testing.T) {
	服务, _, 管理器, _ := 新建测试服务(t, false)
	defer 管理器.Close()
	defer 服务.Close()
	客户端 := 新建客户端(t)
	响应 := 发送登录(t, 客户端, 服务.URL, "admin", 测试密码, "")
	体 := 读取体(t, 响应)
	if strings.Contains(体, 测试密码) || strings.Contains(体, "$2a$") || strings.Contains(体, "$2b$") {
		t.Fatalf("login response leaked credentials: %s", 体)
	}
	if strings.Contains(体, 会话Cookie+"=") {
		t.Fatalf("login body echoed the session cookie: %s", 体)
	}
	for _, 路径 := range []string{"/api/me", "/api/status"} {
		响应 := 发送JSON(t, 客户端, http.MethodGet, 服务.URL+路径, nil)
		内容 := 读取体(t, 响应)
		if strings.Contains(内容, 测试密码) || strings.Contains(内容, "$2a$") || strings.Contains(内容, "$2b$") {
			t.Fatalf("%s leaked credentials: %s", 路径, 内容)
		}
	}
}

func Test审计不记录密码(t *testing.T) {
	服务, _, 管理器, 路径 := 新建测试服务(t, false)
	defer 管理器.Close()
	defer 服务.Close()
	客户端 := 新建客户端(t)
	_ = 发送登录(t, 客户端, 服务.URL, "admin", "wrong-password-value", "")
	if 数据库含(t, 路径, "wrong-password-value") {
		t.Fatal("failed password was written to the database")
	}
	if 数据库含(t, 路径, 测试密码) {
		t.Fatal("successful password was written to the database")
	}
}

func Test首次运行密码文件权限(t *testing.T) {
	目录 := t.TempDir()
	配置值 := 默认测试配置()
	配置值.U数据库 = filepath.Join(目录, "data", "waf.db")
	配置值.U认证.U密码哈希 = ""

	if _, 错误 := U准备凭据(配置值); 错误 != nil {
		t.Fatalf("prepare: %v", 错误)
	}
	文件路径 := filepath.Join(目录, "data", 首运行密码文件名)
	信息, 错误 := os.Stat(文件路径)
	if 错误 != nil {
		t.Fatalf("stat password file: %v", 错误)
	}
	if 权限 := 信息.Mode().Perm(); 权限 != 0o600 {
		t.Fatalf("password file mode=%o, want 600", 权限)
	}
	目录信息, 错误 := os.Stat(filepath.Dir(文件路径))
	if 错误 != nil {
		t.Fatalf("stat data dir: %v", 错误)
	}
	if 权限 := 目录信息.Mode().Perm(); 权限 != 0o700 {
		t.Fatalf("data dir mode=%o, want 700", 权限)
	}
	内容, 错误 := os.ReadFile(文件路径)
	if 错误 != nil {
		t.Fatalf("read password file: %v", 错误)
	}
	if string(内容) == "" || strings.Contains(string(内容), "asterwaf") {
		t.Fatalf("unexpected password file contents")
	}
}

func Test默认管理监听仅限回环(t *testing.T) {
	配置值 := 默认测试配置()
	if 配置值.U管理监听 != "127.0.0.1:9090" {
		t.Fatalf("admin default=%q, want loopback only", 配置值.U管理监听)
	}
	默认 := 配置.U默认配置()
	if 默认.U管理监听 != "127.0.0.1:9090" {
		t.Fatalf("default config admin listen=%q", 默认.U管理监听)
	}
}

func 默认测试配置() 配置.U配置 {
	配置值 := 配置.U默认配置()
	配置值.U模式 = "enforce"
	配置值.U认证.U用户名 = "admin"
	return 配置值
}

// 登录成功必须重置按 IP 与按用户名的失败计数，否则正常用户会被自己的历史失败拖住。
func Test登录成功清除限速计数(t *testing.T) {
	服务, _, 管理器, _ := 新建测试服务(t, false)
	defer 管理器.Close()
	defer 服务.Close()
	客户端 := 新建客户端(t)

	for 次数 := 0; 次数 < 每IP上限-1; 次数++ {
		响应 := 发送登录(t, 客户端, 服务.URL, "admin", "wrong-password", "")
		_ = 响应.Body.Close()
	}
	响应 := 发送登录(t, 客户端, 服务.URL, "admin", 测试密码, "")
	_ = 响应.Body.Close()
	if 响应.StatusCode != http.StatusOK {
		t.Fatalf("expected success below the threshold, got %d", 响应.StatusCode)
	}

	// 成功后计数清零，即使再失败几次也不应立刻触发限速。
	for 次数 := 0; 次数 < 2; 次数++ {
		响应 := 发送登录(t, 新建客户端(t), 服务.URL, "admin", "wrong-password", "")
		_ = 响应.Body.Close()
		if 响应.StatusCode != http.StatusUnauthorized {
			t.Fatalf("post-success failure %d returned %d, want 401", 次数, 响应.StatusCode)
		}
	}
}

// 伪造 X-Forwarded-For 不能在未配置受信代理时绕过登录限速。
func Test登录限速不受伪造XFF影响(t *testing.T) {
	服务, _, 管理器, _ := 新建测试服务(t, false)
	defer 管理器.Close()
	defer 服务.Close()
	客户端 := 新建客户端(t)

	for 次数 := 0; 次数 < 每IP上限; 次数++ {
		响应 := 发送带来源登录(t, 客户端, 服务.URL, "admin", "bad", "", "203.0.113."+itoa(次数+1))
		_ = 响应.Body.Close()
		idle := 响应.StatusCode
		if idle != http.StatusUnauthorized && idle != http.StatusTooManyRequests {
			t.Fatalf("attempt %d status=%d", 次数, idle)
		}
	}
	响应 := 发送登录(t, 客户端, 服务.URL, "admin", 测试密码, "")
	_ = 响应.Body.Close()
	if 响应.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("rotating spoofed XFF bypassed the limit: status=%d", 响应.StatusCode)
	}
}

func 发送带来源登录(t *testing.T, 客户端 *http.Client, 基准, 用户名, 密码, 来源, 伪造 string) *http.Response {
	t.Helper()
	负载, _ := json.Marshal(U登录请求{U用户名: 用户名, U密码: 密码})
	请求, 错误 := http.NewRequest(http.MethodPost, 基准+"/api/login", bytes.NewReader(负载))
	if 错误 != nil {
		t.Fatalf("request: %v", 错误)
	}
	请求.Header.Set("Content-Type", "application/json")
	if 来源 != "" {
		请求.Header.Set("Origin", 来源)
	}
	if 伪造 != "" {
		请求.Header.Set("X-Forwarded-For", 伪造)
	}
	响应, 错误 := 客户端.Do(请求)
	if 错误 != nil {
		t.Fatalf("login: %v", 错误)
	}
	return 响应
}

func itoa(值 int) string {
	if 值 < 10 {
		return string(rune('0' + 值))
	}
	return string(rune('0'+值/10)) + string(rune('0'+值%10))
}

func Test静态资源不越界(t *testing.T) {
	服务, _, 管理器, _ := 新建测试服务(t, false)
	defer 管理器.Close()
	defer 服务.Close()
	客户端 := &http.Client{Timeout: 5 * time.Second}
	for _, 路径 := range []string{
		"/../etc/passwd",
		"/..%2f..%2fetc%2fpasswd",
		"/assets/../../config.yaml",
		"/%2e%2e/%2e%2e/etc/passwd",
	} {
		响应, 错误 := 客户端.Get(服务.URL + 路径)
		if 错误 != nil {
			t.Fatalf("%s: %v", 路径, 错误)
		}
		体 := 读取体(t, 响应)
		if strings.Contains(体, "root:") || strings.Contains(体, "admin_listen") {
			t.Fatalf("%s served a file outside the embedded assets: %s", 路径, 体)
		}
		if 响应.StatusCode == http.StatusOK && strings.Contains(体, "Go build") {
			t.Fatalf("%s leaked source", 路径)
		}
	}
}
