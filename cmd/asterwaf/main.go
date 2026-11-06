package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
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
	"git.asteriaxlabs.com/willtanoe/asterwaf/web"
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
		case "rules":
			if 错误 := 规则命令(os.Args[2:]); 错误 != nil {
				fmt.Fprintf(os.Stderr, "asterwaf rules: %v\n", 错误)
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

	初始化上下文, 取消初始化 := context.WithTimeout(context.Background(), 15*time.Second)
	defer 取消初始化()
	种子模式 := make(map[string]string, len(配置值.U站点))
	for _, 站点值 := range 配置值.U站点 {
		种子模式[站点值.U主机] = 站点值.U模式
	}
	if 错误 := 存储.U种子配置(初始化上下文, 配置值.U访问列表, 配置值.U限流, 种子模式); 错误 != nil {
		return 错误
	}
	规则版本, 错误 := 存储.U种子规则(初始化上下文, 引擎.U默认规则())
	if 错误 != nil {
		return 错误
	}
	规则行, 错误 := 存储.U读取规则(初始化上下文)
	if 错误 != nil {
		return 错误
	}
	规则列表 := make([]引擎.U规则, 0, len(规则行))
	for _, 行 := range 规则行 {
		规则列表 = append(规则列表, 行.U转为规则())
	}
	规则引擎, 错误 := 引擎.U新建(规则列表)
	if 错误 != nil {
		return 错误
	}
	if 错误 := 规则引擎.U安装(规则列表, 规则版本); 错误 != nil {
		return 错误
	}
	访问配置, 错误 := 存储.U读取访问列表(初始化上下文)
	if 错误 != nil {
		return 错误
	}
	限流配置, 错误 := 存储.U读取限流(初始化上下文)
	if 错误 != nil {
		return 错误
	}
	模式表, 错误 := 存储.U读取站点模式(初始化上下文)
	if 错误 != nil {
		return 错误
	}
	生效配置 := 配置值
	生效配置.U访问列表 = 访问配置
	生效配置.U限流 = 限流配置
	for 下标 := range 生效配置.U站点 {
		if 模式, 存在 := 模式表[生效配置.U站点[下标].U主机]; 存在 {
			生效配置.U站点[下标].U模式 = 模式
		}
	}

	静态文件, 错误 := fs.Sub(前端.DistFS, "dist")
	if 错误 != nil {
		return 错误
	}
	处理器, 错误 := 反向代理.U新建(生效配置, 解析器, 规则引擎, 存储, 日志)
	if 错误 != nil {
		return 错误
	}
	管理器, 错误 := 管理接口.U新建(管理接口.U依赖{
		U配置:  生效配置,
		U存储:  存储,
		U引擎:  规则引擎,
		U代理:  处理器,
		U解析器: 解析器,
		U日志:  日志,
		U静态:  静态文件,
	})
	if 错误 != nil {
		return 错误
	}
	defer 管理器.Close()

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
