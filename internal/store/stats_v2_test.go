package 存储

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// v2 统计查询的公共夹具：一个站点两条日志（一条 block 一条 shadow）、
// 一条 rollup（含类别与延迟直方图）、一条访问列表命中。
func 填充v2夹具(t *testing.T, 存储 *U存储) {
	t.Helper()
	现在 := time.Now().UnixMilli()
	存储.U直接写入([]U记录{
		{U请求ID: "v2-1", U时间: 现在 - 60_000, USite: "app.test", U方法: "GET", U路径: "/a",
			U状态: 403, U动作: "block", U类别: "sqli", U规则ID: "sqli.union-select",
			U客户端IP: "203.0.113.9", U耗时毫秒: 3, U请求头: "{}", U字节入: 120, U字节出: 340},
		{U请求ID: "v2-2", U时间: 现在 - 30_000, USite: "api.test", U方法: "POST", U路径: "/b",
			U状态: 200, U动作: "shadow", U类别: "xss", U规则ID: "xss.event",
			U客户端IP: "203.0.113.9", U耗时毫秒: 30, U请求头: "{}", U字节入: 50, U字节出: 900},
	})
}

func 打开v2存储(t *testing.T) *U存储 {
	t.Helper()
	存储, 错误 := U打开(t.TempDir()+"/v2.db", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if 错误 != nil {
		t.Fatalf("open: %v", 错误)
	}
	t.Cleanup(func() { _ = 存储.Close(context.Background()) })
	return 存储
}

func Test检测序列按类别分桶(t *testing.T) {
	存储 := 打开v2存储(t)
	填充v2夹具(t, 存储)
	到 := time.Now().UnixMilli() + 60_000
	从 := 到 - 15*60_000
	点集, 错误 := 存储.U检测序列(context.Background(), 从, 到, "", 60_000)
	if 错误 != nil {
		t.Fatalf("series: %v", 错误)
	}
	类别 := map[string]int64{}
	for _, 点 := range 点集 {
		类别[点.U类别] += 点.U数量
	}
	if 类别["sqli"] != 1 || 类别["xss"] != 1 {
		t.Fatalf("category counts=%v, want sqli=1 xss=1", 类别)
	}
}

func Test状态类与方法与字节(t *testing.T) {
	存储 := 打开v2存储(t)
	填充v2夹具(t, 存储)
	到 := time.Now().UnixMilli() + 60_000
	从 := 到 - 15*60_000
	上下文 := context.Background()
	状态, 错误 := 存储.U状态类序列(上下文, 从, 到, "", 60_000)
	if 错误 != nil {
		t.Fatalf("status series: %v", 错误)
	}
	计数 := map[string]int64{}
	for _, 点 := range 状态 {
		计数[点.U类] += 点.U数量
	}
	if 计数["4xx"] != 1 || 计数["2xx"] != 1 {
		t.Fatalf("status classes=%v, want 4xx=1 2xx=1", 计数)
	}
	方法, 错误 := 存储.U方法计数(上下文, 从, 到, "", 5)
	if 错误 != nil {
		t.Fatalf("methods: %v", 错误)
	}
	if len(方法) != 2 {
		t.Fatalf("methods=%d, want 2", len(方法))
	}
	字节, 错误 := 存储.U字节序列(上下文, 从, 到, "", 60_000)
	if 错误 != nil {
		t.Fatalf("bytes: %v", 错误)
	}
	入, 出 := int64(0), int64(0)
	for _, 点 := range 字节 {
		入 += 点.U字节入
		出 += 点.U字节出
	}
	if 入 != 170 || 出 != 1240 {
		t.Fatalf("bytes in=%d out=%d, want 170/1240", 入, 出)
	}
}

func Test影子拦截与每站点(t *testing.T) {
	存储 := 打开v2存储(t)
	填充v2夹具(t, 存储)
	到 := time.Now().UnixMilli() + 60_000
	从 := 到 - 15*60_000
	上下文 := context.Background()
	影子, 错误 := 存储.U影子拦截(上下文, 从, 到, 10)
	if 错误 != nil {
		t.Fatalf("shadow: %v", 错误)
	}
	if len(影子) != 1 || 影子[0].U站点 != "api.test" || 影子[0].U数量 != 1 {
		t.Fatalf("shadow=%+v, want api.test=1", 影子)
	}
	站点行, 错误 := 存储.U每站点(上下文, 从, 到)
	if 错误 != nil {
		t.Fatalf("per site: %v", 错误)
	}
	if len(站点行) != 2 {
		t.Fatalf("per-site rows=%d, want 2", len(站点行))
	}
	拦截按站点 := map[string]int64{}
	for _, 行 := range 站点行 {
		if 行.U请求 != 1 {
			t.Fatalf("site %s requests=%d, want 1", 行.U站点, 行.U请求)
		}
		拦截按站点[行.U站点] = 行.U拦截
	}
	if 拦截按站点["app.test"] != 1 || 拦截按站点["api.test"] != 0 {
		t.Fatalf("blocked per site=%v, want app=1 api=0", 拦截按站点)
	}
}

func Test规则统计与热图与访问条目(t *testing.T) {
	存储 := 打开v2存储(t)
	填充v2夹具(t, 存储)
	到 := time.Now().UnixMilli() + 60_000
	从 := 到 - 15*60_000
	上下文 := context.Background()
	规则, 错误 := 存储.U规则统计(上下文, 从, 到, "", 10)
	if 错误 != nil {
		t.Fatalf("rule stats: %v", 错误)
	}
	if len(规则) != 2 {
		t.Fatalf("rules=%d, want 2", len(规则))
	}
	for _, 行 := range 规则 {
		if 行.U最后命中 <= 0 {
			t.Fatalf("rule %s last_seen missing", 行.U规则ID)
		}
	}
	热图, 错误 := 存储.U热图(上下文, 从, 到)
	if 错误 != nil {
		t.Fatalf("heatmap: %v", 错误)
	}
	合计 := int64(0)
	for _, 点 := range 热图 {
		合计 += 点.U数量
	}
	if 合计 != 2 {
		t.Fatalf("heatmap total=%d, want 2", 合计)
	}
	条目, 错误 := 存储.U访问条目统计(上下文, 从, 到, 10)
	if 错误 != nil {
		t.Fatalf("access entries: %v", 错误)
	}
	if len(条目) != 0 {
		t.Fatalf("access entries=%d, want 0 (no acl hits in fixture)", len(条目))
	}
	违规, 错误 := 存储.U顶级拦截IP(上下文, 从, 到, 5)
	if 错误 != nil {
		t.Fatalf("offenders: %v", 错误)
	}
	if len(违规) != 1 || 违规[0].U地址 != "203.0.113.9" || 违规[0].U请求 != 1 {
		t.Fatalf("offenders=%+v", 违规)
	}
}

func Test日志摘要(t *testing.T) {
	存储 := 打开v2存储(t)
	填充v2夹具(t, 存储)
	到 := time.Now().UnixMilli() + 60_000
	从 := 到 - 15*60_000
	摘要, 错误 := 存储.U日志摘要(context.Background(), U日志查询{U从: 从, U到: 到, U限制: 50}, 60_000)
	if 错误 != nil {
		t.Fatalf("summary: %v", 错误)
	}
	if 摘要.U行数 != 2 || 摘要.U拦截 != 1 || 摘要.U记录数 != 1 || 摘要.U唯一IP数 != 1 {
		t.Fatalf("summary=%+v", 摘要)
	}
	if len(摘要.U序列) == 0 {
		t.Fatal("summary series empty")
	}
}

func Test数据库大小与限流序列(t *testing.T) {
	存储 := 打开v2存储(t)
	填充v2夹具(t, 存储)
	if 存储.U数据库大小字节() <= 0 {
		t.Fatal("db size missing")
	}
	到 := time.Now().UnixMilli() + 60_000
	从 := 到 - 15*60_000
	序列, 错误 := 存储.U限制序列(context.Background(), 从, 到, 60_000)
	if 错误 != nil {
		t.Fatalf("ratelimit series: %v", 错误)
	}
	if len(序列) != 0 {
		t.Fatalf("ratelimit rows=%d, want 0 in fixture", len(序列))
	}
}

// 概览扩展查询必须走索引：任何全表 SCAN 都会让 7 天窗口拖垮写循环。
func Test概览扩展查询走索引(t *testing.T) {
	存储 := 打开v2存储(t)
	用例 := []struct {
		名称 string
		查询 string
	}{
		{"detection series", "EXPLAIN QUERY PLAN SELECT (minute/60000)*60000 AS b, category, SUM(requests) FROM rollup_minute WHERE minute >= 0 AND minute < 1 AND category != '' GROUP BY b, category"},
		{"status class series", "EXPLAIN QUERY PLAN SELECT (ts/60000)*60000 AS b, status/100, COUNT(*) FROM logs WHERE ts >= 0 AND ts < 1 GROUP BY b"},
		{"method counts", "EXPLAIN QUERY PLAN SELECT method, COUNT(*) FROM logs WHERE ts >= 0 AND ts < 1 GROUP BY method"},
		{"byte series", "EXPLAIN QUERY PLAN SELECT (ts/60000)*60000 AS b, SUM(bytes_in) FROM logs WHERE ts >= 0 AND ts < 1 GROUP BY b"},
		{"shadow would-block", "EXPLAIN QUERY PLAN SELECT site, COUNT(*) FROM logs WHERE ts >= 0 AND ts < 1 AND action = 'shadow' GROUP BY site"},
		{"per site", "EXPLAIN QUERY PLAN SELECT site, SUM(requests) FROM rollup_minute WHERE minute >= 0 AND minute < 1 AND site != '' GROUP BY site"},
		{"rule stats", "EXPLAIN QUERY PLAN SELECT rule_id, MAX(category), COUNT(*), MAX(ts) FROM logs WHERE ts >= 0 AND ts < 1 AND rule_id != '' GROUP BY rule_id"},
		{"user agents", "EXPLAIN QUERY PLAN SELECT user_agent, COUNT(*) FROM logs WHERE ts >= 0 AND ts < 1 GROUP BY user_agent"},
		{"heatmap", "EXPLAIN QUERY PLAN SELECT strftime('%w', ts/1000, 'unixepoch') AS dow, COUNT(*) FROM logs WHERE ts >= 0 AND ts < 1 GROUP BY (strftime('%w', ts/1000, 'unixepoch'))"},
		{"access entry stats", "EXPLAIN QUERY PLAN SELECT rule_id, COUNT(*), MAX(ts) FROM logs WHERE ts >= 0 AND ts < 1 AND rule_id LIKE 'acl:%' GROUP BY rule_id"},
		{"ratelimit series", "EXPLAIN QUERY PLAN SELECT (minute/60000)*60000 AS b, SUM(requests) FROM rollup_minute WHERE minute >= 0 AND minute < 1 AND action = 'ratelimited' GROUP BY b"},
		{"offenders", "EXPLAIN QUERY PLAN SELECT client_ip, COUNT(*) FROM logs WHERE ts >= 0 AND ts < 1 AND action IN ('block','ratelimited') GROUP BY client_ip"},
		{"rule series", "EXPLAIN QUERY PLAN SELECT (minute/60000)*60000 AS b, rule_id, SUM(requests) FROM rollup_minute WHERE minute >= 0 AND minute < 1 AND rule_id != '' GROUP BY b, rule_id"},
		{"unique ip series", "EXPLAIN QUERY PLAN SELECT (ts/60000)*60000 AS b, COUNT(DISTINCT client_ip) FROM logs WHERE ts >= 0 AND ts < 1 GROUP BY b"},
	}
	for _, 用例项 := range 用例 {
		t.Run(用例项.名称, func(t *testing.T) {
			行列表, 错误 := 存储.数据库.Query(用例项.查询)
			if 错误 != nil {
				t.Fatalf("explain: %v", 错误)
			}
			defer func() { _ = 行列表.Close() }()
			var 计划 strings.Builder
			for 行列表.Next() {
				var 标签, 细节 string
				var 链接, 序号 int
				if 错误 := 行列表.Scan(&链接, &序号, &标签, &细节); 错误 != nil {
					t.Fatalf("scan plan: %v", 错误)
				}
				计划.WriteString(细节)
				计划.WriteString("\n")
			}
			文本 := 计划.String()
			if strings.Contains(文本, "SCAN logs") && !strings.Contains(文本, "USING INDEX") && !strings.Contains(文本, "USING COVERING INDEX") {
				t.Fatalf("query scans the logs table without an index:\n%s", 文本)
			}
			if strings.Contains(文本, "SCAN rollup_minute") && !strings.Contains(文本, "USING INDEX") && !strings.Contains(文本, "USING COVERING INDEX") {
				t.Fatalf("query scans rollup_minute without an index:\n%s", 文本)
			}
		})
	}
}
