package 存储

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func Test规范化路径(t *testing.T) {
	测试用例 := map[string]string{
		"/api/users/123456":                           "/api/users/:id",
		"/api/users/12345":                            "/api/users/12345",
		"/api/550e8400-e29b-41d4-a716-446655440000":   "/api/:id",
		"/assets/deadbeefdeadbeefdeadbeefdeadbeef.js": "/assets/:id",
		"/search?q=apple":                             "/search",
		"/":                                           "/",
		"":                                            "/",
		"/v1/orders/9876543210/items/42":              "/v1/orders/:id/items/42",
	}
	for 输入, 期望 := range 测试用例 {
		if 实际 := 规范化路径(输入); 实际 != 期望 {
			t.Fatalf("%q => %q, want %q", 输入, 实际, 期望)
		}
	}
	长路径 := "/" + strings.Repeat("a", 500)
	if 结果 := 规范化路径(长路径); len(结果) > 路径长度上限 {
		t.Fatalf("normalized path length=%d", len(结果))
	}
}

func Test聚合写入(t *testing.T) {
	存储, 错误 := U打开(filepath.Join(t.TempDir(), "rollup.db"), 静默日志())
	if 错误 != nil {
		t.Fatalf("open: %v", 错误)
	}
	defer 关闭(t, 存储)

	基准 := time.Date(2026, 11, 6, 12, 34, 56, 0, time.UTC).UnixMilli()
	记录列表 := []U记录{
		聚合记录(基准+1000, "/api/123456", "203.0.113.1", "allow", 0),
		聚合记录(基准+2000, "/api/654321", "203.0.113.1", "block", 3),
		聚合记录(基准+3000, "/api/123456", "203.0.113.1", "allow", 5000),
	}
	if 错误 := 存储.U直接写入(记录列表); 错误 != nil {
		t.Fatalf("write: %v", 错误)
	}

	分钟 := 聚合时刻(基准, 60_000)
	var 请求, 延迟, 桶0, 桶2, 桶11 int64
	if 错误 := 存储.数据库.QueryRow(`SELECT requests, latency_sum_ms, b0, b2, b11
		FROM rollup_minute WHERE minute = ? AND site = ? AND action = ? AND category = ? AND rule_id = ?`,
		分钟, "app.example.com", "allow", "sqli", "sqli.union-select").Scan(&请求, &延迟, &桶0, &桶2, &桶11); 错误 != nil {
		t.Fatalf("minute rollup: %v", 错误)
	}
	if 请求 != 2 || 延迟 != 5000 || 桶0 != 1 || 桶2 != 0 || 桶11 != 1 {
		t.Fatalf("minute rollup requests=%d latency=%d b0=%d b2=%d b11=%d", 请求, 延迟, 桶0, 桶2, 桶11)
	}

	var 地址请求, 地址拦截 int64
	if 错误 := 存储.数据库.QueryRow(`SELECT requests, blocked FROM rollup_hour_ip
		WHERE hour = ? AND site = ? AND client_ip = ?`, 聚合时刻(基准, 3_600_000), "app.example.com", "203.0.113.1").Scan(&地址请求, &地址拦截); 错误 != nil {
		t.Fatalf("ip rollup: %v", 错误)
	}
	if 地址请求 != 3 || 地址拦截 != 1 {
		t.Fatalf("ip rollup requests=%d blocked=%d", 地址请求, 地址拦截)
	}

	var 路径请求 int64
	if 错误 := 存储.数据库.QueryRow(`SELECT requests FROM rollup_hour_path
		WHERE hour = ? AND site = ? AND path = ?`, 聚合时刻(基准, 3_600_000), "app.example.com", "/api/:id").Scan(&路径请求); 错误 != nil {
		t.Fatalf("path rollup: %v", 错误)
	}
	if 路径请求 != 3 {
		t.Fatalf("path rollup requests=%d", 路径请求)
	}
}

func Test保留清理(t *testing.T) {
	存储, 错误 := U打开(filepath.Join(t.TempDir(), "retention.db"), 静默日志())
	if 错误 != nil {
		t.Fatalf("open: %v", 错误)
	}
	defer 关闭(t, 存储)

	现在 := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	记录列表 := []U记录{
		聚合记录(现在.Add(-15*24*time.Hour).UnixMilli(), "/old-raw", "203.0.113.1", "allow", 1),
		聚合记录(现在.Add(-31*24*time.Hour).UnixMilli(), "/old-minute", "203.0.113.2", "allow", 1),
		聚合记录(现在.Add(-181*24*time.Hour).UnixMilli(), "/old-hour", "203.0.113.3", "allow", 1),
		聚合记录(现在.Add(-1*time.Hour).UnixMilli(), "/recent", "203.0.113.4", "allow", 1),
	}
	if 错误 := 存储.U直接写入(记录列表); 错误 != nil {
		t.Fatalf("write: %v", 错误)
	}
	审计时间 := 现在.Add(-400 * 24 * time.Hour).UnixMilli()
	if _, 错误 := 存储.数据库.Exec(`INSERT INTO audit_log(ts, actor, action) VALUES (?, 'admin', 'login')`, 审计时间); 错误 != nil {
		t.Fatalf("audit insert: %v", 错误)
	}

	if 错误 := 存储.U清理保留(现在); 错误 != nil {
		t.Fatalf("cleanup: %v", 错误)
	}

	var 日志数 int
	if 错误 := 存储.数据库.QueryRow(`SELECT COUNT(*) FROM logs`).Scan(&日志数); 错误 != nil {
		t.Fatalf("logs count: %v", 错误)
	}
	if 日志数 != 1 {
		t.Fatalf("logs=%d, want 1 recent row", 日志数)
	}
	var 分钟数, 小时数, 审计数 int
	if 错误 := 存储.数据库.QueryRow(`SELECT COUNT(*) FROM rollup_minute`).Scan(&分钟数); 错误 != nil {
		t.Fatalf("minute count: %v", 错误)
	}
	if 分钟数 != 2 {
		t.Fatalf("minute rows=%d, want 2 (15d and recent)", 分钟数)
	}
	if 错误 := 存储.数据库.QueryRow(`SELECT COUNT(*) FROM rollup_hour_ip`).Scan(&小时数); 错误 != nil {
		t.Fatalf("hour count: %v", 错误)
	}
	if 小时数 != 3 {
		t.Fatalf("hour rows=%d, want 3 (31d, 15d and recent)", 小时数)
	}
	if 错误 := 存储.数据库.QueryRow(`SELECT COUNT(*) FROM audit_log`).Scan(&审计数); 错误 != nil {
		t.Fatalf("audit count: %v", 错误)
	}
	if 审计数 != 0 {
		t.Fatalf("audit rows=%d", 审计数)
	}
}

func Test包含种子数据(t *testing.T) {
	存储, 错误 := U打开(filepath.Join(t.TempDir(), "seedflag.db"), 静默日志())
	if 错误 != nil {
		t.Fatalf("open: %v", 错误)
	}
	defer 关闭(t, 存储)
	if 存储.U包含种子数据(context.Background()) {
		t.Fatal("empty database reported seed data")
	}
	记录 := 聚合记录(time.Now().UnixMilli(), "/seed", "203.0.113.1", "allow", 1)
	记录.U种子 = true
	if 错误 := 存储.U直接写入([]U记录{记录}); 错误 != nil {
		t.Fatalf("write: %v", 错误)
	}
	if !存储.U包含种子数据(context.Background()) {
		t.Fatal("seed row not detected")
	}
}

func 聚合记录(时间 int64, 路径, 地址, 动作 string, 耗时 int64) U记录 {
	return U记录{
		U请求ID:  "req",
		U时间:    时间,
		USite:  "app.example.com",
		U方法:    "GET",
		U路径:    路径,
		U状态:    200,
		U动作:    动作,
		U类别:    "sqli",
		U规则ID:  "sqli.union-select",
		U客户端IP: 地址,
		U耗时毫秒:  耗时,
		U请求头:   "{}",
	}
}

// 被丢弃的行不会写入日志，也不应出现在任何聚合里。
func Test丢弃的行不进聚合(t *testing.T) {
	存储, 错误 := U打开(filepath.Join(t.TempDir(), "dropped.db"), 静默日志())
	if 错误 != nil {
		t.Fatalf("open: %v", 错误)
	}
	defer 关闭(t, 存储)
	记录 := 聚合记录(time.Now().UnixMilli(), "/kept", "203.0.113.5", "allow", 2)
	if 错误 := 存储.U直接写入([]U记录{记录}); 错误 != nil {
		t.Fatalf("write: %v", 错误)
	}
	// 模拟队列丢弃：只递增计数，不写入任何表。
	存储.丢弃.Add(5)

	var 原始, 聚合 int64
	if 错误 := 存储.数据库.QueryRow(`SELECT COUNT(*) FROM logs`).Scan(&原始); 错误 != nil {
		t.Fatalf("logs: %v", 错误)
	}
	if 错误 := 存储.数据库.QueryRow(`SELECT COALESCE(SUM(requests),0) FROM rollup_minute`).Scan(&聚合); 错误 != nil {
		t.Fatalf("rollup: %v", 错误)
	}
	if 原始 != 1 || 聚合 != 1 {
		t.Fatalf("logs=%d rollup=%d; dropped rows must not appear in either", 原始, 聚合)
	}
	if 存储.U丢弃数() != 5 {
		t.Fatalf("dropped counter=%d", 存储.U丢弃数())
	}
}

// 重启后聚合仍与原始日志一致：重启不会重放或丢失已有行。
func Test重启后聚合一致(t *testing.T) {
	路径 := filepath.Join(t.TempDir(), "restart.db")
	写入 := func() {
		存储, 错误 := U打开(路径, 静默日志())
		if 错误 != nil {
			t.Fatalf("open: %v", 错误)
		}
		基准 := time.Now().UnixMilli()
		for 序号 := 0; 序号 < 10; 序号++ {
			if 错误 := 存储.U直接写入([]U记录{聚合记录(基准-int64(序号)*1000, "/restart", "203.0.113.6", "block", 3)}); 错误 != nil {
				t.Fatalf("write: %v", 错误)
			}
		}
		关闭(t, 存储)
	}
	写入()
	写入()

	源, 错误 := 数据源(路径)
	if 错误 != nil {
		t.Fatalf("reopen: %v", 错误)
	}
	数据库, 错误 := sql.Open("sqlite", 源)
	if 错误 != nil {
		t.Fatalf("reopen: %v", 错误)
	}
	defer func() { _ = 数据库.Close() }()
	var 原始, 聚合 int64
	if 错误 := 数据库.QueryRow(`SELECT COUNT(*) FROM logs`).Scan(&原始); 错误 != nil {
		t.Fatalf("logs: %v", 错误)
	}
	if 错误 := 数据库.QueryRow(`SELECT COALESCE(SUM(requests),0) FROM rollup_minute`).Scan(&聚合); 错误 != nil {
		t.Fatalf("rollup: %v", 错误)
	}
	if 原始 != 20 || 聚合 != 20 {
		t.Fatalf("after restart logs=%d rollup=%d, want 20 each", 原始, 聚合)
	}
}
