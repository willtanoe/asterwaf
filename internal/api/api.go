package 管理接口

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"git.asteriaxlabs.com/willtanoe/asterwaf/internal/clientip"
	配置 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/config"
	规则引擎 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/engine"
	反向代理 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/proxy"
	存储包 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/store"
	"golang.org/x/crypto/bcrypt"
)

const 登录请求体上限 = 4 << 10

type U依赖 struct {
	U配置   配置.U配置
	U存储   *存储包.U存储
	U引擎   *规则引擎.U引擎
	U代理   *反向代理.U处理器
	U解析器  *客户端地址.U解析器
	U日志   *slog.Logger
	U探测间隔 time.Duration
}

// U管理器 是管理端口的处理器，持有会话、登录限速与健康探测。
type U管理器 struct {
	配置   配置.U配置
	存储   *存储包.U存储
	引擎   *规则引擎.U引擎
	代理   *反向代理.U处理器
	解析器  *客户端地址.U解析器
	日志   *slog.Logger
	哈希   []byte
	会话   *U会话管理
	登录限制 *U登录限制
	探测器  *U探测器
	启动时间 time.Time
	多路   *http.ServeMux
}

type 会话上下文键 struct{}

type U登录请求 struct {
	U用户名 string `json:"username"`
	U密码  string `json:"password"`
}

type U登录响应 struct {
	U用户名 string `json:"username"`
	U到期  string `json:"expires_at"`
}

type U当前用户响应 struct {
	U用户名 string `json:"username"`
	U到期  string `json:"expires_at"`
}

type U状态响应 struct {
	U模式    string  `json:"mode"`
	U规则集版本 int64   `json:"ruleset_version"`
	U运行秒数  int64   `json:"uptime_seconds"`
	U丢弃日志数 int64   `json:"dropped_logs"`
	U站点    []U站点健康 `json:"sites"`
}

func U新建(依赖 U依赖) (*U管理器, error) {
	if 依赖.U存储 == nil {
		return nil, errors.New("api: store is required")
	}
	if 依赖.U引擎 == nil {
		return nil, errors.New("api: rule engine is required")
	}
	if 依赖.U解析器 == nil {
		return nil, errors.New("api: client address resolver is required")
	}
	日志 := 依赖.U日志
	if 日志 == nil {
		日志 = slog.Default()
	}
	凭据, 错误 := U准备凭据(依赖.U配置)
	if 错误 != nil {
		return nil, 错误
	}
	间隔 := 依赖.U探测间隔
	if 间隔 == 0 {
		间隔 = 探测间隔
	}
	管理器 := &U管理器{
		配置:   依赖.U配置,
		存储:   依赖.U存储,
		引擎:   依赖.U引擎,
		代理:   依赖.U代理,
		解析器:  依赖.U解析器,
		日志:   日志,
		哈希:   凭据.U哈希,
		会话:   U新建会话(),
		登录限制: U新建登录限制(),
		探测器:  U新建探测器(依赖.U配置.U站点, 间隔, 日志),
		启动时间: time.Now(),
		多路:   http.NewServeMux(),
	}
	管理器.注册路由()
	return 管理器, nil
}

func (管理器 *U管理器) Close() {
	管理器.会话.Close()
	管理器.登录限制.Close()
	管理器.探测器.Close()
}

func (管理器 *U管理器) 注册路由() {
	管理器.多路.HandleFunc("/healthz", 管理器.健康检查)
	管理器.多路.HandleFunc("/api/login", 管理器.登录)
	管理器.多路.HandleFunc("/api/logout", 管理器.登出)
	管理器.多路.HandleFunc("/api/me", 管理器.当前用户)
	管理器.多路.HandleFunc("/api/status", 管理器.状态)
	管理器.多路.HandleFunc("/api/stats/overview", 管理器.概览)
	管理器.多路.HandleFunc("/api/logs", 管理器.日志列表)
	管理器.多路.HandleFunc("/api/logs/", 管理器.日志路由)
	管理器.多路.HandleFunc("/api/rules", 管理器.规则集合)
	管理器.多路.HandleFunc("/api/rules/", 管理器.规则路由)
	管理器.多路.HandleFunc("/api/access-lists", 管理器.访问列表集合)
	管理器.多路.HandleFunc("/api/access-lists/", 管理器.访问列表路由)
	管理器.多路.HandleFunc("/api/rate-limit", 管理器.限流设置)
	管理器.多路.HandleFunc("/api/sites", 管理器.站点集合)
	管理器.多路.HandleFunc("/api/sites/", 管理器.站点路由)
	管理器.多路.HandleFunc("/", 管理器.未找到)
}

// ServeHTTP 实现标准库接口。健康检查之外都要求会话；写请求先校验来源。
func (管理器 *U管理器) ServeHTTP(写 http.ResponseWriter, 请求 *http.Request) {
	if 请求.URL.Path == "/healthz" {
		管理器.多路.ServeHTTP(写, 请求)
		return
	}
	if !U来源允许(请求) {
		写错误(写, http.StatusForbidden, 错误禁止, "request origin does not match the admin host")
		return
	}
	if 请求.URL.Path == "/api/login" {
		管理器.多路.ServeHTTP(写, 请求)
		return
	}
	会话, 令牌, 可以 := 管理器.读取会话(请求)
	if !可以 {
		写错误(写, http.StatusUnauthorized, 错误未认证, "authentication required")
		return
	}
	上下文 := context.WithValue(请求.Context(), 会话上下文键{}, 会话)
	上下文 = context.WithValue(上下文, 令牌上下文键{}, 令牌)
	管理器.多路.ServeHTTP(写, 请求.WithContext(上下文))
}

type 令牌上下文键 struct{}

func (管理器 *U管理器) 读取会话(请求 *http.Request) (会话值, string, bool) {
	饼干, 错误 := 请求.Cookie(会话Cookie)
	if 错误 != nil || 饼干.Value == "" {
		return 会话值{}, "", false
	}
	会话, 可以 := 管理器.会话.U获取(饼干.Value)
	if !可以 {
		return 会话值{}, "", false
	}
	return 会话, 饼干.Value, true
}

func (管理器 *U管理器) 健康检查(写 http.ResponseWriter, 请求 *http.Request) {
	if 请求.Method != http.MethodGet {
		写错误(写, http.StatusMethodNotAllowed, 错误请求, "only GET is supported")
		return
	}
	写JSON(写, http.StatusOK, map[string]string{"status": "ok"})
}

func (管理器 *U管理器) 登录(写 http.ResponseWriter, 请求 *http.Request) {
	if 请求.Method != http.MethodPost {
		写错误(写, http.StatusMethodNotAllowed, 错误请求, "only POST is supported")
		return
	}
	请求.Body = http.MaxBytesReader(写, 请求.Body, 登录请求体上限)
	解码器 := json.NewDecoder(请求.Body)
	解码器.DisallowUnknownFields()
	var 负载 U登录请求
	if 错误 := 解码器.Decode(&负载); 错误 != nil {
		写错误(写, http.StatusBadRequest, 错误请求, "body must be a JSON object with username and password")
		return
	}
	if 错误 := 解码器.Decode(&struct{}{}); !errors.Is(错误, io.EOF) {
		写错误(写, http.StatusBadRequest, 错误请求, "body must contain a single JSON object")
		return
	}
	用户名 := strings.TrimSpace(负载.U用户名)
	if 用户名 == "" || 负载.U密码 == "" {
		写错误(写, http.StatusBadRequest, 错误请求, "username and password are required")
		return
	}
	客户端 := 管理器.解析器.U解析(请求)
	地址 := 客户端.U地址.String()

	允许, 等待 := 管理器.登录限制.U检查(地址, 用户名)
	if !允许 {
		重试 := int(等待 / time.Second)
		if 重试 < 1 {
			重试 = 1
		}
		管理器.审计("login_rate_limited", 用户名, 地址)
		写.Header().Set("Retry-After", strconv.Itoa(重试))
		写错误(写, http.StatusTooManyRequests, 错误限流, "too many login attempts; retry later")
		return
	}

	验证错误 := bcrypt.CompareHashAndPassword(管理器.哈希, []byte(负载.U密码))
	if 验证错误 != nil || 用户名 != 管理器.配置.U认证.U用户名 {
		管理器.登录限制.U记录失败(地址, 用户名)
		管理器.审计("login_failed", 用户名, 地址)
		写错误(写, http.StatusUnauthorized, 错误未认证, "invalid username or password")
		return
	}

	管理器.登录限制.U清除(地址, 用户名)
	令牌, 到期, 错误 := 管理器.会话.U创建(管理器.配置.U认证.U用户名)
	if 错误 != nil {
		管理器.日志.Error("create session failed", "error", 错误)
		写错误(写, http.StatusInternalServerError, 错误内部, "could not create session")
		return
	}
	管理器.设置会话Cookie(写, 令牌, 到期)
	管理器.审计("login_success", 管理器.配置.U认证.U用户名, 地址)
	写JSON(写, http.StatusOK, U登录响应{
		U用户名: 管理器.配置.U认证.U用户名,
		U到期:  到期.UTC().Format(time.RFC3339),
	})
}

func (管理器 *U管理器) 登出(写 http.ResponseWriter, 请求 *http.Request) {
	if 请求.Method != http.MethodPost {
		写错误(写, http.StatusMethodNotAllowed, 错误请求, "only POST is supported")
		return
	}
	会话, 可以 := 请求.Context().Value(会话上下文键{}).(会话值)
	令牌, _ := 请求.Context().Value(令牌上下文键{}).(string)
	if 可以 {
		管理器.会话.U删除(令牌)
		客户端 := 管理器.解析器.U解析(请求)
		管理器.审计("logout", 会话.U用户名, 客户端.U地址.String())
	}
	管理器.清除会话Cookie(写)
	写.WriteHeader(http.StatusNoContent)
}

func (管理器 *U管理器) 当前用户(写 http.ResponseWriter, 请求 *http.Request) {
	if 请求.Method != http.MethodGet {
		写错误(写, http.StatusMethodNotAllowed, 错误请求, "only GET is supported")
		return
	}
	会话, 可以 := 请求.Context().Value(会话上下文键{}).(会话值)
	if !可以 {
		写错误(写, http.StatusUnauthorized, 错误未认证, "authentication required")
		return
	}
	写JSON(写, http.StatusOK, U当前用户响应{
		U用户名: 会话.U用户名,
		U到期:  会话.U到期.UTC().Format(time.RFC3339),
	})
}

func (管理器 *U管理器) 状态(写 http.ResponseWriter, 请求 *http.Request) {
	if 请求.Method != http.MethodGet {
		写错误(写, http.StatusMethodNotAllowed, 错误请求, "only GET is supported")
		return
	}
	写JSON(写, http.StatusOK, U状态响应{
		U模式:    管理器.配置.U模式,
		U规则集版本: 管理器.引擎.U当前版本(),
		U运行秒数:  int64(time.Since(管理器.启动时间).Seconds()),
		U丢弃日志数: 管理器.存储.U丢弃数(),
		U站点:    管理器.探测器.U状态(),
	})
}

func (管理器 *U管理器) 未找到(写 http.ResponseWriter, 请求 *http.Request) {
	写错误(写, http.StatusNotFound, 错误未找到, "endpoint not found")
}

func (管理器 *U管理器) 设置会话Cookie(写 http.ResponseWriter, 令牌 string, 到期 time.Time) {
	http.SetCookie(写, &http.Cookie{
		Name:     会话Cookie,
		Value:    令牌,
		Path:     "/",
		HttpOnly: true,
		Secure:   管理器.配置.U认证.U安全Cookie,
		SameSite: http.SameSiteStrictMode,
		Expires:  到期,
		MaxAge:   int(会话有效期 / time.Second),
	})
}

func (管理器 *U管理器) 清除会话Cookie(写 http.ResponseWriter) {
	http.SetCookie(写, &http.Cookie{
		Name:     会话Cookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   管理器.配置.U认证.U安全Cookie,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
	})
}

func (管理器 *U管理器) 审计(动作, 操作者, 地址 string) {
	上下文, 取消 := context.WithTimeout(context.Background(), 2*time.Second)
	defer 取消()
	错误 := 管理器.存储.U写审计(上下文, 存储包.U审计记录{
		U操作者:   截断(操作者, 120),
		U动作:    动作,
		U客户端IP: 地址,
	})
	if 错误 != nil {
		管理器.日志.Error("write audit row failed", "action", 动作, "error", 错误)
	}
}

func 截断(值 string, 上限 int) string {
	if len(值) <= 上限 {
		return 值
	}
	return 值[:上限]
}
