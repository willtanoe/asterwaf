package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"git.asteriaxlabs.com/willtanoe/asterwaf/internal/api"
	"git.asteriaxlabs.com/willtanoe/asterwaf/internal/clientip"
	"git.asteriaxlabs.com/willtanoe/asterwaf/internal/config"
	"git.asteriaxlabs.com/willtanoe/asterwaf/internal/engine"
	"git.asteriaxlabs.com/willtanoe/asterwaf/internal/proxy"
	"git.asteriaxlabs.com/willtanoe/asterwaf/internal/store"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "stats":
			if 错误 := 统计命令(os.Args[2:]); 错误 != nil {
				fmt.Fprintf(os.Stderr, "asterwaf stats: %v\n", 错误)
				os.Exit(1)
			}
			return
		case "seed":
			if 错误 := 种子命令(os.Args[2:]); 错误 != nil {
				fmt.Fprintf(os.Stderr, "asterwaf seed: %v\n", 错误)
				os.Exit(1)
			}
			return
		}
	}

	配置路径 := flag.String("config", "config.yaml", "path to YAML configuration")
	flag.Parse()

	日志 := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(日志)

	if 错误 := 运行(*配置路径, 日志); 错误 != nil {
		日志.Error("server stopped with an error", "error", 错误)
		os.Exit(1)
	}
}

func 运行(路径 string, 日志 *slog.Logger) error {
	配置值, 错误 := 配置.U加载(路径)
	if 错误 != nil {
		return 错误
	}
	解析器, 错误 := 客户端地址.U新建(配置值.U受信代理)
	if 错误 != nil {
		return 错误
	}
	引擎, 错误 := 引擎.U新建(引擎.U默认规则())
	if 错误 != nil {
		return 错误
	}
	存储, 错误 := 存储.U打开(配置值.U数据库, 日志)
	if 错误 != nil {
		return 错误
	}
	defer func() {
		上下文, 取消 := context.WithTimeout(context.Background(), 5*time.Second)
		defer 取消()
		if 错误 := 存储.Close(上下文); 错误 != nil {
			日志.Error("log store did not shut down cleanly", "error", 错误)
		}
	}()
	if 存储.U包含种子数据(context.Background()) {
		日志.Warn("database contains synthetic seed data; do not treat these statistics as production traffic",
			"database", 配置值.U数据库)
	}
	管理器, 错误 := 管理接口.U新建(管理接口.U依赖{
		U配置:  配置值,
		U存储:  存储,
		U引擎:  引擎,
		U解析器: 解析器,
		U日志:  日志,
	})
	if 错误 != nil {
		return 错误
	}
	defer 管理器.Close()

	处理器, 错误 := 反向代理.U新建(配置值, 解析器, 引擎, 存储, 日志)
	if 错误 != nil {
		return 错误
	}
	代理服务器 := &http.Server{
		Addr:              配置值.U监听,
		Handler:           处理器,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	管理服务器 := &http.Server{
		Addr:              配置值.U管理监听,
		Handler:           管理器,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	信号上下文, 停止 := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer 停止()

	错误通道 := make(chan error, 2)
	go func() {
		错误通道 <- 代理服务器.ListenAndServe()
	}()
	go func() {
		错误通道 <- 管理服务器.ListenAndServe()
	}()
	日志.Info("proxy listening", "addr", 配置值.U监听, "sites", len(配置值.U站点))
	日志.Info("admin listening", "addr", 配置值.U管理监听)

	var 首个错误 error
	select {
	case 错误 := <-错误通道:
		if 错误 != nil && !errors.Is(错误, http.ErrServerClosed) {
			首个错误 = 错误
			日志.Error("server stopped unexpectedly", "error", 错误)
		}
	case <-信号上下文.Done():
		日志.Info("shutdown signal received")
	}

	关闭上下文, 取消 := context.WithTimeout(context.Background(), 10*time.Second)
	defer 取消()
	代理关闭错误 := 代理服务器.Shutdown(关闭上下文)
	管理关闭错误 := 管理服务器.Shutdown(关闭上下文)
	处理器.U关闭空闲连接()
	if 代理关闭错误 != nil {
		_ = 代理服务器.Close()
	}
	if 管理关闭错误 != nil {
		_ = 管理服务器.Close()
	}
	if 首个错误 != nil {
		return 首个错误
	}
	if 代理关闭错误 != nil {
		return 代理关闭错误
	}
	if 管理关闭错误 != nil {
		return 管理关闭错误
	}
	日志.Info("servers stopped")
	return nil
}
