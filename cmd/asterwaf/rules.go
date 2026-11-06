package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"git.asteriaxlabs.com/willtanoe/asterwaf/internal/config"
	引擎 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/engine"
	存储包 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/store"
)

func 规则命令(参数 []string) error {
	if len(参数) == 0 || 参数[0] != "import-defaults" {
		return errors.New("usage: asterwaf rules import-defaults -config config.yaml")
	}
	命令 := flag.NewFlagSet("rules import-defaults", flag.ContinueOnError)
	配置路径 := 命令.String("config", "config.yaml", "path to YAML configuration")
	if 错误 := 命令.Parse(参数[1:]); 错误 != nil {
		return 错误
	}
	配置值, 错误 := 配置.U加载(*配置路径)
	if 错误 != nil {
		return 错误
	}
	日志 := slog.New(slog.NewTextHandler(os.Stderr, nil))
	数据库, 错误 := 存储包.U打开(配置值.U数据库, 日志)
	if 错误 != nil {
		return 错误
	}
	上下文, 取消 := context.WithTimeout(context.Background(), 30*time.Second)
	defer 取消()
	defer func() { _ = 数据库.Close(上下文) }()

	if _, 错误 := 数据库.U种子规则(上下文, 引擎.U默认规则()); 错误 != nil {
		return 错误
	}
	当前行, 错误 := 数据库.U读取规则(上下文)
	if 错误 != nil {
		return 错误
	}
	默认规则 := 引擎.U默认规则()
	默认表 := make(map[string]引擎.U规则, len(默认规则))
	for _, 规则 := range 默认规则 {
		默认表[规则.UID] = 规则
	}
	已见 := make(map[string]struct{}, len(默认规则))
	更新数, 自定义数 := 0, 0
	结果 := make([]存储包.U规则行, 0, len(当前行)+len(默认规则))
	for _, 行 := range 当前行 {
		默认, 存在 := 默认表[行.UID]
		if !存在 {
			结果 = append(结果, 行)
			continue
		}
		已见[行.UID] = struct{}{}
		if 行.U来源 != "default" {
			自定义数++
			结果 = append(结果, 行)
			continue
		}
		行.U名称 = 默认.U名称
		行.U类别 = 默认.U类别
		行.U正则 = 默认.U正则
		行.UScore = 默认.UScore
		行.U默认版本 = 引擎.U默认规则版本
		更新数++
		结果 = append(结果, 行)
	}
	新增数 := 0
	for _, 规则 := range 默认规则 {
		if _, 存在 := 已见[规则.UID]; 存在 {
			continue
		}
		结果 = append(结果, 存储包.U规则行{
			UID: 规则.UID, U名称: 规则.U名称, U类别: 规则.U类别, U正则: 规则.U正则,
			U启用: 规则.U启用, UScore: 规则.UScore, U来源: "default",
			U默认版本: 引擎.U默认规则版本, U顺序: len(结果),
			U创建时间: time.Now().UnixMilli(),
		})
		新增数++
	}
	规则列表 := make([]引擎.U规则, 0, len(结果))
	for _, 行 := range 结果 {
		规则列表 = append(规则列表, 行.U转为规则())
	}
	if _, 错误 := 引擎.U新建(规则列表); 错误 != nil {
		return fmt.Errorf("default rules do not compile: %w", 错误)
	}
	前版本, _ := 数据库.U最新规则版本(上下文)
	前, _ := json.Marshal(map[string]int64{"version": 前版本})
	版本, 错误 := 数据库.U保存规则集(上下文, 结果, 存储包.U审计记录{
		U操作者: "cli", U动作: "rules_import_defaults", U前: string(前),
		U客户端IP: "127.0.0.1",
	}, "import default rules")
	if 错误 != nil {
		return 错误
	}
	fmt.Printf("Updated default rules: %d\n", 更新数)
	fmt.Printf("Inserted missing defaults: %d\n", 新增数)
	fmt.Printf("Skipped custom rules: %d\n", 自定义数)
	fmt.Printf("New ruleset version: %d\n", 版本)
	fmt.Println("Restart asterwaf to activate the imported rules.")
	return nil
}
