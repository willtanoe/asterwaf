package 存储

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"
)

func Test打开应用SQLite参数(t *testing.T) {
	路径 := filepath.Join(t.TempDir(), "test.db")
	存储, 错误 := U打开(路径, 静默日志())
	if 错误 != nil {
		t.Fatalf("open: %v", 错误)
	}
	defer 关闭(t, 存储)

	var 模式 string
	if 错误 := 存储.数据库.QueryRow(`PRAGMA journal_mode`).Scan(&模式); 错误 != nil {
		t.Fatalf("journal_mode: %v", 错误)
	}
	if 模式 != "wal" {
		t.Fatalf("journal_mode=%q", 模式)
	}
	var 同步 int
	if 错误 := 存储.数据库.QueryRow(`PRAGMA synchronous`).Scan(&同步); 错误 != nil {
		t.Fatalf("synchronous: %v", 错误)
	}
	if 同步 != 1 {
		t.Fatalf("synchronous=%d, want 1 (NORMAL)", 同步)
	}
	var 超时 int
	if 错误 := 存储.数据库.QueryRow(`PRAGMA busy_timeout`).Scan(&超时); 错误 != nil {
		t.Fatalf("busy_timeout: %v", 错误)
	}
	if 超时 != 5000 {
		t.Fatalf("busy_timeout=%d", 超时)
	}

	var 索引SQL string
	if 错误 := 存储.数据库.QueryRow(`SELECT sql FROM sqlite_master WHERE type='index' AND name='idx_logs_live_ts'`).Scan(&索引SQL); 错误 != nil {
		t.Fatalf("partial index: %v", 错误)
	}
	if 索引SQL == "" || !contains(索引SQL, "WHERE seed = 0") {
		t.Fatalf("partial index missing seed predicate: %s", 索引SQL)
	}
}

func Test异步日志落库(t *testing.T) {
	路径 := filepath.Join(t.TempDir(), "logs.db")
	存储, 错误 := U打开(路径, 静默日志())
	if 错误 != nil {
		t.Fatalf("open: %v", 错误)
	}
	记录 := U记录{
		U请求ID:  "abc123",
		U时间:    1700000000000,
		USite:  "app.example.com",
		U方法:    "GET",
		U路径:    "/search",
		U查询:    "q=union",
		U状态:    403,
		U动作:    "block",
		U类别:    "sqli",
		U规则ID:  "sqli.union-select",
		U影子:    true,
		U客户端IP: "203.0.113.9",
		U用户代理:  "curl/8",
		U耗时毫秒:  3,
		U请求头:   `{"Host":["app.example.com"]}`,
		U请求体:   `{"password":"<redacted>"}`,
		U匹配:    `{"category":"query","name":"q","value":"<redacted>"}`,
		U种子:    true,
	}
	存储.U加入(记录)
	关闭(t, 存储)

	数据库, 错误 := sql.Open("sqlite", 数据源(路径))
	if 错误 != nil {
		t.Fatalf("reopen: %v", 错误)
	}
	defer func() { _ = 数据库.Close() }()

	var 还原 U记录
	var 影子, 种子 int
	if 错误 := 数据库.QueryRow(`SELECT request_id, ts, site, method, path, query, status, action, category,
		rule_id, shadow, client_ip, user_agent, duration_ms, request_headers, request_body, matched, seed
		FROM logs`).Scan(
		&还原.U请求ID, &还原.U时间, &还原.USite, &还原.U方法, &还原.U路径, &还原.U查询, &还原.U状态,
		&还原.U动作, &还原.U类别, &还原.U规则ID, &影子, &还原.U客户端IP, &还原.U用户代理,
		&还原.U耗时毫秒, &还原.U请求头, &还原.U请求体, &还原.U匹配, &种子,
	); 错误 != nil {
		t.Fatalf("select: %v", 错误)
	}
	还原.U影子 = 影子 == 1
	还原.U种子 = 种子 == 1
	if 还原 != 记录 {
		t.Fatalf("stored record mismatch:\nwant=%+v\ngot=%+v", 记录, 还原)
	}
}

func Test批量写入与关闭排空(t *testing.T) {
	路径 := filepath.Join(t.TempDir(), "batch.db")
	存储, 错误 := U打开(路径, 静默日志())
	if 错误 != nil {
		t.Fatalf("open: %v", 错误)
	}
	for 序号 := 0; 序号 < 600; 序号++ {
		存储.U加入(U记录{
			U请求ID:  "id",
			U时间:    int64(序号),
			USite:  "app.example.com",
			U方法:    "GET",
			U路径:    "/",
			U状态:    200,
			U动作:    "allow",
			U客户端IP: "203.0.113.1",
			U请求头:   "{}",
		})
	}
	关闭(t, 存储)

	数据库, 错误 := sql.Open("sqlite", 数据源(路径))
	if 错误 != nil {
		t.Fatalf("reopen: %v", 错误)
	}
	defer func() { _ = 数据库.Close() }()
	var 数量 int
	if 错误 := 数据库.QueryRow(`SELECT COUNT(*) FROM logs`).Scan(&数量); 错误 != nil {
		t.Fatalf("count: %v", 错误)
	}
	if 数量 != 600 {
		t.Fatalf("rows=%d, want 600", 数量)
	}
	if 存储.U丢弃数() != 0 {
		t.Fatalf("dropped=%d", 存储.U丢弃数())
	}
}

func Test队列满丢弃并计数(t *testing.T) {
	存储 := &U存储{队列: make(chan U记录, 1)}
	存储.U加入(U记录{})
	存储.U加入(U记录{})
	if 存储.U丢弃数() != 1 {
		t.Fatalf("dropped=%d, want 1", 存储.U丢弃数())
	}
}

func Test关闭后加入计入丢弃(t *testing.T) {
	存储, 错误 := U打开(filepath.Join(t.TempDir(), "closed.db"), 静默日志())
	if 错误 != nil {
		t.Fatalf("open: %v", 错误)
	}
	关闭(t, 存储)
	存储.U加入(U记录{})
	if 存储.U丢弃数() != 1 {
		t.Fatalf("dropped=%d", 存储.U丢弃数())
	}
}

func Test迁移幂等(t *testing.T) {
	路径 := filepath.Join(t.TempDir(), "migrate.db")
	for 次数 := 0; 次数 < 2; 次数++ {
		存储, 错误 := U打开(路径, 静默日志())
		if 错误 != nil {
			t.Fatalf("open %d: %v", 次数, 错误)
		}
		关闭(t, 存储)
	}
	数据库, 错误 := sql.Open("sqlite", 数据源(路径))
	if 错误 != nil {
		t.Fatalf("reopen: %v", 错误)
	}
	defer func() { _ = 数据库.Close() }()
	var 版本 int
	if 错误 := 数据库.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&版本); 错误 != nil {
		t.Fatalf("version: %v", 错误)
	}
	if 版本 != 3 {
		t.Fatalf("version=%d", 版本)
	}
}

func 关闭(t *testing.T, 存储 *U存储) {
	t.Helper()
	上下文, 取消 := context.WithTimeout(context.Background(), 5*time.Second)
	defer 取消()
	if 错误 := 存储.Close(上下文); 错误 != nil {
		t.Fatalf("close: %v", 错误)
	}
}

func 静默日志() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func contains(原文, 片段 string) bool {
	for 位置 := 0; 位置+len(片段) <= len(原文); 位置++ {
		if 原文[位置:位置+len(片段)] == 片段 {
			return true
		}
	}
	return false
}

func Test写审计(t *testing.T) {
	存储, 错误 := U打开(filepath.Join(t.TempDir(), "audit.db"), 静默日志())
	if 错误 != nil {
		t.Fatalf("open: %v", 错误)
	}
	defer 关闭(t, 存储)
	if 错误 := 存储.U写审计(context.Background(), U审计记录{
		U操作者: "admin", U动作: "login_success", U客户端IP: "203.0.113.7",
	}); 错误 != nil {
		t.Fatalf("audit: %v", 错误)
	}
	var 操作者, 动作, 地址 string
	if 错误 := 存储.数据库.QueryRow(`SELECT actor, action, client_ip FROM audit_log`).Scan(&操作者, &动作, &地址); 错误 != nil {
		t.Fatalf("select audit: %v", 错误)
	}
	if 操作者 != "admin" || 动作 != "login_success" || 地址 != "203.0.113.7" {
		t.Fatalf("audit row=%s %s %s", 操作者, 动作, 地址)
	}
}

func Test关闭时排空未满批次(t *testing.T) {
	路径 := filepath.Join(t.TempDir(), "drain.db")
	存储, 错误 := U打开(路径, 静默日志())
	if 错误 != nil {
		t.Fatalf("open: %v", 错误)
	}
	// 只放 1 条，不足 256 行也不足 1 秒，Close 必须主动排空。
	存储.U加入(U记录{
		U请求ID: "drain-1", U时间: 1, USite: "app.example.com", U方法: "GET",
		U路径: "/", U状态: 200, U动作: "allow", U客户端IP: "203.0.113.1", U请求头: "{}",
	})
	关闭(t, 存储)

	数据库, 错误 := sql.Open("sqlite", 数据源(路径))
	if 错误 != nil {
		t.Fatalf("reopen: %v", 错误)
	}
	defer func() { _ = 数据库.Close() }()
	var 数量 int
	if 错误 := 数据库.QueryRow(`SELECT COUNT(*) FROM logs WHERE request_id = 'drain-1'`).Scan(&数量); 错误 != nil {
		t.Fatalf("count: %v", 错误)
	}
	if 数量 != 1 {
		t.Fatalf("drained rows=%d, want 1", 数量)
	}
}

func Test关闭后队列不再接收(t *testing.T) {
	存储, 错误 := U打开(filepath.Join(t.TempDir(), "closed2.db"), 静默日志())
	if 错误 != nil {
		t.Fatalf("open: %v", 错误)
	}
	关闭(t, 存储)
	存储.U加入(U记录{U请求ID: "late"})
	if 存储.U丢弃数() != 1 {
		t.Fatalf("dropped=%d, want 1", 存储.U丢弃数())
	}
}
