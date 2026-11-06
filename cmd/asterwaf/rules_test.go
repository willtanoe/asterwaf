package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	配置 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/config"
	引擎 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/engine"
	存储包 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/store"
	"gopkg.in/yaml.v3"
)

// import-defaults 只更新 source=default 的行；已编辑的默认规则与自定义规则都必须保持不变。
func Test导入默认规则保护自定义规则(t *testing.T) {
	路径 := filepath.Join(t.TempDir(), "import.db")
	日志 := slog.New(slog.NewTextHandler(io.Discard, nil))
	数据库, 错误 := 存储包.U打开(路径, 日志)
	if 错误 != nil {
		t.Fatalf("open: %v", 错误)
	}
	上下文 := context.Background()
	默认 := []引擎.U规则{
		{UID: "keep.default", U名称: "default one", U类别: "sqli", U正则: "one", U启用: true},
		{UID: "edited.default", U名称: "default two", U类别: "xss", U正则: "two", U启用: true},
	}
	if _, 错误 := 数据库.U种子规则(上下文, 默认); 错误 != nil {
		t.Fatalf("seed: %v", 错误)
	}
	行, 错误 := 数据库.U读取规则(上下文)
	if 错误 != nil {
		t.Fatalf("read seeded rules: %v", 错误)
	}
	for 下标 := range 行 {
		if 行[下标].UID == "edited.default" {
			行[下标].U正则 = "custom-edited"
			行[下标].U来源 = "custom"
		}
	}
	行 = append(行, 存储包.U规则行{
		UID: "mine", U名称: "custom", U类别: "rce", U正则: "mine", U启用: true, U来源: "custom",
	})
	if _, 错误 := 数据库.U保存规则集(上下文, 行, 存储包.U审计记录{U操作者: "admin", U动作: "rule_update"}, "test"); 错误 != nil {
		t.Fatalf("save: %v", 错误)
	}
	if 错误 := 数据库.Close(上下文); 错误 != nil {
		t.Fatalf("close: %v", 错误)
	}

	配置值 := 配置.U默认配置()
	配置值.U数据库 = 路径
	配置值.U站点 = []配置.U站点{{U主机: "app.example.com", U上游: "http://127.0.0.1:9"}}
	配置路径 := filepath.Join(t.TempDir(), "config.yaml")
	数据, 错误 := yaml.Marshal(配置值)
	if 错误 != nil {
		t.Fatalf("marshal config: %v", 错误)
	}
	if 错误 := os.WriteFile(配置路径, 数据, 0o600); 错误 != nil {
		t.Fatalf("write config: %v", 错误)
	}
	if 错误 := 规则命令([]string{"import-defaults", "-config", 配置路径}); 错误 != nil {
		t.Fatalf("import: %v", 错误)
	}

	验证库, 错误 := 存储包.U打开(路径, 日志)
	if 错误 != nil {
		t.Fatalf("reopen: %v", 错误)
	}
	defer func() { _ = 验证库.Close(上下文) }()
	结果, 错误 := 验证库.U读取规则(上下文)
	if 错误 != nil {
		t.Fatalf("read: %v", 错误)
	}
	找到 := map[string]存储包.U规则行{}
	for _, 行 := range 结果 {
		找到[行.UID] = 行
	}
	if 行, 存在 := 找到["keep.default"]; !存在 || 行.U来源 != "default" || 行.U正则 != "one" {
		t.Fatalf("default rule was not refreshed: %+v", 行)
	}
	if 行, 存在 := 找到["edited.default"]; !存在 || 行.U来源 != "custom" || 行.U正则 != "custom-edited" {
		t.Fatalf("edited default rule was overwritten: %+v", 行)
	}
	if 行, 存在 := 找到["mine"]; !存在 || 行.U正则 != "mine" {
		t.Fatalf("custom rule changed or removed: %+v", 行)
	}
	// 内置规则集中未出现在库里的默认规则应被补入。
	if 行, 存在 := 找到["sqli.union-select"]; !存在 || 行.U来源 != "default" {
		t.Fatalf("missing default rule was not inserted: %+v", 行)
	}
}
