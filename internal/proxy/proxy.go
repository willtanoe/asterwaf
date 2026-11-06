package 反向代理

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	访问列表 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/accesslist"
	"git.asteriaxlabs.com/willtanoe/asterwaf/internal/clientip"
	配置 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/config"
	规则引擎 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/engine"
	检查管线 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/pipeline"
	限流包 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/ratelimit"
)

// U处理器 按 Host 选择站点并转发。站点列表在启动后不变，模式变更由后续阶段接管。
type U处理器 struct {
	解析器 *客户端地址.U解析器
	日志  *slog.Logger
	站点表 map[string]*站点
	配置  配置.U配置
	管线  *检查管线.U管线
	引擎  *规则引擎.U引擎
	访问  *访问列表.U检查器
	限流  *限流包.U限流器
}

type 站点 struct {
	主机    string
	上游    *url.URL
	代理    *httputil.ReverseProxy
	最大请求体 int64
	模式    string
}

// U新建 为每个站点建立独立的 ReverseProxy 与共享传输参数。
func U新建(配置值 配置.U配置, 解析器 *客户端地址.U解析器, 引擎 *规则引擎.U引擎, 日志 *slog.Logger) (*U处理器, error) {
	if 解析器 == nil {
		return nil, fmt.Errorf("proxy: client address resolver is required")
	}
	if 引擎 == nil {
		return nil, fmt.Errorf("proxy: rule engine is required")
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

	访问表, 错误 := 访问列表.U新建(配置值.U访问列表)
	if 错误 != nil {
		return nil, fmt.Errorf("proxy: access lists: %w", 错误)
	}
	限流器, 错误 := 限流包.U新建(配置值.U限流)
	if 错误 != nil {
		return nil, fmt.Errorf("proxy: rate limit: %w", 错误)
	}

	处理器 := &U处理器{
		解析器: 解析器,
		日志:  日志,
		站点表: make(map[string]*站点, len(配置值.U站点)),
		配置:  配置值,
		管线:  检查管线.U新建(配置值.U检查头),
		引擎:  引擎,
		访问:  访问表,
		限流:  限流器,
	}
	for _, 站点配置 := range 配置值.U站点 {
		上游, 错误 := url.Parse(站点配置.U上游)
		if 错误 != nil {
			return nil, fmt.Errorf("proxy: site %s upstream: %w", 站点配置.U主机, 错误)
		}
		主机 := 站点配置.U主机
		最大请求体 := 站点配置.U最大请求体
		if 最大请求体 <= 0 {
			最大请求体 = 配置.U默认最大请求体
		}
		模式 := 站点配置.U模式
		if 模式 == "" {
			模式 = 配置值.U模式
		}
		站点值 := &站点{主机: 主机, 上游: 上游, 最大请求体: 最大请求体, 模式: 模式}
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
	客户端 := 处理器.解析器.U解析(请求)
	访问结果 := 处理器.访问.U检查(客户端.U地址)
	if 访问结果.U拦截 {
		处理器.日志.Warn("request blocked",
			"site", 站点值.主机,
			"method", 请求.Method,
			"path", 请求.URL.RequestURI(),
			"mode", 站点值.模式,
			"action", "block",
			"category", "accesslist",
			"rule_id", 访问结果.U规则ID,
			"client_ip", 客户端.U地址.String(),
			"target_category", "client_ip",
			"target_name", "address",
			"shadow", false,
		)
		写.Header().Set("Content-Type", "text/plain; charset=utf-8")
		写.WriteHeader(http.StatusForbidden)
		_, _ = 写.Write([]byte("request blocked\n"))
		return
	}
	if 访问结果.U白名单 {
		处理器.日志.Debug("request allowlisted",
			"site", 站点值.主机,
			"path", 请求.URL.RequestURI(),
			"rule_id", 访问结果.U规则ID,
			"client_ip", 客户端.U地址.String(),
		)
	} else if 处理器.限流.U启用() {
		放行, 等待 := 处理器.限流.U允许(客户端.U地址)
		if !放行 {
			重试秒 := int(等待 / time.Second)
			if 重试秒 < 1 {
				重试秒 = 1
			}
			处理器.日志.Warn("request rate limited",
				"site", 站点值.主机,
				"method", 请求.Method,
				"path", 请求.URL.RequestURI(),
				"mode", 站点值.模式,
				"action", "ratelimited",
				"category", "ratelimit",
				"rule_id", "ratelimit",
				"client_ip", 客户端.U地址.String(),
				"retry_after_seconds", 重试秒,
				"target_category", "client_ip",
				"target_name", "address",
				"shadow", false,
			)
			写.Header().Set("Retry-After", strconv.Itoa(重试秒))
			写.Header().Set("Content-Type", "text/plain; charset=utf-8")
			写.WriteHeader(http.StatusTooManyRequests)
			_, _ = 写.Write([]byte("rate limit exceeded\n"))
			return
		}
	}

	快照, 错误 := 处理器.管线.U读取(写, 请求, 站点值.最大请求体)
	if 错误 != nil {
		var 超限 *检查管线.U体过大错误
		if errors.As(错误, &超限) {
			写.Header().Set("Content-Type", "text/plain; charset=utf-8")
			写.WriteHeader(http.StatusRequestEntityTooLarge)
			_, _ = 写.Write([]byte("request body too large\n"))
			return
		}
		处理器.日志.Warn("request body could not be read", "site", 站点值.主机, "error", 错误)
		写.Header().Set("Content-Type", "text/plain; charset=utf-8")
		写.WriteHeader(http.StatusBadRequest)
		_, _ = 写.Write([]byte("request body could not be read\n"))
		return
	}
	if len(快照.U体) == 0 && 请求.ContentLength <= 0 {
		请求.Body = http.NoBody
		请求.ContentLength = 0
	} else {
		请求.Body = io.NopCloser(bytes.NewReader(快照.U体))
		请求.ContentLength = int64(len(快照.U体))
	}
	请求.TransferEncoding = nil

	决策 := 处理器.引擎.U决策(站点值.模式, 快照.U目标列表())
	switch 决策.U动作 {
	case 规则引擎.U动作拦截:
		处理器.日志.Warn("request blocked",
			"site", 站点值.主机,
			"method", 快照.U方法,
			"path", 请求.URL.RequestURI(),
			"mode", 决策.U模式,
			"action", 决策.U动作,
			"category", 决策.U类别,
			"rule_id", 决策.U规则ID,
			"client_ip", 客户端.U地址.String(),
			"target_category", 决策.U目标.U类别,
			"target_name", 决策.U目标.U名称,
			"shadow", false,
		)
		写.Header().Set("Content-Type", "text/plain; charset=utf-8")
		写.WriteHeader(http.StatusForbidden)
		_, _ = 写.Write([]byte("request blocked\n"))
		return
	case 规则引擎.U动作记录:
		处理器.日志.Info("request recorded",
			"site", 站点值.主机,
			"method", 快照.U方法,
			"path", 请求.URL.RequestURI(),
			"mode", 决策.U模式,
			"action", 决策.U动作,
			"category", 决策.U类别,
			"rule_id", 决策.U规则ID,
			"client_ip", 客户端.U地址.String(),
			"target_category", 决策.U目标.U类别,
			"target_name", 决策.U目标.U名称,
			"shadow", true,
		)
	default:
		处理器.日志.Debug("request allowed",
			"site", 站点值.主机,
			"method", 快照.U方法,
			"path", 请求.URL.RequestURI(),
			"mode", 决策.U模式,
			"action", 决策.U动作,
		)
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
	_ = 处理器.限流.Close()
	for _, 站点值 := range 处理器.站点表 {
		if 传输, 可以 := 站点值.代理.Transport.(*http.Transport); 可以 {
			传输.CloseIdleConnections()
		}
	}
}
