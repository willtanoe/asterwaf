package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"git.asteriaxlabs.com/willtanoe/asterwaf/internal/config"
	"git.asteriaxlabs.com/willtanoe/asterwaf/internal/store"
)

func 统计命令(参数 []string) error {
	命令 := flag.NewFlagSet("stats", flag.ContinueOnError)
	配置路径 := 命令.String("config", "config.yaml", "path to YAML configuration")
	窗口 := 命令.Duration("since", 24*time.Hour, "window length, for example 24h")
	if 错误 := 命令.Parse(参数); 错误 != nil {
		return 错误
	}
	if *窗口 <= 0 || *窗口 > 7*24*time.Hour {
		return fmt.Errorf("since must be between 1s and 168h")
	}
	配置值, 错误 := 配置.U加载(*配置路径)
	if 错误 != nil {
		return 错误
	}
	日志 := slog.New(slog.NewTextHandler(os.Stderr, nil))
	存储, 错误 := 存储.U打开(配置值.U数据库, 日志)
	if 错误 != nil {
		return 错误
	}
	defer func() {
		上下文, 取消 := context.WithTimeout(context.Background(), 5*time.Second)
		defer 取消()
		_ = 存储.Close(上下文)
	}()

	到 := time.Now().UTC()
	从 := 到.Add(-*窗口)
	摘要, 错误 := 存储.U摘要统计(context.Background(), 从, 到)
	if 错误 != nil {
		return 错误
	}
	fmt.Printf("Window: %s to %s\n", 从.Format(time.RFC3339), 到.Format(time.RFC3339))
	fmt.Printf("Requests: %d\n", 摘要.U请求数)
	fmt.Printf("Allowed: %d\n", 摘要.U允许数)
	fmt.Printf("Blocked: %d\n", 摘要.U拦截数)
	fmt.Printf("Shadow: %d\n", 摘要.U记录数)
	fmt.Printf("Rate limited: %d\n", 摘要.U限流数)
	fmt.Printf("Unique IPs: %d\n", 摘要.U唯一IP数)
	fmt.Println("Top rules:")
	for _, 项 := range 摘要.U规则 {
		fmt.Printf("  %s %d\n", 项.U名称, 项.U数量)
	}
	fmt.Println("Top paths:")
	for _, 项 := range 摘要.U路径 {
		fmt.Printf("  %s %d\n", 项.U名称, 项.U数量)
	}
	return nil
}
