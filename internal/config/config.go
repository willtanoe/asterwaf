package 配置

import (
	"errors"
	"fmt"
	"io"
	"net/netip"
	"net/url"
	"os"
	"strings"

	访问包 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/accesslist"
	限流包 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/ratelimit"

	"gopkg.in/yaml.v3"
)

const (
	// 1 MiB 足够覆盖表单与 API 请求，超出上限的请求会在读取前被拒绝。
	U默认最大请求体 int64 = 1 << 20
	默认健康路径         = "/"
	默认监听           = "127.0.0.1:8080"
	默认管理监听         = "127.0.0.1:9090"
	默认模式           = "enforce"
	默认数据库          = "data/asterwaf.db"
	默认管理员          = "admin"
)

// U站点 是 YAML 中的站点定义，路由与上游只在配置中决定。
type U站点 struct {
	U主机     string `yaml:"host"`
	U上游     string `yaml:"upstream"`
	U模式     string `yaml:"mode,omitempty"`
	U最大请求体  int64  `yaml:"max_body_bytes"`
	U健康检查路径 string `yaml:"health_path"`
}

// U认证配置 是单用户管理面的凭据设置。
type U认证配置 struct {
	U用户名      string `yaml:"username"`
	U密码哈希     string `yaml:"password_hash"`
	U安全Cookie bool   `yaml:"secure_cookie"`
}

// U配置 是进程的完整 YAML 配置。
type U配置 struct {
	U监听   string   `yaml:"listen"`
	U管理监听 string   `yaml:"admin_listen"`
	U模式   string   `yaml:"mode"`
	U受信代理 []string `yaml:"trusted_proxies"`
	U检查头  []string `yaml:"inspect_headers"`
	U访问列表 访问包.U列表  `yaml:"access_lists"`
	U限流   限流包.U配置  `yaml:"rate_limit"`
	U数据库  string   `yaml:"database"`
	U认证   U认证配置    `yaml:"auth"`
	U站点   []U站点    `yaml:"sites"`
}

// U默认配置 返回可被 YAML 覆盖的基线值。
func U默认配置() U配置 {
	return U配置{
		U监听:   默认监听,
		U管理监听: 默认管理监听,
		U模式:   默认模式,
		U数据库:  默认数据库,
		U受信代理: []string{},
		U检查头:  []string{"User-Agent", "Referer", "X-Forwarded-For"},
		U认证: U认证配置{
			U用户名:      默认管理员,
			U密码哈希:     "",
			U安全Cookie: false,
		},
		U限流: 限流包.U配置{
			U启用:  false,
			U每分钟: 限流包.U默认每分钟请求,
			U突发:  限流包.U默认突发,
		},
		U站点: []U站点{},
	}
}

// U加载 读取 YAML，未知键会被拒绝，防止拼写错误静默生效。
func U加载(路径 string) (U配置, error) {
	文件, 错误 := os.Open(路径)
	if 错误 != nil {
		return U配置{}, fmt.Errorf("open config: %w", 错误)
	}
	defer func() { _ = 文件.Close() }()

	配置值 := U默认配置()
	解码器 := yaml.NewDecoder(文件)
	解码器.KnownFields(true)
	if 错误 := 解码器.Decode(&配置值); 错误 != nil && !errors.Is(错误, io.EOF) {
		return U配置{}, fmt.Errorf("parse config: %w", 错误)
	}
	if 错误 := 配置值.U校验(); 错误 != nil {
		return U配置{}, 错误
	}
	return 配置值, nil
}

// U校验 在启动前拒绝无法安全路由的配置。
func (配置值 U配置) U校验() error {
	if strings.TrimSpace(配置值.U监听) == "" {
		return errors.New("config: listen must not be empty")
	}
	if strings.TrimSpace(配置值.U管理监听) == "" {
		return errors.New("config: admin_listen must not be empty")
	}
	if !U模式合法(配置值.U模式) {
		return fmt.Errorf("config: invalid mode %q (want off, shadow or enforce)", 配置值.U模式)
	}
	if _, 错误 := 访问包.U新建(配置值.U访问列表); 错误 != nil {
		return fmt.Errorf("config: access_lists: %w", 错误)
	}
	if 配置值.U限流.U启用 && (配置值.U限流.U每分钟 <= 0 || 配置值.U限流.U突发 <= 0) {
		return errors.New("config: rate_limit requires positive requests_per_minute and burst")
	}
	for 序号, 名称 := range 配置值.U检查头 {
		if strings.TrimSpace(名称) == "" {
			return fmt.Errorf("config: inspect_headers[%d] must not be empty", 序号)
		}
	}
	for 序号, 地址 := range 配置值.U受信代理 {
		if _, 错误 := netip.ParsePrefix(地址); 错误 != nil {
			return fmt.Errorf("config: trusted_proxies[%d]: %w", 序号, 错误)
		}
	}
	if len(配置值.U站点) == 0 {
		return errors.New("config: at least one site is required")
	}
	已见 := make(map[string]struct{}, len(配置值.U站点))
	for 序号 := range 配置值.U站点 {
		站点 := &配置值.U站点[序号]
		站点.U主机 = 规范化主机(站点.U主机)
		if 错误 := 校验主机(站点.U主机); 错误 != nil {
			return fmt.Errorf("config: sites[%d].host: %w", 序号, 错误)
		}
		if _, 重复 := 已见[站点.U主机]; 重复 {
			return fmt.Errorf("config: duplicate site host %q", 站点.U主机)
		}
		已见[站点.U主机] = struct{}{}

		if 站点.U最大请求体 == 0 {
			站点.U最大请求体 = U默认最大请求体
		}
		if 站点.U最大请求体 < 0 {
			return fmt.Errorf("config: sites[%d].max_body_bytes must be positive", 序号)
		}
		if 站点.U健康检查路径 == "" {
			站点.U健康检查路径 = 默认健康路径
		}
		if !strings.HasPrefix(站点.U健康检查路径, "/") {
			return fmt.Errorf("config: sites[%d].health_path must start with /", 序号)
		}
		if 站点.U模式 == "" {
			站点.U模式 = 配置值.U模式
		}
		if !U模式合法(站点.U模式) {
			return fmt.Errorf("config: sites[%d].mode %q is invalid", 序号, 站点.U模式)
		}
		上游, 错误 := url.Parse(站点.U上游)
		if 错误 != nil {
			return fmt.Errorf("config: sites[%d].upstream: %w", 序号, 错误)
		}
		if 上游.Scheme != "http" && 上游.Scheme != "https" {
			return fmt.Errorf("config: sites[%d].upstream must use http or https", 序号)
		}
		if 上游.Host == "" {
			return fmt.Errorf("config: sites[%d].upstream must include a host", 序号)
		}
	}
	return nil
}

func U模式合法(模式 string) bool {
	switch 模式 {
	case "off", "shadow", "enforce":
		return true
	default:
		return false
	}
}

func 规范化主机(主机 string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(主机), "."))
}

func 校验主机(主机 string) error {
	if 主机 == "" {
		return errors.New("host must not be empty")
	}
	if 主机 == "*" {
		return errors.New("host must be exact or *.suffix")
	}
	if strings.Contains(主机, "*") && !strings.HasPrefix(主机, "*.") {
		return errors.New("wildcard host must begin with an asterisk and a dot")
	}
	if strings.Count(主机, "*") > 1 {
		return errors.New("host may contain at most one wildcard")
	}
	if strings.ContainsAny(主机, " /") {
		return errors.New("host must not contain spaces or slashes")
	}
	return nil
}

// U按主机取站点 返回最长匹配：精确匹配优先，通配符取最长后缀。
func (配置值 U配置) U按主机取站点(主机 string) (U站点, bool) {
	目标 := 规范化主机(主机)
	var 命中 U站点
	命中长度 := -1
	for _, 站点 := range 配置值.U站点 {
		if 站点.U主机 == 目标 {
			return 站点, true
		}
		if !strings.HasPrefix(站点.U主机, "*.") {
			continue
		}
		后缀 := strings.TrimPrefix(站点.U主机, "*")
		if !strings.HasSuffix(目标, 后缀) || len(目标) <= len(后缀) {
			continue
		}
		if len(后缀) > 命中长度 {
			命中 = 站点
			命中长度 = len(后缀)
		}
	}
	return 命中, 命中长度 >= 0
}
