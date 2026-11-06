package 存储

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	访问列表 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/accesslist"
	引擎 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/engine"
	限流 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/ratelimit"
)

func Test配置种子只写空表(t *testing.T) {
	存储, 错误 := U打开(filepath.Join(t.TempDir(), "seed.db"), 静默日志())
	if 错误 != nil {
		t.Fatalf("open: %v", 错误)
	}
	defer 关闭(t, 存储)
	上下文 := context.Background()

	初始 := 访问列表.U列表{U仅允许: true, U条目: []访问列表.U条目{
		{UID: "office", U动作: "allow", U地址: "203.0.113.0/24", U启用: true},
	}}
	限流配置 := 限流.U配置{U启用: true, U每分钟: 120, U突发: 10}
	if 错误 := 存储.U种子配置(上下文, 初始, 限流配置, map[string]string{"app.example.com": "shadow"}); 错误 != nil {
		t.Fatalf("seed: %v", 错误)
	}
	读回, 错误 := 存储.U读取访问列表(上下文)
	if 错误 != nil {
		t.Fatalf("read acl: %v", 错误)
	}
	if !读回.U仅允许 || len(读回.U条目) != 1 {
		t.Fatalf("seeded acl=%+v", 读回)
	}
	读回限流, 错误 := 存储.U读取限流(上下文)
	if 错误 != nil {
		t.Fatalf("read rate: %v", 错误)
	}
	if 读回限流 != 限流配置 {
		t.Fatalf("seeded rate=%+v", 读回限流)
	}
	模式, 错误 := 存储.U读取站点模式(上下文)
	if 错误 != nil {
		t.Fatalf("read modes: %v", 错误)
	}
	if 模式["app.example.com"] != "shadow" {
		t.Fatalf("site mode=%+v", 模式)
	}

	覆盖 := 访问列表.U列表{U仅允许: false}
	if 错误 := 存储.U种子配置(上下文, 覆盖,
		限流.U配置{U启用: false, U每分钟: 5, U突发: 1},
		map[string]string{"app.example.com": "enforce"}); 错误 != nil {
		t.Fatalf("second seed: %v", 错误)
	}
	读回, _ = 存储.U读取访问列表(上下文)
	读回限流, _ = 存储.U读取限流(上下文)
	模式, _ = 存储.U读取站点模式(上下文)
	if !读回.U仅允许 || len(读回.U条目) != 1 {
		t.Fatalf("existing acl overwritten: %+v", 读回)
	}
	if 读回限流 != 限流配置 {
		t.Fatalf("existing rate overwritten: %+v", 读回限流)
	}
	if 模式["app.example.com"] != "shadow" {
		t.Fatalf("existing site mode overwritten: %+v", 模式)
	}
}

func Test规则种子与版本(t *testing.T) {
	存储, 错误 := U打开(filepath.Join(t.TempDir(), "rules.db"), 静默日志())
	if 错误 != nil {
		t.Fatalf("open: %v", 错误)
	}
	defer 关闭(t, 存储)
	上下文 := context.Background()
	默认 := []引擎.U规则{
		{UID: "a.rule", U名称: "A", U类别: "sqli", U正则: "aaa", U启用: true},
		{UID: "b.rule", U名称: "B", U类别: "xss", U正则: "bbb", U启用: true},
	}
	版本, 错误 := 存储.U种子规则(上下文, 默认)
	if 错误 != nil {
		t.Fatalf("seed rules: %v", 错误)
	}
	if 版本 != 1 {
		t.Fatalf("initial version=%d", 版本)
	}
	行, 错误 := 存储.U读取规则(上下文)
	if 错误 != nil || len(行) != 2 {
		t.Fatalf("rules=%+v err=%v", 行, 错误)
	}
	if 行[0].U来源 != "default" || 行[0].U顺序 != 0 {
		t.Fatalf("seeded rule metadata=%+v", 行[0])
	}

	行[0].U正则 = "changed"
	行[0].U来源 = "custom"
	行 = append(行, U规则行{
		UID: "c.rule", U名称: "C", U类别: "rce", U正则: "ccc", U启用: true,
		U来源: "custom", U顺序: 2,
	})
	新版本, 错误 := 存储.U保存规则集(上下文, 行, U审计记录{
		U操作者: "admin", U动作: "rule_update", U客户端IP: "203.0.113.7",
	}, "update rule")
	if 错误 != nil {
		t.Fatalf("save rules: %v", 错误)
	}
	if 新版本 != 2 {
		t.Fatalf("new version=%d", 新版本)
	}
	读回, _ := 存储.U读取规则(上下文)
	if len(读回) != 3 || 读回[0].U正则 != "changed" {
		t.Fatalf("saved rules=%+v", 读回)
	}
	审计数 := 查询审计(t, 存储, "rule_update")
	if 审计数 != 1 {
		t.Fatalf("audit rows=%d", 审计数)
	}

	版本列表, 错误 := 存储.U版本列表(上下文)
	if 错误 != nil || len(版本列表) != 2 {
		t.Fatalf("versions=%+v err=%v", 版本列表, 错误)
	}
	if 版本列表[0].UID != 2 || 版本列表[0].U规则数 != 3 {
		t.Fatalf("latest version metadata=%+v", 版本列表[0])
	}
	旧规则, 信息, 错误 := 存储.U版本详情(上下文, 1)
	if 错误 != nil || len(旧规则) != 2 || 信息.U规则数 != 2 {
		t.Fatalf("version 1 rules=%+v info=%+v err=%v", 旧规则, 信息, 错误)
	}
}

func Test访问条目限流与站点模式写入(t *testing.T) {
	存储, 错误 := U打开(filepath.Join(t.TempDir(), "policy.db"), 静默日志())
	if 错误 != nil {
		t.Fatalf("open: %v", 错误)
	}
	defer 关闭(t, 存储)
	上下文 := context.Background()
	if 错误 := 存储.U种子配置(上下文, 访问列表.U列表{}, 限流.U配置{U每分钟: 60, U突发: 5},
		map[string]string{"app.example.com": "enforce"}); 错误 != nil {
		t.Fatalf("seed: %v", 错误)
	}

	条目 := 访问列表.U条目{UID: "deny-one", U动作: "deny", U地址: "198.51.100.9", U启用: true, U来源日志ID: 42}
	if 错误 := 存储.U新增访问条目(上下文, 条目, U审计记录{U操作者: "admin", U动作: "access_list_create"}); 错误 != nil {
		t.Fatalf("add entry: %v", 错误)
	}
	列表, _ := 存储.U读取访问列表(上下文)
	if len(列表.U条目) != 1 || 列表.U条目[0].U来源日志ID != 42 {
		t.Fatalf("acl=%+v", 列表)
	}
	if 错误 := 存储.U删除访问条目(上下文, "deny-one", U审计记录{U操作者: "admin", U动作: "access_list_delete"}); 错误 != nil {
		t.Fatalf("delete entry: %v", 错误)
	}
	if 错误 := 存储.U删除访问条目(上下文, "deny-one", U审计记录{}); 错误 == nil {
		t.Fatal("missing access entry did not return not-found")
	} else {
		var 未找到 U未找到错误
		if !errors.As(错误, &未找到) {
			t.Fatalf("unexpected delete error: %v", 错误)
		}
	}

	if 错误 := 存储.U保存限流(上下文, 限流.U配置{U启用: true, U每分钟: 30, U突发: 3},
		U审计记录{U操作者: "admin", U动作: "rate_limit_update"}); 错误 != nil {
		t.Fatalf("save rate: %v", 错误)
	}
	限流, _ := 存储.U读取限流(上下文)
	if !限流.U启用 || 限流.U每分钟 != 30 || 限流.U突发 != 3 {
		t.Fatalf("rate=%+v", 限流)
	}

	if 错误 := 存储.U保存站点模式(上下文, "app.example.com", "shadow",
		U审计记录{U操作者: "admin", U动作: "site_mode_update"}); 错误 != nil {
		t.Fatalf("save site mode: %v", 错误)
	}
	模式, _ := 存储.U读取站点模式(上下文)
	if 模式["app.example.com"] != "shadow" {
		t.Fatalf("modes=%+v", 模式)
	}
	if 查询审计(t, 存储, "site_mode_update") != 1 {
		t.Fatal("site mode audit missing")
	}
}

func 查询审计(t *testing.T, 存储 *U存储, 动作 string) int {
	t.Helper()
	var 数量 int
	if 错误 := 存储.数据库.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action = ?`, 动作).Scan(&数量); 错误 != nil {
		t.Fatalf("audit count: %v", 错误)
	}
	return 数量
}
