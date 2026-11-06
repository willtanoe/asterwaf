package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"git.asteriaxlabs.com/willtanoe/asterwaf/internal/config"
	合成流量 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/seed"
	"git.asteriaxlabs.com/willtanoe/asterwaf/internal/store"
)

func 种子命令(参数 []string) error {
	命令 := flag.NewFlagSet("seed", flag.ContinueOnError)
	配置路径 := 命令.String("config", "config.yaml", "path to YAML configuration")
	if 错误 := 命令.Parse(参数); 错误 != nil {
		return 错误
	}
	配置值, 错误 := 配置.U加载(*配置路径)
	if 错误 != nil {
		return 错误
	}
	种子路径 := filepath.Join(filepath.Dir(配置值.U数据库), "asterwaf-seed.db")
	for _, 后缀 := range []string{"", "-wal", "-shm"} {
		if 错误 := os.Remove(种子路径 + 后缀); 错误 != nil && !os.IsNotExist(错误) {
			return fmt.Errorf("remove old seed database: %w", 错误)
		}
	}
	日志 := slog.New(slog.NewTextHandler(os.Stderr, nil))
	存储, 错误 := 存储.U打开(种子路径, 日志)
	if 错误 != nil {
		return 错误
	}

	站点 := make([]string, 0, len(配置值.U站点))
	for _, 站点值 := range 配置值.U站点 {
		站点 = append(站点, 站点值.U主机)
	}
	记录列表 := 合成流量.U生成(合成流量.U参数{
		U结束时间:  time.Now().UTC(),
		U天数:    14,
		U站点:    站点,
		U每小时系数: 7,
	})
	fmt.Printf("Seeding %d request rows into %s\n", len(记录列表), 种子路径)
	const 批量 = 500
	for 起点 := 0; 起点 < len(记录列表); 起点 += 批量 {
		终点 := 起点 + 批量
		if 终点 > len(记录列表) {
			终点 = len(记录列表)
		}
		if 错误 := 存储.U直接写入(记录列表[起点:终点]); 错误 != nil {
			_ = 存储.Close(context.Background())
			return 错误
		}
	}
	上下文, 取消 := context.WithTimeout(context.Background(), 10*time.Second)
	defer 取消()
	if 错误 := 存储.Close(上下文); 错误 != nil {
		return 错误
	}
	fmt.Printf("Seed database ready: %s\n", 种子路径)
	fmt.Printf("Set database: %s in %s to inspect it, and expect a startup warning.\n", 种子路径, *配置路径)
	return nil
}
