package 反向代理

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"git.asteriaxlabs.com/willtanoe/asterwaf/internal/clientip"
	配置 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/config"
)

// U处理器 按 Host 选择站点并转发。站点列表在启动后不变，模式变更由后续阶段接管。
type U处理器 struct {
	解析器 *客户端地址.U解析器
	日志  *slog.Logger
	站点表 map[string]*站点
	配置  配置.U配置
}

type 站点 struct {
	主机 string
	上游 *url.URL
	代理 *httputil.ReverseProxy
}

// U新建 为每个站点建立独立的 ReverseProxy 与共享传输参数。
func U新建(配置值 配置.U配置, 解析器 *客户端地址.U解析器, 日志 *slog.Logger) (*U处理器, error) {
	if 解析器 == nil {
		return nil, fmt.Errorf("proxy: client address resolver is required")
	}
	if 日志 == nil {
		日志 = slog.Default()
	}
	传输 := &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   32,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: time.Second,
		// 上游是内网服务，环境变量代理会造成意外外泄与回路。
		Proxy: nil,
	}

	处理器 := &U处理器{
		解析器: 解析器,
		日志:  日志,
		站点表: make(map[string]*站点, len(配置值.U站点)),
		配置:  配置值,
	}
	for _, 站点配置 := range 配置值.U站点 {
		上游, 错误 := url.Parse(站点配置.U上游)
		if 错误 != nil {
			return nil, fmt.Errorf("proxy: site %s upstream: %w", 站点配置.U主机, 错误)
		}
		主机 := 站点配置.U主机
		站点值 := &站点{主机: 主机, 上游: 上游}
		站点值.代理 = &httputil.ReverseProxy{
			Rewrite: func(请求 *httputil.ProxyRequest) {
				重写请求(请求, 站点值.上游, 解析器)
			},
			Transport: 传输,
			ErrorHandler: func(写 http.ResponseWriter, 请求 *http.Request, 错误 error) {
				日志.Warn("upstream request failed",
					"site", 主机,
					"upstream", 上游.String(),
					"error", 错误,
				)
				写.Header().Set("Content-Type", "text/plain; charset=utf-8")
				写.WriteHeader(http.StatusBadGateway)
				_, _ = 写.Write([]byte("upstream unavailable\n"))
			},
		}
		处理器.站点表[主机] = 站点值
	}
	return 处理器, nil
}

// ServeHTTP 实现标准库接口，方法名必须保持英文。
func (处理器 *U处理器) ServeHTTP(写 http.ResponseWriter, 请求 *http.Request) {
	主机 := 请求主机(请求)
	站点配置, 命中 := 处理器.配置.U按主机取站点(主机)
	if !命中 {
		写.Header().Set("Content-Type", "text/plain; charset=utf-8")
		写.WriteHeader(http.StatusNotFound)
		_, _ = 写.Write([]byte("no site configured for host\n"))
		return
	}
	站点值, 存在 := 处理器.站点表[站点配置.U主机]
	if !存在 {
		处理器.日志.Error("route selected a site without an upstream", "host", 站点配置.U主机)
		写.Header().Set("Content-Type", "text/plain; charset=utf-8")
		写.WriteHeader(http.StatusInternalServerError)
		_, _ = 写.Write([]byte("site configuration is inconsistent\n"))
		return
	}
	站点值.代理.ServeHTTP(写, 请求)
}

func 请求主机(请求 *http.Request) string {
	主机 := 请求.Host
	if 裸主机, _, 错误 := net.SplitHostPort(主机); 错误 == nil {
		主机 = 裸主机
	}
	return strings.TrimSuffix(主机, ".")
}

func 重写请求(请求 *httputil.ProxyRequest, 上游 *url.URL, 解析器 *客户端地址.U解析器) {
	结果 := 解析器.U解析(请求.In)

	头 := 请求.Out.Header
	头.Del("Forwarded")
	for 名称 := range 头 {
		if strings.HasPrefix(strings.ToLower(名称), "x-forwarded-") {
			头.Del(名称)
		}
	}
	头.Del("True-Client-IP")
	头.Del("CF-Connecting-IP")
	头.Del("X-Client-IP")

	请求.SetURL(上游)
	// SetURL 会清空 Host；站点按原始 Host 路由，上游也需要看到同一个虚拟主机名。
	请求.Out.Host = 请求.In.Host
	头.Set("X-Forwarded-For", 结果.U地址.String())
	头.Set("X-Real-IP", 结果.U地址.String())
	头.Set("X-Forwarded-Proto", 结果.U协议)
}

// U关闭空闲连接 在进程退出前释放上游连接。
func (处理器 *U处理器) U关闭空闲连接() {
	for _, 站点值 := range 处理器.站点表 {
		if 传输, 可以 := 站点值.代理.Transport.(*http.Transport); 可以 {
			传输.CloseIdleConnections()
		}
	}
}
